package semantic

import (
	"context"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/retrieval"
)

// INVARIANT (D-09 / D-13): find_related_symbols MUST NOT touch the snapshot-
// write surface of *Store. Specifically: no Begin/Commit/Abort/Write methods
// on snapshots, and no compactor flush trigger. The grep gate in CI enforces
// the absence of those identifier tokens in this file; the recorder mocks in
// tools_find_related_test.go enforce it under unit test. Read+ stays
// read-only with respect to committed state — find_related only consumes
// PageRank + RRF over the existing committed snapshot + overlay state via
// narrow accessors.

// D2 surface limits and ranking constants (71-CONTEXT.md):
//   - default k = 20
//   - clamp k to [1, 100]
//   - cluster boost multiplier is a deterministic constant (Pitfall 6:
//     single-goroutine post-fuse pass).
const (
	findRelatedDefaultK    = 20
	findRelatedMinK        = 1
	findRelatedMaxK        = 100
	clusterBoostMultiplier = 1.25 // Pitfall 3 + 6 — applied deterministically post-fuse.
)

// FindRelatedSymbolsArgs is the typed-args input schema for find_related_symbols.
type FindRelatedSymbolsArgs struct {
	Seed  SeedInput `json:"seed"            jsonschema:"seed symbol — symbol_id OR (file_path AND symbol_name)"`
	K     int       `json:"k,omitempty"     jsonschema:"top-N cap; default 20; clamped to [1, 100]"`
	Paths []string  `json:"paths,omitempty" jsonschema:"strict-subset filter on RESULT defining files; the seed itself is NOT constrained"`
}

// RelatedResult is one ranked sibling-symbol entry in the response.
//
// ClusterBoostApplied surfaces whether the deterministic cluster co-membership
// multiplier was applied to this candidate's score (i.e., the candidate shares
// the seed's committed-graph cluster). When the cluster accessor is
// unavailable or returns an error, ClusterBoostApplied is always false and the
// envelope's FallbackReason carries "cluster_boost_unavailable".
type RelatedResult struct {
	SymbolID            string  `json:"symbol_id"`
	Score               float64 `json:"score"`
	ClusterBoostApplied bool    `json:"cluster_boost_applied"`
	Path                string  `json:"path,omitempty"`
}

// FindRelatedSymbolsResult is the find_related_symbols response shape.
type FindRelatedSymbolsResult struct {
	Seed           SeedEnvelope    `json:"seed"`
	Results        []RelatedResult `json:"results"`
	TotalCount     int             `json:"total_count"`
	Freshness      FreshnessV2     `json:"freshness"`
	FallbackReason string          `json:"fallback_reason,omitempty"`
}

// findRelatedSymbolsHelp is the verbose help text for find_related_symbols.
const findRelatedSymbolsHelp = `
## Usage Examples

Find related symbols by stable id (default k=20):
  find_related_symbols(seed={symbol_id: "repo/src/svc.go::ServeHTTP"})

Find related by (file_path, symbol_name) tuple:
  find_related_symbols(seed={file_path: "src/svc.go", symbol_name: "ServeHTTP"})

Narrow to results under a path subtree (seed itself is NOT constrained):
  find_related_symbols(
    seed={symbol_id: "repo/src/svc.go::ServeHTTP"},
    paths=["pkg/sibling/"],
    k=10
  )

## Parameters
- seed (object, required): One of two forms —
    * { symbol_id: <stable graph id> } — direct lookup, no name resolution.
    * { file_path, symbol_name } — name-resolved lookup; ambiguous matches
      surface in seed.ambiguous_candidates.
- k (int, optional): Top-N cap. Default 20; clamped to [1, 100].
- paths ([]string, optional): Strict-subset filter applied to each RESULT
  candidate's defining file path. The seed itself is NOT constrained by
  paths (it was the input, not a candidate).

## Return Shape
- seed (object): echo of resolved seed (symbol_id, resolution, etc.).
- results ([]RelatedResult): up to k entries, ordered by Score descending
  (RRF score after optional cluster co-membership boost). Each entry
  carries:
    * symbol_id (string)
    * score (float64)
    * cluster_boost_applied (bool)
    * path (string, optional) — defining file path used by the paths filter
- total_count (int): number of candidates AFTER paths filtering, BEFORE k
  truncation.
- freshness (FreshnessV2): { graph_version, snapshot_id, extractor_run_id,
  as_of_unix_ms, status ∈ {current|stale|unknown}, source }.
- fallback_reason (string, optional): closed-enum reason —
  "symbol_not_found" | "cluster_boost_unavailable".

## Ranking Pipeline
PersonalizedPageRank(seed) + QueryBleve(anchors=[seed]) → retrieval.Fuse
(weighted RRF) → optional cluster co-membership multiplier → paths filter →
top-k truncate. The cluster boost is single-goroutine and deterministic
(Pitfall 6); cluster membership is read against the SAME graph_version that
PageRank ran on (Pitfall 3).

## Mode Tier
read+ — every session passes; this is the sibling-symbol companion to
find_references / get_callers.

## Determinism
Same seed + k + paths inputs produce byte-identical response ordering
across runs at a fixed graph_version (Phase 62 sort-before-iterate doctrine
asserted by TestFindRelatedSymbols_ConcurrentSameSeed under -race).`

