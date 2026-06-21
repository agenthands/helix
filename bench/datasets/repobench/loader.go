// Package repobench is the dataset-loader-only RepoBench adapter
// (ADAPTER-REPO-01). It does NOT run any upstream harness: it fetches the
// per-language RepoBench HuggingFace parquet at a PINNED immutable rev (pin.go)
// into $HELIX_CACHE_DIR/repobench/<rev>/ (fetch.go), decodes it with arrow-go's
// parquet/pqarrow, and maps each row to a Task carrying the sub-task-relevant
// fields for RepoBench-R (retrieval), RepoBench-C (completion), and RepoBench-P
// (pipeline), for Python and Java.
//
// The three sub-tasks share one row schema (RepoBench v1.1 ships
// repo_name, file_path, context[], import_statement, cropped_code, all_code,
// next_line, gold_snippet_index, token_num, level; splits cross_file_first /
// cross_file_random / in_file):
//
//   - RepoBench-R (retrieval): scored by acc@k — whether the gold cross-file
//     snippet, indexed by GoldSnippetIndex within Context, is ranked within the
//     top-k of the retrieval ordering (score.go AccAtK).
//   - RepoBench-C (completion): scored by EM + normalized ES on NextLine, reusing
//     the Plan 01 exactmatch + editsim scorers (score.go CompletionScore).
//   - RepoBench-P (pipeline = retrieve-then-complete): scored EM/ES on NextLine
//     like -C, after a retrieval step like -R.
//
// It is a LEAF package: it imports only the Go standard library + arrow-go,
// mirroring the bench/ragindex + bench/aider-polyglot + bench/datasets/crosscodeeval
// leaf discipline (NO internal/kernel, internal/semantic, or bench/runtime
// imports). Confining the parquet decode to this package keeps the leaf
// invariant. The score.go scorers additionally import the Plan 01
// exactmatch/editsim leaf packages (themselves stdlib-only), which preserves the
// leaf invariant.
//
// The hermetic fixture test (loader_test.go + score_test.go: committed
// RepoBench-shaped rows under fixtures/{python,java}/{retrieval,completion}.json +
// a committed small testdata/sample.parquet) is the SOLE authoritative proof of
// the row→Task mapping, the arrow-go parquet decode, and the acc@k + EM/ES
// scoring. The live HF fetch (fetch_test.go's TestLiveFetch) is
// HELIX_BENCH_NETWORK-gated and SKIPs cleanly offline — it is NEVER the sole
// proof. Fixture VALUES are sourced from the RepoBench paper (Liu et al., ICLR
// 2024, arXiv:2306.03091) / the tianyang/repobench_*_v1.1 dataset-card field
// list, not the HF auto-loader/viewer; see each fixture's provenance field.
package repobench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// Languages is the fixed set of RepoBench languages this adapter loads. RepoBench
// v1.1 ships Python and Java as separate per-language repos (pin.go); an unknown
// language fails closed before any filepath.Join (validatePathSegment + this
// allow-list).
var Languages = []string{"python", "java"}

// Sub-task identifiers. RepoBench-R is retrieval (acc@k over Context via
// GoldSnippetIndex); RepoBench-C is completion (EM/ES on NextLine); RepoBench-P
// is the retrieve-then-complete pipeline (EM/ES on NextLine after retrieval).
const (
	TaskRetrieval  = "retrieval"
	TaskCompletion = "completion"
	TaskPipeline   = "pipeline"
)

// Task is a loaded, validated RepoBench task. The same struct models all three
// sub-tasks; which fields are authoritative depends on Task:
//
//   - TaskRetrieval (-R): Context + GoldSnippetIndex are authoritative — acc@k
//     ranks GoldSnippetIndex within Context.
//   - TaskCompletion (-C) / TaskPipeline (-P): NextLine is authoritative — EM/ES
//     score the predicted completion against it.
//
// There is no test suite: RepoBench is a completion/retrieval benchmark scored by
// string-level + index-level oracles, never by running code.
type Task struct {
	// Task is the sub-task: TaskRetrieval, TaskCompletion, or TaskPipeline.
	Task string
	// Language is one of Languages (python/java); it populates the result.v2
	// `language` provenance key the aggregator slices by.
	Language string
	// Split is the RepoBench split the row came from: cross_file_first,
	// cross_file_random, or in_file.
	Split string
	// Level is the RepoBench context-length bucket (e.g. "2k", "8k"); carried for
	// provenance + per-level slicing.
	Level string
	// RepoName is the source repository (RepoBench `repo_name`); provenance only.
	RepoName string
	// FilePath is the in-repo path of the completed file (RepoBench `file_path`).
	FilePath string
	// CroppedCode is the in-file left context the completion model continues from
	// (RepoBench `cropped_code`). Never empty for a valid task.
	CroppedCode string
	// Context is the list of candidate cross-file snippets (RepoBench `context`).
	// Authoritative for TaskRetrieval (acc@k ranks GoldSnippetIndex within it).
	Context []string
	// GoldSnippetIndex is the index of the gold cross-file snippet within Context
	// (RepoBench `gold_snippet_index`). Authoritative for TaskRetrieval; -1 when
	// not applicable (e.g. a completion-only row).
	GoldSnippetIndex int
	// NextLine is the single ground-truth completion line (RepoBench `next_line`).
	// Authoritative for TaskCompletion / TaskPipeline (EM/ES score against it).
	NextLine string
}

