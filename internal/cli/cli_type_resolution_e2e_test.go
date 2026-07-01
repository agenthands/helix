//go:build !windows
// +build !windows

package cli_test

// Phase 137 (v2.12) — real-binary C-family type-resolution E2E oracle.
//
// This is the milestone headline: prove, through a REAL `helix` binary driven
// as a subprocess against a live daemon, that a C var→type reference resolves
// to a queryable `has_type` edge. Phase 136 proved the same RESOLVES_TO edge
// in-process (internal/daemon.TestResolveTypeEdges_PositiveCommitsRealEdge:
// exactly one RESOLVES_TO p→Foo); this file proves it end-to-end through the
// shipped CLI surface, where RESOLVES_TO surfaces to agents as `has_type`
// (MapInternalKind, edge_kind_surface.go).
//
// It reuses the HELIX_BIN-gated harness (cli_e2e_test.go): newE2EFixture stands
// up an isolated sandbox + real daemon and t.Skips when no binary is available.
//
// MODE-GATE MECHANISM (resolved empirically — 137 Task 2):
//   - `index_semantic_graph` is review+ (mode_check.go modeTierReview) and is
//     excluded from the default `edit` mode (profile/modes/edit.yaml).
//   - The daemon keeps ONE shared session (daemon.go daemonSessionProvider), so
//     session mode is GLOBAL daemon state — a mode switch on any connection is
//     observed by every later CLI subprocess dialing the same socket.
//   - The flat `switch-mode` CLI verb carries NO target_mode flag
//     (verbs_gen.go: flags nil), so the mode switch itself is issued over the
//     MCP path (forwarder.CallTool switch_mode → target_mode="review"); this is
//     Phase-137 CONTEXT option (b). switch_mode is always-allowed
//     (profile_enforce.go alwaysAllowedCoreTools) and independently validated by
//     validateModeTransition, so it does not weaken authz.
//   - After the switch, BOTH `helix index-semantic-graph --mode-arg=full` and
//     the HEADLINE `helix explain-symbol-deep` run as REAL subprocesses against
//     the (now review-mode) shared session. The headline read (explain, read+)
//     holds in any mode; only the index build needed the elevation.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/forwarder"
)

// cTypeSeed is a SINGLE C translation unit carrying both differential arms:
//   - `struct Foo` + a struct-typed parameter `p` → the POSITIVE case: the
//     var→type link (Phase 135 linkVarTypes) drives a RESOLVES_TO p→Foo edge,
//     surfaced as `has_type`.
//   - a primitive-typed parameter `n` → the NEGATIVE case: no named type, no
//     link, no `has_type` edge (anti-vacuity).
//
// def + use live in ONE file → flat C translation-unit scope (SameScope true) →
// no cross-scope cap → a validated edge (matches the Phase-136 fixture shape).
const cTypeSeed = `struct Foo { int a; };
int use(struct Foo *p) { return p->a; }
int prim(int n) { return n; }
`

// explainDeepResult is the has_type-relevant subset of the daemon's
// ExplainSymbolDeepResult JSON (internal/skill/semantic.ExplainSymbolDeepResult).
// The test parses the REAL subprocess stdout structurally rather than importing
// the daemon package into this external test.
type explainDeepResult struct {
	Seed struct {
		SymbolID   string `json:"symbol_id"`
		Resolution string `json:"resolution"`
	} `json:"seed"`
	EdgesIncoming  []explainEdge `json:"edges_incoming"`
	EdgesOutgoing  []explainEdge `json:"edges_outgoing"`
	FallbackReason string        `json:"fallback_reason"`
}

type explainEdge struct {
	From         string `json:"from"`
	To           string `json:"to"`
	EdgeKind     string `json:"edge_kind"`
	InternalKind string `json:"internal_kind"`
}

