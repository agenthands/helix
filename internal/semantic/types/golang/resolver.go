package golang

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

// Resolver implements types.Resolver for Go source.
//
// The 7-tier ladder is walked top-down; the FIRST tier that produces a
// signal wins. Cross-package chains (D-13 + Pitfall 5) cap at last-in-
// package confidence with validation_state="unresolved" — even when the
// raw signal would normally promote to validated.
type Resolver struct {
	store types.EffectiveReader

	// typeIndex is an optional in-memory map of stable type-key →
	// graph.NodeID, used by the resolver to convert a parsed type name
	// (signature/comment/heuristic) into a target node so the cross-
	// package guard can compare file paths. The production daemon leaves
	// this nil; tests populate it directly. When nil, lower-tier responses
	// resolve with Target=0.
	typeIndex map[string]graph.NodeID
}

// NewResolver constructs a Go resolver over the given EffectiveReader seam.
func NewResolver(store types.EffectiveReader) *Resolver {
	return NewResolverWithIndex(store, nil)
}

// NewResolverWithIndex constructs a Go resolver over the given EffectiveReader
// seam plus an optional stable-type-name → NodeID index. The v2.12 Phase 136
// daemon producer injects the batch nameToNode index so the annotation tier
// binds a real RESOLVES_TO target; callers with no index pass nil.
func NewResolverWithIndex(store types.EffectiveReader, idx map[string]graph.NodeID) *Resolver {
	return &Resolver{store: store, typeIndex: idx}
}

// constructor matches `x := NewFoo()` / `var x = NewFoo()` style.
var constructorRE = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])New(\w+)\s*\(`)

// annotation matches `var x Foo` / `x Foo` (typed declaration).
var annotationRE = regexp.MustCompile(`(?:^|\b)(?:var\s+)?\w+\s+\*?(\w+)\s*$`)

// assignment matches `y = x` (right-hand side identifier).
var assignmentRE = regexp.MustCompile(`(?:^|\b)\w+\s*=\s*(\w+)\s*$`)

// ResolveChain walks the 7-tier ladder against the symbol referenced by
// req.RefNodeID. The ChainTokens are not used at this leaf-hop level —
// the shared chain.go walker already advances RefNodeID per hop.
func (r *Resolver) ResolveChain(ctx context.Context, req types.ChainRequest) (types.ChainResponse, error) {
	// Tier 1: LSP — Phase 61 cascade may have already produced a
	// validated RESOLVES_TO edge.
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

	// Tier 2: annotation — typed declaration (e.g., `var x Foo`).
	// Option A (v2.12 Phase 136): a co-driver-supplied declared type in
	// req.ChainTokens[0] takes precedence over re-parsing the signature;
	// an empty token falls back to parseAnnotation.
	annType := parseAnnotation(sym.Signature)
	if len(req.ChainTokens) > 0 && req.ChainTokens[0] != "" {
		annType = req.ChainTokens[0]
	}
	if annType != "" {
		return r.tierResponse(req, sym, annType, types.EvidenceAnnotation,
			types.ConfidenceAnnotation, "annotation")
	}

	// Tier 3: constructor — `x := NewFoo()` / `make(Foo)`.
	if t := parseConstructor(sym.Signature); t != "" {
		return r.tierResponse(req, sym, t, types.EvidenceConstructor,
			types.ConfidenceConstructor, "constructor")
	}

	// Tier 4: assignment-flow — `y = x` within fixpoint scope.
	if t := parseAssignment(sym.Signature); t != "" {
		return r.tierResponse(req, sym, t, types.EvidenceAssignment,
			types.ConfidenceAssignment, "assignment")
	}

	// Tier 5: GoDoc comment — `// Foo returns *Bar`.
	if t := ParseGoDocType(sym.DocComment); t != "" {
		return r.tierResponse(req, sym, t, types.EvidenceComment,
			types.ConfidenceComment, "comment.godoc")
	}

	// Tier 6: heuristic — name-shape match (e.g., `userRepo` → `User*`).
	if t := guessFromName(sym.StableKey); t != "" {
		return r.tierResponse(req, sym, t, types.EvidenceHeuristic,
			types.ConfidenceHeuristic, "heuristic")
	}

	// Tier 7: unknown — D-12 invariant: ALWAYS emit.
	return types.ChainResponse{
		Resolved:        false,
		Confidence:      types.ConfidenceUnknown,
		EvidenceKind:    types.EvidenceUnknown,
		ValidationState: "unresolved",
		Source:          "unknown",
		Reason:          "go: no signal for ref",
	}, nil
}

