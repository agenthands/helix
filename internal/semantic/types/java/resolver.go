package java

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

// Resolver implements types.Resolver for Java source.
//
// Ladder (top-down, first signal wins): T1 LSP, T2 annotation (typed decl —
// `Foo x`, `final List<String> items`, `@Override`/`@Inject` stripped),
// T3 constructor (`new Foo()`, diamond `new Foo<>()`), T4 assignment
// (`var x = y`, bare `x = y`), T6 heuristic (PascalCase / *Impl / Abstract* /
// get*/set* / suffix), T7 unresolved (D-12 floor). No comment tier (T5, B2).
type Resolver struct {
	store types.EffectiveReader

	// typeIndex is an optional stable-type-name → NodeID map (tests populate;
	// production leaves nil). Only consumer is the D-13 package guard's
	// target-file lookup; nil ⇒ tier 2-6 responses resolve Target=0.
	typeIndex map[string]graph.NodeID
}

// NewResolver constructs a Java resolver over the given EffectiveReader seam.
// This replaces the Phase-62 LSP-conditional stub (v2.11 Phase 134).
func NewResolver(store types.EffectiveReader) *Resolver {
	return NewResolverWithIndex(store, nil)
}

// NewResolverWithIndex constructs a Java resolver over the given EffectiveReader
// seam plus an optional stable-type-name → NodeID index. The v2.12 Phase 136
// daemon producer injects the batch nameToNode index so the annotation tier
// binds a real RESOLVES_TO target; callers with no index pass nil.
func NewResolverWithIndex(store types.EffectiveReader, idx map[string]graph.NodeID) *Resolver {
	return &Resolver{store: store, typeIndex: idx}
}

// javaNewRE matches `new Foo` / `new java.util.List<...>` — construction.
var javaNewRE = regexp.MustCompile(`\bnew\s+([\w.]+(?:<[^>]*>)?)`)

// assignmentRE matches `var x = y` / `x = y` (RHS single identifier).
var assignmentRE = regexp.MustCompile(`^\s*(?:var\s+)?\w+\s*=\s*(\w+)\s*$`)

// leadingAnnotRE strips a leading `@Annotation` / `@Annotation(...)` decoration.
var leadingAnnotRE = regexp.MustCompile(`^\s*@\w+(?:\([^)]*\))?\s*`)

// javaModifiers are access / storage modifiers stripped before the type token.
var javaModifiers = map[string]bool{
	"public": true, "private": true, "protected": true, "static": true,
	"final": true, "abstract": true, "synchronized": true, "transient": true,
	"volatile": true, "native": true, "strictfp": true, "default": true,
}

// ResolveChain walks the Java ladder against the symbol referenced by
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

	// Tier 2: annotation — typed declaration.
	// Option A (v2.12 Phase 136): a co-driver-supplied declared type in
	// req.ChainTokens[0] takes precedence over re-parsing the signature;
	// an empty token falls back to parseAnnotation.
	annType := parseAnnotation(sym.Signature)
	if len(req.ChainTokens) > 0 && req.ChainTokens[0] != "" {
		annType = req.ChainTokens[0]
	}
	if annType != "" {
		return r.tierResponse(req, annType, types.EvidenceAnnotation,
			types.ConfidenceAnnotation, "annotation")
	}

	// Tier 3: constructor — `new Foo()` / diamond `new Foo<>()`.
	if t := parseConstructor(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceConstructor,
			types.ConfidenceConstructor, "constructor")
	}

	// Tier 4: assignment-flow — `var x = y` / bare `x = y`.
	if t := parseAssignment(sym.Signature); t != "" {
		return r.tierResponse(req, t, types.EvidenceAssignment,
			types.ConfidenceAssignment, "assignment")
	}

	// Tier 6: heuristic — PascalCase / *Impl / Abstract* / get* / suffix.
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
		Reason:          "java: no LSP fact + no static signal for ref",
	}, nil
}

