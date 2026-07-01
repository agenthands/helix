package c

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

// Resolver implements types.Resolver for C source.
//
// The ladder is walked top-down; the FIRST tier that produces a signal wins.
// C populates tiers 1 (LSP), 2 (annotation / typed decl), 4 (assignment), 6
// (heuristic) and 7 (unresolved). There is no constructor tier (Tier 3 — C
// has no ctor syntax) and no comment tier in v2.11 (Tier 5 — no doc_comment
// column, red-team B2).
type Resolver struct {
	store types.EffectiveReader

	// typeIndex is an optional stable-type-name → NodeID map (tests populate
	// it; the production daemon leaves it nil). Mirrors golang.Resolver — the
	// only consumer is the D-13 scope guard's target-file lookup. When nil,
	// tier 2-6 responses resolve with Target=0.
	typeIndex map[string]graph.NodeID
}

// NewResolver constructs a C resolver over the given EffectiveReader seam.
// This replaces the Phase-62 LSP-conditional stub (v2.11 Phase 131).
func NewResolver(store types.EffectiveReader) *Resolver {
	return &Resolver{store: store}
}

// assignmentRE matches a bare `y = x` (both sides single identifiers). A
// typed declaration (`Foo x = bar`) has a two-token LHS and is caught by the
// annotation tier first, so this anchored form never fires for typed inits.
var assignmentRE = regexp.MustCompile(`^\s*\w+\s*=\s*(\w+)\s*$`)

// cQualifiers are storage-class / cv keywords stripped before the type token
// in a typed declaration. `struct`/`union`/`enum` are handled separately (the
// following token is the tag name).
var cQualifiers = map[string]bool{
	"const": true, "volatile": true, "static": true, "extern": true,
	"register": true, "inline": true, "auto": true, "restrict": true,
	"_Atomic": true, "signed": true, "unsigned": true,
}

// ResolveChain walks the C ladder against the symbol referenced by
// req.RefNodeID.
func (r *Resolver) ResolveChain(ctx context.Context, req types.ChainRequest) (types.ChainResponse, error) {
	// Tier 1: LSP — Phase 61 cascade may have produced a validated edge.
	edges, err := r.store.QueryEffectiveEdges(ctx, types.EdgeQuery{
		RepoID:    req.RepoID,
		SrcNodeID: req.RefNodeID,
		EdgeKind:  "RESOLVES_TO",
	})
	if err != nil {
		return types.ChainResponse{}, err
	}
	for _, e := range edges {
		if e.Confidence >= 1.0 && strings.HasPrefix(e.Source, "lsp.") {
			return types.ChainResponse{
				Resolved:        true,
				Target:          e.DstNodeID,
				Confidence:      types.ConfidenceLSP,
				EvidenceKind:    types.EvidenceLSP,
				ValidationState: "validated",
				Source:          e.Source,
			}, nil
		}
	}

	// Pull the symbol fact for tiers 2-6.
	sym, _ := r.store.QueryEffectiveSymbol(ctx, req.RepoID, req.RefNodeID)

	// Tier 2: annotation — typed declaration (`struct Foo x`, `Foo *x`).
	if t := parseAnnotation(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceAnnotation,
			types.ConfidenceAnnotation, "annotation")
	}

	// Tier 4: assignment-flow — `y = x` within fixpoint scope.
	if t := parseAssignment(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceAssignment,
			types.ConfidenceAssignment, "assignment")
	}

	// Tier 6: heuristic — name-shape match (e.g., `userCfg` → `Config`).
	if t := guessFromName(sym.StableKey); t != "" {
		return r.tierResponse(req, t, types.EvidenceHeuristic,
			types.ConfidenceHeuristic, "heuristic")
	}

	// Tier 7: unknown — D-12 invariant: ALWAYS emit.
	return types.ChainResponse{
		Resolved:        false,
		Confidence:      types.ConfidenceUnknown,
		EvidenceKind:    types.EvidenceUnknown,
		ValidationState: "unresolved",
		Source:          "unknown",
		Reason:          "c: no signal for ref",
	}, nil
}

// tierResponse builds a ChainResponse for tiers 2-6 and applies the D-13
// scope guard. C scope is flat (SameScope always true — program-wide linkage),
// so the guard never caps in v2.11; it is retained for structural symmetry
// with the other resolvers and as the seam a future header/#include-aware
// scope would activate.
func (r *Resolver) tierResponse(req types.ChainRequest, typeName string,
	kind types.EvidenceKind, conf float64, source string) (types.ChainResponse, error) {

	var target graph.NodeID
	var targetFile string
	if r.typeIndex != nil {
		if id, ok := r.typeIndex[typeName]; ok {
			target = id
			if ts, err := r.store.QueryEffectiveSymbol(context.Background(), req.RepoID, id); err == nil {
				targetFile = ts.FilePath
			}
		}
	}

	if targetFile != "" && !SameScope(req.FilePath, targetFile) {
		capped := conf
		if capped > types.ConfidenceComment {
			capped = types.ConfidenceComment
		}
		return types.ChainResponse{
			Resolved:        false,
			Target:          target,
			Confidence:      capped,
			EvidenceKind:    kind,
			ValidationState: "unresolved",
			Source:          source,
			Reason:          "c: cross-scope chain (D-13) — not resolved in v1",
		}, nil
	}

	return types.ChainResponse{
		Resolved:        true,
		Target:          target,
		Confidence:      conf,
		EvidenceKind:    kind,
		ValidationState: "validated",
		Source:          source,
	}, nil
}

