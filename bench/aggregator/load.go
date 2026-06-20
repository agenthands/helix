package aggregator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/agenthands/helix/bench/runtime"
)

// load.go implements the STATS-01 consumer half: the durable-tree loader and
// the fail-closed N-enforcement gate (D-05). It globs the run directory laid out
// as <runDir>/<task>/<mode>/<run_index>/result.v2.json (the layout
// cellDurablePaths produces), decodes each row preserving the full document AND
// a nullable-pointer metric subset (never fabricating 0 — Pitfall 4), and counts
// VALID rows per (task, mode) where valid = exists + JSON-decodes +
// runtime.Validate==nil. It FAILS CLOSED: if any cell has fewer than expectedN
// valid rows it returns a hard error naming every deficient cell and the caller
// writes NO reports. expectedN is the caller's --runs/manifest value, NEVER
// len(glob) on disk (Pitfall 3 — a disk count hides a partial matrix).

// rowMetrics is the nullable-pointer metric subset the aggregator reads from a
// row's `metrics` object. It MIRRORS the evaluators.Metrics pointer types so an
// absent/null metric stays nil and is never decoded to a fabricated 0 (Pitfall
// 4). Only the fields the downstream rollups consume are listed; additional
// metric keys remain available via Row.Doc["metrics"].
type rowMetrics struct {
	TaskSuccess           *bool    `json:"task_success"`
	VerifiedCorrectness   *bool    `json:"verified_correctness"`
	TokensInput           *int     `json:"tokens_input"`
	TokensOutput          *int     `json:"tokens_output"`
	TokensInputCachedRead *int     `json:"tokens_input_cached_read"`
	TokensInputCacheWrite *int     `json:"tokens_input_cache_write"`
	ToolCalls             *int     `json:"tool_calls"`
	WallTimeSeconds       *float64 `json:"wall_time_seconds"`
	FilesRead             *int     `json:"files_read"`
	FilesModified         *int     `json:"files_modified"`
	EditLocality          *float64 `json:"edit_locality"`
}

// Row is one valid result.v2.json row: the full document preserved verbatim for
// any write-back, plus the decoded nullable metric subset.
type Row struct {
	RunIndex int
	Doc      map[string]json.RawMessage
	Metrics  rowMetrics
}

// Loaded is the grouped result of a successful load: valid rows indexed by
// (task, mode) in deterministic order. It is returned only when EVERY discovered
// cell has at least expectedN valid rows; otherwise Load returns (nil, error).
type Loaded struct {
	// rows: task -> mode -> []Row (run-index ascending).
	rows map[string]map[string][]Row
	// taskOrder / modeOrder preserve deterministic iteration (sorted keys).
	taskOrder []string
}

// Rows returns the grouped valid rows for one (task, mode) cell, or nil if the
// cell was not present.
func (l *Loaded) Rows(task, mode string) []Row {
	if l == nil || l.rows == nil {
		return nil
	}
	if modes, ok := l.rows[task]; ok {
		return modes[mode]
	}
	return nil
}

// Tasks returns the discovered task IDs in deterministic (sorted) order.
func (l *Loaded) Tasks() []string {
	if l == nil {
		return nil
	}
	return l.taskOrder
}

// Modes returns the discovered mode IDs for one task in deterministic (sorted)
// order, or nil if the task was not present. The aggregator orchestrator uses it
// to enumerate the (mode x benchmark) leaderboard rows.
func (l *Loaded) Modes(task string) []string {
	if l == nil || l.rows == nil {
		return nil
	}
	modes, ok := l.rows[task]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(modes))
	for mode := range modes {
		out = append(out, mode)
	}
	sort.Strings(out)
	return out
}

// candidate is one on-disk result.v2.json file before validation.
type candidate struct {
	task     string
	mode     string
	runIndex int
	path     string
}

