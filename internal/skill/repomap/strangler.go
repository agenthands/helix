// strangler.go — Phase 65 65-05 strangler-fig integration helpers.
//
// This file provides the consumer-side wiring between RepoMapSkill (the v1.9
// tree-sitter + PageRank tool surface) and the integ.SemanticLookup seam
// (the Phase 60–64 semantic engine, exposed through the daemon-side
// integSemanticLookup adapter).
//
// Doctrine (matches PLAN must-haves):
//
//   - Zero source change to internal/repomap engine (INTEG-01). The adapter
//     translates integ.RankedFile → repomap.RankedFile so the existing
//     TreeRenderer.RenderBudgeted call site stays untouched.
//   - Sort-before-iterate (Phase 62 CR-03 / S4): adaptRankedFiles sorts by
//     score desc, tiebreak Path asc, regardless of input order. The renderer
//     trusts the sort; the daemon-side production lookup happens to return
//     sorted slices today, but the adapter MUST not depend on that.
//   - Pure functions only — adapter and freshness helper perform no I/O,
//     never trigger indexing. Threat T-65-05-04 (M-readtier) disposition is
//     "mitigate" because of this purity plus the M-readtier grep canary on
//     the daemon-side integSemanticLookup methods.
package repomap

import (
	"context"
	"sort"

	"github.com/agenthands/helix/internal/repomap"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// adaptRankedFiles converts integ-shape RankedFile slices into the renderer's
// expected v1.9 shape. The function applies the Phase 62 CR-03 / S4 doctrine
// (sort by score desc, tiebreak Path asc) before the slice ever reaches the
// renderer — even if the daemon-side lookup already returned sorted results.
//
// Rationale: every Phase 65 consumer (RepoMapSkill, kernel/symbols blast-
// radius bridge in 65-06, kernel/health bridge in 65-07) flows through this
// adapter. Centralizing the sort here means a future ranked source that
// returns unsorted slices cannot silently leak nondeterministic order into
// the renderer.
//
// INTEG-01: this is the only adapter; internal/repomap stays untouched.
func adaptRankedFiles(ranked []integ.RankedFile) []repomap.RankedFile {
	if len(ranked) == 0 {
		return nil
	}
	// Copy to avoid mutating the caller's slice (the daemon-side lookup may
	// reuse its slice across calls).
	cp := make([]integ.RankedFile, len(ranked))
	copy(cp, ranked)
	sort.SliceStable(cp, func(i, j int) bool {
		if cp[i].Score != cp[j].Score {
			return cp[i].Score > cp[j].Score
		}
		return cp[i].Path < cp[j].Path
	})
	out := make([]repomap.RankedFile, 0, len(cp))
	for _, r := range cp {
		out = append(out, repomap.RankedFile{Path: r.Path, Score: r.Score})
	}
	return out
}

// computeFreshness derives the closed-enum freshness marker from a live
// SemanticStatus. Returns "" when the lookup is nil / unavailable / errors —
// the envelope omits the field via omitempty.
//
// Closed-enum mapping (mirrors Phase 64 SPEC §26.2):
//
//   - overlay active + pending LSP        → "structurally_fresh_semantically_pending"
//   - overlay active alone                → "overlay"
//   - pending LSP alone                   → "pending_lsp"
//   - both clear, snapshot present        → "fresh"
//   - any error / unavailable             → "" (omitted on the wire)
func computeFreshness(ctx context.Context, lookup integ.SemanticLookup, ws workspace.WorkspaceKey) string {
	if lookup == nil || !lookup.Available() {
		return ""
	}
	st, err := lookup.Status(ctx, ws)
	if err != nil {
		return ""
	}
	switch {
	case st.OverlayActive && st.PendingLSP > 0:
		return "structurally_fresh_semantically_pending"
	case st.OverlayActive:
		return "overlay"
	case st.PendingLSP > 0:
		return "pending_lsp"
	default:
		return "fresh"
	}
}