// tierResponse builds a ChainResponse for tiers 2-6 and applies the D-13
// package guard: a target in a different package (directory proxy) caps at
// ConfidenceComment + unresolved (M1: Java resolves intra-package only).
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
			Reason:          "java: cross-package chain (D-13) — intra-package only in v1",
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

// normalizeJavaType reduces a Java type expression to its bare type name:
// strips array `[]`, generic args (`List<String>` → `List`) and package
// qualification (`java.util.List` → `List`, last segment). Returns "" if the
// result is not a plausible identifier.
func normalizeJavaType(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "[]", "")
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

// parseAnnotation extracts the type from a Java typed declaration. Leading
// `@Annotation` decorations and access modifiers are stripped; `var`/`new`
// return "" (assignment / constructor tiers).
func parseAnnotation(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return ""
	}
	for {
		stripped := leadingAnnotRE.ReplaceAllString(sig, "")
		if stripped == sig {
			break
		}
		sig = strings.TrimSpace(stripped)
	}
	if i := strings.IndexAny(sig, "={("); i >= 0 {
		sig = strings.TrimSpace(sig[:i])
	}
	fields := strings.Fields(sig)
	for len(fields) > 0 && javaModifiers[fields[0]] {
		fields = fields[1:]
	}
	if len(fields) == 0 || fields[0] == "var" || fields[0] == "new" {
		return ""
	}
	if len(fields) < 2 {
		return ""
	}
	typePart := strings.Join(fields[:len(fields)-1], " ")
	return normalizeJavaType(typePart)
}

// parseConstructor extracts the type from a `new Foo()` / diamond construction.
func parseConstructor(sig string) string {
	if sig == "" {
		return ""
	}
	if m := javaNewRE.FindStringSubmatch(sig); len(m) >= 2 {
		return normalizeJavaType(m[1])
	}
	return ""
}

// parseAssignment extracts the RHS identifier of `var x = y` / bare `x = y`.
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

// javaSuffixRules is sorted ascending by short (CR-03 determinism).
var javaSuffixRules = []suffixRule{
	{short: "Mgr", long: "Manager"},
	{short: "Repo", long: "Repository"},
	{short: "Svc", long: "Service"},
}

// guessFromName implements the Java heuristic: *Impl (strip → interface),
// Abstract* (strip → base), get*/set* (property → type), suffix rules, then a
// PascalCase self-match. Normalizes generics/qualifier first.
func guessFromName(name string) string {
	if name == "" {
		return ""
	}
	norm := normalizeJavaType(name)
	if norm == "" {
		return ""
	}
	// *Impl → the implemented interface (UserServiceImpl → UserService).
	if strings.HasSuffix(norm, "Impl") && len(norm) > len("Impl") {
		return strings.TrimSuffix(norm, "Impl")
	}
	// Abstract* → the concrete base (AbstractHandler → Handler).
	if strings.HasPrefix(norm, "Abstract") && len(norm) > len("Abstract") {
		rest := strings.TrimPrefix(norm, "Abstract")
		if unicode.IsUpper(rune(rest[0])) {
			return rest
		}
	}
	// get*/set* → the property type (getUser → User).
	for _, p := range []string{"get", "set"} {
		if strings.HasPrefix(norm, p) && len(norm) > len(p) {
			rest := norm[len(p):]
			if unicode.IsUpper(rune(rest[0])) {
				return rest
			}
		}
	}
	// Suffix rules (userSvc → UserService).
	for _, r := range javaSuffixRules {
		if strings.HasSuffix(norm, r.short) && len(norm) > len(r.short) {
			prefix := strings.TrimSuffix(norm, r.short)
			if prefix == "" {
				return r.long
			}
			return strings.ToUpper(prefix[:1]) + prefix[1:] + r.long
		}
	}
	// PascalCase self-match (first char upper + at least one lower + len > 2).
	if len(norm) > 2 && unicode.IsUpper(rune(norm[0])) && strings.IndexFunc(norm, unicode.IsLower) >= 0 {
		return norm
	}
	return ""
}

// isIdent reports whether s is a syntactically plausible Java identifier.
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' && r != '$' {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '$' {
			return false
		}
	}
	return true
}
