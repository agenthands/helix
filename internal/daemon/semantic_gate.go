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
