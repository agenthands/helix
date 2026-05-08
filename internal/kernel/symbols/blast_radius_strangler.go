// blast_radius_strangler.go — Phase 65 65-06 two-pass orchestrator wrapping
// the existing pure-LSP analyze_blast_radius primitive (blast.go) with the
// integ.SemanticLookup graph-first / LSP-validates-critical-edges path
// described by D-04 + D-07 + D-08.
//
// Doctrine summary:
//
//   - Source-selection ladder: integ.ChooseSource(cfgGate, lookup, err)
//     decides between SourceTreeSitter / SourceFallback / SourceSemantic
//     before the orchestrator dispatches. Bare lookup-availability checks
//     would defeat the cfg-disabled → tree_sitter contract (D-04 / Pitfall §3).
//
//   - Two-pass algorithm (D-07): Pass 1 calls lookup.ExpandFrom for the cheap
//     persisted-graph expansion; Pass 2 calls lookup.ValidateCriticalEdges
//     for the LSP probe on edges that either cross a public-API boundary or
//     have Pass-1 confidence < 0.80. Validated edges flip to 1.00; refuted
//     edges flip to 0.20 with Refuted=true; non-critical edges keep their
//     Pass 1 confidence byte-identical (Pitfall §6 copy-before-mutate).
//
//   - Confidence cap (D-08; ROADMAP SC #2): every non-semantic path
//     (SourceTreeSitter AND SourceFallback) hard-caps per-node confidence at
//     0.6 before envelope render. The cap MUST be applied uniformly — that
//     is the property TestAnalyzeBlastRadius_CfgDisabled_TreeSitter and
//     TestAnalyzeBlastRadius_LookupUnavailable_Fallback pin.
//
//   - Error contract: a Pass 1 error from ExpandFrom is non-fatal — the
//     orchestrator returns (nil, SourceFallback, ClassifyLookupErr(err), err)
//     and the caller falls through to the LSP fallback path. A Pass 2 error
//     from ValidateCriticalEdges is non-fatal too — the orchestrator keeps
//     the Pass 1 verdicts and reports source=semantic.
//
// Read-tier safety (M-readtier): this file imports only internal/semantic/integ
// (the wave-0 allowlist). The production lookup adapter
// (internal/daemon/semantic_wiring.go integSemanticLookup) is read-only by
// grep canary (internal/daemon/integ_lookup_test.go), so the orchestrator
// can never reach a snapshot-write surface through this seam.
package symbols

