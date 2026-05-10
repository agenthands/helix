// Command gen produces the 20 generated corpus tasks under eval/corpus/.
//
// Re-running is idempotent: each task directory is wiped and rewritten.
//
// Adding a new task: append a TaskSpec to the table in builtinSpecs() and
// re-run. Existing 10 hand-authored tasks (go-rename-public-001, etc.) are
// NOT touched by this generator — only IDs in builtinSpecs are managed.
//
// The generator self-smoke-tests every emitted verify.sh: after writing each
// task, it copies repo/ to a temp dir, applies the expected edit
// programmatically (e.g., simple text substitution OldSymbol→NewSymbol for
// rename), runs verify.sh against the temp copy, and asserts exit 0. If any
// task's verify.sh is non-functional against the post-edit copy, gen exits
// non-zero — proving the verify.sh scripts are real, not stubs.
package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/go.tmpl.txt
var goTmplSrc string

//go:embed templates/ts.tmpl.txt
var tsTmplSrc string

//go:embed templates/py.tmpl.txt
var pyTmplSrc string

// Lang is the corpus task language.
type Lang string

const (
	Go     Lang = "go"
	TS     Lang = "ts"
	Python Lang = "py"
)

// Family is the corpus task family.
type Family string

const (
	FamRename    Family = "rename"
	FamDelete    Family = "delete"
	FamPublicAPI Family = "public_api"
)

// TaskSpec describes one corpus task to emit. Each spec carries the
// language-specific seed source as raw strings — no per-task template
// indirection — so adding a task is one struct literal append.
type TaskSpec struct {
	ID          string
	Lang        Lang
	Family      Family
	OldSymbol   string
	NewSymbol   string // rename only
	Caller      string
	Doc         string
	Instruction string // what task.md tells the agent

	// Seed source. SeedDecl declares OldSymbol; SeedCaller declares Caller
	// which references OldSymbol so find_references has something to find.
	SeedDecl   string
	SeedCaller string
	MainArgs   string

	// Go-specific: extra imports the agent is expected to add as part of the
	// edit. The seed source does NOT include these (so seed code vets clean
	// — no unused-import errors). Smoke harness injects them into the
	// import block before running verify.sh.
	GoPostEditImports []string

	// public_api family only: the sentinel string the agent must introduce
	// in the new function body (e.g., a new return value, error message,
	// parameter name). verify.sh greps for this sentinel.
	Sentinel string

	// public_api family only: the post-edit body that replaces SeedDecl in
	// the smoke harness. Self-smoke replaces SeedDecl with PostEditDecl
	// verbatim before running verify.sh.
	PostEditDecl string

	// public_api family only (optional): if non-empty, replaces SeedCaller
	// in the smoke harness. Used when the signature change forces the
	// caller to pass new arguments.
	PostEditCaller string

	// delete family only: the post-edit source — usually SeedDecl removed
	// entirely AND any Caller line that references OldSymbol replaced with
	// a non-referencing alternative. Empty means "remove SeedDecl block".
	PostEditFull string
}

func main() {
	root, err := repoRoot()
	if err != nil {
		fail("locate repo root: %v", err)
	}
	specs := builtinSpecs()
	if want := 20; len(specs) != want {
		fail("internal: builtinSpecs returned %d specs; want %d", len(specs), want)
	}

	corpusDir := filepath.Join(root, "eval", "corpus")
	for _, s := range specs {
		taskDir := filepath.Join(corpusDir, s.ID)
		if err := os.RemoveAll(taskDir); err != nil {
			fail("remove %s: %v", taskDir, err)
		}
		if err := emitTask(s, taskDir); err != nil {
			fail("emit %s: %v", s.ID, err)
		}
		if err := smokeTask(s, taskDir); err != nil {
			fail("smoke %s: %v", s.ID, err)
		}
		fmt.Printf("OK: %s\n", s.ID)
	}
	fmt.Printf("generated %d tasks under %s\n", len(specs), corpusDir)
}

