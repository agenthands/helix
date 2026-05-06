package semantic

// Config is the typed mirror of the koanf `semantic_index.*` block declared
// in SPEC-DRAFT.md §25. Phase 57 plan P02 lands this typed shape with zero
// values; plan P03 wires the koanf binding (`koanf:"semantic_index"`) onto
// the SerenaConfig.SemanticIndex field that consumes this type AND populates
// the SPEC §25 default values into internal/config/defaults.go.
//
// The split is deliberate: P02's daemon step 6b references
// `cfg.SemanticIndex.Enabled`, which must compile at end of wave 1 even
// without P03 having merged. With Go's bool zero value, Enabled defaults to
// false, so step 6b's `if cfg.SemanticIndex.Enabled { ... }` branch is dead
// at runtime when P03 is absent — safe degradation.
//
// Style note: this file mirrors the ObservabilityConfig nested-struct style
// in internal/config/config.go (one koanf tag per leaf). Every field name
// and its koanf tag mirrors a SPEC-DRAFT.md §25 key verbatim — when SPEC §25
// changes a key name, this file MUST change in lockstep, otherwise the
// koanf binding silently drops the value.
//
// History note: P03 broadened the nested type set so every SPEC §25 key has
// a typed home (P02 shipped a smaller speculative subset that drifted from
// SPEC §25; aligning the field names here is part of P03's RED-gate
// preparation since the coverage tests reference the SPEC §25 keys verbatim).
type Config struct {
	// Enabled gates the entire semantic-graph subsystem. When false, the
	// daemon's bootstrap step 6b skips Open and downstream consumers MUST
	// guard `daemon.SemanticStore() == nil`.
	Enabled bool `koanf:"enabled"`

	// Store holds the DuckDB embedded fact-store settings (path, memory,
	// thread budget). See SPEC-DRAFT.md §25.store.*.
	Store StoreConfig `koanf:"store"`

	// Indexing configures the per-snapshot indexing pipeline (P59).
	Indexing IndexingConfig `koanf:"indexing"`

	// Extraction configures the tree-sitter extraction layer (P59 P02).
	// Per Phase 59 user decision (CONTEXT.md): max_file_size and
	// auto_index_on_activate REMAIN under Indexing (already present from
	// Phase 57). Only the four genuinely-new keys live here.
	Extraction ExtractionConfig `koanf:"extraction"`

	// LiveUpdates configures the fsnotify-driven overlay watcher (P60).
	LiveUpdates LiveUpdatesConfig `koanf:"live_updates"`

	// LSPEnrichment configures the LSP enrichment worker (P61).
	LSPEnrichment LSPEnrichmentConfig `koanf:"lsp_enrichment"`

	// Graph configures the graph-engine projections (P62).
	Graph GraphConfig `koanf:"graph"`

	// PageRank configures the cross-file ranking pass (P62).
	PageRank PageRankConfig `koanf:"pagerank"`

	// Clustering configures the symbol-cluster derivation (P62).
	Clustering ClusteringConfig `koanf:"clustering"`

	// Retrieval configures retrieval-tool defaults (P64).
	Retrieval RetrievalConfig `koanf:"retrieval"`

	// Guardrails configures the GuardrailMiddleware (P66).
	Guardrails GuardrailsConfig `koanf:"guardrails"`

	// Eval configures the eval harness (P67).
	Eval EvalConfig `koanf:"eval"`

	// TypeResolution configures the type-resolution pipeline (P62).
	TypeResolution TypeResolutionConfig `koanf:"type_resolution"`

	// Types configures the Phase 62 P05 type resolver's per-language
	// comment parsers. Lives alongside TypeResolution rather than inside
	// it to mirror SPEC-DRAFT.md §25's `types.*` block layout.
	Types TypesConfig `koanf:"types"`

	// PhaseGraph configures the bootstrap phase-graph runner (v1.11+).
	PhaseGraph PhaseGraphConfig `koanf:"phase_graph"`
}

