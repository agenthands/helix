// source_select_test.go — table-driven matrix for ChooseSource (Pitfall §3
// priority ladder + D-04 + D-05). 65-05/06/07 reuse this primitive on every
// envelope; the matrix here is the contract.
package integ

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agenthands/helix/internal/workspace"
)

// fakeCfg is a tiny test double for ConfigGate.
type fakeCfg struct{ enabled bool }

func (f fakeCfg) SemanticIndexEnabled() bool { return f.enabled }

// availableLookup is a SemanticLookup whose Available() returns true. All
// other methods panic — ChooseSource never invokes them.
type availableLookup struct{}

func (availableLookup) Available() bool { return true }
func (availableLookup) SymbolID(_ context.Context, _ workspace.WorkspaceKey, _ string, _, _ uint32) (SymbolID, error) {
	panic("availableLookup.SymbolID not used by ChooseSource")
}
func (availableLookup) RankFiles(_ context.Context, _ workspace.WorkspaceKey) ([]RankedFile, error) {
	panic("availableLookup.RankFiles not used by ChooseSource")
}
func (availableLookup) RankFromSeeds(_ context.Context, _ workspace.WorkspaceKey, _ []string) ([]RankedFile, error) {
	panic("availableLookup.RankFromSeeds not used by ChooseSource")
}
func (availableLookup) ExpandFrom(_ context.Context, _ workspace.WorkspaceKey, _ SymbolID, _ int) ([]Impact, error) {
	panic("availableLookup.ExpandFrom not used by ChooseSource")
}
func (availableLookup) ValidateCriticalEdges(_ context.Context, _ workspace.WorkspaceKey, _ []Edge) ([]ValidatedEdge, error) {
	panic("availableLookup.ValidateCriticalEdges not used by ChooseSource")
}
func (availableLookup) Status(_ context.Context, _ workspace.WorkspaceKey) (SemanticStatus, error) {
	panic("availableLookup.Status not used by ChooseSource")
}

