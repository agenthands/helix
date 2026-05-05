// Package live ships the live-update pipeline spine: typed change signals,
// the path→kind classifier, the per-workspace coalescer, the handler
// dispatch, and the EditNotifier implementation that bridges kernel edits
// into the semantic overlay.
//
// Phase 60 P04 contract (60-CONTEXT.md):
//
//   - WorkspaceChangeSignal carries paths-only (no kind); ChangeSource is
//     a closed enum {helix_edit, fsnotify, manifest_scan} (D-01).
//   - ClassifyPathChange is the SINGLE site that decides SourceChangeKind,
//     reading filesystem state and comparing against semantic_files
//     content_hash (D-01).
//   - The Service.OnEdit method implements kernel.EditNotifier and is
//     non-blocking: select-default-drop into the per-workspace coalescer
//     channel (D-02).
//   - CoalesceEvents is a pure function that implements SPEC §16.2 verbatim
//     and additionally collapses to a single ChangeBulkUpdate event when
//     the merged set exceeds the configurable threshold.
package live

import (
	"time"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/workspace"
)

// ChangeSource identifies which producer generated a signal. Closed enum
// (60-CONTEXT.md D-01).
type ChangeSource string

const (
	// ChangeSourceHelixEdit is emitted by the kernel edit/fileops tools via
	// the EditNotifier hook installed in 60-03. The classifier short-
	// circuits this source — by the time the kernel hook fires, the on-
	// disk content is already in the post-edit state, so the classifier
	// emits ChangeHelixEdit without a stat / hash round-trip.
	ChangeSourceHelixEdit ChangeSource = "helix_edit"

	// ChangeSourceFsnotify is emitted by the filesystem watcher (60-05A).
	ChangeSourceFsnotify ChangeSource = "fsnotify"

	// ChangeSourceManifestScan is emitted by the periodic manifest scanner
	// (60-05B).
	ChangeSourceManifestScan ChangeSource = "manifest_scan"

	// changeSourceCoalescer is the marker used by the CoalesceEvents
	// threshold-collapse path to tag the synthetic ChangeBulkUpdate event.
	// Exposed via ChangeSourceCoalescerInternal() because the coalescer
	// package needs to reference it without re-declaring the constant.
	changeSourceCoalescer ChangeSource = "coalescer"
)

// ChangeSourceCoalescerInternal returns the sentinel ChangeSource the
// coalescer stamps on synthetic ChangeBulkUpdate events. Exported as a
// thunk (rather than a top-level constant) so consumers of the live
// package see only the four producer-facing sources via the named
// constants above; the coalescer marker is reachable but visually
// distinct, mirroring its conceptual scope (internal pipeline marker,
// not a producer).
func ChangeSourceCoalescerInternal() ChangeSource { return changeSourceCoalescer }

// WorkspaceChangeSignal is the paths-only payload every change source emits.
// Classifier owns the kind-decision; sources MUST NOT pre-classify (D-01).
type WorkspaceChangeSignal struct {
	WorkspaceID workspace.WorkspaceKey
	Paths       []string
	Source      ChangeSource
	ObservedAt  time.Time
}

// SourceChangeKind is the post-classifier change taxonomy. Closed enum that
// also serves as a metric label value (helix_semantic_live_updates_total{kind}).
type SourceChangeKind string

const (
	ChangeFileCreated  SourceChangeKind = "file_created"
	ChangeFileModified SourceChangeKind = "file_modified"
	ChangeFileDeleted  SourceChangeKind = "file_deleted"
	ChangeFileRenamed  SourceChangeKind = "file_renamed"
	ChangeHelixEdit    SourceChangeKind = "helix_edit"
	ChangeBulkUpdate   SourceChangeKind = "bulk_update"
)

// SourceChangeEvent is the post-classifier wire type the coalescer consumes
// and the handler dispatches over.
type SourceChangeEvent struct {
	RepoID     semantic.RepoID
	Kind       SourceChangeKind
	Path       string // primary path for the change
	OldPath    string // populated for ChangeFileRenamed
	Paths      []string // populated only for ChangeBulkUpdate (Phase 61 D-05): the per-file path set the bulk-collapse aggregated, so the handler can mark every affected file partial_reason="bulk_update_pending" without losing per-file granularity. Other Kinds leave this nil.
	Source     ChangeSource
	ObservedAt time.Time
}
