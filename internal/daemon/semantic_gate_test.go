package daemon

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/profile"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/skill/semantic"
)

// semantic_gate_test.go — Phase 81 Plan 04 composition-root gate tests.
//
// These prove the `effSemanticDisabled` resolution + the build-but-block (D-04)
// forcing of integ.NoopLookup{} and a DISABLED ConfigGate on every back-channel
// semantic read consumer (symbols / health / repomap), plus the gating of the
// SemanticSkill Set*Accessor block (the non-ChooseSource consumers, Pitfall 3).
//
// The first three tests drive small unexported helpers (resolveSemanticDisabled
// / gatedSymbolsLookupFn / gatedCfgGate) directly so they need no booted daemon,
// mirroring how Phase 76's no_lsp gating is unit-tested. The accessor-gating
// test (Task 2) boots daemon.New and snapshots the SemanticSkill via the Phase
// 74 WiredAccessorsForTest seam.

// TestEffSemanticDisabled is the D-02/D-03 resolution truth table:
// effSemanticDisabled is true when EITHER the config field
// (cfg.SemanticIndex.BenchDisabled) OR the profile field
// (activeProfile.DisableSemanticSubsystem) is set; false when neither
// (default-off / build-but-block).
func TestEffSemanticDisabled(t *testing.T) {
	cases := []struct {
		name         string
		benchDisable bool
		profDisable  bool
		want         bool
	}{
		{"neither -> enabled (default-off)", false, false, false},
		{"config field only", true, false, true},
		{"profile field only", false, true, true},
		{"both set", true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.SerenaConfig{
				SemanticIndex: semanticpkg.Config{BenchDisabled: tc.benchDisable},
			}
			prof := &profile.Profile{DisableSemanticSubsystem: tc.profDisable}
			got := resolveSemanticDisabled(cfg, prof)
			require.Equal(t, tc.want, got,
				"resolveSemanticDisabled(bench=%v, prof=%v)", tc.benchDisable, tc.profDisable)
		})
	}
}

// TestSemanticGateForcesNoop is the build-but-block (D-04) forcing test:
// with the gate ON and a NON-NIL bundle (sBndl != nil — the store IS built),
// the gated symbolsLookupFn returns integ.NoopLookup{} and the gated cfgGate
// reports DISABLED. Ungated (gate OFF) with a non-nil bundle the lookup is the
// real adapter (Available() may be true) and the cfgGate inherits the config
// flag.
func TestSemanticGateForcesNoop(t *testing.T) {
	// Build a non-nil bundle so the gate is non-vacuous: it must force Noop
	// EVEN THOUGH sBndl != nil (the store stays built under the gate).
	bndl := newGateTestBundle(t)
	require.NotNil(t, bndl, "test bundle must be non-nil so the Noop forcing is non-vacuous")

	// Gate ON: lookup is forced to NoopLookup{} regardless of the live bundle.
	gatedFn := gatedSymbolsLookupFn(bndl, true)
	got := gatedFn()
	_, isNoop := got.(integ.NoopLookup)
	require.True(t, isNoop,
		"with effSemanticDisabled=true the lookup must be integ.NoopLookup{}, got %T", got)
	require.False(t, got.Available(),
		"the gated lookup must report Available()==false (build-but-block, D-04)")

	// The gated cfgGate must report DISABLED even though cfg.SemanticIndex.Enabled
	// is true (disable the GATE, not just the lookup — Pitfall 4).
	cfg := &config.SerenaConfig{SemanticIndex: semanticpkg.Config{Enabled: true}}
	gate := gatedCfgGate(cfg, true)
	require.False(t, gate.SemanticIndexEnabled(),
		"with effSemanticDisabled=true the cfgGate must report disabled even when cfg.Enabled=true (Pitfall 4)")

	// Gate OFF (regression): a non-nil bundle hands back the real adapter and
	// the cfgGate inherits cfg.SemanticIndex.Enabled.
	ungatedFn := gatedSymbolsLookupFn(bndl, false)
	ungated := ungatedFn()
	_, ungatedNoop := ungated.(integ.NoopLookup)
	require.False(t, ungatedNoop,
		"with effSemanticDisabled=false a non-nil bundle must hand back the real adapter, not Noop")
	require.True(t, gatedCfgGate(cfg, false).SemanticIndexEnabled(),
		"with effSemanticDisabled=false the cfgGate inherits cfg.SemanticIndex.Enabled (true)")
}

