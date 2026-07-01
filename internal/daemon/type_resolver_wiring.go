// Package daemon: Phase 62 P05 type-resolver dispatcher wiring.
//
// type_resolver_wiring.go owns:
//
//   - typeStoreAdapter — adapts *semanticstore.Store into the
//     types.EffectiveReader narrow seam (D-11). The Store-level
//     QueryEffectiveEdges / QueryEffectiveSymbols are Phase 57 stubs
//     returning empty slices today; the adapter normalises their
//     `[]any` shape to the typed types.EdgeFact / types.SymbolFact
//     contract.
//
//   - buildTypeResolverDispatcher — constructs a *types.Dispatcher with
//     the 7 v1 language entries (go / typescript / javascript / python /
//     java / php / ruby) per D-11 and the JS-aliases-TS rule.
//
//   - SetSemanticGraph — optional setter the daemon exposes for
//     downstream consumers (Phase 64 will wrap the dispatcher in MCP
//     tools). Per RESEARCH Open Question 4, ranker + resolver are
//     bundled in a single setter to keep the consumer-side seam narrow.
//
// Per CLAUDE.md the daemon owns construction; the dispatcher and adapter
// hold no goroutine state.
package daemon

import (
	"context"

	"github.com/agenthands/helix/internal/semantic/graph"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/semantic/types"
	typesc "github.com/agenthands/helix/internal/semantic/types/c"
	typescsharp "github.com/agenthands/helix/internal/semantic/types/c_sharp"
	typescpp "github.com/agenthands/helix/internal/semantic/types/cpp"
	typesgo "github.com/agenthands/helix/internal/semantic/types/golang"
	typesjava "github.com/agenthands/helix/internal/semantic/types/java"
	typeskotlin "github.com/agenthands/helix/internal/semantic/types/kotlin"
	typesphp "github.com/agenthands/helix/internal/semantic/types/php"
	typespython "github.com/agenthands/helix/internal/semantic/types/python"
	typesruby "github.com/agenthands/helix/internal/semantic/types/ruby"
	typesrust "github.com/agenthands/helix/internal/semantic/types/rust"
	typests "github.com/agenthands/helix/internal/semantic/types/typescript"
)

// typeStoreAdapter adapts the *semanticstore.Store query helpers (Phase 57
// stubs at the moment — they return empty `[]any` slices) into the
// types.EffectiveReader contract that per-language resolvers consume.
//
// Both methods are nil-safe: when the underlying store is unavailable the
// adapter returns empty results without erroring, so the resolver simply
// falls through to its lower tiers.
type typeStoreAdapter struct {
	store *semanticstore.Store
}

func newTypeStoreAdapter(s *semanticstore.Store) *typeStoreAdapter {
	return &typeStoreAdapter{store: s}
}

// QueryEffectiveEdges normalises the store's stub `[]any` return into a
// typed slice of types.EdgeFact. The Phase 57 implementation always
// returns an empty slice; once the schema lands in a future phase, this
// adapter is the one place that needs an updated row → struct shape
// translation.
func (a *typeStoreAdapter) QueryEffectiveEdges(ctx context.Context, q types.EdgeQuery) ([]types.EdgeFact, error) {
	if a == nil || a.store == nil {
		return nil, nil
	}
	rows, err := a.store.QueryEffectiveEdges(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	out := make([]types.EdgeFact, 0, len(rows))
	// Phase 57 schema returns []any{} — the loop body is intentionally
	// reachable only when a future schema lands real rows. Until then the
	// loop is a no-op; we keep it to make the future shape obvious.
	for _, r := range rows {
		if e, ok := r.(types.EdgeFact); ok {
			out = append(out, e)
		}
	}
	return out, nil
}

// QueryEffectiveSymbol returns the LIVE symbol fact for (repo, node). v2.11
// Phase 130: now backed by Store.QueryEffectiveSymbolFact (the real
// committed-snapshot row: Signature / StableKey / Kind / Language / FilePath),
// which unblocks the per-language resolvers' tiers 2-6 (previously this
// returned an empty SymbolFact, starving every resolver to Tier 7). On a miss
// it degrades to NodeID-only (Tier 7) — the D-12 invariant still holds.
func (a *typeStoreAdapter) QueryEffectiveSymbol(ctx context.Context, repoID string, n graph.NodeID) (types.SymbolFact, error) {
	sf, filePath, ok, err := a.store.QueryEffectiveSymbolFact(ctx, repoID, uint64(n))
	if err != nil || !ok {
		return types.SymbolFact{NodeID: n}, err
	}
	return types.SymbolFact{
		NodeID:    n,
		Language:  sf.Language,
		Kind:      sf.Kind,
		StableKey: sf.StableKey,
		FilePath:  filePath,
		Signature: sf.Signature,
	}, nil
}

// Per-language Resolver constructor wrappers.
//
// daemon.go calls types.NewDispatcher directly with these wrappers so the
// dispatcher-registration site is grep-anchored at the bootstrap step.
// The wrappers keep daemon.go free of the per-language package imports.
func goTypeResolver(r types.EffectiveReader) types.Resolver { return typesgo.NewResolver(r) }
func tsTypeResolver(r types.EffectiveReader) types.Resolver { return typests.NewResolver(r) }
func pyTypeResolver(r types.EffectiveReader) types.Resolver { return typespython.NewResolver(r) }
func javaTypeStub(r types.EffectiveReader) types.Resolver   { return typesjava.NewStub(r) }
func phpTypeStub() types.Resolver                           { return typesphp.NewStub() }
func rubyTypeStub() types.Resolver                          { return typesruby.NewStub() }
func csharpTypeResolver(r types.EffectiveReader) types.Resolver { return typescsharp.NewResolver(r) }
func rustTypeStub(r types.EffectiveReader) types.Resolver   { return typesrust.NewStub(r) }
func cTypeResolver(r types.EffectiveReader) types.Resolver { return typesc.NewResolver(r) }
func cppTypeResolver(r types.EffectiveReader) types.Resolver { return typescpp.NewResolver(r) }
func kotlinTypeStub(r types.EffectiveReader) types.Resolver { return typeskotlin.NewStub(r) }

// SetSemanticGraph attaches the daemon's rank engine + type-resolver
// dispatcher to a downstream consumer. Phase 64 (semantic MCP tools) will
// invoke this setter at registration time. In Phase 62 the setter is
// OPTIONAL — when no consumer is wired the dispatcher is merely logged at
// bootstrap and held on the daemon for later attachment.
//
// The pair is bundled per RESEARCH Open Question 4: the rank engine and
// the type-resolver dispatcher are co-consumers (the same MCP tool that
// returns ranked nodes also returns resolved type edges), so a single
// setter keeps the consumer-side seam narrow.
//
// Nil-safe on every input.
func (d *Daemon) SetSemanticGraph(ranker any, resolver types.Resolver) {
	if d == nil {
		return
	}
	d.typeResolver = resolver
	// `ranker` is intentionally typed `any` here so this setter does not
	// import the rank engine's concrete type into the public daemon
	// surface. Phase 64 will narrow this to a typed ranker interface.
	d.semanticGraphRanker = ranker
}

// TypeResolver returns the Phase 62 P05 type-resolver dispatcher, or nil
// when the semantic store is unavailable. Phase 64 consumers MUST nil-check.
func (d *Daemon) TypeResolver() types.Resolver {
	if d == nil {
		return nil
	}
	return d.typeResolver
}
