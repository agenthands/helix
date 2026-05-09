// Package compact: Phase 63 P63-02 Task 2.
//
// config.go declares the Config struct + DefaultConfig + zero-fill
// helper. Mirrored fields land in semantic.MaintenanceConfig
// (semantic_index.maintenance.*) and semantic.LiveUpdatesConfig
// (semantic_index.live_updates.compact_after_idle_ms / lsp_compaction_max_wait_ms /
// max_overlay_files); the daemon wiring resolves both into Config.

package compact

import "time"

// Config is the per-workspace compactor's tunable surface.
type Config struct {
	// CompactAfterIdle is the quiet-period before the timer fires.
	// Default 5s (semantic_index.live_updates.compact_after_idle_ms).
	CompactAfterIdle time.Duration

	// LSPCompactionMaxWait is the upper bound the gate's BlockedLSPPending
	// check uses to decide whether to keep waiting on outstanding LSP
	// enrichment work or fire compaction anyway.
	LSPCompactionMaxWait time.Duration

	// SnapshotRetention is the number of most-recent committed snapshots
	// to KEEP (CONTEXT.md D-04 / COMPACT-04). Default 5.
	SnapshotRetention int

	// MaxOverlayRows is the pre-flight size-guard ceiling. When
	// OverlayRowCount > MaxOverlayRows the compactor emits
	// outcome=partial and skips. Default 4000 (= max_overlay_files * 4).
	MaxOverlayRows int

	// VacuumEnabled gates the VACUUM piggyback. Default false
	// (CONTEXT.md D-05 / planner instruction).
	VacuumEnabled bool

	// VacuumInterval is the minimum elapsed wall-clock time between
	// successful VACUUM runs. Default 168h.
	VacuumInterval time.Duration
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		CompactAfterIdle:     5 * time.Second,
		LSPCompactionMaxWait: 30 * time.Second,
		SnapshotRetention:    5,
		MaxOverlayRows:       4000,
		VacuumEnabled:        false,
		VacuumInterval:       168 * time.Hour,
	}
}

// mergeDefaults fills zero-value fields with DefaultConfig values. Used
// by NewCompactor so callers can pass a partially-zeroed Config.
func mergeDefaults(cfg Config) Config {
	def := DefaultConfig()
	if cfg.CompactAfterIdle <= 0 {
		cfg.CompactAfterIdle = def.CompactAfterIdle
	}
	if cfg.LSPCompactionMaxWait <= 0 {
		cfg.LSPCompactionMaxWait = def.LSPCompactionMaxWait
	}
	if cfg.SnapshotRetention <= 0 {
		cfg.SnapshotRetention = def.SnapshotRetention
	}
	if cfg.MaxOverlayRows <= 0 {
		cfg.MaxOverlayRows = def.MaxOverlayRows
	}
	if cfg.VacuumInterval <= 0 {
		cfg.VacuumInterval = def.VacuumInterval
	}
	return cfg
}
