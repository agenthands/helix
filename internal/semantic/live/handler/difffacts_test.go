// Phase 68 Plan 04 — Tier-1 (full diff) + Tier-2 (added-only) populator
// tests + Tier-3 reason-routing tests + diffSymbols algorithm pin.
//
// RED state: until Task 2 lands the Handler DI setters, the
// LastRecorderSnapshotForTest / DiffSymbolsForTest export_test seams,
// the diffSymbols algorithm and the tuple-return tryFullDiff /
// tryAddedOnlyDiff signatures, this file fails to compile pointing at
// the missing symbols.
package handler_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	dto "github.com/prometheus/client_model/go"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/live/handler"
	"github.com/agenthands/helix/internal/semantic/store"
)

// freshMetrics builds an isolated *obs.Metrics on its own registry so
// tests don't share counter state.
func freshMetrics() *obs.Metrics {
	return obs.Noop(slog.Default().Handler()).Metrics()
}

// sampleValueForFilefactdiff gathers the helix_live_filefactdiff_total
// counter family and returns the value for the (tier, repo) label pair.
// Returns 0 when no matching sample exists.
func sampleValueForFilefactdiff(t *testing.T, m *obs.Metrics, tier, repo string) float64 {
	t.Helper()
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "helix_live_filefactdiff_total" {
			continue
		}
		for _, sm := range mf.GetMetric() {
			if labelsMatch(sm, map[string]string{"tier": tier, "repo": repo}) {
				return sm.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func sampleValueForSynthReason(t *testing.T, m *obs.Metrics, reason string) float64 {
	t.Helper()
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "helix_live_filefactdiff_synthetic_reason_total" {
			continue
		}
		for _, sm := range mf.GetMetric() {
			if labelsMatch(sm, map[string]string{"reason": reason}) {
				return sm.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func labelsMatch(sm *dto.Metric, want map[string]string) bool {
	got := map[string]string{}
	for _, lp := range sm.GetLabel() {
		got[lp.GetName()] = lp.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// ---- test doubles for FileFactStore + ExtractRegistry --------------------

type fakeFileFactStore struct {
	fact store.PriorFileFact
	ok   bool
	err  error
}

func (f *fakeFileFactStore) GetLatestFileFact(_ context.Context, _, _ string) (store.PriorFileFact, bool, error) {
	return f.fact, f.ok, f.err
}

type fakeExtractRegistry struct {
	provider extract.Provider
	has      bool
}

func (r *fakeExtractRegistry) Provider(_ string) (extract.Provider, bool) {
	return r.provider, r.has
}

// fakeExtractProvider implements extract.Provider but only the
// ExtractFile method is exercised by the populator. The other methods
// return zero values; tests never call them.
type fakeExtractProvider struct {
	extract.Provider // embed nil to satisfy unused methods (panic if called — caught by tests)
	ef               *extract.ExtractedFile
	err              error
}

func (p *fakeExtractProvider) ExtractFile(_ context.Context, _, _ string) (*extract.ExtractedFile, error) {
	return p.ef, p.err
}

// newHandlerForPopulator builds a Handler with the noop fakeStore +
// goodHasher (from noop_test.go) plus the optional FileFactStore /
// ExtractRegistry the populator under test consumes.
func newHandlerForPopulator(t *testing.T, fs handler.FileFactStore, reg handler.ExtractRegistry, metrics *obs.Metrics) *handler.Handler {
	t.Helper()
	store := &fakeStore{tx: &fakeTx{epoch: 1}}
	h := handler.New(store, goodHasher, nil, nil)
	if fs != nil {
		h.SetFileFactStore(fs)
	}
	if reg != nil {
		h.SetExtractRegistry(reg)
	}
	if metrics != nil {
		h.FileFactDiffMetrics = metrics
	}
	return h
}

// ---- diffSymbols direct tests --------------------------------------------

func TestDiffSymbols(t *testing.T) {
	type sym = store.PriorSymbol
	type nsym = extract.SymbolFact

	cases := []struct {
		name        string
		prior       []store.PriorSymbol
		curr        []extract.SymbolFact
		wantAdded   int
		wantRemoved int
		wantChanged int
		assertFn    func(t *testing.T, snap graphpkg.FileFactDiff)
	}{
		{
			name:      "added",
			prior:     nil,
			curr:      []nsym{{ID: 1}},
			wantAdded: 1,
			assertFn: func(t *testing.T, snap graphpkg.FileFactDiff) {
				if snap.AddedSymbols[0].NodeID != 1 {
					t.Errorf("added NodeID: got %d, want 1", snap.AddedSymbols[0].NodeID)
				}
				if !snap.AddedSymbols[0].KindChanged {
					t.Error("added must carry KindChanged=true")
				}
			},
		},
		{
			name:        "removed",
			prior:       []sym{{ID: 1}},
			curr:        nil,
			wantRemoved: 1,
			assertFn: func(t *testing.T, snap graphpkg.FileFactDiff) {
				if snap.RemovedSymbols[0].NodeID != 1 {
					t.Errorf("removed NodeID: got %d, want 1", snap.RemovedSymbols[0].NodeID)
				}
			},
		},
		{
			name:        "changed-signature",
			prior:       []sym{{ID: 1, Signature: "A"}},
			curr:        []nsym{{ID: 1, Signature: "B"}},
			wantChanged: 1,
			assertFn: func(t *testing.T, snap graphpkg.FileFactDiff) {
				if !snap.ChangedSymbols[0].SignatureChanged {
					t.Error("expected SignatureChanged=true")
				}
			},
		},
		{
			name:        "changed-visibility",
			prior:       []sym{{ID: 1, Visibility: "private"}},
			curr:        []nsym{{ID: 1, Visibility: "exported"}},
			wantChanged: 1,
			assertFn: func(t *testing.T, snap graphpkg.FileFactDiff) {
				if !snap.ChangedSymbols[0].ExportedChanged {
					t.Error("expected ExportedChanged=true")
				}
			},
		},
		{
			name:        "changed-kind",
			prior:       []sym{{ID: 1, Kind: "func"}},
			curr:        []nsym{{ID: 1, Kind: extract.SymbolKind("method")}},
			wantChanged: 1,
			assertFn: func(t *testing.T, snap graphpkg.FileFactDiff) {
				if !snap.ChangedSymbols[0].KindChanged {
					t.Error("expected KindChanged=true")
				}
			},
		},
		{
			name:        "unchanged",
			prior:       []sym{{ID: 1, Signature: "A", Visibility: "exported", Kind: "func", StableKey: extract.CanonicalizeStableSymbolKey(extract.StableSymbolKey{QualifiedName: "x"})}},
			curr:        []nsym{{ID: 1, Signature: "A", Visibility: "exported", Kind: extract.SymbolKind("func"), StableKey: extract.StableSymbolKey{QualifiedName: "x"}}},
			wantChanged: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &handler.FileFactDiffRecorder{}
			handler.DiffSymbolsForTest(tc.prior, tc.curr, rec)
			snap := rec.Snapshot()
			if got := len(snap.AddedSymbols); got != tc.wantAdded {
				t.Errorf("added: got %d, want %d", got, tc.wantAdded)
			}
			if got := len(snap.RemovedSymbols); got != tc.wantRemoved {
				t.Errorf("removed: got %d, want %d", got, tc.wantRemoved)
			}
			if got := len(snap.ChangedSymbols); got != tc.wantChanged {
				t.Errorf("changed: got %d, want %d", got, tc.wantChanged)
			}
			if tc.assertFn != nil {
				tc.assertFn(t, snap)
			}
		})
	}
}

// TestDiffSymbols_BodyOnlyNotGraphChanging — Pitfall 3 regression: body-only
// edits change SignatureHash (which mixes body bytes in the Go provider)
// but MUST NOT advance graph_version. diffSymbols compares Signature
// (text) not SignatureHash, so an identical Signature/Visibility/Kind/
// StableKey with a different SignatureHash MUST produce zero changed entries.
func TestDiffSymbols_BodyOnlyNotGraphChanging(t *testing.T) {
	stableKey := extract.StableSymbolKey{QualifiedName: "F"}
	priorKey := extract.CanonicalizeStableSymbolKey(stableKey)
	prior := []store.PriorSymbol{{
		ID: 1, Signature: "func F(x int) int",
		SignatureHash: "hash-prior",
		Visibility:    "exported",
		Kind:          "func",
		StableKey:     priorKey,
	}}
	curr := []extract.SymbolFact{{
		ID: 1, Signature: "func F(x int) int",
		SignatureHash: "hash-current-DIFFERENT", // body bytes changed
		Visibility:    "exported",
		Kind:          extract.SymbolKind("func"),
		StableKey:     stableKey,
	}}
	rec := &handler.FileFactDiffRecorder{}
	handler.DiffSymbolsForTest(prior, curr, rec)
	snap := rec.Snapshot()
	if len(snap.ChangedSymbols) != 0 {
		t.Fatalf("body-only edit (same Signature, different SignatureHash) MUST NOT be graph-changing; got %d ChangedSymbols (%+v)",
			len(snap.ChangedSymbols), snap.ChangedSymbols)
	}
	if len(snap.AddedSymbols)+len(snap.RemovedSymbols) != 0 {
		t.Fatalf("unexpected added/removed entries: %+v", snap)
	}
}

// ---- Tier-1 (tryFullDiff) tests ------------------------------------------

func TestTryFullDiff_HappyPath(t *testing.T) {
	prior := store.PriorFileFact{
		Path:             "foo.go",
		ExtractionStatus: "ready",
		Symbols:          []store.PriorSymbol{{ID: 1, Signature: "v1"}},
	}
	fs := &fakeFileFactStore{fact: prior, ok: true}
	ef := &extract.ExtractedFile{
		File:    extract.FileFact{ExtractionStatus: extract.ExtractionStatusReady},
		Symbols: []extract.SymbolFact{{ID: 1, Signature: "v2"}},
	}
	reg := &fakeExtractRegistry{provider: &fakeExtractProvider{ef: ef}, has: true}
	m := freshMetrics()
	h := newHandlerForPopulator(t, fs, reg, m)

	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile: %v", err)
	}

	snap := handler.LastRecorderSnapshotForTest(h)
	if len(snap.ChangedSymbols) != 1 {
		t.Fatalf("expected 1 ChangedSymbol; got %d (%+v)", len(snap.ChangedSymbols), snap.ChangedSymbols)
	}
	if !snap.ChangedSymbols[0].SignatureChanged {
		t.Error("expected SignatureChanged=true on the diff")
	}
	// outcome metric tier="full" incremented.
	if c := sampleValueForFilefactdiff(t, m, "full", "repo-A"); c != 1 {
		t.Errorf("LiveFileFactDiff{tier=full,repo=repo-A} = %v; want 1", c)
	}
}

func TestTryFullDiff_ColdStart(t *testing.T) {
	fs := &fakeFileFactStore{ok: false} // no prior fact
	ef := &extract.ExtractedFile{File: extract.FileFact{ExtractionStatus: extract.ExtractionStatusReady}}
	reg := &fakeExtractRegistry{provider: &fakeExtractProvider{ef: ef}, has: true}
	m := freshMetrics()
	h := newHandlerForPopulator(t, fs, reg, m)

	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile: %v", err)
	}
	// Tier-3 must fire with reason="cold_start" (extract returned Ready
	// so tryAddedOnlyDiff returns "" reason; tryFullDiff returned
	// "cold_start"; combine → cold_start).
	if c := sampleValueForSynthReason(t, m, "cold_start"); c != 1 {
		t.Errorf("synthetic_reason{cold_start} = %v; want 1", c)
	}
}

func TestTryFullDiff_StoreError(t *testing.T) {
	fs := &fakeFileFactStore{err: errors.New("io")}
	reg := &fakeExtractRegistry{has: false}
	m := freshMetrics()
	h := newHandlerForPopulator(t, fs, reg, m)

	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile: %v", err)
	}
	// Tier-1 falls through; Tier-2 absent (no provider); Tier-3 fires
	// with reason="cold_start" (store error path is treated as cold_start —
	// the prior fact is "missing" from the caller's perspective).
	if c := sampleValueForFilefactdiff(t, m, "synthetic", "repo-A"); c != 1 {
		t.Errorf("LiveFileFactDiff{tier=synthetic,repo=repo-A} = %v; want 1", c)
	}
}

// ---- Tier-2 (tryAddedOnlyDiff) tests --------------------------------------

func TestTryAddedOnlyDiff_Partial(t *testing.T) {
	fs := &fakeFileFactStore{ok: false} // no prior fact so Tier-1 falls through
	ef := &extract.ExtractedFile{
		File:    extract.FileFact{ExtractionStatus: extract.ExtractionStatusPartial},
		Symbols: []extract.SymbolFact{{ID: 1}, {ID: 2}},
	}
	reg := &fakeExtractRegistry{provider: &fakeExtractProvider{ef: ef}, has: true}
	m := freshMetrics()
	h := newHandlerForPopulator(t, fs, reg, m)

	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile: %v", err)
	}

	snap := handler.LastRecorderSnapshotForTest(h)
	if len(snap.AddedSymbols) != 2 {
		t.Fatalf("expected 2 AddedSymbols; got %d", len(snap.AddedSymbols))
	}
	for i, d := range snap.AddedSymbols {
		if !d.KindChanged {
			t.Errorf("AddedSymbols[%d].KindChanged=false; want true", i)
		}
	}
	if c := sampleValueForFilefactdiff(t, m, "added-only", "repo-A"); c != 1 {
		t.Errorf("LiveFileFactDiff{tier=added-only,repo=repo-A} = %v; want 1", c)
	}
}

func TestTryAddedOnlyDiff_ReadyNotHandled(t *testing.T) {
	fs := &fakeFileFactStore{ok: false}
	ef := &extract.ExtractedFile{
		File:    extract.FileFact{ExtractionStatus: extract.ExtractionStatusReady},
		Symbols: []extract.SymbolFact{{ID: 1}},
	}
	reg := &fakeExtractRegistry{provider: &fakeExtractProvider{ef: ef}, has: true}
	m := freshMetrics()
	h := newHandlerForPopulator(t, fs, reg, m)

	if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile: %v", err)
	}
	// Tier-2 returns false (status=Ready, owned by Tier-1) — but Tier-1
	// already saw cold_start, so Tier-3 fires with cold_start.
	if c := sampleValueForFilefactdiff(t, m, "added-only", "repo-A"); c != 0 {
		t.Errorf("LiveFileFactDiff{tier=added-only,repo=repo-A} = %v; want 0", c)
	}
	if c := sampleValueForFilefactdiff(t, m, "synthetic", "repo-A"); c != 1 {
		t.Errorf("LiveFileFactDiff{tier=synthetic,repo=repo-A} = %v; want 1", c)
	}
}

// ---- Tier-3 bounded reason metric tests -----------------------------------

func TestTier3_BoundedReasonMetric(t *testing.T) {
	// (a) cold start: no prior fact, no extraction failure.
	t.Run("cold_start", func(t *testing.T) {
		fs := &fakeFileFactStore{ok: false}
		reg := &fakeExtractRegistry{has: false} // no provider → Tier-1 and Tier-2 short-circuit before ExtractFile
		m := freshMetrics()
		h := newHandlerForPopulator(t, fs, reg, m)

		if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "foo.go"); err != nil {
			t.Fatalf("UpdateChangedFile: %v", err)
		}
		if c := sampleValueForSynthReason(t, m, "cold_start"); c != 1 {
			t.Errorf("synth_reason{cold_start}=%v; want 1", c)
		}
		if c := sampleValueForFilefactdiff(t, m, "synthetic", "repo-A"); c != 1 {
			t.Errorf("outcome{tier=synthetic}=%v; want 1", c)
		}
		snap := handler.LastRecorderSnapshotForTest(h)
		if len(snap.ChangedSymbols) != 1 || !snap.ChangedSymbols[0].KindChanged {
			t.Fatalf("Tier-3 synthetic marker missing/incorrect: %+v", snap)
		}
	})

	// (b) extract_failed.
	t.Run("extract_failed", func(t *testing.T) {
		fs := &fakeFileFactStore{ok: false}
		ef := &extract.ExtractedFile{File: extract.FileFact{ExtractionStatus: extract.ExtractionStatusFailed}}
		reg := &fakeExtractRegistry{provider: &fakeExtractProvider{ef: ef}, has: true}
		m := freshMetrics()
		h := newHandlerForPopulator(t, fs, reg, m)

		if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "foo.go"); err != nil {
			t.Fatalf("UpdateChangedFile: %v", err)
		}
		if c := sampleValueForSynthReason(t, m, "extract_failed"); c != 1 {
			t.Errorf("synth_reason{extract_failed}=%v; want 1", c)
		}
		if c := sampleValueForFilefactdiff(t, m, "synthetic", "repo-A"); c != 1 {
			t.Errorf("outcome{tier=synthetic}=%v; want 1", c)
		}
	})

	// (c) extract_unsupported.
	t.Run("extract_unsupported", func(t *testing.T) {
		fs := &fakeFileFactStore{ok: false}
		ef := &extract.ExtractedFile{File: extract.FileFact{ExtractionStatus: extract.ExtractionStatusUnsupported}}
		reg := &fakeExtractRegistry{provider: &fakeExtractProvider{ef: ef}, has: true}
		m := freshMetrics()
		h := newHandlerForPopulator(t, fs, reg, m)

		if err := h.UpdateChangedFile(context.Background(), semantic.RepoID("repo-A"), "foo.go"); err != nil {
			t.Fatalf("UpdateChangedFile: %v", err)
		}
		if c := sampleValueForSynthReason(t, m, "extract_unsupported"); c != 1 {
			t.Errorf("synth_reason{extract_unsupported}=%v; want 1", c)
		}
	})
}

