package config

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "github.com/agenthands/helix/internal/profile" // ensure embedded profiles are loadable
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("/nonexistent/global.yml", "", nil)
	if err != nil {
		t.Fatalf("Load with defaults failed: %v", err)
	}

	if cfg.Daemon.HTTPAddr != ":8080" {
		t.Errorf("expected default http_addr :8080, got %s", cfg.Daemon.HTTPAddr)
	}
	if cfg.Daemon.ShutdownTimeout != 10 {
		t.Errorf("expected default shutdown_timeout 10, got %d", cfg.Daemon.ShutdownTimeout)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("expected default logging format text, got %s", cfg.Logging.Format)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default logging level info, got %s", cfg.Logging.Level)
	}
	// SocketPath should be auto-computed (non-empty)
	if cfg.Daemon.SocketPath == "" {
		t.Error("expected auto-computed socket path, got empty")
	}
}

func TestLoad_CLIOverrides(t *testing.T) {
	overrides := map[string]interface{}{
		"daemon.socket_path": "/custom/path.sock",
		"daemon.http_addr":   ":9090",
	}
	cfg, err := Load("/nonexistent/global.yml", "", overrides)
	if err != nil {
		t.Fatalf("Load with CLI overrides failed: %v", err)
	}

	if cfg.Daemon.SocketPath != "/custom/path.sock" {
		t.Errorf("expected CLI override socket path /custom/path.sock, got %s", cfg.Daemon.SocketPath)
	}
	if cfg.Daemon.HTTPAddr != ":9090" {
		t.Errorf("expected CLI override http_addr :9090, got %s", cfg.Daemon.HTTPAddr)
	}
}

func TestLoad_ProjectConfigOverridesGlobal(t *testing.T) {
	dir := t.TempDir()

	// Create global config
	globalPath := filepath.Join(dir, "global.yml")
	os.WriteFile(globalPath, []byte("daemon:\n  http_addr: \":7070\"\nlogging:\n  level: debug\n"), 0600)

	// Create project config that overrides http_addr
	projectPath := filepath.Join(dir, "project.yml")
	os.WriteFile(projectPath, []byte("daemon:\n  http_addr: \":8888\"\n"), 0600)

	cfg, err := Load(globalPath, projectPath, nil)
	if err != nil {
		t.Fatalf("Load with project config failed: %v", err)
	}

	// Project overrides global for http_addr
	if cfg.Daemon.HTTPAddr != ":8888" {
		t.Errorf("expected project override http_addr :8888, got %s", cfg.Daemon.HTTPAddr)
	}
	// Global sets logging level (not overridden by project)
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected global logging level debug, got %s", cfg.Logging.Level)
	}
}

func TestLoad_DefaultProfile(t *testing.T) {
	cfg, err := Load("/nonexistent/global.yml", "", nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Profile != "full" {
		t.Errorf("expected default profile 'full', got %q", cfg.Profile)
	}
	if cfg.Mode != "" {
		t.Errorf("expected empty default mode, got %q", cfg.Mode)
	}
}

func TestLoad_ObservabilityDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/global.yml", "", nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Observability.AdminAddr != "" {
		t.Errorf("expected empty default admin_addr, got %q", cfg.Observability.AdminAddr)
	}
	if cfg.Observability.EnablePprof {
		t.Errorf("expected default enable_pprof=false, got true")
	}
}

func TestLoad_ObservabilityOverride(t *testing.T) {
	overrides := map[string]interface{}{
		"observability.admin_addr":   "127.0.0.1:9090",
		"observability.enable_pprof": true,
	}
	cfg, err := Load("/nonexistent/global.yml", "", overrides)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Observability.AdminAddr != "127.0.0.1:9090" {
		t.Errorf("expected admin_addr 127.0.0.1:9090, got %q", cfg.Observability.AdminAddr)
	}
	if !cfg.Observability.EnablePprof {
		t.Errorf("expected enable_pprof=true, got false")
	}
}

