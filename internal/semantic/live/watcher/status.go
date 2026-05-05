// Package watcher ships the per-workspace fsnotify watcher manager that
// produces WorkspaceChangeSignal payloads on the LIVE-01 producer side
// of the live-update pipeline (Phase 60 P05A).
//
// One fsnotify.Watcher goroutine per workspace; cross-workspace
// independence per 60-CONTEXT.md D-05. ENOSPC fallback per LIVE-04.
// Editor-fixture coverage per LIVE-02 (Vim swap-rename, JetBrains
// ___jb_tmp___+rename, VS Code atomic-rename + truncate-write).
//
// The package is consumed by 60-05B daemon wiring, which provides a
// Producer (typically the *service.Service from internal/semantic/live/
// service) and the per-workspace Config. The Status() data accessor on
// the Manager is consumed by Phase 65's get_health MCP tool.
//
// Architectural invariants:
//
//   - vet-nokernel2semantic: this package lives under internal/semantic/
//     and MUST NOT import internal/kernel/* (60-01 D-03 cascade).
//   - vet-noduckdb: this package MUST NOT import duckdb-go directly;
//     overlay writes flow through the Producer → coalescer → handler →
//     overlay tx chain (60-04 wiring).
package watcher

import "sync/atomic"

// WatcherStatus is the data accessor consumed by Phase 65 get_health.
//
// Reason is a closed-enum string label intended for both structured
// logging and metric labelling; allowed values:
//
//   - "running"          — fsnotify producing events normally
//   - "inotify_enospc"   — Linux inotify watch-limit exhaustion (LIVE-04)
//   - "not_started"      — Manager has no workspaceWatcher for this key
//   - "closed"           — Stop() was called or the goroutine returned
//
// RemediationHint is populated only for failure modes (currently
// inotify_enospc); otherwise empty. Callers MUST treat it as
// human-readable diagnostic text, never as a machine label.
type WatcherStatus struct {
	Active          bool
	Reason          string
	RemediationHint string
}

// atomicStatus is a tiny wrapper around atomic.Value that survives
// the empty-Load case (returns the zero WatcherStatus rather than
// panicking on the nil interface assertion).
type atomicStatus struct{ v atomic.Value }

// Store atomically replaces the current status snapshot.
func (a *atomicStatus) Store(s WatcherStatus) { a.v.Store(s) }

// Load returns a snapshot of the current status. Returns the zero
// WatcherStatus before the first Store, which conveniently encodes
// Active=false, Reason="" — distinguishable from "running" in the
// status enum.
func (a *atomicStatus) Load() WatcherStatus {
	v := a.v.Load()
	if v == nil {
		return WatcherStatus{}
	}
	return v.(WatcherStatus)
}