// TestSemanticGateChoosesTreeSitter is the Pitfall-4 ladder assertion: with the
// gated (disabled) cfgGate + the gated (Noop) lookup, integ.ChooseSource yields
// SourceTreeSitter (the steady-state v1.9 path) — NOT SourceFallback +
// index_disabled. Disabling the GATE (priority 1) is what makes the ladder
// short-circuit to tree_sitter before it can reach the defensive Noop arm.
func TestSemanticGateChoosesTreeSitter(t *testing.T) {
	bndl := newGateTestBundle(t)
	cfg := &config.SerenaConfig{SemanticIndex: semanticpkg.Config{Enabled: true}}

	gate := gatedCfgGate(cfg, true)
	lookup := gatedSymbolsLookupFn(bndl, true)()

	src, reason := integ.ChooseSource(gate, lookup, nil)
	require.Equal(t, integ.SourceTreeSitter, src,
		"gated ChooseSource must yield tree_sitter (Pitfall 4: disable the GATE so priority-1 wins), not fallback")
	require.Equal(t, integ.FallbackReason(""), reason,
		"tree_sitter source carries no fallback_reason")
}

// newGateTestBundle constructs a minimal non-nil *semanticBundle backed by a
// real (temp) DuckDB store via daemon.New, then reaches the registered bundle
// for the gate helpers. We boot the daemon (semantic enabled, temp store) so
// sBndl is genuinely non-nil — the build-but-block invariant (D-04) requires the
// bundle to exist under the gate.
func newGateTestBundle(t *testing.T) *semanticBundle {
	t.Helper()
	wsDir := t.TempDir()
	t.Chdir(wsDir)

	cfg := newTestConfig(t)
	cfg.SemanticIndex = semanticpkg.Config{
		Enabled: true,
		Store: semanticpkg.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     1,
		},
	}
	logger := newTestLogger()
	d, err := New(cfg, logger)
	require.NoError(t, err, "daemon.New must succeed with semantic-enabled temp config")
	bndl := d.semanticBundleForTest()
	require.NotNil(t, bndl, "semantic bundle must be non-nil when semantic_index is enabled")
	return bndl
}

// TestSemanticSkillAccessorsGated (Task 2) proves the SemanticSkill
// Set*Accessor block (the direct-DuckDB-read tools: find_related_symbols,
// explain_symbol_deep, validate_graph_edge, cluster/impact + the
// ExpandFrom/RankFiles back-channels) is SKIPPED under the gate, so those tools
// have no read path. The complementary regression — wired when not gated — is
// already covered by TestSemanticBundleWiresP1Accessors, but we re-assert the
// ImpactLookup hand-out here for both arms to make the gate explicit.
func TestSemanticSkillAccessorsGated(t *testing.T) {
	wsDir := t.TempDir()
	t.Chdir(wsDir)

	cfg := newTestConfig(t)
	cfg.SemanticIndex = semanticpkg.Config{
		Enabled:       true, // store STILL built (D-04 build-but-block)
		BenchDisabled: true, // ablation gate ON -> effSemanticDisabled
		Store: semanticpkg.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     1,
		},
	}
	logger := newTestLogger()
	d, err := New(cfg, logger)
	require.NoError(t, err, "daemon.New must succeed even under the ablation gate (store still built, D-04)")

	// The bundle/store IS built under the gate (D-04 — the Noop forcing would
	// be vacuous otherwise).
	require.NotNil(t, d.semanticBundleForTest(),
		"semantic bundle must STILL be built under the gate (D-04 build-but-block)")

	sk := semantic.GetSemanticSkill()
	require.NotNil(t, sk, "SemanticSkill must be registered")
	wired := semantic.WiredAccessorsForTest(sk)

	// Under the gate, the accessor block is skipped: every direct-read seam is nil.
	require.False(t, wired.ImpactLookup,
		"ImpactLookup (the ExpandFrom back-channel) must be nil under the gate (Pitfall 3 / A5)")
	require.False(t, wired.SymbolByName, "SymbolByName accessor must be nil under the gate")
	require.False(t, wired.ClusterMap, "ClusterMap accessor must be nil under the gate")
	require.False(t, wired.ClusterMember, "ClusterMember accessor must be nil under the gate")
	require.False(t, wired.ClusterPageRank, "ClusterPageRank accessor must be nil under the gate")
	require.False(t, wired.SymbolEdges, "SymbolEdges accessor must be nil under the gate")
	require.False(t, wired.ClusterMembership, "ClusterMembership accessor must be nil under the gate")
	require.False(t, wired.ExtractorRun, "ExtractorRun accessor must be nil under the gate")
}