// tierResponse builds a ChainResponse for tiers 2-6 and applies the D-13
// cross-package guard. If the resolved type's file is in a different
// package than the request file, the response caps at ConfidenceComment
// with validation_state="unresolved" — even if the raw tier would have
// been validated.
func (r *Resolver) tierResponse(req types.ChainRequest, sym types.SymbolFact, typeName string,
	kind types.EvidenceKind, conf float64, source string) (types.ChainResponse, error) {

	// Resolve the type name to a target NodeID via the optional typeIndex.
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

	// D-13 cross-package guard: if we have a target file and it lives
	// outside the request's package, cap confidence and emit unresolved.
	if targetFile != "" && !SamePackage(req.FilePath, targetFile) {
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
			Reason:          "go: cross-package chain (D-13) — not resolved in v1",
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
// request — Go symbols are resolved through the same ladder as chain hops.
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

// parseAnnotation extracts the type name from a typed-declaration signature
// like `var x Foo` or `x Foo`. Returns "" when the signature carries the
// `:=` short-decl operator (those are constructor-tier candidates).
func parseAnnotation(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return ""
	}
	if strings.Contains(sig, ":=") {
		return ""
	}
	if strings.Contains(sig, "=") && !strings.HasPrefix(sig, "var ") {
		return ""
	}
	m := annotationRE.FindStringSubmatch(sig)
	if len(m) >= 2 {
		// Reject pure lowercase one-word matches (e.g., "y = x" already
		// excluded above; this guards against single identifier inputs).
		t := m[1]
		if isIdent(t) {
			return t
		}
	}
	return ""
}

// parseConstructor matches `x := NewFoo()` or `var x = NewFoo()` style.
func parseConstructor(sig string) string {
	if sig == "" {
		return ""
	}
	m := constructorRE.FindStringSubmatch(sig)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// parseAssignment extracts the right-hand side identifier of `y = x`.
// Returns "" for typed declarations or constructor calls.
func parseAssignment(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" || strings.HasPrefix(sig, "var ") {
		return ""
	}
	if strings.Contains(sig, ":=") {
		return ""
	}
	if !strings.Contains(sig, "=") {
		return ""
	}
	if strings.Contains(sig, "(") {
		// constructor / function-call shape — skip.
		return ""
	}
	m := assignmentRE.FindStringSubmatch(sig)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// suffixRule maps a recognised identifier suffix to its full-form type
// name. The slice ordering is the match-priority order (CR-03 closure):
// sort-before-iterate locks deterministic match selection regardless of
// future additions that may alias one suffix as another's tail.
type suffixRule struct{ short, long string }

// goSuffixRules is sorted ascending by short. Adding a rule MUST preserve
// the sort order so determinism remains structural rather than incidental.
// Today's set is overlap-free at the tail level so behaviour is unchanged
// for existing inputs; the slice form is a contract upgrade that locks
// down future additions.
var goSuffixRules = []suffixRule{
	{short: "Cfg", long: "Config"},
	{short: "Conf", long: "Configuration"},
	{short: "Conn", long: "Connection"},
	{short: "Ctrl", long: "Controller"},
	{short: "Hdlr", long: "Handler"},
	{short: "Mgr", long: "Manager"},
	{short: "Repo", long: "Repository"},
	{short: "Svc", long: "Service"},
}

// guessFromName implements the heuristic name-shape match. The current
// rule converts a camelCase identifier ending in a recognised suffix
// (e.g., "Repo" → "Repository") into its full-form type name.
func guessFromName(name string) string {
	return guessFromNameWithRules(name, goSuffixRules)
}

// guessFromNameWithRules is the test-seam variant. CR-03 invariant:
// callers MUST pass rules sorted ascending by short for deterministic
// match priority.
func guessFromNameWithRules(name string, rules []suffixRule) string {
	if name == "" {
		return ""
	}
	for _, r := range rules {
		if strings.HasSuffix(name, r.short) && len(name) > len(r.short) {
			// Convert the prefix to TitleCase + the long form.
			prefix := strings.TrimSuffix(name, r.short)
			if prefix == "" {
				return r.long
			}
			return strings.ToUpper(prefix[:1]) + prefix[1:] + r.long
		}
	}
	return ""
}

// isIdent reports whether s is a syntactically plausible Go identifier.
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
