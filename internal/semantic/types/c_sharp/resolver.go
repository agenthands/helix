package c_sharp

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

// Resolver implements types.Resolver for C# source.
//
// Ladder (top-down, first signal wins): T1 LSP, T2 annotation (explicit typed
// decl — `Foo x`, `private readonly List<int> items`, `Foo[] arr`; attributes
// stripped; `var` excluded), T3 constructor (`new Foo()`, object initializers,
// record ctors, `var x = new Foo()`), T4 assignment (`var x = y`, bare `x = y`),
// T6 heuristic (PascalCase / I-prefix interface / suffix name-shape), T7
// unresolved (D-12 floor). No comment tier (T5, red-team B2).
type Resolver struct {
	store types.EffectiveReader

	// typeIndex is an optional stable-type-name → NodeID map (tests populate;
	// production leaves nil). Only consumer is the D-13 namespace guard's
	// target-file lookup; nil ⇒ tier 2-6 responses resolve Target=0.
	typeIndex map[string]graph.NodeID
}

// NewResolver constructs a C# resolver over the given EffectiveReader seam.
// This replaces the Phase-62 LSP-conditional stub (v2.11 Phase 133).
func NewResolver(store types.EffectiveReader) *Resolver {
	return &Resolver{store: store}
}

// csNewRE matches `new Foo` / `new ns.Foo<T>` — the construction form.
var csNewRE = regexp.MustCompile(`\bnew\s+([\w.]+(?:<[^>]*>)?)`)

// assignmentRE matches `var x = y` / `x = y` (RHS single identifier).
var assignmentRE = regexp.MustCompile(`^\s*(?:var\s+)?\w+\s*=\s*(\w+)\s*$`)

// leadingAttrRE strips a leading `[Attr(...)]` decoration group.
var leadingAttrRE = regexp.MustCompile(`^\s*\[[^\]]*\]\s*`)

// csModifiers are access / storage modifiers stripped before the type token.
var csModifiers = map[string]bool{
	"public": true, "private": true, "protected": true, "internal": true,
	"static": true, "readonly": true, "const": true, "sealed": true,
	"virtual": true, "override": true, "abstract": true, "volatile": true,
	"extern": true, "unsafe": true, "partial": true, "async": true,
	"ref": true, "out": true, "in": true, "params": true, "event": true,
}

// ResolveChain walks the C# ladder against the symbol referenced by
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

	sym, _ := r.store.QueryEffectiveSymbol(ctx, req.RepoID, req.RefNodeID)

	// Tier 2: annotation — explicit typed declaration.
	if t := parseAnnotation(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceAnnotation,
			types.ConfidenceAnnotation, "annotation")
	}

	// Tier 3: constructor — `new Foo()` / record ctor / `var x = new Foo()`.
	if t := parseConstructor(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceConstructor,
			types.ConfidenceConstructor, "constructor")
	}

	// Tier 4: assignment-flow — `var x = y` / bare `x = y`.
	if t := parseAssignment(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceAssignment,
			types.ConfidenceAssignment, "assignment")
	}

	// Tier 6: heuristic — PascalCase / I-prefix / suffix name-shape.
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
		Reason:          "c_sharp: no signal for ref",
	}, nil
}

// tierResponse builds a ChainResponse for tiers 2-6 and applies the D-13
// namespace guard: a target in a different namespace (directory approximation)
// caps at ConfidenceComment + unresolved (M1: C# resolves intra-namespace only).
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

	if targetFile != "" && !SameNamespace(req.FilePath, targetFile) {
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
			Reason:          "c_sharp: cross-namespace chain (D-13) — intra-namespace only in v1",
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

// normalizeCsType reduces a C# type expression to its bare type name: strips
// array `[]`, nullable `?`, generic args (`List<int>` → `List`) and namespace
// qualification (`System.String` → `String`, last segment). Returns "" if the
// result is not a plausible identifier.
func normalizeCsType(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "[]", "")
	s = strings.TrimSuffix(s, "?")
	if i := strings.IndexByte(s, '<'); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSpace(s)
	if isIdent(s) {
		return s
	}
	return ""
}

// parseAnnotation extracts the type from an explicit C# typed declaration.
// Attributes (`[Attr]`) are stripped; `var` returns "" (assignment tier).
func parseAnnotation(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return ""
	}
	// Strip leading attribute groups (possibly several).
	for {
		stripped := leadingAttrRE.ReplaceAllString(sig, "")
		if stripped == sig {
			break
		}
		sig = strings.TrimSpace(stripped)
	}
	// Cut at the initializer / body boundary.
	if i := strings.IndexAny(sig, "={("); i >= 0 {
		sig = strings.TrimSpace(sig[:i])
	}
	fields := strings.Fields(sig)
	for len(fields) > 0 && csModifiers[fields[0]] {
		fields = fields[1:]
	}
	if len(fields) == 0 || fields[0] == "var" || fields[0] == "new" {
		return "" // var → assignment tier; new → constructor tier
	}
	if len(fields) < 2 {
		return ""
	}
	typePart := strings.Join(fields[:len(fields)-1], " ")
	return normalizeCsType(typePart)
}

// parseConstructor extracts the type from a `new` construction (incl. object
// initializers and record ctors), e.g. `new Foo()`, `var x = new Foo { }`.
func parseConstructor(sig string) string {
	if sig == "" {
		return ""
	}
	if m := csNewRE.FindStringSubmatch(sig); len(m) >= 2 {
		return normalizeCsType(m[1])
	}
	return ""
}

// parseAssignment extracts the RHS identifier of `var x = y` / bare `x = y`.
// Typed decls, ctor calls, and `new` forms do not match.
func parseAssignment(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" || !strings.Contains(sig, "=") {
		return ""
	}
	if strings.Contains(sig, "(") || strings.Contains(sig, "{") || strings.Contains(sig, "new ") {
		return ""
	}
	if m := assignmentRE.FindStringSubmatch(sig); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// suffixRule maps a recognised identifier suffix to its full-form type name.
type suffixRule struct{ short, long string }

// csSuffixRules is sorted ascending by short (CR-03 determinism).
var csSuffixRules = []suffixRule{
	{short: "Ctrl", long: "Controller"},
	{short: "Mgr", long: "Manager"},
	{short: "Repo", long: "Repository"},
	{short: "Svc", long: "Service"},
}

// guessFromName implements the C# heuristic: normalize, try suffix rules, then
// a PascalCase self-match (incl. `I`-prefix interfaces like `IService`).
func guessFromName(name string) string {
	return guessFromNameWithRules(name, csSuffixRules)
}

// guessFromNameWithRules is the test-seam variant (CR-03: rules sorted by short).
func guessFromNameWithRules(name string, rules []suffixRule) string {
	if name == "" {
		return ""
	}
	norm := normalizeCsType(name)
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
	// PascalCase self-match (first char upper + at least one lower + len > 2).
	// Covers `I`-prefix interfaces (`IService`) and PascalCase type-shaped names.
	if len(norm) > 2 && unicode.IsUpper(rune(norm[0])) && strings.IndexFunc(norm, unicode.IsLower) >= 0 {
		return norm
	}
	return ""
}

// isIdent reports whether s is a syntactically plausible C# identifier.
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