// TestSemanticBackgroundPipelinesGated (Phase 81 Plan 07, CR-01) proves the
// daemon-INTERNAL background read pipelines are forced OFF under the gate while
// the store + bundle stay BUILT (D-04 build-but-block):
//
//   - SetFileFactStore(semanticStore) — the read-driver behind GetLatestFileFact
//     / LatestCommittedSnapshot — is NOT wired under the gate (the FileFactStore
//     is the thing the rank/post-commit pipeline reads through).
//   - The bundle is STILL non-nil under the gate (D-04 — otherwise the
//     zero-reads proof is vacuous).
//   - OFF the gate (default arm) the FileFactStore IS wired exactly as before
//     (no behavior change off the gate).
//
// The store-ON dynamic read proof (a real daemon driven end-to-end asserting
// helix_semantic_store_reads_total == 0) is the bench integration test
// TestNoSemanticStoreOnZeroReads — this unit test asserts the STRUCTURAL gate.
func TestSemanticBackgroundPipelinesGated(t *testing.T) {
	// The pure predicate is the single-resolution-point doctrine extension.
	require.True(t, backgroundSemanticReadsDisabled(true),
		"under effSemanticDisabled the background read pipelines must be disabled")
	require.False(t, backgroundSemanticReadsDisabled(false),
		"off the gate the background read pipelines run as before")

	// newGatedDaemon boots daemon.New with semantic ENABLED + LiveUpdates ENABLED
	// (so the live bundle + FileFactStore wiring path is exercised) and the
	// ablation gate toggled per arm.
	newGatedDaemon := func(t *testing.T, gateOn bool) *Daemon {
		t.Helper()
		wsDir := t.TempDir()
		t.Chdir(wsDir)
		cfg := newTestConfig(t)
		cfg.SemanticIndex = semanticpkg.Config{
			Enabled:       true,   // store + bundle STILL built (D-04)
			BenchDisabled: gateOn, // ablation gate -> effSemanticDisabled
			Store: semanticpkg.StoreConfig{
				Kind:        "duckdb",
				Path:        filepath.Join(".helix", "semantic.duckdb"),
				MemoryLimit: "256MiB",
				Threads:     1,
			},
		}
		// LiveUpdates ON so buildLiveBundle returns non-nil and the
		// SetFileFactStore read-driver wiring path is reached.
		cfg.SemanticIndex.LiveUpdates.Enabled = true
		logger := newTestLogger()
		d, err := New(cfg, logger)
		require.NoError(t, err, "daemon.New must succeed (store built under D-04 regardless of gate)")
		return d
	}

	t.Run("gate_on_pipelines_inert_bundle_still_built", func(t *testing.T) {
		d := newGatedDaemon(t, true)
		require.True(t, d.effSemanticDisabledForTest(),
			"the gate must be resolved ON when BenchDisabled=true")
		// D-04 build-but-block: the bundle is STILL built under the gate.
		require.NotNil(t, d.semanticBundleForTest(),
			"semantic bundle must STILL be built under the gate (D-04 build-but-block)")
		require.NotNil(t, d.SemanticStore(),
			"semantic store must STILL be built under the gate (D-04 build-but-block)")
		// The FileFactStore read-driver is NOT wired under the gate.
		require.False(t, d.fileFactStoreWiredForTest(),
			"SetFileFactStore must be SKIPPED under the gate (the read-driver is inert, CR-01)")
	})

	t.Run("gate_off_pipelines_wire_as_before", func(t *testing.T) {
		d := newGatedDaemon(t, false)
		require.False(t, d.effSemanticDisabledForTest(),
			"the gate must be resolved OFF when BenchDisabled=false")
		require.NotNil(t, d.semanticBundleForTest(),
			"semantic bundle is built on the default arm")
		// Off the gate the FileFactStore IS wired exactly as before.
		require.True(t, d.fileFactStoreWiredForTest(),
			"off the gate SetFileFactStore must wire the read-driver as before (no behavior change)")
	})
}
