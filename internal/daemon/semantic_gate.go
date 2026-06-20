package daemon

// semantic_gate.go — Phase 81 Plan 04 composition-root gate helpers (ABLATE-06,
// D-02/D-03/D-04). These factor the `effSemanticDisabled` resolution and the
// gated lookup/cfgGate construction into small pure helpers so the back-channel
// semantic read consumers (symbols / health / repomap, and the guardrail
// middleware lookup) can be uniformly forced to integ.NoopLookup{} + a DISABLED
// ConfigGate without scattering per-callsite flag checks (RESEARCH Anti-Patterns).
//
// Doctrine (mirrors Phase 76's effDisableLSP at daemon.go:287):
//
//   - effSemanticDisabled is resolved ONCE at the composition root, OR'ing the
//     config field (cfg.SemanticIndex.BenchDisabled) with the profile field
//     (activeProfile.DisableSemanticSubsystem). Default-off (D-03).
//
//   - Build-but-block (D-04): the gate forces Noop on the READ consumers but
//     does NOT gate the bundle-build guard — the store STILL gets built.
//
//   - Pitfall 4: the gate DISABLES the ConfigGate (not just the lookup) so
//     integ.ChooseSource hits its FIRST priority and yields SourceTreeSitter
//     (the steady-state v1.9 path), NOT SourceFallback + index_disabled.

import (
	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/profile"
	"github.com/agenthands/helix/internal/semantic/integ"
)

// resolveSemanticDisabled resolves effSemanticDisabled ONCE at the composition
// root (D-02), OR'ing the koanf-resolved config field with the active profile
// field. Precedence is already collapsed upstream (CLI > profile > default-off,
// D-03); this is the final OR. Nil-safe on both operands.
func resolveSemanticDisabled(cfg *config.SerenaConfig, activeProfile *profile.Profile) bool {
	benchDisabled := cfg != nil && cfg.SemanticIndex.BenchDisabled
	profileDisabled := activeProfile != nil && activeProfile.DisableSemanticSubsystem
	return benchDisabled || profileDisabled
}

// gatedSymbolsLookupFn builds the SemanticLookup closure handed to the symbols /
// health back-channel read consumers. Under the gate it returns integ.NoopLookup{}
// (build-but-block, D-04) EVEN when the bundle is non-nil; ungated it returns the
// real adapter (normalized to NoopLookup{} when the bundle is nil, as before).
func gatedSymbolsLookupFn(sBndl *semanticBundle, effSemanticDisabled bool) func() integ.SemanticLookup {
	return func() integ.SemanticLookup {
		if effSemanticDisabled {
			return integ.NoopLookup{}
		}
		if sBndl != nil {
			return sBndl.integLookupAccessor()
		}
		return integ.NoopLookup{}
	}
}

// gatedCfgGate builds the production ConfigGate for the ChooseSource consumers.
// It DISABLES the gate (not just the lookup) under effSemanticDisabled so the
// priority-1 arm of integ.ChooseSource wins -> SourceTreeSitter (Pitfall 4).
func gatedCfgGate(cfg *config.SerenaConfig, effSemanticDisabled bool) *daemonCfgGate {
	enabled := cfg != nil && cfg.SemanticIndex.Enabled && !effSemanticDisabled
	return &daemonCfgGate{enabled: enabled}
}

// backgroundSemanticReadsDisabled extends the gate doctrine to the
// daemon-INTERNAL background read pipelines (Phase 81 Plan 07, CR-01).
//
// Plan 04 gated the TOOL-FACING SemanticLookup hand-outs + the SemanticSkill
// accessor block via gatedSymbolsLookupFn / gatedCfgGate above. But the
// daemon-internal activation pipelines reach the COUNTED DuckDB read chokepoint
// (s.queryContext / s.queryRowContext) without any gate:
//
//   - SetFileFactStore(semanticStore) wires the FileFactStore that drives
//     GetLatestFileFact / LatestCommittedSnapshot reads.
//   - The SetActivateCallback read-drivers — ScheduleInitialExtraction,
//     live.startWorkspace, rank.ensureScheduler, compactBndl.ensureCompactor,
//     sBndl.ensureRetrieval — drive QueryEffectiveAdjacency / CountStaleScoreRows
//     and the initial-walk extraction reads.
//
// Under D-04 build-but-block the store + bundle are NON-nil, so the existing
// nil-checks do NOT stop these reads on a store-ON no_semantic arm. This
// predicate is the single-resolution-point that says "the background read
// pipelines are part of the gated surface" — mirroring the gatedSymbolsLookupFn
// / gatedCfgGate doctrine, so the gating is documented intent, not an orphan
// inline `!effSemanticDisabled` scattered across five call sites.
//
// CRITICAL D-04 invariant: this gates the READ-DRIVERS, NOT construction. The
// store-Open guard (daemon.go ~324 `if cfg.SemanticIndex.Enabled`) and the
// newSemanticBundle guard (~518) MUST stay free of effSemanticDisabled so the
// zero-reads proof (the bench store-ON cell) is non-vacuous.
//
// WR-04 — SCOPE OF THE helix_semantic_store_reads_total PROOF: the bench
// zero-reads gate (bench/runtime/cell.go assertNoSemanticReads) observes ONLY
// the counter incremented by internal/semantic/store/effective_graph.go's
// counting wrappers (s.queryContext / s.queryRowContext). Tx-scoped raw reads
// that bypass those wrappers (e.g. snapshot.go / overlay.go tx-scoped
// *sql.Tx.QueryContext / QueryRowContext) are NOT counted. The guarantee this
// gate proves is therefore "no read reached the COUNTED effective-graph path on
// the no_semantic arm" — which is exactly the read surface this predicate
// un-wires — not "the DuckDB file was provably untouched by every path". If a
// future background read path is added, route it through the counting wrappers
// (or extend the gate here) rather than trusting the counter as a universal
// read-detector.
func backgroundSemanticReadsDisabled(effSemanticDisabled bool) bool {
	return effSemanticDisabled
}