// StoreConfig holds the DuckDB embedded fact-store settings. SPEC-DRAFT.md
// §25.store.*. Defaults populated by P03 in internal/config/defaults.go.
type StoreConfig struct {
	// Kind names the store implementation. Phase 57 ships only "duckdb".
	Kind string `koanf:"kind"`
	// Path is workspace-relative (default: ".helix/semantic.duckdb").
	// MUST be a relative path; absolute paths and `..` segments are rejected
	// at Open time (T-57-02-01 mitigation).
	Path string `koanf:"path"`
	// MemoryLimit is the DuckDB memory budget (e.g., "1GiB").
	MemoryLimit string `koanf:"memory_limit"`
	// Threads is the DuckDB worker-thread count.
	Threads int `koanf:"threads"`
}

// ExtractionConfig holds tree-sitter extraction settings (P59 P02).
// Per Phase 59 user decision (CONTEXT.md): max_file_size and
// initial_extraction_on_activation REMAIN under IndexingConfig (already
// present as Indexing.MaxFileSize and Indexing.AutoIndexOnActivate from
// Phase 57). Only the four genuinely-new keys live here.
//
// Field set mirrors SPEC §25.extraction.* verbatim.
type ExtractionConfig struct {
	// ExtractionReadyTimeout bounds RequireReady's wait for an initial
	// extraction to surface ready|partial|failed. Human-readable duration
	// (e.g., "30s").
	ExtractionReadyTimeout string `koanf:"extraction_ready_timeout"`

	// ExtractionFileTimeout bounds per-file extraction. Beyond this the
	// file is dropped to PartialReasonTimeout.
	ExtractionFileTimeout string `koanf:"extraction_file_timeout"`

	// MaxParallelFiles caps the per-workspace extraction worker pool.
	MaxParallelFiles int `koanf:"max_parallel_files"`

	// AllowPartialResults gates whether RequireReady accepts a "partial"
	// outcome as ready. When false, a partial-only extraction blocks
	// consumer queries until ready or failed.
	AllowPartialResults bool `koanf:"allow_partial_results"`
}

// IndexingConfig holds per-snapshot indexing-pipeline settings (P59).
// Field set mirrors SPEC §25.indexing.* verbatim.
type IndexingConfig struct {
	Mode                   string  `koanf:"mode"`                      // "lazy" | "eager" | "on_demand"
	AutoIndexOnActivate    bool    `koanf:"auto_index_on_activate"`    // default false
	MaxFileSize            string  `koanf:"max_file_size"`             // human-readable size, e.g., "2MiB"
	MaxFiles               int     `koanf:"max_files"`                 // ceiling for indexed file count
	FullReindexChangeRatio float64 `koanf:"full_reindex_change_ratio"` // default 0.25
	SnapshotRetention      int     `koanf:"snapshot_retention"`        // default 5
	IncludeGenerated       bool    `koanf:"include_generated"`         // default false
	RequiredForReadyz      bool    `koanf:"required_for_readyz"`       // default false
}

// LiveUpdatesConfig holds fsnotify overlay-watcher settings (P60).
// Field set mirrors SPEC §25.live_updates.* verbatim.
//
// Phase 60 D-05 added the trailing three keys (WatcherEnabled,
// ManifestScanEnabled, ManifestScanInterval) — these gate the 60-05A
// watcher and 60-05B scanner separately so an operator can disable one
// without losing the other (e.g., disable the watcher on a noisy
// containerized FS while keeping the periodic scan).
type LiveUpdatesConfig struct {
	Enabled                  bool   `koanf:"enabled"`                      // default true
	DebounceMS               int    `koanf:"debounce_ms"`                  // default 250
	MaxBatchDelayMS          int    `koanf:"max_batch_delay_ms"`           // default 1500
	BulkChangeThreshold      int    `koanf:"bulk_change_threshold"`        // default 200
	CompactAfterIdleMS       int    `koanf:"compact_after_idle_ms"`        // default 5000
	LSPRevalidateAfterIdleMS int    `koanf:"lsp_revalidate_after_idle_ms"` // default 750
	LSPCompactionMaxWaitMS   int    `koanf:"lsp_compaction_max_wait_ms"`   // default 3000
	MaxOverlayFiles          int    `koanf:"max_overlay_files"`            // default 1000
	MaxOverlayAge            string `koanf:"max_overlay_age"`              // default "30m"

	// Phase 60 D-05 (60-05B):
	// WatcherEnabled gates fsnotify start in 60-05A's watcher.Manager.
	// Default true.
	WatcherEnabled bool `koanf:"watcher_enabled"`
	// ManifestScanEnabled gates the 60-05B periodic scanner.
	// Default true.
	ManifestScanEnabled bool `koanf:"manifest_scan_enabled"`
	// ManifestScanInterval is the human-readable duration (e.g. "10s")
	// between scanner cycles. Parsed via time.ParseDuration in the daemon
	// wiring; values <= 0 fall back to the scanner's 10s default.
	ManifestScanInterval string `koanf:"manifest_scan_interval"`
}

