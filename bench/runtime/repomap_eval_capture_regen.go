//go:build ignore

// repomap_eval_capture_regen.go is the LOCAL-ONLY regenerator for the committed
// RepoMap-eval captured rankings (REPOEVAL-01/02). It is invoked by
// `make bench-repomap-eval` (NOT compiled into any binary — the //go:build ignore
// tag excludes it from the package build). For each exercise in the committed
// gold corpus (bench/evaluators/repomapeval/testdata/gold/{go,python,rust}.json)
// it clones the vendored fixture into a per-cell sandbox, spawns the warm daemon
// (HELIX_BIN), dials it over the gRPC StreamMCP wire, calls get_repo_map (uniform)
// and get_context (seeded from the exercise's files.solution stub), PARSES the
// rendered {"tree": treeText} envelope into an ordered file:symbol list per the
// CORPUS.md parse contract (file order = PageRank-prefix appearance order; symbol
// order = in-file elided-def appearance order), and writes the pre-parsed ordered
// JSON into testdata/captured/{go,python,rust}.json so the stdlib leaf never
// parses tree text.
//
// Usage (from repo root):
//
//	go build -o ./helix ./cmd/helix
//	HELIX_BIN="$(pwd)/helix" go run bench/runtime/repomap_eval_capture_regen.go
//
// FAIL-NOT-SKIP (the Phase 100 determinism contract): when HELIX_BIN is set this
// regenerator os.Exit(2)s if HELIX_BIN is unset/empty, get_repo_map returns empty
// output, a language bucket parses to zero exercises, or a "did it RUN" sentinel
// count is unmet — a missing capture is a HARD failure, never a silent skip. Only
// deterministic bytes are written (no latency / timestamp / absolute path). The
// committed artifact under version control IS the CI contract; the hermetic golden
// (go test ./bench/evaluators/repomapeval/, no binary) is the authoritative proof.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	aiderpolyglot "github.com/agenthands/helix/bench/datasets/aider-polyglot"
	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	"github.com/agenthands/helix/bench/runtime/subprocess"
	"github.com/agenthands/helix/internal/forwarder"
	"github.com/agenthands/helix/internal/repomap"
)

// corpusLanguages mirrors the committed testdata/{gold,captured} files (D-03).
var corpusLanguages = []string{"go", "python", "rust"}

// captureBudget is the token budget passed to get_repo_map / get_context. It is
// generous so the whole tiny exercise renders (gold should always fit).
const captureBudget = 8192

// callDeadline bounds a single tools/call (mirrors drive.go driveDeadline).
const callDeadline = 30 * time.Second

// capturedExercise is the on-disk shape (mirrors repomapeval.CapturedExercise).
type capturedExercise struct {
	RepoMap []string `json:"repo_map"`
	Context []string `json:"context"`
}

func main() {
	helixBin := os.Getenv("HELIX_BIN")
	if helixBin == "" {
		fmt.Fprintln(os.Stderr, "HELIX_BIN must be set (build with: go build -o ./helix ./cmd/helix)")
		os.Exit(2)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "getwd:", err)
		os.Exit(1)
	}
	goldDir := filepath.Join(repoRoot, "bench", "evaluators", "repomapeval", "testdata", "gold")
	capturedDir := filepath.Join(repoRoot, "bench", "evaluators", "repomapeval", "testdata", "captured")
	fixturesRoot := filepath.Join(repoRoot, "bench", "datasets", "aider-polyglot", "fixtures")

	totalRun := 0
	for _, lang := range corpusLanguages {
		gold, err := loadGold(goldDir, lang)
		if err != nil {
			fmt.Fprintln(os.Stderr, "load gold:", err)
			os.Exit(1)
		}
		if len(gold) == 0 {
			// Empty language bucket is a HARD failure (fail-not-skip).
			fmt.Fprintf(os.Stderr, "gold bucket %q parsed to zero exercises\n", lang)
			os.Exit(2)
		}

		bucket := make(map[string]capturedExercise, len(gold))
		for ex := range gold {
			ce, err := captureExercise(repoRoot, fixturesRoot, helixBin, lang, ex)
			if err != nil {
				fmt.Fprintf(os.Stderr, "capture %s/%s: %v\n", lang, ex, err)
				os.Exit(2)
			}
			if len(ce.RepoMap) == 0 {
				// Empty get_repo_map output is a HARD failure (fail-not-skip).
				fmt.Fprintf(os.Stderr, "capture %s/%s: get_repo_map produced zero file:symbol IDs\n", lang, ex)
				os.Exit(2)
			}
			bucket[ex] = ce
			totalRun++
		}

		out, err := json.MarshalIndent(bucket, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "marshal captured:", err)
			os.Exit(1)
		}
		out = append(out, '\n')
		dst := filepath.Join(capturedDir, lang+".json")
		if err := os.WriteFile(dst, out, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write captured:", err)
			os.Exit(1)
		}
		fmt.Printf("regenerated %s (%d exercises)\n", dst, len(bucket))
	}

	// "Did it RUN" sentinel: at least one exercise per language must have been
	// captured, else the regenerator produced nothing meaningful (fail-not-skip).
	if totalRun < len(corpusLanguages) {
		fmt.Fprintf(os.Stderr, "sentinel: only %d exercises captured across %d languages — refusing to commit\n",
			totalRun, len(corpusLanguages))
		os.Exit(2)
	}
	fmt.Printf("captured %d exercises total\n", totalRun)
}

