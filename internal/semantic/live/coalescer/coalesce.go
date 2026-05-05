// Package coalescer ships SPEC §16.2 event coalescing for the live-update
// pipeline. The package is split into two layers:
//
//   - coalesce.go (this file) is purely functional: CoalesceEvents +
//     MergeChange take an input slice and return a deterministic merged
//     slice with no I/O, no goroutines, no time dependencies. This is the
//     unit-tested algorithmic core (60-04 acceptance #4 and #5).
//   - coalescer.go ships the per-workspace single-goroutine debounce +
//     flush + dispatch around the pure core.
//
// The two-layer split is load-bearing for testability: the merge rules are
// table-driven; the goroutine concerns (timer races, context cancellation,
// non-blocking enqueue) carry their own concurrency-shaped tests.
package coalescer

import (
	"sort"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
)

// CoalesceEvents implements SPEC §16.2 verbatim. Pure: same input slice
// + same threshold yields byte-identical output (ordering pinned by a
// post-merge sort by Path).
//
// Rules (from SPEC §16.2):
//
//	modified+modified → modified  (last-write-wins on ObservedAt)
//	created+created   → created   (last-write-wins; idempotent)
//	created+modified  → created   (created wins; takes new ObservedAt)
//	created+deleted   → no-op     (path drops out of result)
//	modified+deleted  → deleted
//	deleted+created   → modified  (resurrection)
//	rename+modified-on-newpath → rename (rename absorbs the new-side modify)
//	any other combo  → last-write-wins (cur replaces prev)
//
// Threshold collapse: if threshold > 0 AND len(merged) > threshold, the
// result is exactly one synthetic ChangeBulkUpdate event tagged with the
// first non-empty RepoID in the input. Pass threshold <= 0 to disable
// collapse (useful in tests that want to inspect the merged set directly).
func CoalesceEvents(events []live.SourceChangeEvent, threshold int) []live.SourceChangeEvent {
	if len(events) == 0 {
		return nil
	}
	byKey := make(map[string]live.SourceChangeEvent, len(events))
	for _, ev := range events {
		key := mergeKey(ev)
		prev, ok := byKey[key]
		if !ok {
			byKey[key] = ev
			continue
		}
		merged, keep := MergeChange(prev, ev)
		if !keep {
			delete(byKey, key)
			continue
		}
		byKey[key] = merged
	}
	if len(byKey) == 0 {
		return nil
	}
	if threshold > 0 && len(byKey) > threshold {
		// Phase 61 D-05: the synthetic ChangeBulkUpdate event carries the
		// merged path set so the handler can mark every affected file
		// partial_reason="bulk_update_pending" via OverlayTx.MarkFileSemanticPending.
		// Without Paths, the producer would lose all per-file granularity at
		// the collapse boundary. We collect deterministic-order paths
		// (alphabetic) so dispatch is reproducible across runs.
		paths := make([]string, 0, len(byKey))
		for _, ev := range byKey {
			// Use Path for non-rename kinds; for renames, take the new
			// path (Path field) — that's the path the file lives at after
			// the bulk operation lands.
			paths = append(paths, ev.Path)
		}
		sort.Strings(paths)
		return []live.SourceChangeEvent{{
			RepoID: firstRepoID(events),
			Kind:   live.ChangeBulkUpdate,
			Paths:  paths,
			Source: changeSourceCoalescerInternal(),
		}}
	}
	result := make([]live.SourceChangeEvent, 0, len(byKey))
	for _, v := range byKey {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].OldPath < result[j].OldPath
		}
		return result[i].Path < result[j].Path
	})
	return result
}

// MergeChange applies the SPEC §16.2 merge table. Returns (merged, keep)
// where keep=false means the path should drop out (created+deleted no-op).
//
// MergeChange is pure: no I/O, no time.Now, no shared state.
func MergeChange(prev, cur live.SourceChangeEvent) (live.SourceChangeEvent, bool) {
	switch {
	case prev.Kind == live.ChangeFileModified && cur.Kind == live.ChangeFileModified:
		return cur, true
	case prev.Kind == live.ChangeFileCreated && cur.Kind == live.ChangeFileCreated:
		// last-write-wins, idempotent — explicit case rather than relying
		// on the default branch so the test matrix can pin it.
		return cur, true
	case prev.Kind == live.ChangeFileCreated && cur.Kind == live.ChangeFileModified:
		prev.ObservedAt = cur.ObservedAt
		return prev, true
	case prev.Kind == live.ChangeFileCreated && cur.Kind == live.ChangeFileDeleted:
		return live.SourceChangeEvent{}, false
	case prev.Kind == live.ChangeFileModified && cur.Kind == live.ChangeFileDeleted:
		return cur, true
	case prev.Kind == live.ChangeFileDeleted && cur.Kind == live.ChangeFileCreated:
		return live.SourceChangeEvent{
			Kind:       live.ChangeFileModified,
			Path:       cur.Path,
			RepoID:     cur.RepoID,
			Source:     cur.Source,
			ObservedAt: cur.ObservedAt,
		}, true
	case prev.Kind == live.ChangeFileRenamed && cur.Kind == live.ChangeFileModified && cur.Path == prev.Path:
		// rename already covers the new-side path; keep rename, advance time
		prev.ObservedAt = cur.ObservedAt
		return prev, true
	default:
		// last-write-wins fallback for any unmapped combo
		return cur, true
	}
}

// mergeKey computes the bucket key for an event. Renames are keyed by the
// (OldPath→Path) pair so rename(a→b) does not silently merge with create(b).
func mergeKey(ev live.SourceChangeEvent) string {
	if ev.Kind == live.ChangeFileRenamed {
		return ev.OldPath + "→" + ev.Path // U+2192 RIGHTWARDS ARROW
	}
	return ev.Path
}

// firstRepoID returns the first non-empty RepoID in events, or "" if none.
func firstRepoID(events []live.SourceChangeEvent) semantic.RepoID {
	for _, ev := range events {
		if ev.RepoID != "" {
			return ev.RepoID
		}
	}
	return ""
}

// changeSourceCoalescerInternal returns the package-internal sentinel
// ChangeSource that tags the synthetic bulk_update event.  It is hidden
// behind a thunk so the live package can keep changeSourceCoalescer
// unexported but still pass it across package boundaries.
func changeSourceCoalescerInternal() live.ChangeSource {
	return live.ChangeSourceCoalescerInternal()
}
