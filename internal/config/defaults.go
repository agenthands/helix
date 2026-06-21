// Package config holds the koanf-backed daemon configuration. The
// `defaults.go` file is the canonical source for every Phase 1+ default
// value in the 4-layer precedence chain (CLI > project > user > profile).
//
// Phase 57 added the `semantic_index.*` family per SPEC-DRAFT.md §25; see
// `internal/semantic/config.go` for the typed mirror. The
// `SerenaConfig.SemanticIndex` field's `koanf:"semantic_index"` binding tag
// was added by P03 atop the stub field landed in P02.
//
// Float defaults MUST be wrapped `float64(...)` — koanf's confmap provider
// decodes untyped Go literals as `int`, which then fails to bind to a
// float64 struct field (the resulting value is the zero float). The
// observability tracing-ratio default below is the canonical precedent.
//
// Slice defaults MUST be declared as `[]string{...}` (or the appropriate
// concrete slice type) — a bare `[]any{...}` does not bind cleanly to a
// `[]string` struct field through koanf's mapstructure decoder.
package config

import (
	"os"
	"path/filepath"
)

// DefaultConfig returns built-in default values.
func DefaultConfig() map[string]interface{} {
	homeDir, _ := os.UserHomeDir()
	return map[string]interface{}{
		"daemon.socket_path":                 "", // empty means auto-compute from UID
		"daemon.grpc_addr":                   "", // empty = unix-socket only; loopback-only opt-in (Phase 94 RETIRE-04)
		"daemon.shutdown_timeout":            10,
		"logging.format":                     "text",
		"logging.level":                      "info",
		"logging.dir":                        filepath.Join(homeDir, ".helix", "logs"),
		"profile":                            "full",     // default profile is the neutral escape hatch (D-02)
		"mode":                               "",         // empty = use profile's default_mode
		"observability.admin_addr":           "",         // empty = admin listener disabled (D-02, D-05)
		"observability.enable_pprof":         false,      // zero attack surface by default (D-12)
		"observability.tracing_endpoint":     "",         // empty = tracing disabled (D-11)
		"observability.tracing_sample_ratio": float64(0), // 0.0 = off by default (D-11)
		"observability.service_name":         "helix",    // OTel resource service.name (D-11)
		"degradation.timeout_read":           5,          // seconds (D-02)
		"degradation.timeout_search":         15,         // seconds (D-02)
		"degradation.timeout_edit":           10,         // seconds (D-02)
		"degradation.timeout_index":          120,        // seconds (D-02)
		"degradation.timeout_diagnostics":    20,         // seconds (D-02)
		"degradation.memory_limit_mb":        0,          // 0 = don't set (D-09)
		"degradation.restart_budget":         3,          // consecutive crashes before circuit stays open (D-05)

		// Phase 57: SPEC §25 semantic_index.* defaults (STORE-04, STORE-05).
		// Order mirrors SPEC-DRAFT.md §25 verbatim for diffability.
		"semantic_index.enabled": true, // SPEC §25 (STORE-01)

		// store.*
		"semantic_index.store.kind":         "duckdb",                 // SPEC §25 (STORE-01)
		"semantic_index.store.path":         ".helix/semantic.duckdb", // SPEC §25 (STORE-01)
		"semantic_index.store.memory_limit": "1GiB",                   // SPEC §25
		"semantic_index.store.threads":      4,                        // SPEC §25

		// indexing.*
		"semantic_index.indexing.mode":                      "lazy",        // SPEC §25
		"semantic_index.indexing.auto_index_on_activate":    false,         // SPEC §25
		"semantic_index.indexing.max_file_size":             "2MiB",        // SPEC §25
		"semantic_index.indexing.max_files":                 200000,        // SPEC §25
		"semantic_index.indexing.full_reindex_change_ratio": float64(0.25), // SPEC §25 (koanf float gotcha)
		"semantic_index.indexing.snapshot_retention":        5,             // SPEC §25
		"semantic_index.indexing.include_generated":         false,         // SPEC §25
		"semantic_index.indexing.required_for_readyz":       false,         // SPEC §25

		// extraction.* — Phase 59 P02 (the four genuinely-new keys).
		// Per user decision: max_file_size and auto_index_on_activate STAY
		// under indexing.* (above) — they are NOT duplicated here.
		"semantic_index.extraction.extraction_ready_timeout": "30s", // SPEC §25
		"semantic_index.extraction.extraction_file_timeout":  "3s",  // SPEC §25
		"semantic_index.extraction.max_parallel_files":       4,     // SPEC §25
		"semantic_index.extraction.allow_partial_results":    true,  // SPEC §25

		// live_updates.*
		"semantic_index.live_updates.enabled":                      true,  // SPEC §25
		"semantic_index.live_updates.debounce_ms":                  250,   // SPEC §25
		"semantic_index.live_updates.max_batch_delay_ms":           1500,  // SPEC §25
		"semantic_index.live_updates.bulk_change_threshold":        200,   // SPEC §25
		"semantic_index.live_updates.compact_after_idle_ms":        5000,  // SPEC §25
		"semantic_index.live_updates.lsp_revalidate_after_idle_ms": 750,   // SPEC §25
		"semantic_index.live_updates.lsp_compaction_max_wait_ms":   3000,  // SPEC §25
		"semantic_index.live_updates.max_overlay_files":            1000,  // SPEC §25
		"semantic_index.live_updates.max_overlay_age":              "30m", // SPEC §25

		// Phase 60 D-05: live-updates watcher + manifest scanner toggles
		// (SPEC §25 extension; not a new section). watcher_enabled gates
		// fsnotify start in 60-05A; manifest_scan_enabled + interval gate
		// the 60-05B scanner. All three flow through the standard 4-layer
		// koanf precedence chain (CLI > project > user > profile defaults).
		"semantic_index.live_updates.watcher_enabled":        true,
		"semantic_index.live_updates.manifest_scan_enabled":  true,
		"semantic_index.live_updates.manifest_scan_interval": "10s",

		// Phase 63 P63-02 Task 3: maintenance.* — config-gated VACUUM
		// cadence (CONTEXT.md D-05). Default OFF per planner decision —
		// VACUUM is shipped as documented no-op-by-DuckDB; the
		// infrastructure ships so a future phase can swap COPY FROM
		// DATABASE repack in without re-architecting the gate.
		"semantic_index.maintenance.vacuum_enabled":  false,
		"semantic_index.maintenance.vacuum_interval": "168h",

		// lsp_enrichment.*
		"semantic_index.lsp_enrichment.enabled":                   true,   // SPEC §25
		"semantic_index.lsp_enrichment.timeout_per_file":          "5s",   // SPEC §25
		"semantic_index.lsp_enrichment.timeout_total":             "120s", // SPEC §25
		"semantic_index.lsp_enrichment.max_symbols_per_file":      200,    // SPEC §25
		"semantic_index.lsp_enrichment.max_references_per_symbol": 1000,   // SPEC §25
		"semantic_index.lsp_enrichment.max_references_per_file":   5000,   // SPEC §25
		"semantic_index.lsp_enrichment.max_call_hierarchy_depth":  2,      // SPEC §25
		"semantic_index.lsp_enrichment.max_type_hierarchy_depth":  2,      // SPEC §25

		// Phase 61 P03 D-02 + D-04: enrichment-worker concurrency cap and
		// foreground-yield observation window. Both keys flow through the
		// standard 4-layer koanf precedence chain (CLI > project > user >
		// profile defaults).
		"semantic_index.lsp_enrichment.max_concurrent_workers": 1,   // P61 D-02
		"semantic_index.lsp_enrichment.yield_check_window_ms":  200, // P61 D-04 (ms)

		// graph.*
		"semantic_index.graph.min_edge_confidence":                  float64(0.50), // SPEC §25 (koanf float gotcha)
		"semantic_index.graph.max_loaded_nodes":                     1000000,       // SPEC §25
		"semantic_index.graph.max_loaded_edges":                     5000000,       // SPEC §25
		"semantic_index.graph.max_local_pagerank_nodes":             5000,          // SPEC §25
		"semantic_index.graph.max_incremental_cluster_repair_nodes": 10000,         // SPEC §25

		// pagerank.*
		"semantic_index.pagerank.damping":        float64(0.85),     // SPEC §25 (koanf float gotcha)
		"semantic_index.pagerank.epsilon":        float64(0.000001), // SPEC §25 (koanf float gotcha — 1e-6)
		"semantic_index.pagerank.max_iterations": 100,               // SPEC §25

		// Phase 62 P02: scheduler debounce + full-recompute thresholds (D-08).
		"semantic_index.pagerank.repair_debounce_ms":       2000,          // P62 D-08
		"semantic_index.pagerank.full_recompute_idle_ms":   60000,         // P62 D-08
		"semantic_index.pagerank.full_recompute_threshold": float64(0.25), // P62 D-08 (koanf float gotcha)

		// clustering.*
		"semantic_index.clustering.enabled":                         true, // SPEC §25
		"semantic_index.clustering.max_component_size_before_split": 5000, // SPEC §25
		"semantic_index.clustering.label_propagation_iterations":    20,   // SPEC §25

		// retrieval.*
		"semantic_index.retrieval.default_max_tokens":          8000, // SPEC §25
		"semantic_index.retrieval.include_evidence_by_default": true, // SPEC §25

		// guardrails.*
		"semantic_index.guardrails.enabled":                            true,   // SPEC §25
		"semantic_index.guardrails.enforcement":                        "warn", // SPEC §25
		"semantic_index.guardrails.require_impact_for_public_api_edit": true,   // SPEC §25
		"semantic_index.guardrails.require_references_before_rename":   true,   // SPEC §25
		"semantic_index.guardrails.require_references_before_delete":   true,   // SPEC §25
		"semantic_index.guardrails.require_verify_after_edit":          true,   // SPEC §25
		"semantic_index.guardrails.stale_graph_policy":                 "warn", // SPEC §25

		// guardrails.* D-22 additions (Phase 66 Plan 02):
		"semantic_index.guardrails.receipt_ttl": "5m", // D-04: default receipt TTL

		// Per-rule enforcement defaults (D-22 locked table):
		"semantic_index.guardrails.rules.G-001.enforcement": "warn",    // D-22
		"semantic_index.guardrails.rules.G-002.enforcement": "enforce", // D-22
		"semantic_index.guardrails.rules.G-003.enforcement": "enforce", // D-22
		"semantic_index.guardrails.rules.G-004.enforcement": "warn",    // D-22
		"semantic_index.guardrails.rules.G-005.enforcement": "enforce", // D-22

		// Per-tool enforcement defaults (D-22 locked table):
		"semantic_index.guardrails.tools.rename_symbol.enforcement":      "enforce", // D-22
		"semantic_index.guardrails.tools.safe_delete_symbol.enforcement": "enforce", // D-22
		"semantic_index.guardrails.tools.replace_symbol_body.enforcement": "enforce", // D-22
		"semantic_index.guardrails.tools.fuzzy_edit.enforcement":          "warn",    // D-22
		"semantic_index.guardrails.tools.replace_in_file.enforcement":     "warn",    // D-22

		// G-004 thresholds (D-16):
		"semantic_index.guardrails.G-004.max_changed_lines":    50,              // D-16
		"semantic_index.guardrails.G-004.max_files":            1,               // D-16
		"semantic_index.guardrails.G-004.max_file_change_ratio": float64(0.30),  // D-16 (koanf float gotcha)
		"semantic_index.guardrails.G-004.enable_ratio_trigger": true,            // D-16

		// eval.*
		"semantic_index.eval.enabled":                  true,                                                           // SPEC §25
		"semantic_index.eval.default_modes":            []string{"baseline", "native", "semantic", "semantic_guarded"}, // SPEC §25 (koanf slice gotcha)
		"semantic_index.eval.output_dir":               ".helix/eval",                                                  // SPEC §25
		"semantic_index.eval.track_costs":              true,                                                           // SPEC §25
		"semantic_index.eval.track_tool_behavior":      true,                                                           // SPEC §25
		"semantic_index.eval.redact_source_in_reports": false,                                                          // SPEC §25

		// type_resolution.*
		"semantic_index.type_resolution.enabled":                 true,          // SPEC §25
		"semantic_index.type_resolution.max_chain_depth":         8,             // SPEC §25
		"semantic_index.type_resolution.max_fixpoint_iterations": 8,             // SPEC §25
		"semantic_index.type_resolution.min_confidence_for_edge": float64(0.45), // SPEC §25 (koanf float gotcha)
		"semantic_index.type_resolution.comment_fallbacks":       true,          // SPEC §25
		"semantic_index.type_resolution.emit_unresolved_edges":   true,          // SPEC §25

		// Phase 62 P02: types.* — comment-parser allowlist (D-12).
		// Slice declared as []string to bind cleanly through koanf
		// mapstructure decoder (koanf slice gotcha).
		"semantic_index.types.comment_parsers_enabled": []string{
			"tsdoc", "jsdoc", "godoc", "python_type_comments", "phpdoc", "yard",
		},

		// phase_graph.*
		"semantic_index.phase_graph.enabled":             true, // SPEC §25
		"semantic_index.phase_graph.validate_on_startup": true, // SPEC §25
		"semantic_index.phase_graph.fail_on_cycle":       true, // SPEC §25
		"semantic_index.phase_graph.dump_dot_on_error":   true, // SPEC §25
	}
}