// registerFindRelatedSymbols wires find_related_symbols into the MCP server
// with kernel-style typed-args registration + tracing.
func registerFindRelatedSymbols(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "find_related_symbols",
		Description: "Top-k semantically related symbols around a seed (read+).",
	}, kernel.WrapToolSpan(tracer, "find_related_symbols",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args FindRelatedSymbolsArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleFindRelatedSymbols(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "find_related_symbols",
		Description:      "Top-k semantically related symbols around a seed (read+).",
		BriefDescription: "Related symbols around a seed",
		HelpText:         findRelatedSymbolsHelp,
	})
}

// handleFindRelatedSymbols is the testable handler body. Order is load-bearing:
//  1. checkMode(modeTierRead) — every session passes; retained for code-review
//     visibility and the Phase 66 GuardrailMiddleware precedent.
//  2. validatePaths(args.Paths, ws.RepoRoot) — reject path traversal.
//  3. Clamp k to [1, 100]; default 20.
//  4. resolveSeed → ResolvedSeed; short-circuit on not_found.
//  5. CurrentGraphVersion capture BEFORE invoking PageRank (Pitfall 3 — the
//     same gv pins the cluster boost read).
//  6. Fan-out QueryBleve + PersonalizedPageRank with anchors=[seed].
//  7. retrieval.Fuse → []FusedCandidate (RRF + 3-key tiebreak).
//  8. Single-goroutine post-fuse cluster boost (Pitfall 6) — when
//     ClusterMembershipAccessor is wired AND the seed's cluster id is
//     readable AND the candidate shares the seed's cluster id, multiply
//     Score by clusterBoostMultiplier. Cluster-accessor errors degrade
//     gracefully with FallbackReason="cluster_boost_unavailable".
//  9. Re-sort post-boost by (Score desc, SymbolID asc) to preserve the same
//     determinism doctrine retrieval.Fuse uses internally.
//  10. Apply paths filter to RESULTS (strict-subset). total_count =
//     len(filtered).
//  11. Truncate to k.
//  12. Assemble FreshnessV2 envelope (status=current when snapshot_id != 0
//     && !overlay_active && runID != "").
//  13. Issue context-gathered receipt; return jsonResult.
//
// HARD INVARIANT (D-09 / D-13): this function MUST NOT reach the snapshot-
// write surface of *Store nor the per-workspace compactor's flush trigger.
// The recorder mocks in tools_find_related_test.go fail loudly on any future
// regression that reaches them.
func (s *SemanticSkill) handleFindRelatedSymbols(ctx context.Context, args FindRelatedSymbolsArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check.
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Workspace + path-traversal validation.
	ws := s.workspaceKey(ctx)
	if err := validatePaths(args.Paths, ws.RepoRoot); err != nil {
		return errorResult(err.Error())
	}
	repoID := ws.Hash()

	// 3. Clamp k.
	k := args.K
	if k <= 0 {
		k = findRelatedDefaultK
	}
	if k < findRelatedMinK {
		k = findRelatedMinK
	}
	if k > findRelatedMaxK {
		k = findRelatedMaxK
	}

	// 4. Resolve seed.
	resolved, err := s.resolveSeed(ctx, ws, args.Seed)
	if err != nil {
		return errorResult(err.Error())
	}
	seedEnv := SeedEnvelope{
		SymbolID:            string(resolved.SymbolID),
		Resolution:          resolved.Resolution,
		AmbiguousCandidates: resolved.AmbiguousCandidates,
	}

	// 5. Short-circuit on not_found — emit structured envelope with the
	//    freshness payload still populated.
	if resolved.Resolution == ResolutionNotFound {
		return jsonResult(FindRelatedSymbolsResult{
			Seed:           seedEnv,
			Results:        []RelatedResult{},
			TotalCount:     0,
			Freshness:      s.assembleFreshness(ctx, repoID),
			FallbackReason: "symbol_not_found",
		})
	}

	// 6. Capture committed graph_version BEFORE PageRank so both PageRank
	//    seed expansion AND the cluster-member read observe the same gv
	//    (Pitfall 3).
	s.mu.Lock()
	store := s.store
	retr := s.retrieval
	cm := s.clusterMembership
	s.mu.Unlock()

	var graphVersion uint64
	if store != nil {
		if gv, gErr := store.CurrentGraphVersion(ctx, repoID); gErr == nil {
			graphVersion = gv
		} else if s.logger != nil {
			s.logger.Warn("find_related_symbols: CurrentGraphVersion read failed",
				"repo_id", repoID, "err", gErr)
		}
	}

	// 7. Fan-out: anchors are exactly [seed]; empty task drives graph-only bias.
	anchors := []string{string(resolved.SymbolID)}
	var (
		textRanksRaw  []TextRank
		graphRanksRaw []GraphRank
	)
	if retr != nil {
		if tr, tErr := retr.QueryBleve("", anchors); tErr == nil {
			textRanksRaw = tr
		} else if s.logger != nil {
			s.logger.Warn("find_related_symbols: QueryBleve failed",
				"repo_id", repoID, "err", tErr)
		}
		if gr, gErr := retr.PersonalizedPageRank(ctx, repoID, anchors); gErr == nil {
			graphRanksRaw = gr
		} else if s.logger != nil {
			s.logger.Warn("find_related_symbols: PersonalizedPageRank failed",
				"repo_id", repoID, "err", gErr)
		}
	}

	// 8. Translate skill-package TextRank/GraphRank → retrieval-package types
	//    for Fuse (mirrors tools_context.go:249-256).
	textForFuse := make([]retrieval.TextRank, 0, len(textRanksRaw))
	for _, t := range textRanksRaw {
		textForFuse = append(textForFuse, retrieval.TextRank{SymbolID: t.SymbolID, Score: t.Score})
	}
	graphForFuse := make([]retrieval.GraphRank, 0, len(graphRanksRaw))
	for _, g := range graphRanksRaw {
		graphForFuse = append(graphForFuse, retrieval.GraphRank{SymbolID: g.SymbolID, Score: g.Score})
	}

	// 9. Fuse via weighted RRF; gv lookup is per-snapshot (same gv for every
	//    candidate satisfies the 3-key tiebreak — score desc → gv desc →
	//    symbol_id asc — because gv is constant; ties break on symbol_id).
	gvLookup := func(symbolID string) uint64 { return graphVersion }
	fused := retrieval.Fuse(textForFuse, graphForFuse, retrieval.DefaultRRFConfig(), gvLookup)

	// 10. Post-fuse cluster co-membership boost (Pitfall 6: single-goroutine
	//     pass). Read the seed's cluster id under the SAME graph_version that
	//     PageRank ran on (Pitfall 3 — operational invariant: gv was captured
	//     in step 6 BEFORE this point, so any concurrent gv mutation cannot
	//     drift the cluster read off the PageRank-time gv).
	var (
		boostedSet     map[string]struct{}
		fallbackReason string
	)
	if cm != nil {
		seedClusterID, _, cErr := cm.ClusterIDOf(ctx, repoID, resolved.SymbolID)
		if cErr != nil {
			fallbackReason = "cluster_boost_unavailable"
			if s.logger != nil {
				s.logger.Warn("find_related_symbols: seed ClusterIDOf failed; cluster boost skipped",
					"repo_id", repoID, "symbol_id", resolved.SymbolID, "err", cErr)
			}
		} else if seedClusterID != 0 {
			boostedSet = make(map[string]struct{}, len(fused))
			for i := range fused {
				candID, _, lErr := cm.ClusterIDOf(ctx, repoID, integ.SymbolID(fused[i].SymbolID))
				if lErr != nil {
					// Per-candidate failure → leave that candidate un-boosted
					// (do not poison the whole pass).
					continue
				}
				if candID != 0 && candID == seedClusterID {
					fused[i].Score *= clusterBoostMultiplier
					boostedSet[fused[i].SymbolID] = struct{}{}
				}
			}
		}
	}

	// 11. Re-sort post-boost by (Score desc, SymbolID asc). Same 3-key
	//     tiebreak retrieval.Fuse uses, simplified: graph_version is constant
	//     across all candidates in this handler so the desc-gv arm is a
	//     no-op; we keep score-desc → symbol_id-asc explicit for clarity.
	sort.SliceStable(fused, func(i, j int) bool {
		if fused[i].Score != fused[j].Score {
			return fused[i].Score > fused[j].Score
		}
		return fused[i].SymbolID < fused[j].SymbolID
	})

	// 12. paths filter on results (strict-subset on each candidate's
	//     defining file path). The seed itself is NOT in `fused` because
	//     PageRank seeds are excluded from their own neighbor set.
	filtered := make([]RelatedResult, 0, len(fused))
	for _, fc := range fused {
		path := pathFromSymbolID(fc.SymbolID)
		if !pathsAllow(args.Paths, path) {
			continue
		}
		_, boosted := boostedSet[fc.SymbolID]
		filtered = append(filtered, RelatedResult{
			SymbolID:            fc.SymbolID,
			Score:               fc.Score,
			ClusterBoostApplied: boosted,
			Path:                path,
		})
	}
	total := len(filtered)
	if total > k {
		filtered = filtered[:k]
	}

	// 13. Assemble FreshnessV2 envelope (shared helper extracted in 71-03).
	freshness := s.assembleFreshness(ctx, repoID)

	result := FindRelatedSymbolsResult{
		Seed:           seedEnv,
		Results:        filtered,
		TotalCount:     total,
		Freshness:      freshness,
		FallbackReason: fallbackReason,
	}

	// 14. Receipt issuance on success path.
	guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
		guardrails.ContextGatheredScope{
			TargetSymbols:   []integ.SymbolID{resolved.SymbolID},
			TaskHash:        string(resolved.SymbolID),
			TokenBudgetUsed: len(filtered),
			MaxTokens:       k,
		}, "find_related_symbols")
	return jsonResult(result)
}

// pathFromSymbolID extracts the defining file path from a stable SymbolID of
// the form "<path>::<name>". When the symbol id does not carry a `::`
// separator (a future custom format), returns the empty string and the paths
// filter, if non-empty, refuses the candidate.
func pathFromSymbolID(symbolID string) string {
	if i := strings.Index(symbolID, "::"); i >= 0 {
		return symbolID[:i]
	}
	return ""
}

// pathsAllow returns whether a candidate's defining file path is within at
// least one of the configured paths prefixes (strict-subset semantics, Phase
// 70 D2 + Phase 71 D2). Empty allowedPaths permits any candidate.
func pathsAllow(allowedPaths []string, candidatePath string) bool {
	if len(allowedPaths) == 0 {
		return true
	}
	if candidatePath == "" {
		// Without a defining-file anchor we cannot prove containment; refuse
		// rather than leak.
		return false
	}
	for _, p := range allowedPaths {
		if strings.HasPrefix(candidatePath, p) {
			return true
		}
	}
	return false
}
