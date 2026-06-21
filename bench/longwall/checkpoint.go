// Package longwall implements a per-cell checkpoint/resume state machine for
// long-horizon benchmark runs whose expected wall-time exceeds a day. It lets a
// Terminal-Bench (or any cell-matrix) run RESUME after a harness restart without
// re-running already-completed cells (Phase 88 SC#3).
//
// The package is a pure stdlib leaf: it deliberately does NOT import bench/runtime,
// bench/container, or any Docker surface, so its three hermetic invariants
// (write/read round-trip, resume-skips-done, idempotent re-entry) fully prove the
// >24h resume guarantee with an injected clock and NO real 24h run.
package longwall

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Cell status values. Only a clean success writes StatusDone; a failed or partial
// (running, never done) checkpoint always re-runs on the next pass.
const (
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// CellState is the persisted checkpoint for a single matrix cell.
type CellState struct {
	CellKey   string `json:"cell_key"`
	Status    string `json:"status"`     // one of running|done|failed
	ResultRef string `json:"result_ref"` // path/handle to the cell's result artifact
	UpdatedAt string `json:"updated_at"` // RFC3339; sourced from the injected clock
}

// Store persists per-cell checkpoints under dir. now is injected so UpdatedAt is
// golden-stable in hermetic tests (mirrors internal/semantic/compact.NewCompactionGate);
// nil defaults to time.Now.
type Store struct {
	dir string
	now func() time.Time
}

// NewStore constructs a checkpoint store rooted at dir. now == nil -> time.Now.
func NewStore(dir string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{dir: dir, now: now}
}

// validateSegment ensures a cellKey component cannot escape the checkpoint dir
// (T-88-03-02): non-empty, no path separators, no traversal. Mirrors the
// validatePathSegment discipline used elsewhere in bench/.
func validateSegment(name, seg string) error {
	if seg == "" {
		return fmt.Errorf("longwall: empty %s segment", name)
	}
	if strings.ContainsRune(seg, '/') || strings.ContainsRune(seg, '\\') || strings.ContainsRune(seg, os.PathSeparator) {
		return fmt.Errorf("longwall: %s segment %q contains a path separator", name, seg)
	}
	if seg == "." || seg == ".." || strings.Contains(seg, "..") {
		return fmt.Errorf("longwall: %s segment %q contains a traversal sequence", name, seg)
	}
	return nil
}

// cellKey builds a stable, path-safe key from the five matrix coordinates. It
// returns an error (rather than panicking) on an invalid (separator-bearing /
// empty / traversal-bearing) segment. Although a malformed coordinate is usually
// a caller programming error, Lang/Task are routinely sourced from dataset
// DIRECTORY names and dataset-file contents (external data), so a single bad cell
// must degrade to a per-cell failure at the driver boundary, NOT abort the entire
// long-wall run (WR-03). The validation stays TOTAL — only the failure mode
// changes from panic to a returned error.
func cellKey(benchmark, lang, mode, task string, runIndex int) (string, error) {
	// Validate in a FIXED order so the surfaced error is deterministic (map
	// iteration order is randomized and would make the error message unstable).
	segs := []struct{ name, seg string }{
		{"benchmark", benchmark},
		{"lang", lang},
		{"mode", mode},
		{"task", task},
	}
	for _, s := range segs {
		if err := validateSegment(s.name, s.seg); err != nil {
			return "", err
		}
	}
	// "__" join keeps the key a single path-safe filename component.
	return strings.Join([]string{benchmark, lang, mode, task, "r" + strconv.Itoa(runIndex)}, "__"), nil
}

// path is the on-disk location of a cell's checkpoint file.
func (s *Store) path(key string) string {
	return filepath.Join(s.dir, key+".json")
}

// writeCheckpoint atomically persists cs to <dir>/<cellKey>.json via a temp file
// in the SAME directory + os.Rename. A crash mid-write leaves either the old file
// or the fully-written new one, never a torn checkpoint (T-88-03-01). This is the
// writeCacheAtomic temp+rename primitive copied locally from
// bench/datasets/swebench-utboost/fetch.go (the leaf has no bench/runtime import).
func (s *Store) writeCheckpoint(cs CellState) error {
	body, err := json.Marshal(cs)
	if err != nil {
		return fmt.Errorf("longwall: marshal checkpoint %q: %w", cs.CellKey, err)
	}
	dst := s.path(cs.CellKey)
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("longwall: mkdir checkpoint dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(dst)+"-*")
	if err != nil {
		return fmt.Errorf("longwall: create temp checkpoint: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("longwall: write temp checkpoint: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("longwall: close temp checkpoint: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("longwall: rename temp checkpoint into %s: %w", dst, err)
	}
	return nil
}

// readCheckpoint loads the checkpoint for key. A missing file is NOT an error: it
// returns (zero, false, nil) so the scheduler runs the cell. A parse error
// surfaces (never a silently fabricated done state — T-88-03-03).
func (s *Store) readCheckpoint(key string) (CellState, bool, error) {
	raw, err := os.ReadFile(s.path(key))
	if err != nil {
		if os.IsNotExist(err) {
			return CellState{}, false, nil
		}
		return CellState{}, false, fmt.Errorf("longwall: read checkpoint %q: %w", key, err)
	}
	var cs CellState
	if err := json.Unmarshal(raw, &cs); err != nil {
		return CellState{}, false, fmt.Errorf("longwall: parse checkpoint %q: %w", key, err)
	}
	return cs, true, nil
}