// TestLoad_EmptyAdminAddrOverrideIsNotApplied documents the runDaemon guard
// (Pitfall #5): callers must not put an empty string into the overrides map
// for observability.admin_addr, otherwise a project config value would be
// silently wiped. This test exercises the *absence* of the override — i.e.
// when the guard is correctly applied upstream, a project-supplied value
// survives.
func TestLoad_EmptyAdminAddrOverrideIsNotApplied(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "project.yml")
	os.WriteFile(projectPath, []byte("observability:\n  admin_addr: \"127.0.0.1:7777\"\n"), 0600)

	// Simulate the runDaemon guard: adminAddr flag is empty, so NOTHING is
	// placed in the overrides map for observability.admin_addr.
	overrides := map[string]interface{}{}
	cfg, err := Load("/nonexistent/global.yml", projectPath, overrides)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Observability.AdminAddr != "127.0.0.1:7777" {
		t.Errorf("expected project admin_addr 127.0.0.1:7777 to survive empty CLI flag, got %q", cfg.Observability.AdminAddr)
	}
}

func TestResolveProfile_KnownProfile(t *testing.T) {
	cfg := &SerenaConfig{Profile: "claude-code"}
	store, prof, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil ProfileStore")
	}
	if prof == nil {
		t.Fatal("expected non-nil Profile for claude-code")
	}
}

func TestResolveProfile_UnknownFallsBackToFull(t *testing.T) {
	cfg := &SerenaConfig{Profile: "nonexistent-profile"}
	_, prof, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	// Should fall back to the "full" default profile
	if prof == nil {
		t.Fatal("expected fallback to full profile, got nil")
	}
}

func TestResolveProfile_CLIOverrideTakesPrecedence(t *testing.T) {
	dir := t.TempDir()

	// Create project config with profile=claude-code
	projectPath := filepath.Join(dir, "project.yml")
	os.WriteFile(projectPath, []byte("profile: claude-code\n"), 0600)

	// CLI overrides to ci-bot
	overrides := map[string]interface{}{
		"profile": "ci-bot",
	}

	cfg, err := Load("/nonexistent/global.yml", projectPath, overrides)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Profile != "ci-bot" {
		t.Errorf("expected CLI override profile 'ci-bot', got %q", cfg.Profile)
	}

	// Resolve should find the ci-bot profile
	_, prof, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	if prof == nil {
		t.Fatal("expected non-nil profile for ci-bot")
	}
}

func TestResolveProfile_DescriptionOverridesAccessible(t *testing.T) {
	cfg := &SerenaConfig{Profile: "claude-code"}
	_, prof, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	// ToolDescriptionOverrides should be accessible (may be empty for some profiles)
	if prof.ToolDescriptionOverrides == nil {
		// This is acceptable - not all profiles have overrides
		t.Log("claude-code profile has no tool description overrides (acceptable)")
	}
}

