// strangler_test.go — Phase 65 65-05 RED tests for SetSemanticLookup wiring,
// JSON envelope, and source-selection priority ladder.
//
// Test matrix (mirrors the 65-05 PLAN <behavior> block):
//
//  1. TestRepoMap_SetSemanticLookup_Wired — semantic-on success path.
//  2. TestRepoMap_LookupErrFallsBack — lookup err → SourceFallback path.
//  3. TestRepoMap_LookupNotWired_TreeSitterPath — config-disabled →
//     SourceTreeSitter (D-04 / Pitfall §3).
//  4. TestGetContext_Semantic — RankFromSeeds delegation.
//
// Tests use a stub fakeLookup (test double) to drive the lookup arm without
// pulling in the daemon-side production adapter.
package repomap

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/repomap"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/treesitter"
	"github.com/agenthands/helix/internal/workspace"
)

// fakeLookup is a hand-rolled SemanticLookup test double used by 65-05 RED
// tests. It returns canned RankFiles / RankFromSeeds slices, an injectable
// error, and a stable Status. The fakeLookup is M-readtier safe by
// construction: it never touches the snapshot-write surface.
type fakeLookup struct {
	available  bool
	ranked     []integ.RankedFile
	rankSeeded []integ.RankedFile
	err        error
	status     integ.SemanticStatus
}

func (f *fakeLookup) Available() bool { return f.available }
func (f *fakeLookup) SymbolID(_ context.Context, _ workspace.WorkspaceKey, _ string, _, _ uint32) (integ.SymbolID, error) {
	return "", integ.ErrNoSnapshot
}
func (f *fakeLookup) RankFiles(_ context.Context, _ workspace.WorkspaceKey) ([]integ.RankedFile, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.ranked, nil
}
func (f *fakeLookup) RankFromSeeds(_ context.Context, _ workspace.WorkspaceKey, _ []string) ([]integ.RankedFile, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rankSeeded, nil
}
func (f *fakeLookup) ExpandFrom(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID, _ int) ([]integ.Impact, error) {
	return nil, nil
}
func (f *fakeLookup) ValidateCriticalEdges(_ context.Context, _ workspace.WorkspaceKey, edges []integ.Edge) ([]integ.ValidatedEdge, error) {
	out := make([]integ.ValidatedEdge, 0, len(edges))
	for _, e := range edges {
		out = append(out, integ.ValidatedEdge{Edge: e})
	}
	return out, nil
}
func (f *fakeLookup) Status(_ context.Context, _ workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	return f.status, nil
}
func (f *fakeLookup) LocateSymbol(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (string, uint32, uint32, bool, error) {
	// Phase 65 65-12 Task 1: repomap tests never drive the kernel-side
	// blast-radius LSP probe; surface a clean miss.
	return "", 0, 0, false, nil
}
func (f *fakeLookup) Visibility(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (integ.Visibility, error) {
	return integ.VisUnknown, nil
}
func (f *fakeLookup) IsEntrypointReachable(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID) (bool, error) {
	return false, nil
}

// fakeCfg is the test ConfigGate double (matches the production daemonCfgGate
// shape). semanticIndexEnabled drives the priority-ladder gate.
type fakeCfg struct {
	enabled bool
}

func (f *fakeCfg) SemanticIndexEnabled() bool { return f.enabled }

// envelopeFields captures the on-wire envelope shape parsed from a tool
// result. Tests assert against these closed-enum strings rather than against
// the Go-level closed-enum types so a wire-shape regression is caught.
type envelopeFields struct {
	Source         string `json:"source"`
	FallbackReason string `json:"fallback_reason,omitempty"`
	GraphVersion   uint64 `json:"graph_version,omitempty"`
	Freshness      string `json:"freshness,omitempty"`
	Tree           string `json:"tree"`
}

// newStranglerTestSkill builds a RepoMapSkill with a real workspace + cache
// (so the v1.9 tree-sitter path renders real output) plus knobs for the
// lookup + cfg gate. The skill structure mirrors newIntegrationSkill.
func newStranglerTestSkill(t *testing.T) (*RepoMapSkill, string) {
	t.Helper()
	dir := t.TempDir()

	// Write Go source files; reuse the integration-test fixture shape.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.go"), []byte(`package main

type Server struct{ port int }
func NewServer(p int) *Server { return &Server{port: p} }
func (s *Server) Start() error { return nil }
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "handler.go"), []byte(`package main

import "fmt"

func HandleRequest(s *Server) {
	fmt.Println("handling request")
	s.Start()
}
`), 0o644))

	dbPath := filepath.Join(dir, ".cache", "tags.db")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))
	cache, err := repomap.NewTagCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { cache.Close() })

	registry := treesitter.NewGrammarRegistry()
	elider := repomap.NewElisionRenderer(registry)
	extractor, err := repomap.NewTagExtractor(registry)
	require.NoError(t, err)
	t.Cleanup(func() { extractor.Close() })

	s := &RepoMapSkill{
		cache:     cache,
		extractor: extractor,
		elider:    elider,
		logger:    slog.Default(),
		rootDir:   dir,
	}
	return s, dir
}