import (
	"context"
	"strings"

	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// blastRadiusExpansionDepth is the depth bound passed to lookup.ExpandFrom.
// Phase 65 65-06 caps depth at 2 by default per the SemanticLookup contract.
const blastRadiusExpansionDepth = 2

// nonCriticalConfidenceThreshold names the Pass-1 confidence cutoff. Edges
// at or above this value are non-critical IF they also stay within the
// owning package; below it, every edge is critical regardless of the package
// boundary.
const nonCriticalConfidenceThreshold = 0.80

// fallbackConfidenceCap is the D-08 / ROADMAP SC #2 hard cap applied on
// every non-semantic path (SourceTreeSitter AND SourceFallback). Tools must
// not surface confidence > 0.6 on a path that could not consult the
// persisted graph.
const fallbackConfidenceCap = 0.6

// analyzeBlastRadiusViaLookup is the semantic-arm orchestrator. Caller has
// already verified ChooseSource returned SourceSemantic and resolved the
// SymbolID for the cursor position.
//
// Returns:
//   - impacts: the Pass-1 result with Pass-2 verdicts applied (or untouched
//     when Pass 2 errors). Caller renders these via
//     formatBlastRadiusEnvelopeFromImpacts.
//   - src + reason: SourceSemantic + "" on success; SourceFallback +
//     ClassifyLookupErr(err) on Pass 1 error so caller can propagate to the
//     fallback envelope.
//   - err: non-nil only on Pass 1 error — caller drops to the LSP fallback.
//
// Pass 2 errors are absorbed (logged debug at the call site if desired):
// keeping Pass 1 verdicts is the doctrine when the LSP probe is unavailable
// (RESEARCH Code Example 2).
//
// WR-03 (Phase 65 65-11): the orchestrator now also returns the persisted
// graph_version (from a single lookup.Status call after Pass 1 succeeds) so
// formatBlastRadiusEnvelopeFromImpacts can stamp it on the semantic envelope.
// Status errors are non-fatal — the orchestrator returns graphVersion=0 and
// lets the envelope omit the field via integ.MarshalEnvelope's omitempty.
//
// Signature stays single-arg per WR-5: no lspProbeFn parameter (that lands
// in 65-12 Task 2 which migrates the orchestrator to the LSP-validation
// arm).
func analyzeBlastRadiusViaLookup(
	ctx context.Context,
	lookup integ.SemanticLookup,
	ws workspace.WorkspaceKey,
	sym integ.SymbolID,
) ([]integ.Impact, integ.Source, integ.FallbackReason, uint64, error) {
	// Pass 1: persisted-graph expansion. Cheap, depth-bounded.
	impacts, err := lookup.ExpandFrom(ctx, ws, sym, blastRadiusExpansionDepth)
	if err != nil {
		return nil, integ.SourceFallback, integ.ClassifyLookupErr(err), 0, err
	}

	// Pitfall §6: copy Pass 1 BEFORE applying any Pass 2 verdicts. Tests
	// (TestApplyValidationVerdicts_NoMutation) pin this contract.
	impactsCopy := make([]integ.Impact, len(impacts))
	copy(impactsCopy, impacts)

	// Pass 2: filter critical edges, run the LSP probe.
	critical := filterCritical(impactsCopy)
	if len(critical) > 0 {
		verdicts, vErr := lookup.ValidateCriticalEdges(ctx, ws, edgesOf(critical))
		if vErr == nil {
			applyValidationVerdicts(impactsCopy, verdicts)
		}
		// vErr non-nil → leave Pass 1 confidences; the LSP probe is unavailable
		// but the persisted graph is the canonical answer at this point.
		// Caller still reports source=semantic.
	}

	// WR-03: thread graph_version through to the envelope. Single Status
	// call at the orchestrator boundary — Status errors are non-fatal
	// (graphVersion=0 means MarshalEnvelope omits the key via omitempty).
	var graphVersion uint64
	if status, sErr := lookup.Status(ctx, ws); sErr == nil {
		graphVersion = status.GraphVersion
	}
	return impactsCopy, integ.SourceSemantic, "", graphVersion, nil
}

// filterCritical returns the subset of impacts whose Pass-2 LSP probe
// matters. Per D-07: an impact is critical if (a) any of its evidence edges
// cross a public-API boundary (different package AND target is exported),
// OR (b) its Pass-1 confidence is below the non-critical threshold.
//
// The check is OR, not AND — a low-confidence in-package edge is critical
// because the LSP can confirm/refute it cheaply.
func filterCritical(impacts []integ.Impact) []integ.Impact {
	out := make([]integ.Impact, 0, len(impacts))
	for _, im := range impacts {
		if im.Confidence < nonCriticalConfidenceThreshold || crossesPublicAPI(im) {
			out = append(out, im)
		}
	}
	return out
}

// crossesPublicAPI reports whether any of the impact's evidence edges crosses
// a public-API boundary — defined as different package on each side AND the
// target symbol is exported (capitalized first identifier).
//
// Heuristic per D-07 (PLAN.md): the SymbolID format is approximated as
// "<package-path>:<qualified-name>"; package() splits on the last ':' and
// isExported() checks the first non-empty character of the qualified name
// is uppercase ASCII. Real implementations may consult richer EXTRACT-02
// metadata; this heuristic matches the Phase 65 RESEARCH proposed Code
// Example 2.
func crossesPublicAPI(im integ.Impact) bool {
	for _, e := range im.Evidence.Edges {
		fromPkg := symbolPackage(e.From)
		toPkg := symbolPackage(e.To)
		if fromPkg != toPkg && isExported(e.To) {
			return true
		}
	}
	return false
}

// symbolPackage extracts the package path component of a SymbolID. Phase 59
// EXTRACT-02 SymbolIDs carry "<package>:<name>" or "<file>:<name>" — we split
// on the LAST ':' so package paths containing colons (rare but possible in
// some languages) round-trip correctly.
func symbolPackage(sym integ.SymbolID) string {
	s := string(sym)
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return ""
	}
	return s[:idx]
}

