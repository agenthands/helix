package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/health"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

type fakeProbeReady struct{}

func (fakeProbeReady) Available() bool                 { return true }
func (fakeProbeReady) Probe(ctx context.Context) error { return nil }

type fakeProbeDisabled struct{}

func (fakeProbeDisabled) Available() bool                 { return false }
func (fakeProbeDisabled) Probe(ctx context.Context) error { return errors.New("disabled") }

type fakeProbeUnhealthy struct{}

func (fakeProbeUnhealthy) Available() bool                 { return true }
func (fakeProbeUnhealthy) Probe(ctx context.Context) error { return errors.New("connection refused") }

// TestSemanticStoreStatus_Ready proves a probe that reports Available + nil
// Probe error renders as state=ready. SC-1.
func TestSemanticStoreStatus_Ready(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), fakeProbeReady{})
	if st.State != "ready" {
		t.Fatalf("State=%q, want ready", st.State)
	}
}

// TestSemanticStoreStatus_Disabled proves an Available()==false probe
// renders as state=disabled (the v1.10 CGO=0 / disabled-flag path).
func TestSemanticStoreStatus_Disabled(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), fakeProbeDisabled{})
	if st.State != "disabled" {
		t.Fatalf("State=%q, want disabled", st.State)
	}
}

// TestSemanticStoreStatus_NilProbe proves the nil-probe (no daemon wiring)
// path renders as state=disabled rather than panicking.
func TestSemanticStoreStatus_NilProbe(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), nil)
	if st.State != "disabled" {
		t.Fatalf("State=%q, want disabled (nil probe)", st.State)
	}
}

// TestSemanticStoreStatus_Unhealthy proves an Available()==true probe with
// a non-nil Probe error renders as state=unhealthy. WR-NEW-01: the Reason
// field is a closed enum, never the raw err.Error() text. A generic
// connection error maps to the "db_error" bucket.
func TestSemanticStoreStatus_Unhealthy(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), fakeProbeUnhealthy{})
	if st.State != "unhealthy" {
		t.Fatalf("State=%q, want unhealthy", st.State)
	}
	if st.Reason != health.SemanticReasonDBError {
		t.Fatalf("Reason=%q, want %q (closed enum)", st.Reason, health.SemanticReasonDBError)
	}
	// Belt-and-braces: assert the raw err.Error() text is NOT leaked.
	if st.Reason == "connection refused" {
		t.Fatalf("Reason leaked raw err text; closed enum violated")
	}
}

// TestSemanticStoreStatus_Unhealthy_NilHandle proves an error wrapping
// serr.ErrUnsupported maps to the "nil_handle" bucket. WR-NEW-01 +
// REVIEW.md CR-02: classifier matches the sentinel via errors.Is,
// never substring text.
func TestSemanticStoreStatus_Unhealthy_NilHandle(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"sentinel_direct", serr.ErrUnsupported},
		{"sentinel_wrapped_fmt_errorf",
			fmt.Errorf("semantic store unavailable: %w", serr.ErrUnsupported)},
		{"sentinel_double_wrapped",
			fmt.Errorf("probe failed: %w",
				fmt.Errorf("semantic store unavailable: %w", serr.ErrUnsupported))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := fakeProbeWithErr{err: tc.err}
			st := health.ComputeSemanticStoreStatus(context.Background(), p)
			if st.State != "unhealthy" {
				t.Fatalf("State=%q, want unhealthy", st.State)
			}
			if st.Reason != health.SemanticReasonNilHandle {
				t.Fatalf("Reason=%q, want %q", st.Reason, health.SemanticReasonNilHandle)
			}
		})
	}
}

// TestSemanticStoreStatus_Unhealthy_DBErrorTextLooksLikeNilHandle is the
// CR-02 regression guard: an error whose .Error() string contains the
// substring "store unavailable" but does NOT wrap serr.ErrUnsupported
// MUST classify as db_error, not nil_handle. WR-NEW-01.
func TestSemanticStoreStatus_Unhealthy_DBErrorTextLooksLikeNilHandle(t *testing.T) {
	err := errors.New("semantic store unavailable: connection reset")
	p := fakeProbeWithErr{err: err}
	st := health.ComputeSemanticStoreStatus(context.Background(), p)
	if st.State != "unhealthy" {
		t.Fatalf("State=%q, want unhealthy", st.State)
	}
	if st.Reason != health.SemanticReasonDBError {
		t.Fatalf("Reason=%q, want %q (substring text must NOT route to nil_handle)",
			st.Reason, health.SemanticReasonDBError)
	}
}

