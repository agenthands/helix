package typescript

import (
	"context"
	"regexp"
	"strings"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

// Resolver implements types.Resolver for TypeScript / JavaScript.
type Resolver struct {
	store     types.EffectiveReader
	typeIndex map[string]graph.NodeID // optional name → node target seam
}

// NewResolver constructs a TS/JS resolver over the given EffectiveReader seam.
func NewResolver(store types.EffectiveReader) *Resolver {
	return NewResolverWithIndex(store, nil)
}

// NewResolverWithIndex constructs a TS/JS resolver over the given
// EffectiveReader seam plus an optional stable-type-name → NodeID index. The
// v2.12 Phase 136 daemon producer injects the batch nameToNode index so the
// annotation tier binds a real RESOLVES_TO target; callers with no index pass nil.
func NewResolverWithIndex(store types.EffectiveReader, idx map[string]graph.NodeID) *Resolver {
	return &Resolver{store: store, typeIndex: idx}
}

// `let x: Foo` / `const x: Foo` / `var x: Foo`.
var tsAnnotationRE = regexp.MustCompile(`(?:let|const|var)?\s*\w+\s*:\s*([A-Za-z_][A-Za-z0-9_]*)`)

// `new Foo()` / `... = new Foo(`.
var tsConstructorRE = regexp.MustCompile(`\bnew\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// `y = x` (right-hand identifier; no annotation, no `new`).
var tsAssignmentRE = regexp.MustCompile(`(?:^|\b)\w+\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\s*$`)

// ResolveChain walks the 7-tier ladder.
func (r *Resolver) ResolveChain(ctx context.Context, req types.ChainRequest) (types.ChainResponse, error) {
	// Tier 1: LSP.
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

	// Tier 2: annotation `: Foo`. Option A (v2.12 Phase 136): a co-driver-
	// supplied declared type in req.ChainTokens[0] takes precedence; an empty
	// token falls back to parseTSAnnotation.
	annType := parseTSAnnotation(sym.Signature)
	if len(req.ChainTokens) > 0 && req.ChainTokens[0] != "" {
		annType = req.ChainTokens[0]
	}
	if annType != "" {
		return r.tier(req, sym, annType, types.EvidenceAnnotation, types.ConfidenceAnnotation, "annotation")
	}
	// Tier 3: constructor `new Foo()`.
	if t := parseTSConstructor(sym.Signature); t != "" {
		return r.tier(req, sym, t, types.EvidenceConstructor, types.ConfidenceConstructor, "constructor")
	}
	// Tier 4: assignment `y = x`.
	if t := parseTSAssignment(sym.Signature); t != "" {
		return r.tier(req, sym, t, types.EvidenceAssignment, types.ConfidenceAssignment, "assignment")
	}
	// Tier 5: TSDoc / JSDoc comment.
	if t := ParseTSDocType(sym.DocComment); t != "" {
		return r.tier(req, sym, t, types.EvidenceComment, types.ConfidenceComment, "comment.tsdoc")
	}
	// Tier 6: heuristic name-shape.
	if t := guessFromName(sym.StableKey); t != "" {
		return r.tier(req, sym, t, types.EvidenceHeuristic, types.ConfidenceHeuristic, "heuristic")
	}

	return types.ChainResponse{
		Resolved:        false,
		Confidence:      types.ConfidenceUnknown,
		EvidenceKind:    types.EvidenceUnknown,
		ValidationState: "unresolved",
		Source:          "unknown",
		Reason:          "ts: no signal for ref",
	}, nil
}

func (r *Resolver) tier(req types.ChainRequest, sym types.SymbolFact, typeName string,
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
			Reason:          "ts: cross-package chain (D-13) — not resolved in v1",
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

func parseTSAnnotation(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" || strings.Contains(sig, "new ") || !strings.Contains(sig, ":") {
		return ""
	}
	m := tsAnnotationRE.FindStringSubmatch(sig)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func parseTSConstructor(sig string) string {
	if sig == "" {
		return ""
	}
	m := tsConstructorRE.FindStringSubmatch(sig)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func parseTSAssignment(sig string) string {
	sig = strings.TrimSpace(sig)
	if sig == "" || strings.Contains(sig, ":") || strings.Contains(sig, "(") {
		return ""
	}
	if !strings.Contains(sig, "=") {
		return ""
	}
	m := tsAssignmentRE.FindStringSubmatch(sig)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// suffixRule maps a recognised camelCase suffix to its full-form type
// name. Phase 62 CR-03 closure: sort-before-iterate locks deterministic
// match priority regardless of future suffix additions that may alias
// one suffix as another's tail.
type suffixRule struct{ short, long string }

// tsSuffixRules is sorted ascending by short. Adding a rule MUST
// preserve the sort order so determinism remains structural. TS/JS
// convention: camelCase tokens. Today's set is overlap-free at the tail
// level; the structural sort guards future additions.
var tsSuffixRules = []suffixRule{
	{short: "Cfg", long: "Config"},
	{short: "Conn", long: "Connection"},
	{short: "Ctrl", long: "Controller"},
	{short: "Mgr", long: "Manager"},
	{short: "Repo", long: "Repository"},
	{short: "Svc", long: "Service"},
}

// guessFromName implements the heuristic name-shape match for TS/JS.
func guessFromName(name string) string {
	return guessFromNameWithRules(name, tsSuffixRules)
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
			prefix := strings.TrimSuffix(name, r.short)
			if prefix == "" {
				return r.long
			}
			return strings.ToUpper(prefix[:1]) + prefix[1:] + r.long
		}
	}
	return ""
}