// loadGold reads goldDir/<language>.json into an exercise->IDs map.
func loadGold(goldDir, language string) (map[string][]string, error) {
	raw, err := os.ReadFile(filepath.Join(goldDir, language+".json"))
	if err != nil {
		return nil, err
	}
	var m map[string][]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// captureExercise clones the fixture, spawns the daemon, dials it, and captures
// the get_repo_map (uniform) + get_context (seeded) rankings as ordered
// file:symbol lists.
func captureExercise(repoRoot, fixturesRoot, helixBin, lang, exercise string) (capturedExercise, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fixtureDir := filepath.Join(fixturesRoot, lang, "exercises", "practice", exercise)
	ex, err := aiderpolyglot.LoadExercise(fixtureDir, lang)
	if err != nil {
		return capturedExercise{}, fmt.Errorf("load exercise: %w", err)
	}

	outDir, err := os.MkdirTemp("", "repomap-eval-regen-*")
	if err != nil {
		return capturedExercise{}, fmt.Errorf("mkdtemp: %w", err)
	}
	defer os.RemoveAll(outDir)

	sb, err := benchsandbox.New("repomap-eval-baseline", helixBin, outDir)
	if err != nil {
		return capturedExercise{}, fmt.Errorf("sandbox: %w", err)
	}
	defer func() { _ = sb.Cleanup() }()

	const mode = "read"
	if err := sb.Prepare(exercise, mode); err != nil {
		return capturedExercise{}, fmt.Errorf("prepare: %w", err)
	}
	if err := sb.CloneRepo(fixtureDir, exercise, mode); err != nil {
		return capturedExercise{}, fmt.Errorf("clone: %w", err)
	}
	workDir := sb.RepoFor(exercise, mode)

	cfgPath, err := writeReadOnlyConfig(sb, exercise, mode)
	if err != nil {
		return capturedExercise{}, fmt.Errorf("write config: %w", err)
	}
	h, err := subprocess.StartDaemon(ctx, sb, exercise, mode, "bench-full", cfgPath)
	if err != nil {
		return capturedExercise{}, fmt.Errorf("start daemon: %w", err)
	}
	defer func() { _ = h.Kill() }()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	sess, err := forwarder.OpenSession(ctx, sb.SocketFor(exercise, mode), "", logger, "repomap-eval-regen")
	if err != nil {
		return capturedExercise{}, fmt.Errorf("open session: %w", err)
	}
	defer sess.Close()

	actCtx, actCancel := context.WithTimeout(ctx, callDeadline)
	if _, err := sess.CallTool(actCtx, "activate_project", map[string]any{"repo_path": workDir}); err != nil {
		actCancel()
		return capturedExercise{}, fmt.Errorf("activate_project: %w", err)
	}
	actCancel()

	repoMapTree, err := callTree(ctx, sess, "get_repo_map", map[string]any{"token_budget": captureBudget})
	if err != nil {
		return capturedExercise{}, fmt.Errorf("get_repo_map: %w", err)
	}
	// get_context is seeded from the exercise's files.solution stub path(s).
	seeds := make([]any, 0, len(ex.Config.Files.Solution))
	for _, rel := range ex.Config.Files.Solution {
		seeds = append(seeds, rel)
	}
	contextTree, err := callTree(ctx, sess, "get_context", map[string]any{
		"files":        seeds,
		"token_budget": captureBudget,
	})
	if err != nil {
		return capturedExercise{}, fmt.Errorf("get_context: %w", err)
	}

	return capturedExercise{
		RepoMap: parseTreeToIDs(repoMapTree),
		Context: parseTreeToIDs(contextTree),
	}, nil
}

// callTree calls a repomap tool and extracts the "tree" text from the
// {"tree": treeText} envelope returned in the first text content block.
func callTree(ctx context.Context, sess *forwarder.Session, tool string, args map[string]any) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, callDeadline)
	defer cancel()
	res, err := sess.CallTool(callCtx, tool, args)
	if err != nil {
		return "", err
	}
	if res == nil || res.IsError {
		return "", fmt.Errorf("tool %q returned error", tool)
	}
	text := firstText(res)
	if text == "" {
		return "", fmt.Errorf("tool %q returned empty content", tool)
	}
	var env struct {
		Tree string `json:"tree"`
	}
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		return "", fmt.Errorf("parse envelope: %w", err)
	}
	return env.Tree, nil
}

