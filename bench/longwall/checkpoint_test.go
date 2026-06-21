package longwall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedClock returns a constant time so checkpoint UpdatedAt is golden-stable
// and the >24h resume scenario is provable WITHOUT a real 24h run.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestCheckpointRoundTrip: a done checkpoint written for a cellKey is read back
// as done with the injected clock's fixed UpdatedAt (atomic write/read round-trip).
func TestCheckpointRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fixed := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	s := NewStore(dir, fixedClock(fixed))

	key := cellKey("multiswebench", "go", "agent", "task-001", 0)
	want := CellState{
		CellKey:   key,
		Status:    StatusDone,
		ResultRef: "results/task-001.json",
		UpdatedAt: fixed.Format(time.RFC3339),
	}
	if err := s.writeCheckpoint(want); err != nil {
		t.Fatalf("writeCheckpoint: %v", err)
	}

	got, ok, err := s.readCheckpoint(key)
	if err != nil {
		t.Fatalf("readCheckpoint: %v", err)
	}
	if !ok {
		t.Fatalf("readCheckpoint: ok=false, want true for a written checkpoint")
	}
	if got.Status != StatusDone {
		t.Errorf("Status = %q, want %q", got.Status, StatusDone)
	}
	if got.CellKey != key {
		t.Errorf("CellKey = %q, want %q", got.CellKey, key)
	}
	if got.ResultRef != want.ResultRef {
		t.Errorf("ResultRef = %q, want %q", got.ResultRef, want.ResultRef)
	}
	// UpdatedAt is the injected clock's fixed time, NOT wall time.
	if got.UpdatedAt != fixed.Format(time.RFC3339) {
		t.Errorf("UpdatedAt = %q, want injected-clock %q", got.UpdatedAt, fixed.Format(time.RFC3339))
	}
}

// TestCheckpointNotFound: a missing checkpoint reads as (zero, false, nil) so the
// scheduler runs the cell — an ENOENT is NOT an error that aborts.
func TestCheckpointNotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, nil) // nil clock -> time.Now default

	got, ok, err := s.readCheckpoint(cellKey("multiswebench", "go", "agent", "missing", 0))
	if err != nil {
		t.Fatalf("readCheckpoint missing: unexpected error %v (ENOENT must not abort)", err)
	}
	if ok {
		t.Fatalf("readCheckpoint missing: ok=true, want false")
	}
	if got != (CellState{}) {
		t.Errorf("readCheckpoint missing: got %+v, want zero CellState", got)
	}
}

// TestCheckpointAtomic: after a successful write there is no leftover .tmp-* file
// and the written file unmarshals cleanly (a reader never observes a torn state).
func TestCheckpointAtomic(t *testing.T) {
	dir := t.TempDir()
	fixed := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	s := NewStore(dir, fixedClock(fixed))

	key := cellKey("terminalbench", "python", "agent", "long-task", 3)
	if err := s.writeCheckpoint(CellState{CellKey: key, Status: StatusRunning, UpdatedAt: fixed.Format(time.RFC3339)}); err != nil {
		t.Fatalf("writeCheckpoint: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	jsonCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file after successful write: %s", e.Name())
		}
		if strings.HasSuffix(e.Name(), ".json") {
			jsonCount++
		}
	}
	if jsonCount != 1 {
		t.Errorf("got %d .json files, want exactly 1", jsonCount)
	}

	// The persisted file unmarshals cleanly.
	raw, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var cs CellState
	if err := json.Unmarshal(raw, &cs); err != nil {
		t.Fatalf("Unmarshal persisted checkpoint: %v", err)
	}
	if cs.Status != StatusRunning {
		t.Errorf("persisted Status = %q, want %q", cs.Status, StatusRunning)
	}
}

// TestCellKey: cellKey is a stable function of its five components and path-safe.
func TestCellKey(t *testing.T) {
	a := cellKey("multiswebench", "go", "agent", "task-001", 0)
	b := cellKey("multiswebench", "go", "agent", "task-001", 0)
	if a != b {
		t.Errorf("cellKey not stable: %q != %q", a, b)
	}
	if c := cellKey("multiswebench", "go", "agent", "task-001", 1); c == a {
		t.Errorf("cellKey collision across run_index: %q == %q", c, a)
	}
	// Path-safe: the key must not contain OS path separators that would escape
	// the <ckptDir>/<cellKey>.json join.
	if strings.ContainsRune(a, os.PathSeparator) {
		t.Errorf("cellKey contains path separator: %q", a)
	}
	if strings.Contains(a, "..") {
		t.Errorf("cellKey contains traversal sequence: %q", a)
	}
}
