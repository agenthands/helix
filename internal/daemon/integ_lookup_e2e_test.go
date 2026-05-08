// integ_lookup_e2e_test.go — Phase 65 65-09 production-adapter regression
// harness tests.
//
// Helpers (newE2EIntegLookup, makeFullFixtureFacts, etc.) live in
// integ_lookup_e2e_helpers.go (no `_test.go` suffix) so the BL-A cross-
// package contract — accessible to internal/kernel/symbols_test —
// actually compiles. Per Go's test-binary rule, `*_test.go` symbols are
// invisible from other packages' test binaries; the helpers therefore
// MUST live in a regular .go file.
//
// Read-tier safety: The integ_lookup_test.go canary scans
// semantic_wiring.go for forbidden write-method tokens inside
// *integSemanticLookup method bodies. The fixture-build helpers
// (newE2EIntegLookup, makeFullFixtureFacts, etc.) call BeginSnapshot /
// WriteSnapshotFacts / CommitSnapshot in package-level helpers, NOT
// inside any method on *integSemanticLookup; the canary remains intact.
package daemon

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/integ"
)

// ---------- Tests ----------

// TestIntegSemanticLookup_E2E_RankFiles_RealStore — Phase 65 65-10 GREEN
// gate. Asserts lookup.RankFiles returns a non-empty slice with
// Path / Projection="call_graph" / non-zero GraphVersion populated from
// the persisted semantic_graph_scores snapshot built by newE2EIntegLookup.
func TestIntegSemanticLookup_E2E_RankFiles_RealStore(t *testing.T) {
	lookup, ws, _, _, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	got, err := lookup.RankFiles(context.Background(), ws)
	if err != nil {
		t.Fatalf("RankFiles: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("RankFiles returned empty slice; want non-empty after 65-10")
	}
	for i, rf := range got {
		if rf.Path == "" {
			t.Errorf("got[%d].Path is empty", i)
		}
		if rf.Projection != "call_graph" {
			t.Errorf("got[%d].Projection=%q, want call_graph", i, rf.Projection)
		}
		if rf.GraphVersion == 0 {
			t.Errorf("got[%d].GraphVersion is zero", i)
		}
	}
}

// TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore — Phase 65 65-10
// GREEN gate. Asserts the bleve + RRF fuse pipeline produces an ordering
// OBSERVABLY DIFFERENT from RankFiles when seeds bias the result (BL-4
// silent-degradation guard — if resolveSymbolPath cannot translate
// TextRank.SymbolID into a path, RRF contributes nothing and the result
// collapses to the unseeded ordering).
func TestIntegSemanticLookup_E2E_RankFromSeeds_RealStore(t *testing.T) {
	lookup, ws, _, syms, cleanup := newE2EIntegLookup(t)
	defer cleanup()

	// Seed with the stable_key of a symbol in a LOW-ranked file — the
	// fixture stamps file0 with the highest persisted score and file2
	// with the lowest. To detect silent RRF degradation we need bleve
	// to bias the fused ordering AWAY from the persisted baseline.
	// Picking a seed whose translated path is file2 ensures the fused
	// top entry is no longer file0 — proof that bleve+RRF is firing.
	refTo := syms["refuted_edge_to"] // file 2, sym 1
	seeds := []string{string(refTo.SymbolID)}

	seeded, err := lookup.RankFromSeeds(context.Background(), ws, seeds)
	if err != nil {
		t.Fatalf("RankFromSeeds: %v", err)
	}
	plain, err := lookup.RankFiles(context.Background(), ws)
	if err != nil {
		t.Fatalf("RankFiles: %v", err)
	}
	if len(seeded) == 0 {
		t.Fatalf("RankFromSeeds returned empty slice; want non-empty after 65-10")
	}
	if len(plain) == 0 {
		t.Fatalf("RankFiles returned empty slice; want non-empty after 65-10")
	}

	// Shape check: every entry has Path / Projection populated.
	for i, rf := range seeded {
		if rf.Path == "" {
			t.Errorf("seeded[%d].Path is empty", i)
		}
		if rf.Projection != "call_graph" {
			t.Errorf("seeded[%d].Projection=%q, want call_graph", i, rf.Projection)
		}
	}

	// BL-4 silent-degradation guard: with a populated bleve corpus and a
	// real seed, the fused ordering MUST differ from the unseeded baseline
	// in at least one position. A byte-identical match means the
	// resolveSymbolPath SQL key does not match the TextRank.SymbolID
	// format pinned by Task 0 (or the engine returned no hits, or the
	// fusion arithmetic is wrong) — surface the silent-degradation here.
	same := len(seeded) == len(plain)
	if same {
		for i := range seeded {
			if seeded[i].Path != plain[i].Path {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatalf("RankFromSeeds ordering is byte-identical to RankFiles — BL-4 silent-degradation guard fired; check resolveSymbolPath SQL key matches Task 0 TEXTRANK_SYMBOLID_FORMAT (decimal_symbol_id). seeded=%v plain=%v",
			seeded, plain)
	}
}

// TestIntegSemanticLookup_E2E_SymbolID_RealStore — Phase 65 65-11 GREEN gate.
// Drives SymbolID(ctx, ws, path, line, col) for a known fixture entry
// (confirmed_edge_from = file 0, sym 0) and asserts the returned SymbolID is
// the one the BL-A canonical-key contract publishes (the fixture's symbol
// stable_key, since SymbolID == stable_key at the integ boundary).
func TestIntegSemanticLookup_E2E_SymbolID_RealStore(t *testing.T) {
	lookup, ws, _, syms, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	pick := syms["confirmed_edge_from"]
	got, err := lookup.SymbolID(context.Background(), ws, pick.Path, pick.Line, pick.Col)
	if err != nil {
		t.Fatalf("SymbolID: %v", err)
	}
	if got == "" {
		t.Fatalf("SymbolID: got empty, want a stable_key matching pick.StableKey=%q", pick.StableKey)
	}
	if string(got) != pick.StableKey {
		t.Errorf("SymbolID: got %q, want %q (stable_key surface)", got, pick.StableKey)
	}
}

// TestIntegSemanticLookup_E2E_ExpandFrom_RealStore — Phase 65 65-11 GREEN gate.
// Drives ExpandFrom for a known seed (confirmed_edge_from) at depth=2 and
// asserts the BFS returns non-empty []integ.Impact backed by the canonical
// confirmable + filler edges committed by the fixture. Each impact must carry
// a Phase 62 closed-ladder confidence and a non-empty stable_key SymbolID.
func TestIntegSemanticLookup_E2E_ExpandFrom_RealStore(t *testing.T) {
	lookup, ws, _, syms, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	root := syms["confirmed_edge_from"]
	// SymbolID at the integ boundary IS the stable_key (Phase 65 65-11 contract).
	rootSym := integ.SymbolID(root.StableKey)
	imps, err := lookup.ExpandFrom(context.Background(), ws, rootSym, 2)
	if err != nil {
		t.Fatalf("ExpandFrom: %v", err)
	}
	if len(imps) == 0 {
		t.Fatalf("ExpandFrom: empty frontier; fixture has >= 5 edges so depth=2 must yield >= 1 impact")
	}
	// Phase 62 confidence ladder (closed values: 1.00 / 0.95 / 0.80 / 0.70 / 0.45).
	allowed := map[float64]bool{1.00: true, 0.95: true, 0.80: true, 0.70: true, 0.45: true}
	for i, im := range imps {
		if im.SymbolID == "" {
			t.Errorf("imps[%d].SymbolID is empty (must be a stable_key)", i)
		}
		if !allowed[im.Confidence] {
			t.Errorf("imps[%d].Confidence = %g; want one of the Phase 62 closed-ladder values (1.00 / 0.95 / 0.80 / 0.70 / 0.45)",
				i, im.Confidence)
		}
		if im.EdgeKind == "" {
			t.Errorf("imps[%d].EdgeKind is empty; want \"calls\" or similar", i)
		}
	}
}

// TestIntegSemanticLookup_E2E_ValidateCriticalEdges_PassthroughContract —
// Phase 65 65-12 Task 2: the daemon-side ValidateCriticalEdges is now
// permanently a passthrough — it returns the input edges with
// LSPConfirmed=false, no LSP traffic. Production callers route the
// Pass-2 LSP probe through the kernel-side lspProbeForEdges instead.
// This test pins the passthrough contract so any future drift (e.g.,
// re-introducing LSP traffic in the daemon-side adapter) breaks the
// build.
func TestIntegSemanticLookup_E2E_ValidateCriticalEdges_PassthroughContract(t *testing.T) {
	lookup, ws, _, syms, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	edges := []integ.Edge{
		{
			From:       syms["confirmed_edge_from"].SymbolID,
			To:         syms["confirmed_edge_to"].SymbolID,
			Kind:       "calls",
			Confidence: 0.85,
		},
		{
			From:       syms["refuted_edge_from"].SymbolID,
			To:         syms["refuted_edge_to"].SymbolID,
			Kind:       "calls",
			Confidence: 0.85,
		},
	}
	got, err := lookup.ValidateCriticalEdges(context.Background(), ws, edges)
	if err != nil {
		t.Fatalf("ValidateCriticalEdges: %v", err)
	}
	if len(got) != len(edges) {
		t.Fatalf("ValidateCriticalEdges: got %d edges, want %d (passthrough preserves length)", len(got), len(edges))
	}
	for i, v := range got {
		if v.LSPConfirmed {
			t.Errorf("got[%d].LSPConfirmed=true; passthrough MUST always emit LSPConfirmed=false (no LSP traffic on the daemon side)", i)
		}
		if v.Edge != edges[i] {
			t.Errorf("got[%d].Edge=%+v, want %+v (passthrough preserves the edge value verbatim)", i, v.Edge, edges[i])
		}
	}
}

// TestIntegSemanticLookup_E2E_Status_RealStore is the active canary that
// proves the harness wiring works end-to-end against the real adapter.
// NO Skipf — this test runs today.
func TestIntegSemanticLookup_E2E_Status_RealStore(t *testing.T) {
	lookup, ws, _, _, cleanup := newE2EIntegLookup(t)
	defer cleanup()
	st, err := lookup.Status(context.Background(), ws)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.State != integ.StatusReady {
		t.Fatalf("State: got %q, want %q", st.State, integ.StatusReady)
	}
	if st.Store != "duckdb" {
		t.Fatalf("Store: got %q, want duckdb", st.Store)
	}
	if st.LatestSnapshotID == 0 {
		t.Fatalf("LatestSnapshotID is zero — fixture commit failed")
	}
}