// Load globs the durable run tree under runDir, groups VALID rows by (task,
// mode), and enforces the fail-closed N-gate against expectedN. expectedN is the
// caller's expected run count (from --runs/manifest); it is NEVER derived from
// the number of files on disk (Pitfall 3). If any discovered cell has fewer than
// expectedN valid rows, Load returns (nil, error) naming every deficient cell —
// the caller must write NOTHING.
func Load(runDir string, expectedN int) (*Loaded, error) {
	cands, err := globRows(runDir)
	if err != nil {
		return nil, err
	}

	// Fail-CLOSED on zero discovery (CR-01). filepath.Glob returns (nil, nil) for
	// a non-existent directory, so a typo'd / missing / empty runDir would yield
	// zero candidates, an empty byTask, an empty (vacuously-passing) N-gate, and a
	// clean, authoritative-looking EMPTY report — the exact "partial/empty matrix
	// silently accepted" failure the fail-closed gate (D-05, Pitfall 3) exists to
	// prevent. An empty/wrong runDir is an ERROR, never an empty success report.
	if len(cands) == 0 {
		return nil, fmt.Errorf("aggregate: no result.v2.json rows discovered under %q "+
			"(expected <task>/<mode>/<run_index>/result.v2.json)", runDir)
	}

	// Group VALID rows by (task, mode). A row is valid iff it reads, decodes, and
	// passes runtime.Validate. Invalid rows are excluded so they cannot pad the
	// count toward expectedN (the invalid-row-counts-as-deficient contract).
	byTask := make(map[string]map[string][]Row)
	for _, c := range cands {
		row, ok := validRow(c)
		if !ok {
			// Ensure the cell is still discovered so the gate can flag it as
			// deficient rather than silently absent.
			ensureCell(byTask, c.task, c.mode)
			continue
		}
		ensureCell(byTask, c.task, c.mode)
		byTask[c.task][c.mode] = append(byTask[c.task][c.mode], row)
	}

	// Fail-closed N-gate (D-05): name every (task, mode) with fewer than
	// expectedN valid rows. expectedN is the function argument, not len(glob).
	deficient := deficientCells(byTask, expectedN)
	if len(deficient) > 0 {
		return nil, fmt.Errorf("aggregate: insufficient runs: %v", deficient)
	}

	// Deterministic ordering: sort task keys and per-task rows by run index.
	taskOrder := make([]string, 0, len(byTask))
	for task := range byTask {
		taskOrder = append(taskOrder, task)
	}
	sort.Strings(taskOrder)
	for _, modes := range byTask {
		for _, rows := range modes {
			sort.Slice(rows, func(i, j int) bool { return rows[i].RunIndex < rows[j].RunIndex })
		}
	}

	return &Loaded{rows: byTask, taskOrder: taskOrder}, nil
}

// ensureCell makes sure the (task, mode) cell exists in the map so a cell whose
// only rows are invalid is still discovered (and therefore flagged deficient).
func ensureCell(byTask map[string]map[string][]Row, task, mode string) {
	if _, ok := byTask[task]; !ok {
		byTask[task] = make(map[string][]Row)
	}
	if _, ok := byTask[task][mode]; !ok {
		byTask[task][mode] = nil
	}
}

// globRows enumerates every <runDir>/<task>/<mode>/<run_index>/result.v2.json
// candidate. It is rooted at runDir via filepath.Join and only joins discovered
// directory entries under runDir — it never accepts a caller-supplied raw path
// (T-82-05-02). run_index segments that are not non-negative integers are
// skipped.
func globRows(runDir string) ([]candidate, error) {
	pattern := filepath.Join(runDir, "*", "*", "*", "result.v2.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("aggregate: glob run dir: %w", err)
	}
	sort.Strings(matches)

	cands := make([]candidate, 0, len(matches))
	for _, m := range matches {
		// .../<task>/<mode>/<run_index>/result.v2.json
		runDirSeg := filepath.Dir(m)
		idxSeg := filepath.Base(runDirSeg)
		modeDir := filepath.Dir(runDirSeg)
		mode := filepath.Base(modeDir)
		taskDir := filepath.Dir(modeDir)
		task := filepath.Base(taskDir)

		runIndex, convErr := strconv.Atoi(idxSeg)
		if convErr != nil || runIndex < 0 {
			// Non-numeric / negative run_index segment is not a real run dir.
			continue
		}
		cands = append(cands, candidate{task: task, mode: mode, runIndex: runIndex, path: m})
	}
	return cands, nil
}

// validRow reads, decodes, and schema-validates a candidate. It returns the
// decoded Row and true only when the file exists, JSON-decodes into a full doc,
// AND runtime.Validate(bytes)==nil. Any failure returns ok=false so the row is
// excluded from the valid count (valid != file-present, Pitfall 3).
func validRow(c candidate) (Row, bool) {
	b, err := os.ReadFile(c.path)
	if err != nil {
		return Row{}, false
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return Row{}, false
	}
	if err := runtime.Validate(b); err != nil {
		return Row{}, false
	}
	var rm rowMetrics
	if raw, ok := doc["metrics"]; ok {
		// A decode failure on the metrics object leaves rm at its zero (all-nil)
		// value; nil stays nil, never a fabricated 0 (Pitfall 4).
		_ = json.Unmarshal(raw, &rm)
	}
	return Row{RunIndex: c.runIndex, Doc: doc, Metrics: rm}, true
}

// deficientCells returns sorted "<task>/<mode>: got X want N" strings for every
// discovered cell with fewer than expectedN valid rows.
func deficientCells(byTask map[string]map[string][]Row, expectedN int) []string {
	var deficient []string
	for task, modes := range byTask {
		for mode, rows := range modes {
			if len(rows) < expectedN {
				deficient = append(deficient,
					fmt.Sprintf("%s/%s: got %d want %d", task, mode, len(rows), expectedN))
			}
		}
	}
	sort.Strings(deficient)
	return deficient
}
