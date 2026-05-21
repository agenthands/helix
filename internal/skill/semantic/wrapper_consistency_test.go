// wrapper_consistency_test.go — Phase 73-04 Task 1: D-04 SC#3 static
// wrapper-consistency gate for the 6 Phase 71/72 P1 handler files.
//
// INVARIANT (D-04): every P1 handler MUST (a) call the checkMode( helper on
// at least one non-comment line, (b) reference FreshnessV2 on at least one
// non-comment line, and (c) be registered via a named register* function whose
// name appears in the RegisterAll body in register.go. This gate makes a future
// refactor that drops any of those three from a P1 handler a CI failure.
//
// Pattern mirrors readonly_gate_test.go (Phase 71-05 Task 3): line-by-line
// bufio.Scanner scan, TrimLeft+HasPrefix comment stripping, per-file t.Run
// subtests. The gate is intentionally a simple string-match scan (not a full
// Go parser) — the purpose is to catch obvious regressions, not to be a full
// static analyzer.
//
// p1WrapperGatedFiles is a distinct identifier from gatedHandlerFiles
// (readonly_gate_test.go) to avoid a package-level name collision; both lists
// cover the same 6 P1 handler files.

package semantic

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// p1WrapperGatedFiles lists the 6 Phase 71/72 P1 handler files. Adding a new
// P1 handler MUST extend this list so the SC#3 gate covers it automatically.
var p1WrapperGatedFiles = []string{
	"tools_explain_symbol.go",
	"tools_find_related.go",
	"tools_validate_edge.go",
	"tools_cluster_map.go",
	"tools_explain_cluster.go",
	"tools_change_impact.go",
}

// p1RegisterNames lists the 6 P1 register* function names that MUST appear in
// the register.go RegisterAll body. Each name corresponds to one of the files
// in p1WrapperGatedFiles.
var p1RegisterNames = []string{
	"registerExplainSymbolDeep",
	"registerFindRelatedSymbols",
	"registerValidateGraphEdge",
	"registerGetClusterMap",
	"registerExplainCluster",
	"registerGetChangeImpactGraph",
}

// p1RequiredTokens lists the literal tokens that MUST appear on at least one
// non-comment line in each P1 handler file.
//
// Note: the gate checks that checkMode( is CALLED but does NOT pin which mode
// tier is used — 5 handlers use modeTierRead, tools_change_impact.go uses
// modeTierReview. Only the presence of the call is asserted.
var p1RequiredTokens = []string{
	"checkMode(",
	"FreshnessV2",
}

// TestWrapperConsistency_P1Handlers scans each of the 6 P1 handler files and
// asserts that both required tokens (checkMode( and FreshnessV2) appear on at
// least one non-comment line. Also asserts that all 6 P1 register* function
// names appear in register.go (RegisterAll coverage).
func TestWrapperConsistency_P1Handlers(t *testing.T) {
	// Part 1: per-file token scan.
	for _, file := range p1WrapperGatedFiles {
		file := file
		t.Run(file, func(t *testing.T) {
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("open %s: %v", file, err)
			}
			defer f.Close()

			found := make(map[string]bool, len(p1RequiredTokens))

			scanner := bufio.NewScanner(f)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				raw := scanner.Text()
				trimmed := strings.TrimLeft(raw, " \t")

				// Skip pure comment lines (// -style starters). The gate
				// intentionally does NOT track multi-line /* … */ blocks —
				// the P1 handler files use only `//` line comments, and the
				// simpler rule keeps the gate auditable.
				if strings.HasPrefix(trimmed, "//") {
					continue
				}

				for _, tok := range p1RequiredTokens {
					if strings.Contains(trimmed, tok) {
						found[tok] = true
					}
				}
			}
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan %s: %v", file, err)
			}

			for _, tok := range p1RequiredTokens {
				if !found[tok] {
					t.Errorf("%s: required token %q not found on any non-comment line (SC#3 gate violation)", file, tok)
				}
			}
		})
	}

	// Part 2: RegisterAll coverage — all 6 P1 register* names MUST appear in
	// register.go.
	t.Run("register.go/RegisterAll", func(t *testing.T) {
		f, err := os.Open("register.go")
		if err != nil {
			t.Fatalf("open register.go: %v", err)
		}
		defer f.Close()

		content, err := readAll(f)
		if err != nil {
			t.Fatalf("read register.go: %v", err)
		}

		for _, name := range p1RegisterNames {
			if !strings.Contains(content, name) {
				t.Errorf("register.go: P1 register function %q not found in RegisterAll body (SC#3 gate violation)", name)
			}
		}
	})
}

// readAll reads all content from an *os.File into a string using bufio.Scanner.
func readAll(f *os.File) (string, error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteByte('\n')
	}
	return sb.String(), scanner.Err()
}