// TestRepoMap_SetSemanticLookup_Wired exercises the semantic-on success path.
// The fake lookup reports Available()==true and returns 3 known RankedFile
// entries; the JSON envelope must report source=="semantic" with the three
// paths visible in env.Tree.
func TestRepoMap_SetSemanticLookup_Wired(t *testing.T) {
	s, dir := newStranglerTestSkill(t)

	lookup := &fakeLookup{
		available: true,
		ranked: []integ.RankedFile{
			{Path: filepath.Join(dir, "server.go"), Score: 1.0, Projection: "call_graph", GraphVersion: 7},
			{Path: filepath.Join(dir, "handler.go"), Score: 0.5, Projection: "call_graph", GraphVersion: 7},
		},
		status: integ.SemanticStatus{State: integ.StatusReady, Store: "duckdb", LatestSnapshotID: 42, GraphVersion: 7},
	}
	s.SetSemanticLookup(lookup)
	s.SetConfigGate(&fakeCfg{enabled: true})

	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{
		"token_budget": float64(4096),
	})
	require.NoError(t, err)

	var env envelopeFields
	require.NoError(t, json.Unmarshal([]byte(result), &env), "result must be a JSON envelope: %q", result)
	assert.Equal(t, "semantic", env.Source, "semantic-on path must stamp source=semantic")
	assert.Equal(t, uint64(7), env.GraphVersion, "graph_version must reflect lookup.Status")
	assert.Empty(t, env.FallbackReason, "success path must not set fallback_reason")
	assert.Contains(t, env.Tree, "server.go", "tree must include the ranked paths")
	assert.Contains(t, env.Tree, "handler.go")
}

// TestRepoMap_LookupErrFallsBack exercises the err → SourceFallback path.
// fakeLookup returns Available()==true but RankFiles errors with
// ErrIndexBuilding. The skill must consult ChooseSource, classify the err
// via ClassifyLookupErr, render the v1.9 path, and stamp
// source=="fallback" + fallback_reason=="index_building".
func TestRepoMap_LookupErrFallsBack(t *testing.T) {
	s, _ := newStranglerTestSkill(t)

	lookup := &fakeLookup{
		available: true,
		err:       integ.ErrIndexBuilding,
	}
	s.SetSemanticLookup(lookup)
	s.SetConfigGate(&fakeCfg{enabled: true})

	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{})
	require.NoError(t, err)

	var env envelopeFields
	require.NoError(t, json.Unmarshal([]byte(result), &env), "result must be a JSON envelope: %q", result)
	assert.Equal(t, "fallback", env.Source, "lookup err must trigger SourceFallback")
	assert.Equal(t, "index_building", env.FallbackReason, "ClassifyLookupErr(ErrIndexBuilding)")
	assert.Contains(t, env.Tree, ".go", "fallback tree text must come from v1.9 path")
}

// TestRepoMap_LookupNotWired_TreeSitterPath exercises the config-off /
// pre-wiring path. cfg.SemanticIndexEnabled()==false MUST emit
// source=="tree_sitter" with no fallback_reason (D-04 / Pitfall §3).
func TestRepoMap_LookupNotWired_TreeSitterPath(t *testing.T) {
	s, _ := newStranglerTestSkill(t)

	// No SetSemanticLookup; cfg gate reports disabled.
	s.SetConfigGate(&fakeCfg{enabled: false})

	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{})
	require.NoError(t, err)

	var env envelopeFields
	require.NoError(t, json.Unmarshal([]byte(result), &env), "result must be a JSON envelope: %q", result)
	assert.Equal(t, "tree_sitter", env.Source, "config-disabled must emit tree_sitter (D-04 / Pitfall §3)")
	assert.Empty(t, env.FallbackReason, "tree_sitter steady state has no fallback_reason")
	assert.Contains(t, env.Tree, ".go", "tree must come from v1.9 path")
}