// TestLoad_SemanticIndexDefaults asserts every Phase 57 SPEC §25 default
// surfaces on cfg.SemanticIndex via the standard 4-layer koanf precedence
// when no overrides are present.
//
// Style mirrors TestLoad_ObservabilityDefaults; values mirror SPEC-DRAFT.md
// §25 verbatim. Phase 57 P02 landed the SerenaConfig.SemanticIndex stub
// field (zero values, no koanf tag); P03 (this plan) wires the koanf tag
// and the defaults map.
func TestLoad_SemanticIndexDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/global.yml", "", nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Top-level toggle.
	if !cfg.SemanticIndex.Enabled {
		t.Errorf("expected default SemanticIndex.Enabled=true, got false")
	}

	// store.* — SPEC §25.store.
	if cfg.SemanticIndex.Store.Kind != "duckdb" {
		t.Errorf("expected store.kind=duckdb, got %q", cfg.SemanticIndex.Store.Kind)
	}
	if cfg.SemanticIndex.Store.Path != ".helix/semantic.duckdb" {
		t.Errorf("expected store.path=.helix/semantic.duckdb, got %q", cfg.SemanticIndex.Store.Path)
	}
	if cfg.SemanticIndex.Store.MemoryLimit != "1GiB" {
		t.Errorf("expected store.memory_limit=1GiB, got %q", cfg.SemanticIndex.Store.MemoryLimit)
	}
	if cfg.SemanticIndex.Store.Threads != 4 {
		t.Errorf("expected store.threads=4, got %d", cfg.SemanticIndex.Store.Threads)
	}

	// indexing.* — SPEC §25.indexing.
	if cfg.SemanticIndex.Indexing.Mode != "lazy" {
		t.Errorf("expected indexing.mode=lazy, got %q", cfg.SemanticIndex.Indexing.Mode)
	}
	if cfg.SemanticIndex.Indexing.AutoIndexOnActivate {
		t.Errorf("expected indexing.auto_index_on_activate=false, got true")
	}
	if cfg.SemanticIndex.Indexing.MaxFileSize != "2MiB" {
		t.Errorf("expected indexing.max_file_size=2MiB, got %q", cfg.SemanticIndex.Indexing.MaxFileSize)
	}
	if cfg.SemanticIndex.Indexing.MaxFiles != 200000 {
		t.Errorf("expected indexing.max_files=200000, got %d", cfg.SemanticIndex.Indexing.MaxFiles)
	}
	if cfg.SemanticIndex.Indexing.FullReindexChangeRatio != 0.25 {
		t.Errorf("expected indexing.full_reindex_change_ratio=0.25, got %v", cfg.SemanticIndex.Indexing.FullReindexChangeRatio)
	}
	if cfg.SemanticIndex.Indexing.SnapshotRetention != 5 {
		t.Errorf("expected indexing.snapshot_retention=5, got %d", cfg.SemanticIndex.Indexing.SnapshotRetention)
	}
	if cfg.SemanticIndex.Indexing.IncludeGenerated {
		t.Errorf("expected indexing.include_generated=false, got true")
	}
	if cfg.SemanticIndex.Indexing.RequiredForReadyz {
		t.Errorf("expected indexing.required_for_readyz=false, got true")
	}

	// live_updates.* — SPEC §25.live_updates.
	if !cfg.SemanticIndex.LiveUpdates.Enabled {
		t.Errorf("expected live_updates.enabled=true, got false")
	}
	if cfg.SemanticIndex.LiveUpdates.DebounceMS != 250 {
		t.Errorf("expected live_updates.debounce_ms=250, got %d", cfg.SemanticIndex.LiveUpdates.DebounceMS)
	}
	if cfg.SemanticIndex.LiveUpdates.MaxBatchDelayMS != 1500 {
		t.Errorf("expected live_updates.max_batch_delay_ms=1500, got %d", cfg.SemanticIndex.LiveUpdates.MaxBatchDelayMS)
	}
	if cfg.SemanticIndex.LiveUpdates.BulkChangeThreshold != 200 {
		t.Errorf("expected live_updates.bulk_change_threshold=200, got %d", cfg.SemanticIndex.LiveUpdates.BulkChangeThreshold)
	}
	if cfg.SemanticIndex.LiveUpdates.CompactAfterIdleMS != 5000 {
		t.Errorf("expected live_updates.compact_after_idle_ms=5000, got %d", cfg.SemanticIndex.LiveUpdates.CompactAfterIdleMS)
	}
	if cfg.SemanticIndex.LiveUpdates.LSPRevalidateAfterIdleMS != 750 {
		t.Errorf("expected live_updates.lsp_revalidate_after_idle_ms=750, got %d", cfg.SemanticIndex.LiveUpdates.LSPRevalidateAfterIdleMS)
	}
	if cfg.SemanticIndex.LiveUpdates.LSPCompactionMaxWaitMS != 3000 {
		t.Errorf("expected live_updates.lsp_compaction_max_wait_ms=3000, got %d", cfg.SemanticIndex.LiveUpdates.LSPCompactionMaxWaitMS)
	}
	if cfg.SemanticIndex.LiveUpdates.MaxOverlayFiles != 1000 {
		t.Errorf("expected live_updates.max_overlay_files=1000, got %d", cfg.SemanticIndex.LiveUpdates.MaxOverlayFiles)
	}
	if cfg.SemanticIndex.LiveUpdates.MaxOverlayAge != "30m" {
		t.Errorf("expected live_updates.max_overlay_age=30m, got %q", cfg.SemanticIndex.LiveUpdates.MaxOverlayAge)
	}

	// lsp_enrichment.* — SPEC §25.lsp_enrichment.
	if !cfg.SemanticIndex.LSPEnrichment.Enabled {
		t.Errorf("expected lsp_enrichment.enabled=true, got false")
	}
	if cfg.SemanticIndex.LSPEnrichment.TimeoutPerFile != "5s" {
		t.Errorf("expected lsp_enrichment.timeout_per_file=5s, got %q", cfg.SemanticIndex.LSPEnrichment.TimeoutPerFile)
	}
	if cfg.SemanticIndex.LSPEnrichment.TimeoutTotal != "120s" {
		t.Errorf("expected lsp_enrichment.timeout_total=120s, got %q", cfg.SemanticIndex.LSPEnrichment.TimeoutTotal)
	}
	if cfg.SemanticIndex.LSPEnrichment.MaxSymbolsPerFile != 200 {
		t.Errorf("expected lsp_enrichment.max_symbols_per_file=200, got %d", cfg.SemanticIndex.LSPEnrichment.MaxSymbolsPerFile)
	}
	if cfg.SemanticIndex.LSPEnrichment.MaxReferencesPerSymbol != 1000 {
		t.Errorf("expected lsp_enrichment.max_references_per_symbol=1000, got %d", cfg.SemanticIndex.LSPEnrichment.MaxReferencesPerSymbol)
	}
	if cfg.SemanticIndex.LSPEnrichment.MaxReferencesPerFile != 5000 {
		t.Errorf("expected lsp_enrichment.max_references_per_file=5000, got %d", cfg.SemanticIndex.LSPEnrichment.MaxReferencesPerFile)
	}
	if cfg.SemanticIndex.LSPEnrichment.MaxCallHierarchyDepth != 2 {
		t.Errorf("expected lsp_enrichment.max_call_hierarchy_depth=2, got %d", cfg.SemanticIndex.LSPEnrichment.MaxCallHierarchyDepth)
	}
	if cfg.SemanticIndex.LSPEnrichment.MaxTypeHierarchyDepth != 2 {
		t.Errorf("expected lsp_enrichment.max_type_hierarchy_depth=2, got %d", cfg.SemanticIndex.LSPEnrichment.MaxTypeHierarchyDepth)
	}

	// graph.* — SPEC §25.graph.
	if cfg.SemanticIndex.Graph.MinEdgeConfidence != 0.50 {
		t.Errorf("expected graph.min_edge_confidence=0.50, got %v", cfg.SemanticIndex.Graph.MinEdgeConfidence)
	}
	if cfg.SemanticIndex.Graph.MaxLoadedNodes != 1000000 {
		t.Errorf("expected graph.max_loaded_nodes=1000000, got %d", cfg.SemanticIndex.Graph.MaxLoadedNodes)
	}
	if cfg.SemanticIndex.Graph.MaxLoadedEdges != 5000000 {
		t.Errorf("expected graph.max_loaded_edges=5000000, got %d", cfg.SemanticIndex.Graph.MaxLoadedEdges)
	}
	if cfg.SemanticIndex.Graph.MaxLocalPagerankNodes != 5000 {
		t.Errorf("expected graph.max_local_pagerank_nodes=5000, got %d", cfg.SemanticIndex.Graph.MaxLocalPagerankNodes)
	}
	if cfg.SemanticIndex.Graph.MaxIncrementalClusterRepairNodes != 10000 {
		t.Errorf("expected graph.max_incremental_cluster_repair_nodes=10000, got %d", cfg.SemanticIndex.Graph.MaxIncrementalClusterRepairNodes)
	}

	// pagerank.* — SPEC §25.pagerank.
	if cfg.SemanticIndex.PageRank.Damping != 0.85 {
		t.Errorf("expected pagerank.damping=0.85, got %v", cfg.SemanticIndex.PageRank.Damping)
	}
	// epsilon = 1e-6: tiny float, compare with epsilon tolerance (per plan acceptance).
	if math.Abs(cfg.SemanticIndex.PageRank.Epsilon-0.000001) > 1e-9 {
		t.Errorf("expected pagerank.epsilon=1e-6, got %v", cfg.SemanticIndex.PageRank.Epsilon)
	}
	if cfg.SemanticIndex.PageRank.MaxIterations != 100 {
		t.Errorf("expected pagerank.max_iterations=100, got %d", cfg.SemanticIndex.PageRank.MaxIterations)
	}

	// clustering.* — SPEC §25.clustering.
	if !cfg.SemanticIndex.Clustering.Enabled {
		t.Errorf("expected clustering.enabled=true, got false")
	}
	if cfg.SemanticIndex.Clustering.MaxComponentSizeBeforeSplit != 5000 {
		t.Errorf("expected clustering.max_component_size_before_split=5000, got %d", cfg.SemanticIndex.Clustering.MaxComponentSizeBeforeSplit)
	}
	if cfg.SemanticIndex.Clustering.LabelPropagationIterations != 20 {
		t.Errorf("expected clustering.label_propagation_iterations=20, got %d", cfg.SemanticIndex.Clustering.LabelPropagationIterations)
	}

	// retrieval.* — SPEC §25.retrieval.
	if cfg.SemanticIndex.Retrieval.DefaultMaxTokens != 8000 {
		t.Errorf("expected retrieval.default_max_tokens=8000, got %d", cfg.SemanticIndex.Retrieval.DefaultMaxTokens)
	}
	if !cfg.SemanticIndex.Retrieval.IncludeEvidenceByDefault {
		t.Errorf("expected retrieval.include_evidence_by_default=true, got false")
	}

	// guardrails.* — SPEC §25.guardrails.
	if !cfg.SemanticIndex.Guardrails.Enabled {
		t.Errorf("expected guardrails.enabled=true, got false")
	}
	if cfg.SemanticIndex.Guardrails.Enforcement != "warn" {
		t.Errorf("expected guardrails.enforcement=warn, got %q", cfg.SemanticIndex.Guardrails.Enforcement)
	}
	if !cfg.SemanticIndex.Guardrails.RequireImpactForPublicAPIEdit {
		t.Errorf("expected guardrails.require_impact_for_public_api_edit=true, got false")
	}
	if !cfg.SemanticIndex.Guardrails.RequireReferencesBeforeRename {
		t.Errorf("expected guardrails.require_references_before_rename=true, got false")
	}
	if !cfg.SemanticIndex.Guardrails.RequireReferencesBeforeDelete {
		t.Errorf("expected guardrails.require_references_before_delete=true, got false")
	}
	if !cfg.SemanticIndex.Guardrails.RequireVerifyAfterEdit {
		t.Errorf("expected guardrails.require_verify_after_edit=true, got false")
	}
	if cfg.SemanticIndex.Guardrails.StaleGraphPolicy != "warn" {
		t.Errorf("expected guardrails.stale_graph_policy=warn, got %q", cfg.SemanticIndex.Guardrails.StaleGraphPolicy)
	}

	// eval.* — SPEC §25.eval.
	if !cfg.SemanticIndex.Eval.Enabled {
		t.Errorf("expected eval.enabled=true, got false")
	}
	wantModes := []string{"baseline", "native", "semantic", "semantic_guarded"}
	if !reflect.DeepEqual(cfg.SemanticIndex.Eval.DefaultModes, wantModes) {
		t.Errorf("expected eval.default_modes=%v, got %v", wantModes, cfg.SemanticIndex.Eval.DefaultModes)
	}
	if cfg.SemanticIndex.Eval.OutputDir != ".helix/eval" {
		t.Errorf("expected eval.output_dir=.helix/eval, got %q", cfg.SemanticIndex.Eval.OutputDir)
	}
	if !cfg.SemanticIndex.Eval.TrackCosts {
		t.Errorf("expected eval.track_costs=true, got false")
	}
	if !cfg.SemanticIndex.Eval.TrackToolBehavior {
		t.Errorf("expected eval.track_tool_behavior=true, got false")
	}
	if cfg.SemanticIndex.Eval.RedactSourceInReports {
		t.Errorf("expected eval.redact_source_in_reports=false, got true")
	}

	// type_resolution.* — SPEC §25.type_resolution.
	if !cfg.SemanticIndex.TypeResolution.Enabled {
		t.Errorf("expected type_resolution.enabled=true, got false")
	}
	if cfg.SemanticIndex.TypeResolution.MaxChainDepth != 8 {
		t.Errorf("expected type_resolution.max_chain_depth=8, got %d", cfg.SemanticIndex.TypeResolution.MaxChainDepth)
	}
	if cfg.SemanticIndex.TypeResolution.MaxFixpointIterations != 8 {
		t.Errorf("expected type_resolution.max_fixpoint_iterations=8, got %d", cfg.SemanticIndex.TypeResolution.MaxFixpointIterations)
	}
	if cfg.SemanticIndex.TypeResolution.MinConfidenceForEdge != 0.45 {
		t.Errorf("expected type_resolution.min_confidence_for_edge=0.45, got %v", cfg.SemanticIndex.TypeResolution.MinConfidenceForEdge)
	}
	if !cfg.SemanticIndex.TypeResolution.CommentFallbacks {
		t.Errorf("expected type_resolution.comment_fallbacks=true, got false")
	}
	if !cfg.SemanticIndex.TypeResolution.EmitUnresolvedEdges {
		t.Errorf("expected type_resolution.emit_unresolved_edges=true, got false")
	}

	// phase_graph.* — SPEC §25.phase_graph.
	if !cfg.SemanticIndex.PhaseGraph.Enabled {
		t.Errorf("expected phase_graph.enabled=true, got false")
	}
	if !cfg.SemanticIndex.PhaseGraph.ValidateOnStartup {
		t.Errorf("expected phase_graph.validate_on_startup=true, got false")
	}
	if !cfg.SemanticIndex.PhaseGraph.FailOnCycle {
		t.Errorf("expected phase_graph.fail_on_cycle=true, got false")
	}
	if !cfg.SemanticIndex.PhaseGraph.DumpDotOnError {
		t.Errorf("expected phase_graph.dump_dot_on_error=true, got false")
	}
}