// isExported reports whether the qualified-name suffix of a SymbolID begins
// with an ASCII uppercase letter (Go convention; conservative heuristic for
// the languages where EXTRACT-02 ships today). Rune-aware splits would be a
// follow-up enhancement once EXTRACT-02 carries richer visibility metadata.
func isExported(sym integ.SymbolID) bool {
	s := string(sym)
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		idx = -1
	}
	tail := s[idx+1:]
	if tail == "" {
		return false
	}
	c := tail[0]
	return c >= 'A' && c <= 'Z'
}

// edgesOf flattens the edges of all impacts into a single slice. Used to
// build the input to lookup.ValidateCriticalEdges.
func edgesOf(impacts []integ.Impact) []integ.Edge {
	total := 0
	for _, im := range impacts {
		total += len(im.Evidence.Edges)
	}
	out := make([]integ.Edge, 0, total)
	for _, im := range impacts {
		out = append(out, im.Evidence.Edges...)
	}
	return out
}

// applyValidationVerdicts applies Pass-2 ValidatedEdge verdicts to the
// impacts slice. The pre-WR-05 form broke after the first matching verdict,
// which masked refutations on sibling edges when a confirmation appeared
// first. WR-05 (Phase 65 65-11 Task 2) replaces that with sawConfirmed /
// sawRefuted accumulators evaluated AFTER walking every evidence edge:
//
//   - sawRefuted=true (regardless of sawConfirmed) →
//       Confidence=0.20, Refuted=true
//   - sawConfirmed=true && !sawRefuted →
//       Confidence=1.00, Refuted unchanged
//   - neither → impact untouched
//
// The doctrine: a single refutation taints the impact. The persisted graph
// said the edge exists; the LSP says it doesn't. We trust the LSP probe
// over the graph, regardless of how many sibling edges were confirmed.
//
// The caller MUST pass a COPY of the Pass 1 slice — applyValidationVerdicts
// mutates impacts[i].Confidence and impacts[i].Refuted in place. Pitfall §6
// + TestApplyValidationVerdicts_NoMutation enforce copy-before-mutate.
func applyValidationVerdicts(impacts []integ.Impact, verdicts []integ.ValidatedEdge) {
	if len(verdicts) == 0 {
		return
	}
	verdictByEdge := make(map[integ.Edge]integ.ValidatedEdge, len(verdicts))
	for _, v := range verdicts {
		verdictByEdge[v.Edge] = v
	}
	for i := range impacts {
		var sawConfirmed, sawRefuted bool
		for _, e := range impacts[i].Evidence.Edges {
			v, ok := verdictByEdge[e]
			if !ok {
				continue
			}
			if v.LSPConfirmed {
				sawConfirmed = true
			} else {
				sawRefuted = true
			}
		}
		switch {
		case sawRefuted:
			// Refutation taints regardless of sibling confirmations (WR-05).
			impacts[i].Confidence = 0.20
			impacts[i].Refuted = true
		case sawConfirmed:
			impacts[i].Confidence = 1.00
		}
	}
}

// capConfidences walks br.PerNode and clamps every Confidence to ≤ cap. Used
// on every non-semantic path (SourceTreeSitter AND SourceFallback) per D-08
// + ROADMAP SC #2. Idempotent and nil-safe.
func capConfidences(br *BlastRadius, cap float64) {
	if br == nil {
		return
	}
	for i := range br.PerNode {
		if br.PerNode[i].Confidence > cap {
			br.PerNode[i].Confidence = cap
		}
	}
}