// TestChooseSource_PriorityLadder asserts the Pitfall §3 priority ladder:
//
//  1. config disabled         → SourceTreeSitter, "" (D-04, takes priority over lookup state)
//  2. lookup nil / unavailable → SourceFallback, FallbackReasonIndexDisabled (defensive D-05)
//  3. err != nil               → SourceFallback, ClassifyLookupErr(err)
//  4. else                     → SourceSemantic, ""
//
// Every closed-enum FallbackReason value is exercised through the err arm of
// the ladder.
func TestChooseSource_PriorityLadder(t *testing.T) {
	type tc struct {
		name       string
		cfg        ConfigGate
		lookup     SemanticLookup
		err        error
		wantSource Source
		wantReason FallbackReason
	}

	cases := []tc{
		{
			name:       "config disabled overrides everything (NoopLookup, no err)",
			cfg:        fakeCfg{enabled: false},
			lookup:     NoopLookup{},
			err:        nil,
			wantSource: SourceTreeSitter,
			wantReason: "",
		},
		{
			name:       "config disabled overrides everything (NoopLookup, with err)",
			cfg:        fakeCfg{enabled: false},
			lookup:     NoopLookup{},
			err:        ErrIndexBuilding,
			wantSource: SourceTreeSitter,
			wantReason: "",
		},
		{
			name:       "config disabled with available lookup still emits tree_sitter",
			cfg:        fakeCfg{enabled: false},
			lookup:     availableLookup{},
			err:        nil,
			wantSource: SourceTreeSitter,
			wantReason: "",
		},
		{
			name:       "nil ConfigGate treated as disabled",
			cfg:        nil,
			lookup:     availableLookup{},
			err:        nil,
			wantSource: SourceTreeSitter,
			wantReason: "",
		},
		{
			name:       "enabled, lookup nil → defensive index_disabled",
			cfg:        fakeCfg{enabled: true},
			lookup:     nil,
			err:        nil,
			wantSource: SourceFallback,
			wantReason: FallbackReasonIndexDisabled,
		},
		{
			name:       "enabled, NoopLookup → defensive index_disabled",
			cfg:        fakeCfg{enabled: true},
			lookup:     NoopLookup{},
			err:        nil,
			wantSource: SourceFallback,
			wantReason: FallbackReasonIndexDisabled,
		},
		{
			name:       "enabled, available, ErrNoSnapshot → no_snapshot_yet",
			cfg:        fakeCfg{enabled: true},
			lookup:     availableLookup{},
			err:        ErrNoSnapshot,
			wantSource: SourceFallback,
			wantReason: FallbackReasonNoSnapshotYet,
		},
		{
			name:       "enabled, available, ErrIndexBuilding → index_building",
			cfg:        fakeCfg{enabled: true},
			lookup:     availableLookup{},
			err:        ErrIndexBuilding,
			wantSource: SourceFallback,
			wantReason: FallbackReasonIndexBuilding,
		},
		{
			name:       "enabled, available, ErrIndexErrored → index_error",
			cfg:        fakeCfg{enabled: true},
			lookup:     availableLookup{},
			err:        ErrIndexErrored,
			wantSource: SourceFallback,
			wantReason: FallbackReasonIndexError,
		},
		{
			name:       "enabled, available, ErrBleveRebuilding → bleve_rebuilding",
			cfg:        fakeCfg{enabled: true},
			lookup:     availableLookup{},
			err:        ErrBleveRebuilding,
			wantSource: SourceFallback,
			wantReason: FallbackReasonBleveRebuilding,
		},
		{
			name:       "enabled, available, random err → index_error (default classifier branch)",
			cfg:        fakeCfg{enabled: true},
			lookup:     availableLookup{},
			err:        errors.New("random failure"),
			wantSource: SourceFallback,
			wantReason: FallbackReasonIndexError,
		},
		{
			name:       "enabled, available, wrapped sentinel → classifier follows errors.Is",
			cfg:        fakeCfg{enabled: true},
			lookup:     availableLookup{},
			err:        fmt.Errorf("upstream: %w", ErrNoSnapshot),
			wantSource: SourceFallback,
			wantReason: FallbackReasonNoSnapshotYet,
		},
		{
			name:       "enabled, available, no err → SourceSemantic",
			cfg:        fakeCfg{enabled: true},
			lookup:     availableLookup{},
			err:        nil,
			wantSource: SourceSemantic,
			wantReason: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotSrc, gotReason := ChooseSource(c.cfg, c.lookup, c.err)
			if gotSrc != c.wantSource {
				t.Errorf("Source: got %q, want %q", gotSrc, c.wantSource)
			}
			if gotReason != c.wantReason {
				t.Errorf("FallbackReason: got %q, want %q", gotReason, c.wantReason)
			}
		})
	}
}

// TestChooseSource_NoRawErrText asserts ChooseSource never inspects err.Error()
// — the only path from err to FallbackReason is ClassifyLookupErr (errors.Is).
// This is enforced by source_test.go for the classifier; here we belt-and-
// braces it at the ChooseSource boundary by passing an error whose Error()
// string mimics a sentinel name and verifying we DON'T spuriously match.
func TestChooseSource_NoRawErrText(t *testing.T) {
	mimic := errors.New("integ: index build in progress") // text identical to ErrIndexBuilding.Error()
	cfg := fakeCfg{enabled: true}
	src, reason := ChooseSource(cfg, availableLookup{}, mimic)
	if src != SourceFallback {
		t.Fatalf("Source: got %q, want %q", src, SourceFallback)
	}
	// The mimic is NOT errors.Is the sentinel; classifier defaults to
	// FallbackReasonIndexError. If this ever flips to FallbackReasonIndexBuilding
	// then someone introduced a string comparison — WR-NEW-01 violation.
	if reason != FallbackReasonIndexError {
		t.Fatalf("FallbackReason: got %q, want %q (WR-NEW-01: classifier must use errors.Is, not text match)", reason, FallbackReasonIndexError)
	}
}