// TestLoad_SemanticIndexPrecedence covers 4 representative SPEC §25 keys,
// one per layer, confirming CLI > project > user > profile resolution
// flows correctly through the existing 4-layer koanf machinery once P03's
// koanf binding tag is in place.
//
// Style mirrors TestLoad_ProjectConfigOverridesGlobal (lines 53-77).
func TestLoad_SemanticIndexPrecedence(t *testing.T) {
	dir := t.TempDir()

	// User layer (~/.helix/helix_config.yml equivalent — passed as globalPath)
	userPath := filepath.Join(dir, "user.yml")
	userYAML := "" +
		"semantic_index:\n" +
		"  store:\n" +
		"    path: \"/custom/user/semantic.duckdb\"\n" +
		"    threads: 8\n" +
		"  indexing:\n" +
		"    mode: \"eager\"\n"
	if err := os.WriteFile(userPath, []byte(userYAML), 0600); err != nil {
		t.Fatalf("write user yml: %v", err)
	}

	// Project layer (.helix/project.yml equivalent — overrides threads only)
	projectPath := filepath.Join(dir, "project.yml")
	projectYAML := "" +
		"semantic_index:\n" +
		"  store:\n" +
		"    threads: 16\n" +
		"  indexing:\n" +
		"    mode: \"on_demand\"\n"
	if err := os.WriteFile(projectPath, []byte(projectYAML), 0600); err != nil {
		t.Fatalf("write project yml: %v", err)
	}

	// CLI layer (highest precedence) — overrides indexing.mode only
	cliOverrides := map[string]interface{}{
		"semantic_index.indexing.mode": "lazy", // CLI wins over project's "on_demand"
	}

	cfg, err := Load(userPath, projectPath, cliOverrides)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Assertion 1 (PROFILE layer survives): semantic_index.enabled was not
	// touched by user/project/CLI — should remain the SPEC §25 default `true`.
	if !cfg.SemanticIndex.Enabled {
		t.Errorf("profile-layer default lost: expected enabled=true, got false")
	}

	// Assertion 2 (USER layer wins over PROFILE): store.path was set by user,
	// not touched by project or CLI.
	if cfg.SemanticIndex.Store.Path != "/custom/user/semantic.duckdb" {
		t.Errorf("user-layer override lost: expected store.path=/custom/user/semantic.duckdb, got %q",
			cfg.SemanticIndex.Store.Path)
	}

	// Assertion 3 (PROJECT layer wins over USER): both layers set threads;
	// project=16 must beat user=8.
	if cfg.SemanticIndex.Store.Threads != 16 {
		t.Errorf("project-layer override lost: expected store.threads=16 (project beat user=8), got %d",
			cfg.SemanticIndex.Store.Threads)
	}

	// Assertion 4 (CLI layer wins over all): indexing.mode set by user, project,
	// and CLI; CLI="lazy" must win.
	if cfg.SemanticIndex.Indexing.Mode != "lazy" {
		t.Errorf("CLI-layer override lost: expected indexing.mode=lazy (CLI beat project=on_demand), got %q",
			cfg.SemanticIndex.Indexing.Mode)
	}
}
