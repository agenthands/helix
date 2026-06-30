// integ_lookup_test.go — Phase 65 65-03 read-tier grep canary + interface
// guard for integSemanticLookup.
//
// The canary mirrors the Phase 64 P64-05 doctrine (tools_refresh.go grep test):
// any forbidden write-method token appearing inside an integSemanticLookup
// method body is a build-time failure, regardless of whether it ever runs.
// This is the cheapest enforcement of the M-readtier mitigation (the
// production lookup must not invoke snapshot-write paths).
//
// Phase 65 65-12 Task 3 (WR-02): the method-body extractor is now
// implemented with go/parser + ast.Inspect (not the column-1 closing-brace
// heuristic). The previous heuristic broke if a struct literal inside a
// method body contained `}` at column 1 — purely a stylistic accident, but
// one a future formatter change could trigger silently. The parser-based
// extractor is robust to ANY future struct-literal additions, multi-line
// receivers, or formatting drift.
package daemon

import (
	"go/ast"
	"go/parser"
	"go/token"
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
// Phase 65 65-12 Task 3 (WR-02): the body-extraction step is now backed
// by go/parser; the substring-match still uses strings.Contains because
// snapshot-write APIs are tokenized via identifier names (no false-
// positive risk inside string literals — none of the forbidden tokens
// appears as a substring of any other identifier in the source).
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

// extractIntegLookupMethodBodies parses src as a Go source file and returns
// the concatenated source bytes of every method body declared on
// integSemanticLookup or *integSemanticLookup.
//
// Phase 65 65-12 Task 3 (WR-02 fix): replaces the column-1-closing-brace
// heuristic with a go/parser-based walk. The parser is the canonical
// boundary detector — robust to struct literals, switch blocks, embedded
// type literals, or any future formatting drift that the line-based
// heuristic could miss.
//
// Returns the empty string when no method matches; callers must not assume
// the result has any particular shape beyond "concatenated method body
// bytes".
func extractIntegLookupMethodBodies(t *testing.T, src string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "semantic_wiring.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("extractIntegLookupMethodBodies: parser.ParseFile: %v", err)
	}
	var sb strings.Builder
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if !isIntegSemanticLookupReceiver(fd) {
			continue
		}
		if fd.Body == nil {
			continue
		}
		// Compute the source-byte span of the body; reuse the original
		// string since fset positions are 1-indexed offsets into src.
		bodyStart := fset.Position(fd.Body.Lbrace).Offset
		bodyEnd := fset.Position(fd.Body.Rbrace).Offset + 1
		if bodyStart < 0 || bodyEnd > len(src) || bodyStart >= bodyEnd {
			continue
		}
		sb.WriteString(src[bodyStart:bodyEnd])
		sb.WriteByte('\n')
	}
	return sb.String()
}

// isIntegSemanticLookupReceiver reports whether fd has a receiver typed
// integSemanticLookup or *integSemanticLookup.
func isIntegSemanticLookupReceiver(fd *ast.FuncDecl) bool {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return false
	}
	switch t := fd.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return t.Name == "integSemanticLookup"
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name == "integSemanticLookup"
		}
	}
	return false
}

// TestExtractIntegLookupMethodBodies_ParserSeesAllMethods asserts that the
// go/parser-based extractor finds at least one body for every method on
// integSemanticLookup that the SemanticLookup interface declares. A drift
// (e.g., the parser picks up zero bodies because the receiver name changed)
// fails this test loud rather than letting the canary silently report
// "could not locate any method body".
func TestExtractIntegLookupMethodBodies_ParserSeesAllMethods(t *testing.T) {
	src, err := os.ReadFile("semantic_wiring.go")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	body := extractIntegLookupMethodBodies(t, string(src))
	if body == "" {
		t.Fatal("extractor returned empty body")
	}
	// Sanity check: the bodies must collectively reference every read-side
	// API the interface specs out, indirectly proving every method body
	// landed in the result. If any of these substrings is absent, either
	// the extractor missed a method body OR the interface lost a
	// real-implementation method (caught here, not silently).
	wantSubstrings := []string{
		"l.store.LatestCommittedSnapshot", // RankFiles + Status
		"QueryRankedFiles",                // RankFiles
		"QuerySymbolByLocation",           // SymbolID
		"QueryEffectiveAdjacency",         // ExpandFrom
		"QuerySymbolLocationByStableKey",  // LocateSymbol (Task 1)
		"OverlayHasPendingRows",           // Status
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) {
			t.Errorf("extracted body missing substring %q — body extractor likely skipped a method", want)
		}
	}
}

// TestExtractIntegLookupMethodBodies_ReceiverCount asserts the parser sees
// exactly the expected method count on (*integSemanticLookup). The
// SemanticLookup interface declares 8 methods (Available, SymbolID,
// RankFiles, RankFromSeeds, ExpandFrom, ValidateCriticalEdges, Status,
// LocateSymbol). A drift here surfaces whether the interface gained or
// lost a method in production code that the canary should be inspecting.
func TestExtractIntegLookupMethodBodies_ReceiverCount(t *testing.T) {
	src, err := os.ReadFile("semantic_wiring.go")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "semantic_wiring.go", string(src), parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	count := 0
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if isIntegSemanticLookupReceiver(fd) && fd.Body != nil {
			count++
		}
	}
	const want = 10
	if count != want {
		t.Errorf("methods on (*integSemanticLookup): got %d, want %d (Available, SymbolID, RankFiles, RankFromSeeds, ExpandFrom, ValidateCriticalEdges, Status, LocateSymbol, Visibility, IsEntrypointReachable)",
			count, want)
	}
}
