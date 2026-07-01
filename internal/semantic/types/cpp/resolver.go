package cpp

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

// Resolver implements types.Resolver for C++ source.
//
// Ladder (top-down, first signal wins): T1 LSP, T2 annotation (explicit typed
// declaration — `Foo x`, `Foo x{...}`, `std::vector<int> v`, `const Foo& r`),
// T3 constructor (type derived from a ctor when the decl is `auto`, or a `new`
// expression — `auto x = Foo{}`, `auto p = new Foo()`, bare `new Foo()`), T4
// assignment (bare `y = x`), T6 heuristic (PascalCase / suffix name-shape),
// T7 unresolved (D-12 floor). No comment tier (T5, red-team B2).
type Resolver struct {
	store types.EffectiveReader

	// typeIndex is an optional stable-type-name → NodeID map (tests populate;
	// the production daemon leaves it nil). Only consumer is the D-13 scope
	// guard's target-file lookup; nil ⇒ tier 2-6 responses resolve Target=0.
	typeIndex map[string]graph.NodeID
}

// NewResolver constructs a C++ resolver over the given EffectiveReader seam.
// This replaces the Phase-62 LSP-conditional stub (v2.11 Phase 132).
func NewResolver(store types.EffectiveReader) *Resolver {
	return &Resolver{store: store}
}

// cppNewRE matches `new Foo` / `new ns::Foo<T>` — the heap-construction form.
var cppNewRE = regexp.MustCompile(`\bnew\s+([\w:]+(?:<[^>]*>)?)`)

// cppAutoCtorRE matches `auto x = Foo{...}` / `auto x = Foo(...)` — a ctor whose
// return type the `auto` declarator derives. The callee token is the type.
var cppAutoCtorRE = regexp.MustCompile(`^\s*(?:const\s+)?auto\b[*&\s]*\w+\s*=\s*([\w:]+(?:<[^>]*>)?)\s*[({]`)

// assignmentRE matches a bare `y = x` (RHS single identifier).
var assignmentRE = regexp.MustCompile(`^\s*[\w:]+\s*=\s*(\w+)\s*$`)

// cppQualifiers are storage-class / cv keywords stripped before the type token.
var cppQualifiers = map[string]bool{
	"const": true, "volatile": true, "static": true, "extern": true,
	"register": true, "inline": true, "mutable": true, "constexpr": true,
	"thread_local": true, "signed": true, "unsigned": true,
}

// ResolveChain walks the C++ ladder against the symbol referenced by
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

	// Tier 2: annotation — explicit typed declaration (`Foo x`, `Foo x{...}`).
	if t := parseAnnotation(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceAnnotation,
			types.ConfidenceAnnotation, "annotation")
	}

	// Tier 3: constructor — `auto x = Foo{}` / `new Foo()` (type via ctor).
	if t := parseConstructor(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceConstructor,
			types.ConfidenceConstructor, "constructor")
	}

	// Tier 4: assignment-flow — `y = x` within fixpoint scope.
	if t := parseAssignment(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceAssignment,
			types.ConfidenceAssignment, "assignment")
	}

	// Tier 6: heuristic — name-shape match (PascalCase / suffix).
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
		Reason:          "cpp: no signal for ref",
	}, nil
}

// tierResponse builds a ChainResponse for tiers 2-6 and applies the D-13 scope
// guard. C++ scope is flat (SameScope always true — program-wide linkage), so
// the guard never caps in v2.11; retained for structural symmetry + as the seam
// a future namespace-aware scope would activate.
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
			Reason:          "cpp: cross-scope chain (D-13) — not resolved in v1",
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

// ResolveSymbol delegates to ResolveChain over a synthetic single-token request.
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

// normalizeCppType reduces a C++ type expression to its bare type name: strips
// pointer/reference sigils, template arguments (`Foo<T>` → `Foo`) and namespace
// qualification (`ns::Foo` → `Foo`, last segment). Returns "" if the result is
// not a plausible identifier.
func normalizeCppType(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("*", "", "&", "").Replace(s)
	if i := strings.IndexByte(s, '<'); i >= 0 { // drop template args
		s = s[:i]
	}
	if i := strings.LastIndex(s, "::"); i >= 0 { // last qualified segment
		s = s[i+2:]
	}
	s = strings.TrimSpace(s)
	if isIdent(s) {
		return s
	}
	return ""
}