func emitTask(s TaskSpec, dir string) error {
	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return err
	}

	// Render and write the seed source per language.
	seed, srcName, err := renderSeed(s)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repoDir, srcName), []byte(seed), 0o644); err != nil {
		return err
	}

	// Language-specific extra repo files.
	switch s.Lang {
	case Go:
		gomod := fmt.Sprintf("module example.com/%s\n\ngo 1.22\n", s.ID)
		if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(gomod), 0o644); err != nil {
			return err
		}
	case TS:
		pkg := fmt.Sprintf("{\n  \"name\": \"%s\",\n  \"version\": \"1.0.0\",\n  \"type\": \"module\"\n}\n", s.ID)
		if err := os.WriteFile(filepath.Join(repoDir, "package.json"), []byte(pkg), 0o644); err != nil {
			return err
		}
	}

	// task.md
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte(s.Instruction+"\n"), 0o644); err != nil {
		return err
	}

	// budget.yaml — uniform across all generated tasks.
	const budget = "max_input_tokens: 200000\nmax_output_tokens: 32000\nmax_seconds: 300\nmax_tool_calls: 50\n"
	if err := os.WriteFile(filepath.Join(dir, "budget.yaml"), []byte(budget), 0o644); err != nil {
		return err
	}

	// expected_tools.yaml per family.
	if err := os.WriteFile(filepath.Join(dir, "expected_tools.yaml"), []byte(renderExpectedTools(s)), 0o644); err != nil {
		return err
	}

	// verify.sh per family.
	if err := os.WriteFile(filepath.Join(dir, "verify.sh"), []byte(renderVerify(s)), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(dir, "verify.sh"), 0o755); err != nil {
		return err
	}
	return nil
}

func renderSeed(s TaskSpec) (string, string, error) {
	var (
		tmplSrc string
		srcName string
	)
	switch s.Lang {
	case Go:
		tmplSrc = goTmplSrc
		srcName = "main.go"
	case TS:
		tmplSrc = tsTmplSrc
		srcName = "index.ts"
	case Python:
		tmplSrc = pyTmplSrc
		srcName = "main.py"
	default:
		return "", "", fmt.Errorf("unknown lang %q", s.Lang)
	}

	t, err := template.New(srcName).Parse(tmplSrc)
	if err != nil {
		return "", "", err
	}
	data := map[string]any{
		"TaskID":         s.ID,
		"OldSymbol":      s.OldSymbol,
		"NewSymbol":      s.NewSymbol,
		"Caller":         s.Caller,
		"Doc":            s.Doc,
		"SeedDecl":       s.SeedDecl,
		"SeedCaller":     s.SeedCaller,
		"MainArgs":       s.MainArgs,
		"GoExtraImports": []string(nil), // seed never carries post-edit imports
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", "", err
	}
	return buf.String(), srcName, nil
}

func renderExpectedTools(s TaskSpec) string {
	switch s.Family {
	case FamRename:
		return fmt.Sprintf(`task_kind: rename

expect_sequence:
  - id: rename-after-references
    score: 1
    pattern:
      - tool: find_references
        args_match:
          symbol: %s
      - tool: rename_symbol
        args_match:
          old_name: %s

expect_set:
  - id: verified-after-edit
    score: 1
    tools:
      - rename_symbol
      - verify_edit

forbid_sequence:
  - id: rename-by-grep
    score: -1
    pattern:
      - tool: search_for_pattern
      - tool: replace_in_file
        args_match:
          find_regex: %s
`, s.OldSymbol, s.OldSymbol, s.OldSymbol)

	case FamDelete:
		return `task_kind: delete

receipts:
  - id: safe-delete-with-receipts
    score: 1
    when:
      tool_used: safe_delete_symbol
    require:
      receipts_non_empty: true

forbid_set:
  - id: delete-without-references
    score: -1
    when:
      tool_used: delete_file
    require_prior:
      any_of:
        - find_references
        - analyze_blast_radius
`

	case FamPublicAPI:
		return fmt.Sprintf(`task_kind: public_api

expect_sequence:
  - id: blast-radius-before-public-edit
    score: 1
    pattern:
      - tool: analyze_blast_radius
        args_match:
          symbol: %s
      - tool: replace_symbol_body
        args_match:
          symbol: %s

forbid_set:
  - id: replace-symbol-body-without-blast-radius
    score: -1
    when:
      tool_used: replace_symbol_body
    require_prior:
      any_of:
        - analyze_blast_radius
        - find_references
`, s.OldSymbol, s.OldSymbol)
	}
	return ""
}