// validatePathSegment rejects names that could escape a join root once they
// become a path segment — a clone of crosscodeeval.validatePathSegment /
// aiderpolyglot.validatePathSegment (T-86-04-03), kept in this leaf package so it
// carries no bench/runtime import. It rejects "", anything filepath.Clean
// rewrites ("..", "a//b"), any embedded separator, and a leading dot. MUST run
// BEFORE any filepath.Join.
func validatePathSegment(name, kind string) error {
	if name == "" {
		return fmt.Errorf("repobench: %s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("repobench: %s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

// validLanguage reports whether lang is one of the supported RepoBench languages.
// It is the allow-list half of the language guard (validatePathSegment is the
// path-safety half); an unknown language fails closed.
func validLanguage(lang string) bool {
	for _, l := range Languages {
		if l == lang {
			return true
		}
	}
	return false
}

// decodeParquet decodes a RepoBench parquet payload (held entirely in memory —
// the caller size-caps the download via io.LimitReader, T-86-04-04) into []Task
// for the given language. It reads the columns it needs (task, split, level,
// repo_name, file_path, cropped_code, next_line, gold_snippet_index, context_json)
// and validates each row decode-then-validate: a missing required column or an
// empty cropped_code is a typed error, never a panic, so a completion
// coordinator can null-on-failure rather than crash. The list-valued RepoBench
// `context` field is carried as a JSON-encoded string column (`context_json`) so
// the parquet stays a flat string/int table; it is decoded back to []string
// here. The parquet decode is confined to this package to preserve the leaf
// invariant.
//
// When the parquet carries an optional `language` column, the per-row value is
// authoritative and rows for other languages are skipped (the hermetic fixture
// bundles both languages in one file); when absent (a real per-language parquet),
// every row is tagged with the requested `language` arg.
func decodeParquet(ctx context.Context, raw []byte, language string) ([]Task, error) {
	if !validLanguage(language) {
		return nil, fmt.Errorf("repobench: unsupported language %q", language)
	}
	rdr := bytes.NewReader(raw)
	tbl, err := pqarrow.ReadTable(ctx, rdr, nil, pqarrow.ArrowReadProperties{}, memory.DefaultAllocator)
	if err != nil {
		return nil, fmt.Errorf("repobench: decode parquet (%s): %w", language, err)
	}
	defer tbl.Release()

	tasks, err := stringColumn(tbl, "task")
	if err != nil {
		return nil, err
	}
	crops, err := stringColumn(tbl, "cropped_code")
	if err != nil {
		return nil, err
	}
	// Optional columns: a missing optional column yields a nil slice and the row
	// default is used.
	splits, _ := stringColumn(tbl, "split")
	levels, _ := stringColumn(tbl, "level")
	repos, _ := stringColumn(tbl, "repo_name")
	files, _ := stringColumn(tbl, "file_path")
	nextLines, _ := stringColumn(tbl, "next_line")
	contexts, _ := stringColumn(tbl, "context_json")
	golds, _ := int64Column(tbl, "gold_snippet_index")
	langs, _ := stringColumn(tbl, "language")

	if len(tasks) != len(crops) {
		return nil, fmt.Errorf("repobench: column length mismatch task=%d cropped_code=%d", len(tasks), len(crops))
	}

	out := make([]Task, 0, len(tasks))
	for i := range tasks {
		rowLang := language
		if i < len(langs) && langs[i] != "" {
			rowLang = langs[i]
			if rowLang != language {
				continue // skip other-language rows in a mixed fixture
			}
		}
		if tasks[i] == "" {
			return nil, fmt.Errorf("repobench: %s row %d has empty task sub-type", language, i)
		}
		if crops[i] == "" {
			return nil, fmt.Errorf("repobench: %s row %d has empty cropped_code", language, i)
		}

		var contextList []string
		if i < len(contexts) && contexts[i] != "" {
			if err := json.Unmarshal([]byte(contexts[i]), &contextList); err != nil {
				return nil, fmt.Errorf("repobench: %s row %d context_json decode: %w", language, i, err)
			}
		}

		// gold_snippet_index is an untrusted parquet int64. Bound-check it in
		// int64 space BEFORE narrowing to a platform int: on a 32-bit build an
		// int64 > 2^31-1 would wrap to a negative/unrelated value, and the error
		// would then report the wrapped value, not the real one (WR-02). We reject
		// an out-of-int-range value here and report the ORIGINAL int64.
		gold := -1
		if i < len(golds) {
			g := golds[i]
			if g < math.MinInt || g > math.MaxInt {
				return nil, fmt.Errorf("repobench: %s row %d gold_snippet_index %d out of platform int range", language, i, g)
			}
			gold = int(g)
		}

		t := Task{
			Task:             tasks[i],
			Language:         rowLang,
			Split:            stringAt(splits, i),
			Level:            stringAt(levels, i),
			RepoName:         stringAt(repos, i),
			FilePath:         stringAt(files, i),
			CroppedCode:      crops[i],
			Context:          contextList,
			GoldSnippetIndex: gold,
			NextLine:         stringAt(nextLines, i),
		}

		// Decode-then-validate the sub-task invariants.
		switch t.Task {
		case TaskRetrieval:
			if len(t.Context) == 0 {
				return nil, fmt.Errorf("repobench: %s row %d is retrieval but has empty context", language, i)
			}
			if t.GoldSnippetIndex < 0 || t.GoldSnippetIndex >= len(t.Context) {
				return nil, fmt.Errorf("repobench: %s row %d gold_snippet_index %d out of range [0,%d)", language, i, t.GoldSnippetIndex, len(t.Context))
			}
		case TaskCompletion, TaskPipeline:
			if t.NextLine == "" {
				return nil, fmt.Errorf("repobench: %s row %d is %s but has empty next_line", language, i, t.Task)
			}
		default:
			return nil, fmt.Errorf("repobench: %s row %d has unknown task %q", language, i, t.Task)
		}

		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("repobench: %s parquet decoded zero tasks", language)
	}
	return out, nil
}

// stringAt returns s[i] or "" when i is out of range (an optional column that was
// shorter than the required columns, or absent entirely).
func stringAt(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}

// stringColumn extracts a UTF-8 string column from an arrow.Table by name,
// flattening every chunk in column order. A column whose name is absent returns a
// typed error (so a required column missing is caught explicitly); a non-string
// column is also a typed error. It never panics on a malformed table.
func stringColumn(tbl arrow.Table, name string) ([]string, error) {
	idx := columnIndex(tbl, name)
	if idx < 0 {
		return nil, fmt.Errorf("repobench: required column %q absent from parquet schema", name)
	}
	col := tbl.Column(idx)
	out := make([]string, 0, tbl.NumRows())
	for _, chunk := range col.Data().Chunks() {
		sa, ok := chunk.(*array.String)
		if !ok {
			return nil, fmt.Errorf("repobench: column %q is not a UTF-8 string column (got %T)", name, chunk)
		}
		for r := 0; r < sa.Len(); r++ {
			if sa.IsNull(r) {
				out = append(out, "")
				continue
			}
			out = append(out, sa.Value(r))
		}
	}
	return out, nil
}

// int64Column extracts an int64 column from an arrow.Table by name. An absent
// column is a typed error (callers that treat it as optional ignore the error).
func int64Column(tbl arrow.Table, name string) ([]int64, error) {
	idx := columnIndex(tbl, name)
	if idx < 0 {
		return nil, fmt.Errorf("repobench: column %q absent from parquet schema", name)
	}
	col := tbl.Column(idx)
	out := make([]int64, 0, tbl.NumRows())
	for _, chunk := range col.Data().Chunks() {
		ia, ok := chunk.(*array.Int64)
		if !ok {
			return nil, fmt.Errorf("repobench: column %q is not an int64 column (got %T)", name, chunk)
		}
		for r := 0; r < ia.Len(); r++ {
			if ia.IsNull(r) {
				out = append(out, -1)
				continue
			}
			out = append(out, ia.Value(r))
		}
	}
	return out, nil
}

// columnIndex returns the schema index of the named column, or -1 if absent.
func columnIndex(tbl arrow.Table, name string) int {
	for i, f := range tbl.Schema().Fields() {
		if f.Name == name {
			return i
		}
	}
	return -1
}

// Load fetches (network-gated) and decodes the RepoBench parquet for the given
// language at its pinned rev, returning the per-language []Task. The language is
// validatePathSegment-checked AND allow-list-checked before any filepath.Join or
// URL build (T-86-04-03). The fetch is cached under
// $HELIX_CACHE_DIR/repobench/<rev>/ (fetch.go); offline it returns an error the
// caller treats as "skip live, rely on the hermetic fixture proof". Load is the
// live entry point; LoadParquetBytes is the hermetic decode the fixture test
// drives directly over committed bytes.
func Load(ctx context.Context, language string) ([]Task, error) {
	if err := validatePathSegment(language, "language"); err != nil {
		return nil, err
	}
	if !validLanguage(language) {
		return nil, fmt.Errorf("repobench: unsupported language %q", language)
	}
	rev := PinnedRev(language)
	if rev == "" {
		return nil, fmt.Errorf("repobench: no pinned rev for language %q", language)
	}
	raw, err := Fetch(ctx, rev, language)
	if err != nil {
		return nil, err
	}
	return decodeParquet(ctx, raw, language)
}

// LoadParquetBytes decodes an in-memory RepoBench parquet payload for the given
// language WITHOUT any network — the hermetic decode path the fixture test drives
// over the committed testdata/sample.parquet. It is the sole-proof surface: it
// exercises the exact arrow-go decode + row→Task mapping Load uses, minus the
// fetch.
func LoadParquetBytes(ctx context.Context, raw []byte, language string) ([]Task, error) {
	if err := validatePathSegment(language, "language"); err != nil {
		return nil, err
	}
	return decodeParquet(ctx, raw, language)
}