// newCTypeE2EFixture mirrors newE2EFixture (shared sandbox + real-daemon
// bringup, HELIX_BIN-gated) but seeds a single C file instead of the default
// main.go. It returns the live fixture plus the ABSOLUTE seed path — the daemon
// stores file paths verbatim from the full-walk (semantic_files.path is
// absolute), so the explain-symbol-deep seed's file_path must be absolute to
// resolve (a relative path returns resolution="not_found").
func newCTypeE2EFixture(t *testing.T, runID, filename, content string) (*e2eFixture, string) {
	t.Helper()
	f := newE2EFixture(t, runID)
	// Overwrite the default main.go seed with the C fixture, and remove main.go
	// so only the .c file is present when the workspace is walked (mirrors the
	// seedChainFixture overwrite pattern).
	seed := filepath.Join(f.repoDir, filename)
	if err := os.WriteFile(seed, []byte(content), 0o600); err != nil {
		t.Fatalf("seed C fixture: %v", err)
	}
	_ = os.Remove(filepath.Join(f.repoDir, "main.go"))
	return f, seed
}

// switchModeReview flips the shared daemon session to review mode over the MCP
// path (the flat switch-mode CLI verb carries no target_mode flag). See the
// MODE-GATE MECHANISM note at the top of this file.
func (f *e2eFixture) switchModeReview(t *testing.T, ctx context.Context) {
	t.Helper()
	res, err := forwarder.CallTool(ctx, f.socket, "", f.logger, "cli-type-e2e",
		"switch_mode", map[string]any{"target_mode": "review"})
	if err != nil {
		t.Fatalf("switch_mode(review): %v", err)
	}
	if res.IsError {
		t.Fatalf("switch_mode(review) reported error: %s", toolText(res))
	}
}

// indexFull runs `helix index-semantic-graph --mode-arg=full` as a REAL
// subprocess against the fixture daemon (review mode already set) and fails if
// the build does not commit a snapshot.
func (f *e2eFixture) indexFull(t *testing.T, ctx context.Context) {
	t.Helper()
	out, err := f.runCLIVerbInDir(ctx, f.repoDir, "index-semantic-graph", "--mode-arg=full")
	if err != nil {
		t.Fatalf("real `helix index-semantic-graph` failed: %v\noutput:\n%s", err, out)
	}
	var idx struct {
		Status  string `json:"status"`
		Partial bool   `json:"partial"`
	}
	if err := json.Unmarshal([]byte(out), &idx); err != nil {
		t.Fatalf("parse index result: %v\noutput:\n%s", err, out)
	}
	if idx.Status != "committed" || idx.Partial {
		t.Fatalf("index did not commit: status=%q partial=%v\noutput:\n%s", idx.Status, idx.Partial, out)
	}
}

// explainSymbol runs `helix explain-symbol-deep --seed-json=...` as a REAL
// subprocess (dir=repoDir) and parses the verbatim JSON stdout (explain is a
// classOpaque verb — the CLI passes the daemon envelope through unchanged).
func (f *e2eFixture) explainSymbol(t *testing.T, ctx context.Context, absPath, name string) explainDeepResult {
	t.Helper()
	seedJSON := fmt.Sprintf(`{"file_path":%q,"symbol_name":%q}`, absPath, name)
	out, err := f.runCLIVerbInDir(ctx, f.repoDir, "explain-symbol-deep", "--seed-json="+seedJSON)
	if err != nil {
		t.Fatalf("real `helix explain-symbol-deep` (%s) failed: %v\noutput:\n%s", name, err, out)
	}
	var res explainDeepResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("parse explain result (%s): %v\noutput:\n%s", name, err, out)
	}
	return res
}

// hasTypeEdges returns the `has_type` edges (surfaced RESOLVES_TO) present in
// the union of the symbol's incoming + outgoing edges. The production emit
// direction is ref→type (p→Foo, an OUTGOING edge from p), but the helper scans
// both directions so the assertion does not silently pass on a direction flip.
func hasTypeEdges(res explainDeepResult) []explainEdge {
	var out []explainEdge
	for _, e := range append(append([]explainEdge{}, res.EdgesOutgoing...), res.EdgesIncoming...) {
		if e.EdgeKind == "has_type" {
			out = append(out, e)
		}
	}
	return out
}

