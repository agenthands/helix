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
// in internal/config/config.go (one koanf tag per leaf).
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

// IndexingConfig holds per-snapshot indexing-pipeline settings (P59).
type IndexingConfig struct {
	BatchSize       int      `koanf:"batch_size"`
	IncludeGlobs    []string `koanf:"include_globs"`
	ExcludeGlobs    []string `koanf:"exclude_globs"`
	MaxFileSizeKB   int      `koanf:"max_file_size_kb"`
	ParallelWorkers int      `koanf:"parallel_workers"`
}

// LiveUpdatesConfig holds fsnotify overlay-watcher settings (P60).
type LiveUpdatesConfig struct {
	DebounceMS  int `koanf:"debounce_ms"`
	MaxOverlay  int `koanf:"max_overlay"`
	IdleFlushMS int `koanf:"idle_flush_ms"`
}

// LSPEnrichmentConfig holds LSP enrichment-worker settings (P61).
type LSPEnrichmentConfig struct {
	Enabled        bool `koanf:"enabled"`
	TimeoutMS      int  `koanf:"timeout_ms"`
	MaxConcurrency int  `koanf:"max_concurrency"`
}

// GraphConfig holds graph-engine projection settings (P62).
type GraphConfig struct {
	Projections []string `koanf:"projections"`
}

// PageRankConfig holds cross-file ranking parameters (P62).
type PageRankConfig struct {
	Damping     float64 `koanf:"damping"`
	Iterations  int     `koanf:"iterations"`
	Tolerance   float64 `koanf:"tolerance"`
	Personalized bool   `koanf:"personalized"`
}

// ClusteringConfig holds symbol-cluster parameters (P62).
type ClusteringConfig struct {
	Algorithm  string  `koanf:"algorithm"`
	MinSize    int     `koanf:"min_size"`
	MaxSize    int     `koanf:"max_size"`
	Resolution float64 `koanf:"resolution"`
}

// RetrievalConfig holds retrieval-tool defaults (P64).
type RetrievalConfig struct {
	DefaultLimit int     `koanf:"default_limit"`
	MaxLimit     int     `koanf:"max_limit"`
	MinScore     float64 `koanf:"min_score"`
}

// GuardrailsConfig holds GuardrailMiddleware settings (P66).
type GuardrailsConfig struct {
	Enabled        bool     `koanf:"enabled"`
	BlockedTools   []string `koanf:"blocked_tools"`
	WarnThreshold  int      `koanf:"warn_threshold"`
}

// EvalConfig holds eval-harness settings (P67).
type EvalConfig struct {
	Enabled    bool   `koanf:"enabled"`
	OutputDir  string `koanf:"output_dir"`
	Iterations int    `koanf:"iterations"`
}

// TypeResolutionConfig holds type-resolution pipeline parameters (P62).
type TypeResolutionConfig struct {
	Enabled        bool `koanf:"enabled"`
	MaxDepth       int  `koanf:"max_depth"`
	CommentFallback bool `koanf:"comment_fallback"`
}

// PhaseGraphConfig holds bootstrap phase-graph-runner settings (v1.11+).
type PhaseGraphConfig struct {
	Enabled bool   `koanf:"enabled"`
	DotPath string `koanf:"dot_path"`
}