// TestFileFactDiffOutcomeMetric — sanity check that the outcome counter
// only ever carries the three closed-enum tier values (no others).
func TestFileFactDiffOutcomeMetric(t *testing.T) {
	// Tier-1 full
	{
		fs := &fakeFileFactStore{fact: store.PriorFileFact{
			Path: "foo.go", Symbols: []store.PriorSymbol{{ID: 1, Signature: "a"}},
		}, ok: true}
		ef := &extract.ExtractedFile{
			File:    extract.FileFact{ExtractionStatus: extract.ExtractionStatusReady},
			Symbols: []extract.SymbolFact{{ID: 1, Signature: "b"}},
		}
		reg := &fakeExtractRegistry{provider: &fakeExtractProvider{ef: ef}, has: true}
		m := freshMetrics()
		h := newHandlerForPopulator(t, fs, reg, m)
		_ = h.UpdateChangedFile(context.Background(), semantic.RepoID("r"), "foo.go")
		if c := sampleValueForFilefactdiff(t, m, "full", "r"); c != 1 {
			t.Errorf("full=%v; want 1", c)
		}
	}
	// Tier-2 added-only
	{
		fs := &fakeFileFactStore{ok: false}
		ef := &extract.ExtractedFile{
			File:    extract.FileFact{ExtractionStatus: extract.ExtractionStatusPartial},
			Symbols: []extract.SymbolFact{{ID: 1}},
		}
		reg := &fakeExtractRegistry{provider: &fakeExtractProvider{ef: ef}, has: true}
		m := freshMetrics()
		h := newHandlerForPopulator(t, fs, reg, m)
		_ = h.UpdateChangedFile(context.Background(), semantic.RepoID("r"), "foo.go")
		if c := sampleValueForFilefactdiff(t, m, "added-only", "r"); c != 1 {
			t.Errorf("added-only=%v; want 1", c)
		}
	}
	// Tier-3 synthetic
	{
		fs := &fakeFileFactStore{ok: false}
		reg := &fakeExtractRegistry{has: false}
		m := freshMetrics()
		h := newHandlerForPopulator(t, fs, reg, m)
		_ = h.UpdateChangedFile(context.Background(), semantic.RepoID("r"), "foo.go")
		if c := sampleValueForFilefactdiff(t, m, "synthetic", "r"); c != 1 {
			t.Errorf("synthetic=%v; want 1", c)
		}
	}
}
