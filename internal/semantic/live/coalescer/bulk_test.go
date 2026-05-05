package coalescer_test

import (
	"fmt"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/coalescer"
)

// TestBulkUpdateCollapse asserts that a merged set exceeding the configured
// threshold collapses to a single ChangeBulkUpdate event tagged with the
// first observed RepoID.  Below the threshold, per-path output is preserved.
func TestBulkUpdateCollapse(t *testing.T) {
	const repoID semantic.RepoID = "ws1"
	const threshold = 200

	var events []live.SourceChangeEvent
	for i := 0; i < 250; i++ {
		events = append(events, live.SourceChangeEvent{
			RepoID: repoID,
			Kind:   live.ChangeFileModified,
			Path:   fmt.Sprintf("file%03d.go", i),
		})
	}
	out := coalescer.CoalesceEvents(events, threshold)
	if len(out) != 1 || out[0].Kind != live.ChangeBulkUpdate {
		t.Fatalf("over threshold: expected single bulk_update, got %d events: %+v",
			len(out), out)
	}
	if out[0].RepoID != repoID {
		t.Fatalf("bulk_update: expected RepoID=%q, got %q", repoID, out[0].RepoID)
	}

	// Under threshold: per-path output preserved.
	out2 := coalescer.CoalesceEvents(events[:150], threshold)
	if len(out2) != 150 {
		t.Fatalf("under threshold: expected 150 events, got %d", len(out2))
	}
	for _, ev := range out2 {
		if ev.Kind != live.ChangeFileModified {
			t.Fatalf("under threshold: expected ChangeFileModified, got %v", ev.Kind)
		}
	}
}

// TestBulkUpdateCollapse_ThresholdZeroDisablesCollapse: a non-positive
// threshold disables collapse — useful for tests that want to inspect the
// merged set directly.
func TestBulkUpdateCollapse_ThresholdZeroDisablesCollapse(t *testing.T) {
	var events []live.SourceChangeEvent
	for i := 0; i < 1000; i++ {
		events = append(events, live.SourceChangeEvent{
			RepoID: "ws1",
			Kind:   live.ChangeFileModified,
			Path:   fmt.Sprintf("file%04d.go", i),
		})
	}
	out := coalescer.CoalesceEvents(events, 0)
	if len(out) != 1000 {
		t.Fatalf("threshold=0 should preserve all events; got %d", len(out))
	}
}