// ResolveSymbol delegates to ResolveChain over a synthetic single-token
// request — C symbols resolve through the same ladder as chain hops.
func (r *Resolver) ResolveSymbol(ctx context.Context, req types.SymbolRequest) (types.SymbolResponse, error) {
	cr := types.ChainRequest{
		RepoID:      req.RepoID,
		Language:    req.Language,
		FilePath:    req.FilePath,
		RefNodeID:   req.SymbolNodeID,
		RefKind:     "RESOLVES_TO",
		ChainTokens: []string{""},
	}
	resp, err := r.ResolveChain(ctx, cr)
	if err != nil {
		return types.SymbolResponse{}, err
	}
	return types.SymbolResponse{
		Resolved:        resp.Resolved,
		TypeNodeID:      resp.Target,
		Confidence:      resp.Confidence,
		EvidenceKind:    resp.EvidenceKind,
		ValidationState: resp.ValidationState,
		Source:          resp.Source,
		Reason:          resp.Reason,
	}, nil
}

// parseAnnotation extracts the declared type name from a C typed-declaration
// signature: `struct Foo x`, `Foo *x`, `const Foo x`, `Foo x = init`. Returns
// "" for a bare assignment (`x = y`) or a single identifier (no declarator).
//
// `struct`/`union`/`enum Tag x` returns the tag ("Foo"). Leading storage-class
// / cv qualifiers are stripped; the pointer `*` is not part of the type name.
func parseAnnotation(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return ""
	}
	// A typed declaration may carry an initializer (`Foo x = init`); the type
	// is the LHS. Cut at the first `=` and parse the declarator half.
	if i := strings.IndexByte(sig, '='); i >= 0 {
		sig = strings.TrimSpace(sig[:i])
	}
	// Tokenize, stripping the pointer `*` (not part of the type name).
	fields := strings.Fields(sig)
	toks := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimLeft(f, "*")
		f = strings.TrimRight(f, "*")
		if f != "" {
			toks = append(toks, f)
		}
	}
	// Drop leading qualifiers; handle struct/union/enum tag.
	for len(toks) > 0 {
		head := toks[0]
		if head == "struct" || head == "union" || head == "enum" {
			if len(toks) >= 2 && isIdent(toks[1]) {
				return toks[1]
			}
			return ""
		}
		if cQualifiers[head] {
			toks = toks[1:]
			continue
		}
		break
	}
	// A typed declaration needs a type token AND a declarator (≥2 tokens);
	// a lone identifier is just a name, not a typed decl.
	if len(toks) < 2 {
		return ""
	}
	if isIdent(toks[0]) {
		return toks[0]
	}
	return ""
}

// parseAssignment extracts the right-hand side identifier of a bare `y = x`.
// Typed declarations (two-token LHS) do not match the anchored regex, and
// function-call RHS (`x = f()`) is rejected.
func parseAssignment(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" || !strings.Contains(sig, "=") {
		return ""
	}
	if strings.Contains(sig, "(") {
		return ""
	}
	m := assignmentRE.FindStringSubmatch(sig)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// suffixRule maps a recognised identifier suffix to its full-form type name.
type suffixRule struct{ short, long string }

// cSuffixRules is sorted ascending by short (CR-03 determinism: match priority
// is structural). The heuristic is best-effort and fires on camelCase-suffixed
// C identifiers; idiomatic snake_case C simply does not match (honest low hit
// rate — the tier is a fallback, not a primary signal for C).
var cSuffixRules = []suffixRule{
	{short: "Cfg", long: "Config"},
	{short: "Conn", long: "Connection"},
	{short: "Ctx", long: "Context"},
	{short: "Hdlr", long: "Handler"},
	{short: "Mgr", long: "Manager"},
}

// guessFromName implements the heuristic name-shape match for C.
func guessFromName(name string) string {
	return guessFromNameWithRules(name, cSuffixRules)
}

// guessFromNameWithRules is the test-seam variant (CR-03: rules sorted by short).
func guessFromNameWithRules(name string, rules []suffixRule) string {
	if name == "" {
		return ""
	}
	for _, r := range rules {
		if strings.HasSuffix(name, r.short) && len(name) > len(r.short) {
			prefix := strings.TrimSuffix(name, r.short)
			if prefix == "" {
				return r.long
			}
			return strings.ToUpper(prefix[:1]) + prefix[1:] + r.long
		}
	}
	return ""
}

// isIdent reports whether s is a syntactically plausible C identifier.
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}
