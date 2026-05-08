// source_select.go — ChooseSource priority-ladder helper for the SPEC §24
// source field. Phase 65 D-04 + D-05 + Pitfall §3.
//
// Doctrine:
//
//   - Config disabled is the FIRST gate. When semantic_index is off,
//     consumers emit SourceTreeSitter (the v1.9 steady state), NOT
//     SourceFallback with reason=index_disabled. D-04 + Pitfall §3.
//
//   - Defensive index_disabled is reserved for the wiring-bug path: config
//     says enabled, but the SemanticLookup the daemon handed us is the
//     NoopLookup (Available()==false). This should never happen in steady
//     state; if it does, surface as SourceFallback +
//     FallbackReasonIndexDisabled so the agent sees a closed-enum reason.
//
//   - Error classification is the ONLY path from a Go error to a closed-enum
//     FallbackReason. ClassifyLookupErr (source.go) uses errors.Is
//     exclusively; ChooseSource never inspects raw error text. WR-NEW-01.
//
//   - The success arm is unconditional: cfg enabled, lookup available, no
//     err → SourceSemantic, "" (no fallback_reason).
//
// ChooseSource is pure. It performs no I/O, never triggers indexing, and is
// safe to call on every MCP request. Threat T-65-04-03 (M-cold) disposition
// is "accept" because of this purity.
package integ

// ConfigGate is the minimal contract ChooseSource needs from the daemon's
// config layer. Production-side, internal/daemon implements this against
// koanf-resolved Config.SemanticIndex.Enabled. Test-side, a tiny test
// double (`fakeCfg`) implements it.
type ConfigGate interface {
	// SemanticIndexEnabled reports whether the semantic_index feature flag
	// is enabled in effective config (CLI > project > user > profile-default
	// precedence already resolved).
	SemanticIndexEnabled() bool
}

// ChooseSource implements the Pitfall §3 priority ladder. It returns the
// (Source, FallbackReason) pair that the consumer must place on its MCP
// envelope.
//
// Priority order (first match wins):
//
//  1. cfg == nil OR !cfg.SemanticIndexEnabled()
//     → SourceTreeSitter, "" (D-04: steady-state v1.9 path)
//  2. lookup == nil OR !lookup.Available()
//     → SourceFallback, FallbackReasonIndexDisabled (defensive D-05)
//  3. err != nil
//     → SourceFallback, ClassifyLookupErr(err) (errors.Is ladder)
//  4. else
//     → SourceSemantic, ""
//
// ChooseSource never inspects raw error text; the only path from a Go error
// to a closed-enum FallbackReason is ClassifyLookupErr. WR-NEW-01.
func ChooseSource(cfg ConfigGate, lookup SemanticLookup, err error) (Source, FallbackReason) {
	// 1. Config gate (Pitfall §3 + D-04). Highest priority; even if the
	// lookup is unavailable or an error is in flight, "feature off" wins.
	if cfg == nil || !cfg.SemanticIndexEnabled() {
		return SourceTreeSitter, ""
	}
	// 2. Defensive wiring-bug path (D-05). Config says enabled but the
	// daemon never replaced NoopLookup, OR the caller passed nil.
	if lookup == nil || !lookup.Available() {
		return SourceFallback, FallbackReasonIndexDisabled
	}
	// 3. Error classifier path. ClassifyLookupErr uses errors.Is only —
	// raw error text never leaves this package (WR-NEW-01).
	if err != nil {
		return SourceFallback, ClassifyLookupErr(err)
	}
	// 4. Success path.
	return SourceSemantic, ""
}
