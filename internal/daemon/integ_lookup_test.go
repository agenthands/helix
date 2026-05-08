// integ_lookup_test.go — Phase 65 65-03 read-tier grep canary + interface
// guard for integSemanticLookup.
//
// The canary mirrors the Phase 64 P64-05 doctrine (tools_refresh.go grep test):
// any forbidden write-method token appearing inside an integSemanticLookup
// method body is a build-time failure, regardless of whether it ever runs.
// This is the cheapest enforcement of the M-readtier mitigation (the
// production lookup must not invoke snapshot-write paths).
package daemon

import (
	"os"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/semantic/integ"
)

// TestIntegSemanticLookup_InterfaceAssertion is a compile-time guard. The
// production adapter type *integSemanticLookup must satisfy integ.SemanticLookup.
// A method shape drift on either side breaks the build via this assertion.
func TestIntegSemanticLookup_InterfaceAssertion(t *testing.T) {
	var _ integ.SemanticLookup = (*integSemanticLookup)(nil)
}

// TestIntegSemanticLookup_ReadTierCanary scans semantic_wiring.go for any
// forbidden write-method token appearing inside the bodies of
// integSemanticLookup methods. Forbidden tokens are the snapshot-mutation
// surface the M-readtier mitigation excludes from the read+ tier.
//
// The check is intentionally string-based (no go/parser), matching the Phase
// 64 P64-05 grep-canary precedent.
func TestIntegSemanticLookup_ReadTierCanary(t *testing.T) {
	src, err := os.ReadFile("semantic_wiring.go")
	if err != nil {
		t.Fatalf("read semantic_wiring.go: %v", err)
	}
	body := extractIntegLookupMethodBodies(t, string(src))
	if body == "" {
		t.Fatal("could not locate any *integSemanticLookup method body in semantic_wiring.go")
	}
	forbidden := []string{
		"BeginSnapshot",
		"CommitSnapshot",
		"AbortSnapshot",
		"WriteSnapshotFacts",
		"OnFlush",
		"BumpGraphVersion",
	}
	for _, tok := range forbidden {
		if strings.Contains(body, tok) {
			t.Fatalf("integSemanticLookup body contains forbidden write-method token %q — read+ tier breach (M-readtier)", tok)
		}
	}
}

// extractIntegLookupMethodBodies returns the concatenated bodies of every
// method declared on *integSemanticLookup or integSemanticLookup. The
// extractor is intentionally simple: it scans line-by-line for a function
// header that declares a receiver of (*integSemanticLookup) and accumulates
// lines until a closing "}" appears at column 1 (matching the package's
// gofmt'd style).
func extractIntegLookupMethodBodies(t *testing.T, src string) string {
	t.Helper()
	var sb strings.Builder
	lines := strings.Split(src, "\n")
	const headerNeedle = "(l *integSemanticLookup)"
	const altHeaderNeedle = "(*integSemanticLookup)"
	inMethod := false
	for _, line := range lines {
		if !inMethod {
			// Match either the canonical (l *integSemanticLookup) shape
			// (used by every method body the canary guards) or the bare
			// (*integSemanticLookup) shape (used by the compile-time
			// interface assertion at end-of-file). Only the former opens a
			// method body; the latter is a single line and is skipped.
			if strings.HasPrefix(line, "func ") && strings.Contains(line, headerNeedle) {
				inMethod = true
				sb.WriteString(line)
				sb.WriteByte('\n')
				continue
			}
			// Skip the compile-time assertion form (no body).
			_ = altHeaderNeedle
			continue
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
		if line == "}" {
			inMethod = false
		}
	}
	return sb.String()
}