// formatBlastRadiusEnvelope produces the canonical JSON envelope for a
// BlastRadius (LSP-derived) plus a closed-enum (source, fallback_reason)
// header. Used on the SourceTreeSitter and SourceFallback paths.
//
// Wire shape:
//
//	{
//	  "source":          "tree_sitter" | "fallback" | "semantic",
//	  "fallback_reason": <closed-enum-string-or-omitted>,
//	  "per_node":        [ { "symbol_id": ..., "confidence": ..., "refuted": ... }, ... ],
//	  "summary":         "<human-readable summary>"
//	}
//
// Per the SPEC §24 envelope contract, MarshalEnvelope owns the source /
// fallback_reason / graph_version / freshness keys; the per_node + summary
// keys are tool-specific payload merged at the top level.
func formatBlastRadiusEnvelope(br *BlastRadius, src integ.Source, reason integ.FallbackReason) ([]byte, error) {
	perNode := make([]map[string]interface{}, 0, len(br.PerNode))
	for _, n := range br.PerNode {
		entry := map[string]interface{}{
			"symbol_id":  n.SymbolID,
			"confidence": n.Confidence,
		}
		if n.Refuted {
			entry["refuted"] = true
		}
		if len(n.Evidence.LSPLocations) > 0 {
			entry["evidence"] = encodeEvidence(n.Evidence)
		}
		perNode = append(perNode, entry)
	}
	payload := map[string]interface{}{
		"per_node": perNode,
		"summary":  formatBlastRadius(br),
	}
	return integ.MarshalEnvelope(integ.Envelope{
		Source:         src,
		FallbackReason: reason,
	}, payload)
}

// formatBlastRadiusEnvelopeFromImpacts produces the canonical JSON envelope
// for the SourceSemantic path — the orchestrator returns []integ.Impact, not
// a *BlastRadius, because the persisted graph is authoritative.
//
// WR-03 (Phase 65 65-11): the graphVersion argument is the value threaded
// from analyzeBlastRadiusViaLookup's lookup.Status call after Pass 1.
// graphVersion=0 causes MarshalEnvelope to omit the key (omitempty contract).
func formatBlastRadiusEnvelopeFromImpacts(impacts []integ.Impact, src integ.Source, reason integ.FallbackReason, graphVersion uint64) ([]byte, error) {
	perNode := make([]map[string]interface{}, 0, len(impacts))
	for _, im := range impacts {
		entry := map[string]interface{}{
			"symbol_id":  string(im.SymbolID),
			"confidence": im.Confidence,
		}
		if im.Refuted {
			entry["refuted"] = true
		}
		entry["evidence"] = encodeEvidence(im.Evidence)
		perNode = append(perNode, entry)
	}
	payload := map[string]interface{}{
		"per_node": perNode,
	}
	return integ.MarshalEnvelope(integ.Envelope{
		Source:         src,
		FallbackReason: reason,
		GraphVersion:   graphVersion,
	}, payload)
}

// encodeEvidence renders an integ.Evidence as a JSON-friendly map. Plain
// values only — no pointers, no closures.
func encodeEvidence(ev integ.Evidence) map[string]interface{} {
	out := make(map[string]interface{}, 3)
	if len(ev.Edges) > 0 {
		edges := make([]map[string]interface{}, 0, len(ev.Edges))
		for _, e := range ev.Edges {
			edges = append(edges, map[string]interface{}{
				"from":       string(e.From),
				"to":         string(e.To),
				"kind":       e.Kind,
				"confidence": e.Confidence,
			})
		}
		out["edges"] = edges
	}
	if len(ev.Ranks) > 0 {
		out["ranks"] = ev.Ranks
	}
	if len(ev.LSPLocations) > 0 {
		locs := make([]map[string]interface{}, 0, len(ev.LSPLocations))
		for _, l := range ev.LSPLocations {
			locs = append(locs, map[string]interface{}{
				"path": l.Path,
				"line": l.Line,
				"col":  l.Col,
			})
		}
		out["lsp_locations"] = locs
	}
	return out
}