// TestGetContext_Semantic exercises the RankFromSeeds delegation. The fake
// returns a canned ranked-from-seeds slice; the envelope must report
// source=="semantic" and the tree must contain the seeded files.
func TestGetContext_Semantic(t *testing.T) {
	s, dir := newStranglerTestSkill(t)

	seedA := filepath.Join(dir, "server.go")
	seedB := filepath.Join(dir, "handler.go")
	lookup := &fakeLookup{
		available: true,
		rankSeeded: []integ.RankedFile{
			{Path: seedA, Score: 1.0, Projection: "call_graph", GraphVersion: 11},
			{Path: seedB, Score: 0.7, Projection: "call_graph", GraphVersion: 11},
		},
		status: integ.SemanticStatus{State: integ.StatusReady, Store: "duckdb", GraphVersion: 11},
	}
	s.SetSemanticLookup(lookup)
	s.SetConfigGate(&fakeCfg{enabled: true})

	result, err := s.ExecuteTool("get_context", map[string]interface{}{
		"files":        []interface{}{seedA, seedB},
		"token_budget": float64(4096),
	})
	require.NoError(t, err)

	var env envelopeFields
	require.NoError(t, json.Unmarshal([]byte(result), &env), "result must be a JSON envelope: %q", result)
	assert.Equal(t, "semantic", env.Source, "RankFromSeeds delegation must stamp source=semantic")
	assert.Equal(t, uint64(11), env.GraphVersion, "graph_version must reflect lookup.Status")
	assert.Contains(t, env.Tree, "server.go", "tree must include the seeded paths")
	assert.Contains(t, env.Tree, "handler.go")
}

// TestRepoMap_LookupNotWired_ButSemanticEnabled_DefensiveDisabled exercises
// the defensive D-05 path: cfg says enabled, but SetSemanticLookup was never
// called, so lookup() returns NoopLookup{} which reports Available()==false.
// The envelope must report source=="fallback" + fallback_reason=="index_disabled"
// per the priority ladder's defensive arm.
func TestRepoMap_LookupNotWired_ButSemanticEnabled_DefensiveDisabled(t *testing.T) {
	s, _ := newStranglerTestSkill(t)

	// SetConfigGate enabled but no SetSemanticLookup (never wired).
	s.SetConfigGate(&fakeCfg{enabled: true})

	result, err := s.ExecuteTool("get_repo_map", map[string]interface{}{})
	require.NoError(t, err)

	var env envelopeFields
	require.NoError(t, json.Unmarshal([]byte(result), &env), "result must be a JSON envelope: %q", result)
	assert.Equal(t, "fallback", env.Source, "defensive D-05: cfg enabled + lookup unwired → SourceFallback")
	assert.Equal(t, "index_disabled", env.FallbackReason, "defensive arm uses index_disabled reason")
	assert.Contains(t, env.Tree, ".go", "fallback tree text must come from v1.9 path")
}

// TestAdaptRankedFiles_SortsByScoreDescThenPathAsc verifies the
// adaptRankedFiles adapter maintains the sort-before-iterate doctrine
// (Phase 62 CR-03 / S4): score desc, tiebreak Path asc.
func TestAdaptRankedFiles_SortsByScoreDescThenPathAsc(t *testing.T) {
	in := []integ.RankedFile{
		{Path: "z.go", Score: 0.5},
		{Path: "a.go", Score: 0.8},
		{Path: "m.go", Score: 0.5},
		{Path: "b.go", Score: 0.9},
	}
	out := adaptRankedFiles(in)
	require.Len(t, out, 4)
	assert.Equal(t, "b.go", out[0].Path, "highest score wins")
	assert.Equal(t, 0.9, out[0].Score)
	assert.Equal(t, "a.go", out[1].Path)
	// Tied at 0.5 → Path asc: m.go, then z.go.
	assert.Equal(t, "m.go", out[2].Path, "score tie breaks by Path asc")
	assert.Equal(t, "z.go", out[3].Path)
}

// sentinel: ensure we exercised every error-classification branch the skill
// depends on.
var _ = errors.Is