// firstText returns the first non-empty text content block of a CallToolResult.
func firstText(res any) string {
	// The SDK CallToolResult.Content is []Content; marshal to JSON and pluck the
	// text fields to avoid a hard SDK type dependency in this ignore-tagged file.
	b, err := json.Marshal(res)
	if err != nil {
		return ""
	}
	var shape struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(b, &shape); err != nil {
		return ""
	}
	for _, c := range shape.Content {
		if c.Text != "" {
			return c.Text
		}
	}
	return ""
}

// parseTreeToIDs parses the rendered RepoMap tree text into an ordered
// file:symbol ID list per the CORPUS.md parse contract: file order = appearance
// order of file lines in the tree; symbol order = in-file elided-def appearance
// order under that file. Directory nesting is tracked so the relpath is
// reconstructed; symbol names are extracted from the indented elided-def lines
// per language.
func parseTreeToIDs(tree string) []string {
	var ids []string
	var dirStack []string
	curFile := ""
	for _, raw := range strings.Split(tree, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		line := strings.TrimRight(raw, " ")
		trimmed := strings.TrimSpace(line)

		// Elided-def content lines are indented +4 relative to their file line.
		// A file line carries no leading content indent we can rely on alone, so
		// distinguish: directory lines end with "/", file lines have a known ext,
		// everything else (deeper indent) is content under curFile.
		switch {
		case strings.HasSuffix(trimmed, "/"):
			depth := indent / 2
			name := strings.TrimSuffix(trimmed, "/")
			if depth < len(dirStack) {
				dirStack = dirStack[:depth]
			}
			dirStack = append(dirStack, name)
			curFile = ""
		case repomap.LangFromExt(trimmed) != "":
			// A file leaf line (basename with a known extension).
			curFile = filepath.Join(append(append([]string(nil), dirStack...), trimmed)...)
		default:
			if curFile == "" {
				continue
			}
			if sym := symbolFromDefLine(trimmed, repomap.LangFromExt(curFile)); sym != "" {
				ids = append(ids, curFile+":"+sym)
			}
		}
	}
	return dedupeStable(ids)
}

// symbolFromDefLine extracts a symbol name from an elided-def line for the given
// language. It is heuristic (the regenerator is local-only); it recognizes the
// common top-level def shapes for go/python/rust.
func symbolFromDefLine(line, lang string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	switch lang {
	case "go":
		// "func Name(", "func (r *T) Method(", "type Name struct".
		if fields[0] == "func" {
			return goFuncName(fields)
		}
		if fields[0] == "type" && len(fields) >= 2 {
			return identHead(fields[1])
		}
	case "python":
		// "def name(", "class Name:".
		if (fields[0] == "def" || fields[0] == "class") && len(fields) >= 2 {
			return identHead(fields[1])
		}
	case "rust":
		// "pub fn name(", "fn name(", "struct Name", "enum Name", "impl ...".
		i := 0
		if fields[0] == "pub" {
			i = 1
		}
		if i < len(fields) {
			switch fields[i] {
			case "fn", "struct", "enum", "trait":
				if i+1 < len(fields) {
					return identHead(fields[i+1])
				}
			}
		}
	}
	return ""
}

// goFuncName extracts the function/method name from a "func ..." field list,
// handling the "func (recv *T) Method(" receiver form (returns "T.Method").
func goFuncName(fields []string) string {
	// fields[0] == "func".
	if len(fields) < 2 {
		return ""
	}
	if fields[1] == "(" || strings.HasPrefix(fields[1], "(") {
		// Receiver method: find the closing ")" then the next ident is the method.
		recv := ""
		mi := -1
		for i := 1; i < len(fields); i++ {
			if strings.Contains(fields[i], ")") {
				// The receiver type is the token before ")", strip * and ().
				recv = identHead(strings.TrimLeft(strings.Trim(fields[i], "()"), "*"))
				if recv == "" && i-1 >= 1 {
					recv = identHead(strings.TrimLeft(strings.Trim(fields[i-1], "()"), "*"))
				}
				mi = i + 1
				break
			}
		}
		if mi >= 0 && mi < len(fields) {
			m := identHead(fields[mi])
			if recv != "" {
				return recv + "." + m
			}
			return m
		}
		return ""
	}
	return identHead(fields[1])
}

// identHead returns the leading identifier of s (up to the first non-ident rune
// such as "(", "[", "{", ":", "<").
func identHead(s string) string {
	for i, r := range s {
		if r == '_' || r == '.' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return s[:i]
	}
	return s
}

// dedupeStable removes duplicate IDs preserving first-appearance order.
func dedupeStable(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// writeReadOnlyConfig writes a minimal read-mode cell config for the daemon.
// It mirrors the bench-full profile the aider regen uses; store stays off so the
// capture is deterministic.
func writeReadOnlyConfig(sb *benchsandbox.Sandbox, task, mode string) (string, error) {
	dir := sb.ModeDir(task, mode)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	cfgPath := filepath.Join(dir, "helix-cell.yml")
	const cfg = "profile: bench-full\nstore:\n  enabled: false\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		return "", err
	}
	return cfgPath, nil
}