// TestCLI_E2E_CTypeResolution is the Phase-137 headline oracle (SC4 + anti-
// vacuity + determinism), all through the REAL `helix` binary.
func TestCLI_E2E_CTypeResolution(t *testing.T) {
	f, seed := newCTypeE2EFixture(t, "ctype-e2e", "types.c", cTypeSeed)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Activate the workspace, elevate to review, then build a full snapshot via
	// the REAL index subprocess.
	f.mcpActivate(t, ctx)
	f.switchModeReview(t, ctx)
	f.indexFull(t, ctx)

	// --- HEADLINE (assert FIRST): the positive fixture yields a has_type edge
	//     via the real `helix explain-symbol-deep` (SC4). ---
	pos := f.explainSymbol(t, ctx, seed, "p")
	if pos.Seed.Resolution != "exact" {
		t.Fatalf("positive seed p: resolution=%q fallback=%q, want exact (indexed symbol)",
			pos.Seed.Resolution, pos.FallbackReason)
	}
	posEdges := hasTypeEdges(pos)
	if len(posEdges) != 1 {
		t.Fatalf("HEADLINE: real `helix explain-symbol-deep` on p returned %d has_type edges, want EXACTLY 1: %+v\nfull outgoing=%+v",
			len(posEdges), posEdges, pos.EdgesOutgoing)
	}
	// The has_type edge must be the production direction (p→Foo) and target Foo.
	edge := posEdges[0]
	if edge.InternalKind != "RESOLVES_TO" {
		t.Errorf("has_type edge internal_kind=%q, want RESOLVES_TO (Pitfall 2)", edge.InternalKind)
	}
	if edge.From != pos.Seed.SymbolID {
		t.Errorf("has_type edge from=%q, want the seed symbol p (%q) — production direction is ref→type",
			edge.From, pos.Seed.SymbolID)
	}
	if !containsToken(edge.To, "Foo") || !containsToken(edge.To, "struct") {
		t.Errorf("has_type edge target=%q, want Foo's struct symbol", edge.To)
	}

	// --- DIFFERENTIAL anti-vacuity: the negative fixture (primitive-typed
	//     param n) yields NO has_type edge, while still resolving exact (so the
	//     zero is a real absence, not a missing symbol). ---
	neg := f.explainSymbol(t, ctx, seed, "n")
	if neg.Seed.Resolution != "exact" {
		t.Fatalf("negative seed n: resolution=%q fallback=%q, want exact (n IS indexed — the zero must be non-vacuous)",
			neg.Seed.Resolution, neg.FallbackReason)
	}
	if negEdges := hasTypeEdges(neg); len(negEdges) != 0 {
		t.Fatalf("differential anti-vacuity: primitive param n returned %d has_type edges, want 0: %+v",
			len(negEdges), negEdges)
	}

	// --- DETERMINISM (SC6): re-index the same fixture via a second REAL index
	//     subprocess and assert the has_type edge set for p is identical
	//     (same count, same target). ---
	f.indexFull(t, ctx)
	pos2 := f.explainSymbol(t, ctx, seed, "p")
	pos2Edges := hasTypeEdges(pos2)
	if len(pos2Edges) != len(posEdges) {
		t.Fatalf("determinism: has_type edge count changed across re-index: %d vs %d", len(posEdges), len(pos2Edges))
	}
	if pos2Edges[0].To != edge.To || pos2Edges[0].From != edge.From {
		t.Fatalf("determinism: has_type edge changed across re-index:\n first=%+v\n second=%+v", edge, pos2Edges[0])
	}
}

// containsToken reports whether s contains the substring tok. The stable-key
// symbol id is NUL-delimited (\u0000field\u0000field...), so the type name /
// kind appear as embedded tokens; a substring check is sufficient and avoids
// coupling the test to the exact stable-key layout.
func containsToken(s, tok string) bool {
	for i := 0; i+len(tok) <= len(s); i++ {
		if s[i:i+len(tok)] == tok {
			return true
		}
	}
	return false
}