// parseAnnotation extracts the declared type from an explicit typed C++
// declaration. Returns "" for an `auto` declarator (constructor tier handles
// it), a bare assignment, or a single identifier.
func parseAnnotation(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return ""
	}
	// Cut at the initializer boundary: `Foo x{5}`/`Foo x = e`/`Foo x(1)` → decl.
	if i := strings.IndexAny(sig, "={("); i >= 0 {
		sig = strings.TrimSpace(sig[:i])
	}
	fields := strings.Fields(sig)
	// Strip leading qualifiers.
	for len(fields) > 0 && cppQualifiers[fields[0]] {
		fields = fields[1:]
	}
	if len(fields) == 0 || fields[0] == "auto" || fields[0] == "new" {
		return "" // auto → constructor tier
	}
	if len(fields) < 2 {
		return "" // need type + declarator
	}
	// The type portion is everything but the trailing declarator token.
	typePart := strings.Join(fields[:len(fields)-1], " ")
	return normalizeCppType(typePart)
}

// parseConstructor extracts the type from a ctor-derived declaration: a `new`
// expression (`new Foo()`) or an `auto`-declared ctor call (`auto x = Foo{}`).
func parseConstructor(sig string) string {
	if sig == "" {
		return ""
	}
	if m := cppNewRE.FindStringSubmatch(sig); len(m) >= 2 {
		return normalizeCppType(m[1])
	}
	if m := cppAutoCtorRE.FindStringSubmatch(sig); len(m) >= 2 {
		return normalizeCppType(m[1])
	}
	return ""
}

// parseAssignment extracts the RHS identifier of a bare `y = x`. Typed decls
// (two-token LHS), ctor calls, and `new`/`auto` forms do not match.
func parseAssignment(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" || !strings.Contains(sig, "=") {
		return ""
	}
	if strings.Contains(sig, "(") || strings.Contains(sig, "{") || strings.Contains(sig, "new ") {
		return ""
	}
	if strings.HasPrefix(sig, "auto") {
		return ""
	}
	if m := assignmentRE.FindStringSubmatch(sig); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// suffixRule maps a recognised identifier suffix to its full-form type name.
type suffixRule struct{ short, long string }

// cppSuffixRules is sorted ascending by short (CR-03 determinism).
var cppSuffixRules = []suffixRule{
	{short: "Cfg", long: "Config"},
	{short: "Conn", long: "Connection"},
	{short: "Ctx", long: "Context"},
	{short: "Mgr", long: "Manager"},
	{short: "Ptr", long: "Pointer"},
}

// guessFromName implements the C++ heuristic: normalize the name (strip
// template/qualifier), try the suffix rules, then a PascalCase self-match
// (a `Widget`-shaped identifier is a weak signal it IS a `Widget`).
func guessFromName(name string) string {
	return guessFromNameWithRules(name, cppSuffixRules)
}

// guessFromNameWithRules is the test-seam variant (CR-03: rules sorted by short).
func guessFromNameWithRules(name string, rules []suffixRule) string {
	if name == "" {
		return ""
	}
	norm := normalizeCppType(name)
	if norm == "" {
		return ""
	}
	for _, r := range rules {
		if strings.HasSuffix(norm, r.short) && len(norm) > len(r.short) {
			prefix := strings.TrimSuffix(norm, r.short)
			if prefix == "" {
				return r.long
			}
			return strings.ToUpper(prefix[:1]) + prefix[1:] + r.long
		}
	}
	// PascalCase self-match: first char upper + at least one lower (excludes
	// ALL_CAPS macro-style constants) + len > 2.
	if len(norm) > 2 && unicode.IsUpper(rune(norm[0])) && strings.IndexFunc(norm, unicode.IsLower) >= 0 {
		return norm
	}
	return ""
}

// isIdent reports whether s is a syntactically plausible C++ identifier.
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
