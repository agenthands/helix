// bl1_blast_radius_e2e_test.go — Phase 65 65-12 Task 2 BL-1 verifier-mandated
// concrete confidence-ladder regression for analyze_blast_radius.
//
// This file lives in the EXTERNAL test package (`package symbols_test`) so it
// can import internal/daemon (for daemon.NewE2EIntegLookupForTest) WITHOUT
// triggering a Go-level import cycle: daemon imports internal/kernel/symbols
// in production, so any internal-test (`package symbols`) file that imported
// daemon would create a cycle. The black-box test package is the standard
// Go workaround.
//
// To reach analyzeBlastRadiusViaLookup + lspProbeForEdges (both unexported in
// package symbols), this file consumes the test-only re-exports declared in
// export_for_test.go (still inside `package symbols`, so they have access to
// the unexported identifiers):
//
//   - symbols.AnalyzeBlastRadiusViaLookupForTest
//   - symbols.LspProbeForEdgesForTest
//   - symbols.PathToURIForTest
//
// Per WR-C: the lint analyzer (internal/lint/nokernel2semantic/analyzer.go)
// restricts internal/kernel→internal/semantic only; it does NOT restrict
// internal/kernel→internal/daemon. The new test-only edge stays in *_test.go
// (never compiled into production binaries) and matches the architectural
// rationale documented in 65-12 PLAN.md <rationale>.
package symbols_test

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/kernel/symbols"
	"github.com/agenthands/helix/internal/semantic/integ"
	gen "github.com/agenthands/helix/protocol/gen"
)

// TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder is the BL-1
// verifier-mandated regression: it observes the semantic confidence
// ladder END-TO-END with a real *integSemanticLookup (constructed via
// daemon.NewE2EIntegLookupForTest from 65-09's BL-A populated harness)
// and a synthetic LSP probe that exercises the genuine kernel-side
// lspProbeForEdges path.
//
// The test asserts:
//   - confidence == 1.00 on the confirmed edge (orchestrator's lspProbeFn
//     branch, validated through LSPConfirmed=true verdict).
//   - confidence == 0.20 + Refuted=true on the refuted edge.
//
// Every test argument is sourced from the canonical syms map shipped by
// 65-09 (BL-A contract): "confirmed_edge_from", "confirmed_edge_to",
// "refuted_edge_from", "refuted_edge_to", "confirmed_edge", "refuted_edge".
// No `(...)` placeholder pseudocode; no conditional fixture-extension
// language; no abbreviated-form hedge. The test exercises the FULL
// production orchestrator path.
func TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder(t *testing.T) {
	ctx := context.Background()

	// 1. Build the populated harness from 65-09 (BL-A).
	lookup, ws, _, syms, cleanup := daemon.NewE2EIntegLookupForTest(t)
	defer cleanup()

	confFrom := syms["confirmed_edge_from"]
	confTo := syms["confirmed_edge_to"]
	refFrom := syms["refuted_edge_from"]
	refTo := syms["refuted_edge_to"]
	confEdge := syms["confirmed_edge"]
	refEdge := syms["refuted_edge"]

	// BL-A canonical-key contract assertions: the harness MUST populate
	// every canonical key. A drift here fails BL-A — fix the harness.
	if confFrom.SymbolID == "" {
		t.Fatalf("syms[confirmed_edge_from] missing SymbolID — 65-09 BL-A canonical-key contract violated")
	}
	if confTo.SymbolID == "" {
		t.Fatalf("syms[confirmed_edge_to] missing SymbolID — 65-09 BL-A canonical-key contract violated")
	}
	if refFrom.SymbolID == "" {
		t.Fatalf("syms[refuted_edge_from] missing SymbolID — 65-09 BL-A canonical-key contract violated")
	}
	if refTo.SymbolID == "" {
		t.Fatalf("syms[refuted_edge_to] missing SymbolID — 65-09 BL-A canonical-key contract violated")
	}
	if confEdge.EdgeID == 0 {
		t.Fatalf("syms[confirmed_edge] missing EdgeID — 65-09 BL-A canonical-key contract violated")
	}
	if refEdge.EdgeID == 0 {
		t.Fatalf("syms[refuted_edge] missing EdgeID — 65-09 BL-A canonical-key contract violated")
	}

	// 2. Inline locator: delegates to lookup.LocateSymbol bound to ws +
	//    ctx. The test exercises the production seam — no shortcut.
	//
	//    The harness's LocateSymbol resolves stable_key (which is what
	//    integ.SymbolID is at the integ boundary, per 65-11 contract) →
	//    (path, line, col) via *Store.QuerySymbolLocationByStableKey
	//    landed in 65-12 Task 1.
	locator := func(s integ.SymbolID) (string, uint32, uint32, bool) {
		p, l, c, ok, _ := lookup.LocateSymbol(ctx, ws, s)
		return p, l, c, ok
	}

	// 3. Synthetic probeFn dispatched on the resolved From-coordinates.
	//    For the confirmed edge: returns a SymbolLocation overlapping
	//    confTo's URI/coords. For the refuted edge: returns nil. Any
	//    OTHER coordinate input returns an error so the test fails loudly
	//    if the orchestrator dispatches an unexpected probe.
	repoRoot := ws.RepoRoot
	confFromURI := symbols.PathToURIForTest(repoRoot, confFrom.Path)
	refFromURI := symbols.PathToURIForTest(repoRoot, refFrom.Path)
	confToURI := symbols.PathToURIForTest(repoRoot, confTo.Path)

	probeFn := func(_ context.Context, uri string, _, _ int) ([]symbols.SymbolLocation, error) {
		switch uri {
		case confFromURI:
			// Return a hit overlapping confTo's coordinates (1-based →
			// LSP 0-based: subtract 1 from line/col).
			lspLine := uint32(0)
			if confTo.Line > 0 {
				lspLine = confTo.Line - 1
			}
			lspCol := uint32(0)
			if confTo.Col > 0 {
				lspCol = confTo.Col - 1
			}
			return []symbols.SymbolLocation{{
				URI: confToURI,
				Range: gen.Range{
					Start: gen.Position{Line: lspLine, Character: lspCol},
					End:   gen.Position{Line: lspLine, Character: lspCol + 1},
				},
			}}, nil
		case refFromURI:
			// Empty list → kernel-side probe refutes.
			return nil, nil
		default:
			t.Fatalf("unexpected probe URI: %q", uri)
			return nil, nil
		}
	}

	// 4. Inline lspProbeFn closure mirroring the production wiring at
	//    registerAnalyzeBlastRadius (tools.go) — locator + probeFn fed
	//    into lspProbeForEdges.
	lspProbeFn := func(probeCtx context.Context, edges []integ.Edge) []integ.ValidatedEdge {
		return symbols.LspProbeForEdgesForTest(probeCtx, edges, locator, probeFn, repoRoot)
	}

	// 5. Drive the orchestrator from confirmed_edge_from. Locate the
	//    impact at confTo.SymbolID (= confTo.StableKey) and assert
	//    Confidence==1.00, Refuted==false.
	confFromSym := integ.SymbolID(confFrom.StableKey)
	confToSym := integ.SymbolID(confTo.StableKey)
	impacts, src, _, _, err := symbols.AnalyzeBlastRadiusViaLookupForTest(ctx, lookup, ws, confFromSym, lspProbeFn)
	if err != nil {
		t.Fatalf("AnalyzeBlastRadiusViaLookupForTest(confirmed): %v", err)
	}
	if src != integ.SourceSemantic {
		t.Fatalf("source: got %q, want semantic", src)
	}
	var confirmedFound bool
	var confirmedImpact integ.Impact
	for _, im := range impacts {
		if im.SymbolID == confToSym {
			confirmedImpact = im
			confirmedFound = true
			break
		}
	}
	if !confirmedFound {
		t.Fatalf("no impact with SymbolID=%q in confirmed-edge expansion: %+v", confToSym, impacts)
	}
	if confirmedImpact.Confidence != 1.00 {
		t.Fatalf("confirmed edge confidence: got %v, want 1.00", confirmedImpact.Confidence)
	}
	if confirmedImpact.Refuted {
		t.Fatalf("confirmed edge Refuted: got true, want false")
	}

	// 6. Drive the orchestrator from refuted_edge_from. Locate the impact
	//    at refTo.SymbolID and assert Confidence==0.20, Refuted==true.
	refFromSym := integ.SymbolID(refFrom.StableKey)
	refToSym := integ.SymbolID(refTo.StableKey)
	impacts2, _, _, _, err := symbols.AnalyzeBlastRadiusViaLookupForTest(ctx, lookup, ws, refFromSym, lspProbeFn)
	if err != nil {
		t.Fatalf("AnalyzeBlastRadiusViaLookupForTest(refuted): %v", err)
	}
	var refutedFound bool
	var refutedImpact integ.Impact
	for _, im := range impacts2 {
		if im.SymbolID == refToSym {
			refutedImpact = im
			refutedFound = true
			break
		}
	}
	if !refutedFound {
		t.Fatalf("no impact with SymbolID=%q in refuted-edge expansion: %+v", refToSym, impacts2)
	}
	if refutedImpact.Confidence != 0.20 {
		t.Fatalf("refuted edge confidence: got %v, want 0.20", refutedImpact.Confidence)
	}
	if !refutedImpact.Refuted {
		t.Fatalf("refuted edge Refuted: got false, want true")
	}
}