// LSPEnrichmentConfig holds LSP enrichment-worker settings (P61).
// Field set mirrors SPEC §25.lsp_enrichment.* verbatim.
//
// Phase 61 P03 D-02 + D-04 added the trailing two fields:
//   - MaxConcurrentWorkers caps the number of enrichment-worker goroutines
//     draining the 2-lane queue (D-02; default 1, global across languages).
//   - YieldCheckWindowMs bounds how recent a foreground lease has to be to
//     count as "busy" by Pool.ForegroundBusy between cascade steps (D-04;
//     default 200ms).
type LSPEnrichmentConfig struct {
	Enabled                bool   `koanf:"enabled"`
	TimeoutPerFile         string `koanf:"timeout_per_file"`          // default "5s"
	TimeoutTotal           string `koanf:"timeout_total"`             // default "120s"
	MaxSymbolsPerFile      int    `koanf:"max_symbols_per_file"`      // default 200
	MaxReferencesPerSymbol int    `koanf:"max_references_per_symbol"` // default 1000
	MaxReferencesPerFile   int    `koanf:"max_references_per_file"`   // default 5000
	MaxCallHierarchyDepth  int    `koanf:"max_call_hierarchy_depth"`  // default 2
	MaxTypeHierarchyDepth  int    `koanf:"max_type_hierarchy_depth"`  // default 2

	// Phase 61 P03 D-02: enrichment-worker concurrency cap (default 1,
	// global across languages). Cap is read once at daemon startup; runtime
	// reload not in scope.
	MaxConcurrentWorkers int `koanf:"max_concurrent_workers"`
	// Phase 61 P03 D-04: foreground-lease look-back window for
	// Pool.ForegroundBusy (default 200ms). Daemon writes via
	// *lspool.Pool.SetYieldCheckWindow at bootstrap.
	YieldCheckWindowMs int `koanf:"yield_check_window_ms"`
}

// GraphConfig holds graph-engine projection settings (P62).
// Field set mirrors SPEC §25.graph.* verbatim.
type GraphConfig struct {
	MinEdgeConfidence                float64 `koanf:"min_edge_confidence"`                  // default 0.50
	MaxLoadedNodes                   int     `koanf:"max_loaded_nodes"`                     // default 1_000_000
	MaxLoadedEdges                   int     `koanf:"max_loaded_edges"`                     // default 5_000_000
	MaxLocalPagerankNodes            int     `koanf:"max_local_pagerank_nodes"`             // default 5000
	MaxIncrementalClusterRepairNodes int     `koanf:"max_incremental_cluster_repair_nodes"` // default 10_000
}

// PageRankConfig holds cross-file ranking parameters (P62).
// Field set mirrors SPEC §25.pagerank.* verbatim.
//
// Phase 62 P02 (D-08): the trailing three fields configure the
// RankScheduler's debounce + full-recompute behaviour. The scheduler
// itself lands in P03; the keys ship in P02 so the config layer is
// stable across waves.
type PageRankConfig struct {
	Damping       float64 `koanf:"damping"`        // default 0.85
	Epsilon       float64 `koanf:"epsilon"`        // default 1e-6
	MaxIterations int     `koanf:"max_iterations"` // default 100

	// Phase 62 P02 D-08: incremental-repair debounce window.
	RepairDebounceMs int `koanf:"repair_debounce_ms"` // default 2000

	// Phase 62 P02 D-08: longer idle window before a full recompute fires.
	FullRecomputeIdleMs int `koanf:"full_recompute_idle_ms"` // default 60000

	// Phase 62 P02 D-08: stale-row fraction that triggers a full recompute
	// once the long idle elapses.
	FullRecomputeThreshold float64 `koanf:"full_recompute_threshold"` // default 0.25
}