// TestSemanticStoreStatus_Unhealthy_ProbeTimeout proves a context-deadline
// error from the probe maps to the "probe_timeout" bucket. WR-NEW-01.
func TestSemanticStoreStatus_Unhealthy_ProbeTimeout(t *testing.T) {
	p := fakeProbeWithErr{err: context.DeadlineExceeded}
	st := health.ComputeSemanticStoreStatus(context.Background(), p)
	if st.State != "unhealthy" {
		t.Fatalf("State=%q, want unhealthy", st.State)
	}
	if st.Reason != health.SemanticReasonProbeTimeout {
		t.Fatalf("Reason=%q, want %q", st.Reason, health.SemanticReasonProbeTimeout)
	}
}

// fakeProbeWithErr returns a configurable error from Probe. Used by the
// closed-enum classifier tests above.
type fakeProbeWithErr struct{ err error }

func (fakeProbeWithErr) Available() bool                  { return true }
func (p fakeProbeWithErr) Probe(_ context.Context) error  { return p.err }

// TestSemanticStoreStatus_JSONShape proves the marshalled block uses
// snake_case field names per the documented contract.
func TestSemanticStoreStatus_JSONShape(t *testing.T) {
	st := health.SemanticStoreStatus{State: "ready"}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"state":"ready"`) {
		t.Fatalf("json shape mismatch: %s", b)
	}
}

// ---------------------------------------------------------------------------
// Phase 65 65-07 — semantic_index block + top-level source field.
// ---------------------------------------------------------------------------

// fakeIndexAccessor is a hand-rolled SemanticIndexAccessor test double.
type fakeIndexAccessor struct {
	status integ.SemanticStatus
	err    error
}

func (f *fakeIndexAccessor) Status(_ context.Context, _ workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	return f.status, f.err
}

// fakeSemLookup is a minimal SemanticLookup test double for envelope-source
// tests. Only Available() is consulted by integ.ChooseSource(cfgGate, lookup, nil).
type fakeSemLookup struct{ available bool }

func (f *fakeSemLookup) Available() bool { return f.available }
func (f *fakeSemLookup) SymbolID(_ context.Context, _ workspace.WorkspaceKey, _ string, _, _ uint32) (integ.SymbolID, error) {
	return integ.SymbolID(""), integ.ErrIndexErrored
}
func (f *fakeSemLookup) RankFiles(_ context.Context, _ workspace.WorkspaceKey) ([]integ.RankedFile, error) {
	return nil, integ.ErrIndexErrored
}
func (f *fakeSemLookup) RankFromSeeds(_ context.Context, _ workspace.WorkspaceKey, _ []string) ([]integ.RankedFile, error) {
	return nil, integ.ErrIndexErrored
}
func (f *fakeSemLookup) ExpandFrom(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID, _ int) ([]integ.Impact, error) {
	return nil, integ.ErrIndexErrored
}
func (f *fakeSemLookup) ValidateCriticalEdges(_ context.Context, _ workspace.WorkspaceKey, edges []integ.Edge) ([]integ.ValidatedEdge, error) {
	return nil, integ.ErrIndexErrored
}
func (f *fakeSemLookup) Status(_ context.Context, _ workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	return integ.SemanticStatus{}, integ.ErrIndexErrored
}

// fakeCfgGate matches the production daemonCfgGate shape.
type fakeCfgGate struct{ enabled bool }

func (f *fakeCfgGate) SemanticIndexEnabled() bool { return f.enabled }

// TestGetHealth_SC1Preserved proves that adding the semantic_index block does
// NOT rename or alter the existing semantic_store block (Pitfall §4 + Phase 57
// SC-1 envelope shape preserved). When semIndex==nil, the semantic_index field
// is omitted (omitempty) and only semantic_store remains.
func TestGetHealth_SC1Preserved(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/sc1-preserved", Language: "go"}
	semStore := health.SemanticStoreStatus{State: "ready"}
	semIdx := health.ComputeSemanticIndexBlock(context.Background(), nil, ws)
	src, reason := integ.ChooseSource(&fakeCfgGate{enabled: false}, nil, nil)

	envBytes, err := health.BuildEnvelopeJSON(nil, semStore, semIdx, src, reason)
	if err != nil {
		t.Fatalf("BuildEnvelopeJSON: %v", err)
	}
	envStr := string(envBytes)

	if !strings.Contains(envStr, `"semantic_store":`) {
		t.Fatalf("semantic_store block missing — SC-1 broken: %s", envStr)
	}
	if !strings.Contains(envStr, `"state": "ready"`) {
		t.Fatalf("semantic_store.state=ready missing: %s", envStr)
	}
	if strings.Contains(envStr, `"semantic_index":`) {
		t.Fatalf("semantic_index should be omitted when accessor is nil: %s", envStr)
	}
}

// TestGetHealth_SemanticIndexBlock proves that when a SemanticIndexAccessor is
// wired and reports a fully-populated SemanticStatus, the envelope carries the
// semantic_index block with all eight SPEC §24.5 fields. INTEG-04.
func TestGetHealth_SemanticIndexBlock(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/idx-block", Language: "go"}
	acc := &fakeIndexAccessor{
		status: integ.SemanticStatus{
			State:            integ.StatusReady,
			Store:            "duckdb",
			LatestSnapshotID: 42,
			GraphVersion:     184,
			OverlayActive:    true,
			PendingLSP:       3,
			LastLiveUpdateMs: 1234,
			LastErrorReason:  "",
		},
	}
	semIdx := health.ComputeSemanticIndexBlock(context.Background(), acc, ws)
	src, reason := integ.ChooseSource(&fakeCfgGate{enabled: true}, &fakeSemLookup{available: true}, nil)

	envBytes, err := health.BuildEnvelopeJSON(nil, health.SemanticStoreStatus{State: "ready"}, semIdx, src, reason)
	if err != nil {
		t.Fatalf("BuildEnvelopeJSON: %v", err)
	}
	envStr := string(envBytes)

	if !strings.Contains(envStr, `"semantic_index":`) {
		t.Fatalf("semantic_index block missing: %s", envStr)
	}
	mustContain := []string{
		`"enabled": true`,
		`"store": "duckdb"`,
		`"latest_snapshot_status": "ready"`,
		`"graph_version": 184`,
		`"overlay_active": true`,
		`"pending_lsp_revalidations": 3`,
		`"last_live_update_ms": 1234`,
	}
	for _, sub := range mustContain {
		if !strings.Contains(envStr, sub) {
			t.Fatalf("envelope missing %q:\n%s", sub, envStr)
		}
	}
	if strings.Contains(envStr, `"last_error":`) {
		t.Fatalf("last_error should be omitted when LastErrorReason is empty: %s", envStr)
	}
	// semantic_store still present (additive evolution).
	if !strings.Contains(envStr, `"semantic_store":`) {
		t.Fatalf("semantic_store block missing — additive evolution broken: %s", envStr)
	}
}

// TestGetHealth_SemanticIndexBlock_Building proves StatusBuilding maps to
// latest_snapshot_status="building".
func TestGetHealth_SemanticIndexBlock_Building(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/idx-building", Language: "go"}
	acc := &fakeIndexAccessor{
		status: integ.SemanticStatus{State: integ.StatusBuilding, Store: "duckdb"},
	}
	semIdx := health.ComputeSemanticIndexBlock(context.Background(), acc, ws)
	if semIdx.LatestSnapshotStatus != "building" {
		t.Fatalf("LatestSnapshotStatus=%q, want building", semIdx.LatestSnapshotStatus)
	}
	if !semIdx.Enabled {
		t.Fatalf("Enabled=false, want true (StatusBuilding implies enabled)")
	}
}

// TestGetHealth_SemanticIndexBlock_LastErrorClosedEnum proves that
// LastErrorReason from the accessor is surfaced as the closed-enum
// FallbackReason value, not raw text. WR-NEW-01.
func TestGetHealth_SemanticIndexBlock_LastErrorClosedEnum(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/idx-err-enum", Language: "go"}
	acc := &fakeIndexAccessor{
		status: integ.SemanticStatus{
			State:           integ.StatusError,
			Store:           "duckdb",
			LastErrorReason: string(integ.FallbackReasonIndexError),
		},
	}
	semIdx := health.ComputeSemanticIndexBlock(context.Background(), acc, ws)
	if semIdx.LastError != "index_error" {
		t.Fatalf("LastError=%q, want %q", semIdx.LastError, "index_error")
	}
	if semIdx.LatestSnapshotStatus != "error" {
		t.Fatalf("LatestSnapshotStatus=%q, want error", semIdx.LatestSnapshotStatus)
	}
}

// TestComputeSemanticIndexBlock_NilAccessor proves passing nil yields an
// Enabled=false block which should be omitted from the envelope.
func TestComputeSemanticIndexBlock_NilAccessor(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/idx-nil", Language: "go"}
	blk := health.ComputeSemanticIndexBlock(context.Background(), nil, ws)
	if blk.Enabled {
		t.Fatalf("Enabled=true, want false (nil accessor)")
	}

	envBytes, err := health.BuildEnvelopeJSON(nil, health.SemanticStoreStatus{State: "disabled"}, blk, integ.SourceTreeSitter, "")
	if err != nil {
		t.Fatalf("BuildEnvelopeJSON: %v", err)
	}
	if strings.Contains(string(envBytes), `"semantic_index":`) {
		t.Fatalf("semantic_index should be omitted when Enabled=false: %s", envBytes)
	}
}

// TestComputeSemanticIndexBlock_AccessorError proves that an accessor returning
// a non-nil error is mapped to LatestSnapshotStatus="error" with the
// closed-enum LastError="index_error" — never raw error text. WR-NEW-01.
func TestComputeSemanticIndexBlock_AccessorError(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/idx-err", Language: "go"}
	acc := &fakeIndexAccessor{err: errors.New("connection refused")}
	blk := health.ComputeSemanticIndexBlock(context.Background(), acc, ws)
	if blk.LatestSnapshotStatus != "error" {
		t.Fatalf("LatestSnapshotStatus=%q, want error", blk.LatestSnapshotStatus)
	}
	if blk.LastError != "index_error" {
		t.Fatalf("LastError=%q, want index_error (closed enum)", blk.LastError)
	}
	if blk.LastError == "connection refused" {
		t.Fatalf("LastError leaked raw err text; closed enum violated")
	}
}

// TestGetHealth_TopLevelSourceField pins the INTEG-05 envelope-uniformity
// contract: every get_health envelope stamps a closed-enum top-level "source"
// field via integ.ChooseSource(cfgGate, lookup, nil). Three rows lock the
// priority ladder.
func TestGetHealth_TopLevelSourceField(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/src-row", Language: "go"}
	cases := []struct {
		name          string
		cfg           integ.ConfigGate
		lookup        integ.SemanticLookup
		wantSource    string
		wantReasonSub string // empty means "no fallback_reason key"
	}{
		{
			name:          "cfgDisabled",
			cfg:           &fakeCfgGate{enabled: false},
			lookup:        &fakeSemLookup{available: true},
			wantSource:    "tree_sitter",
			wantReasonSub: "",
		},
		{
			name:          "cfgEnabledLookupReady",
			cfg:           &fakeCfgGate{enabled: true},
			lookup:        &fakeSemLookup{available: true},
			wantSource:    "semantic",
			wantReasonSub: "",
		},
		{
			name:          "cfgEnabledLookupUnavail",
			cfg:           &fakeCfgGate{enabled: true},
			lookup:        &fakeSemLookup{available: false},
			wantSource:    "fallback",
			wantReasonSub: "index_disabled",
		},
	}
	allowed := map[string]bool{"semantic": true, "tree_sitter": true, "fallback": true}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, reason := integ.ChooseSource(tc.cfg, tc.lookup, nil)
			semIdx := health.ComputeSemanticIndexBlock(context.Background(), nil, ws)
			envBytes, err := health.BuildEnvelopeJSON(nil, health.SemanticStoreStatus{State: "ready"}, semIdx, src, reason)
			if err != nil {
				t.Fatalf("BuildEnvelopeJSON: %v", err)
			}
			envStr := string(envBytes)
			wantSrcField := `"source": "` + tc.wantSource + `"`
			if !strings.Contains(envStr, wantSrcField) {
				t.Fatalf("envelope missing %q:\n%s", wantSrcField, envStr)
			}
			if !allowed[tc.wantSource] {
				t.Fatalf("test row asserts non-closed-enum source %q", tc.wantSource)
			}
			if tc.wantReasonSub == "" {
				if strings.Contains(envStr, `"fallback_reason":`) {
					t.Fatalf("fallback_reason should be omitted when empty:\n%s", envStr)
				}
			} else {
				wantReasonField := `"fallback_reason": "` + tc.wantReasonSub + `"`
				if !strings.Contains(envStr, wantReasonField) {
					t.Fatalf("envelope missing %q:\n%s", wantReasonField, envStr)
				}
			}
		})
	}
}