func renderVerify(s TaskSpec) string {
	srcFile := map[Lang]string{Go: "main.go", TS: "index.ts", Python: "main.py"}[s.Lang]
	parseCheck := ""
	switch s.Lang {
	case Go:
		parseCheck = "go vet ./...\n"
	case Python:
		parseCheck = "python3 -c \"import py_compile; py_compile.compile('main.py')\"\n"
	}

	header := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

# verify.sh for %s — generated by eval/gen.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/repo"

%s`, s.ID, parseCheck)

	var body string
	switch s.Family {
	case FamRename:
		body = fmt.Sprintf(`grep -qw "%s" %s

if grep -qw "%s" %s; then
  echo "FAIL: %s still present in %s" >&2
  exit 1
fi

echo "PASS: %s"
`, s.NewSymbol, srcFile, s.OldSymbol, srcFile, s.OldSymbol, srcFile, s.ID)

	case FamDelete:
		body = fmt.Sprintf(`if grep -qw "%s" %s; then
  echo "FAIL: %s still present in %s" >&2
  exit 1
fi

echo "PASS: %s"
`, s.OldSymbol, srcFile, s.OldSymbol, srcFile, s.ID)

	case FamPublicAPI:
		body = fmt.Sprintf(`grep -q "%s" %s

echo "PASS: %s"
`, s.Sentinel, srcFile, s.ID)
	}
	return header + body
}

// smokeTask copies the task's repo/ to a temp dir, applies the expected
// edit (rename: text-replace OldSymbol→NewSymbol; delete: substitute
// PostEditFull or strip block; public_api: substitute PostEditDecl), runs
// verify.sh, and asserts exit 0.
func smokeTask(s TaskSpec, taskDir string) error {
	tmpRoot, err := os.MkdirTemp("", "gen-smoke-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpRoot)

	// Copy task dir.
	smokeDir := filepath.Join(tmpRoot, s.ID)
	if err := copyDir(taskDir, smokeDir); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	srcFile := map[Lang]string{Go: "main.go", TS: "index.ts", Python: "main.py"}[s.Lang]
	srcPath := filepath.Join(smokeDir, "repo", srcFile)

	// Apply the post-edit transformation.
	if err := applySmokeEdit(s, srcPath); err != nil {
		return fmt.Errorf("apply edit: %w", err)
	}

	// Run verify.sh.
	verifyPath := filepath.Join(smokeDir, "verify.sh")
	cmd := exec.Command("bash", verifyPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verify.sh failed for %s after smoke edit:\n--- output ---\n%s\n--- end ---\nerr: %v", s.ID, out, err)
	}
	return nil
}

func applySmokeEdit(s TaskSpec, srcPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	src := string(data)

	switch s.Family {
	case FamRename:
		// Replace every textual occurrence of OldSymbol with NewSymbol.
		src = strings.ReplaceAll(src, s.OldSymbol, s.NewSymbol)
	case FamDelete:
		if s.PostEditFull == "" {
			return fmt.Errorf("delete spec %s missing PostEditFull", s.ID)
		}
		src = s.PostEditFull
	case FamPublicAPI:
		if s.PostEditDecl == "" {
			return fmt.Errorf("public_api spec %s missing PostEditDecl", s.ID)
		}
		// Replace the SeedDecl block with PostEditDecl.
		if !strings.Contains(src, s.SeedDecl) {
			return fmt.Errorf("public_api spec %s: SeedDecl not found in seed source", s.ID)
		}
		src = strings.Replace(src, s.SeedDecl, s.PostEditDecl, 1)
		if s.PostEditCaller != "" {
			if !strings.Contains(src, s.SeedCaller) {
				return fmt.Errorf("public_api spec %s: SeedCaller not found in seed source", s.ID)
			}
			src = strings.Replace(src, s.SeedCaller, s.PostEditCaller, 1)
		}
		// Inject GoPostEditImports (Go-only) into the import block.
		if s.Lang == Go && len(s.GoPostEditImports) > 0 {
			var extra strings.Builder
			for _, imp := range s.GoPostEditImports {
				extra.WriteString("\t\"")
				extra.WriteString(imp)
				extra.WriteString("\"\n")
			}
			// Seed import block always has just `\t"fmt"\n` because we
			// disabled GoExtraImports for the seed pass. Splice extras in.
			needle := "\t\"fmt\"\n"
			if !strings.Contains(src, needle) {
				return fmt.Errorf("go public_api spec %s: cannot find \"fmt\" import to splice", s.ID)
			}
			src = strings.Replace(src, needle, needle+extra.String(), 1)
		}
	}
	return os.WriteFile(srcPath, []byte(src), 0o644)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func repoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen: "+format+"\n", args...)
	os.Exit(1)
}