// TypesConfig holds Phase 62 P05 type-resolver settings that don't sit
// under TypeResolution (the latter mirrors SPEC §25.type_resolution.*
// verbatim and we keep that mapping stable).
type TypesConfig struct {
	// CommentParsersEnabled is the Phase 62 P05 D-12 comment-parser
	// allowlist. Default ships the v1 lang scope: TSDoc, JSDoc, GoDoc,
	// Python type comments, PHPDoc, YARD.
	CommentParsersEnabled []string `koanf:"comment_parsers_enabled"`
}

// ClusteringConfig holds symbol-cluster parameters (P62).
// Field set mirrors SPEC §25.clustering.* verbatim.
type ClusteringConfig struct {
	Enabled                     bool `koanf:"enabled"`                         // default true
	MaxComponentSizeBeforeSplit int  `koanf:"max_component_size_before_split"` // default 5000
	LabelPropagationIterations  int  `koanf:"label_propagation_iterations"`    // default 20
}

// RetrievalConfig holds retrieval-tool defaults (P64).
// Field set mirrors SPEC §25.retrieval.* verbatim.
type RetrievalConfig struct {
	DefaultMaxTokens         int  `koanf:"default_max_tokens"`          // default 8000
	IncludeEvidenceByDefault bool `koanf:"include_evidence_by_default"` // default true
}

// GuardrailsConfig holds GuardrailMiddleware settings (P66).
// Field set mirrors SPEC §25.guardrails.* verbatim.
type GuardrailsConfig struct {
	Enabled                       bool   `koanf:"enabled"`                            // default true
	Enforcement                   string `koanf:"enforcement"`                        // "warn" | "block"
	RequireImpactForPublicAPIEdit bool   `koanf:"require_impact_for_public_api_edit"` // default true
	RequireReferencesBeforeRename bool   `koanf:"require_references_before_rename"`   // default true
	RequireReferencesBeforeDelete bool   `koanf:"require_references_before_delete"`   // default true
	RequireVerifyAfterEdit        bool   `koanf:"require_verify_after_edit"`          // default true
	StaleGraphPolicy              string `koanf:"stale_graph_policy"`                 // "warn" | "block"
}

// EvalConfig holds eval-harness settings (P67).
// Field set mirrors SPEC §25.eval.* verbatim.
type EvalConfig struct {
	Enabled               bool     `koanf:"enabled"`                  // default true
	DefaultModes          []string `koanf:"default_modes"`            // default [baseline, native, semantic, semantic_guarded]
	OutputDir             string   `koanf:"output_dir"`               // default ".helix/eval"
	TrackCosts            bool     `koanf:"track_costs"`              // default true
	TrackToolBehavior     bool     `koanf:"track_tool_behavior"`      // default true
	RedactSourceInReports bool     `koanf:"redact_source_in_reports"` // default false
}

// TypeResolutionConfig holds type-resolution pipeline parameters (P62).
// Field set mirrors SPEC §25.type_resolution.* verbatim.
type TypeResolutionConfig struct {
	Enabled               bool    `koanf:"enabled"`                  // default true
	MaxChainDepth         int     `koanf:"max_chain_depth"`          // default 8
	MaxFixpointIterations int     `koanf:"max_fixpoint_iterations"`  // default 8
	MinConfidenceForEdge  float64 `koanf:"min_confidence_for_edge"`  // default 0.45
	CommentFallbacks      bool    `koanf:"comment_fallbacks"`        // default true
	EmitUnresolvedEdges   bool    `koanf:"emit_unresolved_edges"`    // default true
}

// PhaseGraphConfig holds bootstrap phase-graph-runner settings (v1.11+).
// Field set mirrors SPEC §25.phase_graph.* verbatim.
type PhaseGraphConfig struct {
	Enabled           bool `koanf:"enabled"`             // default true
	ValidateOnStartup bool `koanf:"validate_on_startup"` // default true
	FailOnCycle       bool `koanf:"fail_on_cycle"`       // default true
	DumpDotOnError    bool `koanf:"dump_dot_on_error"`   // default true
}
