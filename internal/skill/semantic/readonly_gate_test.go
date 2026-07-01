// Package-level in-tree D-09 read-only gate (Phase 71-05 Task 3).
//
// 71-01's grep-gate location task established that the repo carries no
// external CI runner (no .github/workflows/*.yml, no scripts/, no Makefile
// rule) that lints handler files for the snapshot-write forbidden tokens
// (Begin/Commit/Abort/Write Snapshot[Facts]). This file is the equivalent
// in-tree gate per 71-CONTEXT.md D6: a string-match scan over the three
// Phase 71 P1 read-tool handler source files asserting the forbidden tokens
// never appear outside comments.
//
// The recorder-canary tests inside each tool's *_test.go file already enforce
// the invariant at runtime; this static gate makes the invariant resistant to
// future refactors that might re-route through an inadvertent type assertion
// or a transitively-imported helper that the per-tool canaries don't cover.
//
// Scope intentionally narrow: only Begin/Commit/Abort/Write Snapshot[Facts]
// identifiers, only outside Go comment lines (`//` start after trim). String
// literals carrying these substrings (e.g., error messages, the INVARIANT
// comment block) are explicitly tolerated when on commented lines.

package semantic

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// gateForbiddenTokens lists the snapshot-write identifiers that MUST NOT
// appear outside Go comment lines in the three Phase 71 handler files.
//
// All four are exact-match identifiers (no regex / substring fuzz) — matches
// the production *Store API surface and is robust against future renames that
// would force a re-gate review here.
var gateForbiddenTokens = []string{
	"BeginSnapshot",
	"CommitSnapshot",
	"AbortSnapshot",
	"WriteSnapshotFacts",
}

// gatedHandlerFiles enumerates the three Phase 71 P1 single-symbol read-tool
// handler files. Adding a new read-only handler MUST extend this list (and
// the new file MUST carry the same INVARIANT header).
var gatedHandlerFiles = []string{
	"tools_explain_symbol.go",
	"tools_find_related.go",
	"tools_validate_edge.go",
	"tools_cluster_map.go",     // Phase 72 addition
	"tools_explain_cluster.go", // Phase 72 addition
	"tools_change_impact.go",     // Phase 72 addition
	"tools_trace_data_flow.go",   // v2.10 addition
}

// TestReadOnlyGate_Phase71Handlers scans each handler file line-by-line,
// strips Go comment lines (lines whose first non-whitespace prefix is `//`),
// and asserts none of the forbidden snapshot-write tokens appear on the
// remaining code lines.
//
// Comment-line stripping is intentionally permissive: any line whose first
// non-whitespace character starts with `//` is treated as a comment, even if
// the line happens to embed code-like substrings inside. This matches the
// CONTEXT.md D6 mandate that the gate use a simple `grep -v '^//' | grep -E
// 'forbidden'` pipeline equivalent — not a full Go-parser-aware scan. The
// purpose is to catch obvious regressions (a future change that adds a real
// `store.BeginSnapshot(...)` call), not to be a full static analyzer.
func TestReadOnlyGate_Phase71Handlers(t *testing.T) {
	for _, file := range gatedHandlerFiles {
		file := file
		t.Run(file, func(t *testing.T) {
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("open %s: %v", file, err)
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				raw := scanner.Text()
				trimmed := strings.TrimLeft(raw, " \t")

				// Skip pure comment lines (// or /* */ -style starters). The
				// gate intentionally does NOT track multi-line /* … */ blocks
				// — the three handler files use only `//` line comments, and
				// the simpler rule keeps the gate auditable.
				if strings.HasPrefix(trimmed, "//") {
					continue
				}

				for _, tok := range gateForbiddenTokens {
					if strings.Contains(trimmed, tok) {
						t.Errorf("%s:%d D-09 gate violation: forbidden token %q on non-comment line:\n\t%s",
							file, lineNum, tok, raw)
					}
				}
			}
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan %s: %v", file, err)
			}
		})
	}
}
