// Package crosscodeeval is the dataset-loader-only CrossCodeEval adapter
// (ADAPTER-CCE-01). It does NOT run any upstream harness: it fetches the
// CrossCodeEval HuggingFace parquet at a PINNED immutable rev (pin.go) into
// $HELIX_CACHE_DIR/crosscodeeval/<rev>/ (fetch.go), decodes it with arrow-go's
// parquet/pqarrow, and maps each row to a Task{Prompt, GroundTruth, Language,
// Split} for Python / Java / TypeScript / C#. The Language field populates the
// result.v2 `language` provenance key consumed by the aggregator's ByLanguage
// per-language slice.
//
// It is a LEAF package: it imports only the Go standard library + arrow-go,
// mirroring the bench/ragindex + bench/aider-polyglot leaf discipline (NO
// internal/kernel, internal/semantic, or bench/runtime imports). Confining the
// parquet decode to this package keeps the leaf invariant.
//
// The hermetic fixture test (loader_test.go: committed CCE-shaped rows under
// fixtures/{python,java,typescript,csharp}/task.json + a committed small
// testdata/sample.parquet) is the SOLE authoritative proof of the row→Task
// mapping and the arrow-go parquet decode. The live HF fetch (fetch_test.go's
// TestLiveFetch) is HELIX_BENCH_NETWORK-gated and SKIPs cleanly offline — it is
// NEVER the sole proof (the Vincentvmt/CrossCodeEval HF viewer has a known cast
// error, RESEARCH Pitfall 3, so fixture VALUES are sourced from the CCE paper
// arXiv:2310.11248 / the official amazon-science/cceval repo, not the buggy
// viewer; see each fixture's provenance comment).
package crosscodeeval

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// Languages is the fixed set of CrossCodeEval languages this adapter loads. The
// Vincentvmt/CrossCodeEval mirror ships these four; an unknown language fails
// closed before any filepath.Join (validatePathSegment + this allow-list).
var Languages = []string{"python", "java", "typescript", "csharp"}

// Task is a loaded, validated CrossCodeEval completion task. It is the unit the
// Plan 01 EM / edit-similarity / identifier-match scorers grade and the Plan 02
// multi-oracle gate consumes. There is no test suite: the ground truth is a
// single completion string per task (CCE is a line-completion benchmark).
type Task struct {
	// Prompt is the left context the model completes from (CCE `prompt`): the
	// cropped in-file code up to the cursor, optionally prefixed with the
	// cross-file retrieved context. Never empty for a valid task.
	Prompt string
	// GroundTruth is the single target completion line (CCE `groundtruth`): the
	// string the EM/ES/identifier oracles score the model output against. Never
	// empty for a valid task.
	GroundTruth string
	// Language is one of Languages (python/java/typescript/csharp); it populates
	// the result.v2 `language` provenance key the aggregator slices by.
	Language string
	// Split is the dataset split the row came from (CCE ships a single eval
	// split; carried for provenance + forward compatibility).
	Split string
}

// validatePathSegment rejects names that could escape a join root once they
// become a path segment — a clone of aiderpolyglot.validatePathSegment /
// bench/runtime/validate.go's predicate (T-86-03-03), kept in this leaf package
// so it carries no bench/runtime import. It rejects "", anything filepath.Clean
// rewrites ("..", "a//b"), any embedded separator, and a leading dot. MUST run
// BEFORE any filepath.Join.
func validatePathSegment(name, kind string) error {
	if name == "" {
		return fmt.Errorf("crosscodeeval: %s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("crosscodeeval: %s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

// validLanguage reports whether lang is one of the four supported CCE languages.
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

// decodeParquet decodes a CrossCodeEval parquet payload (held entirely in memory
// — the caller size-caps the download via io.LimitReader, T-86-03-04) into
// []Task for the given language. It reads ONLY the columns it needs (prompt,
// groundtruth, optionally split) and validates each row decode-then-validate:
// a missing required column or an empty prompt/groundtruth is a typed error,
// never a panic, so a completion coordinator can null-on-failure rather than
// crash. The parquet decode is confined to this package to preserve the leaf
// invariant.
func decodeParquet(ctx context.Context, raw []byte, language string) ([]Task, error) {
	if !validLanguage(language) {
		return nil, fmt.Errorf("crosscodeeval: unsupported language %q", language)
	}
	rdr := bytes.NewReader(raw)
	tbl, err := pqarrow.ReadTable(ctx, rdr, nil, pqarrow.ArrowReadProperties{}, memory.DefaultAllocator)
	if err != nil {
		return nil, fmt.Errorf("crosscodeeval: decode parquet (%s): %w", language, err)
	}
	defer tbl.Release()

	prompts, err := stringColumn(tbl, "prompt")
	if err != nil {
		return nil, err
	}
	truths, err := stringColumn(tbl, "groundtruth")
	if err != nil {
		return nil, err
	}
	// Split is optional: if absent, default to "test" (CCE ships a single eval
	// split). When present it is read per-row.
	splits, _ := stringColumn(tbl, "split")
	// Language is optional in the parquet: a real CCE per-language parquet is
	// already single-language, so when the column is absent every row is tagged
	// with the requested `language` arg. When present (e.g. the multi-language
	// hermetic fixture), the per-row value is authoritative and rows for other
	// languages are skipped — so Load(<lang>) over a mixed parquet still yields
	// only that language's tasks.
	langs, _ := stringColumn(tbl, "language")

	if len(prompts) != len(truths) {
		return nil, fmt.Errorf("crosscodeeval: column length mismatch prompt=%d groundtruth=%d", len(prompts), len(truths))
	}

	tasks := make([]Task, 0, len(prompts))
	for i := range prompts {
		rowLang := language
		if i < len(langs) && langs[i] != "" {
			rowLang = langs[i]
			if rowLang != language {
				// Skip rows for a different language (mixed-language fixture).
				continue
			}
		}
		if prompts[i] == "" || truths[i] == "" {
			return nil, fmt.Errorf("crosscodeeval: %s row %d has empty prompt or groundtruth", language, i)
		}
		split := "test"
		if i < len(splits) && splits[i] != "" {
			split = splits[i]
		}
		tasks = append(tasks, Task{
			Prompt:      prompts[i],
			GroundTruth: truths[i],
			Language:    rowLang,
			Split:       split,
		})
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("crosscodeeval: %s parquet decoded zero tasks", language)
	}
	return tasks, nil
}

// stringColumn extracts a UTF-8 string column from an arrow.Table by name,
// flattening every chunk in column order. A column whose name is absent returns
// a typed error (so a required column missing is caught explicitly, not by a
// silent nil); a non-string column is also a typed error. It never panics on a
// malformed table.
func stringColumn(tbl arrow.Table, name string) ([]string, error) {
	idx := -1
	schema := tbl.Schema()
	for i, f := range schema.Fields() {
		if f.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("crosscodeeval: required column %q absent from parquet schema", name)
	}
	col := tbl.Column(idx)
	out := make([]string, 0, tbl.NumRows())
	for _, chunk := range col.Data().Chunks() {
		sa, ok := chunk.(*array.String)
		if !ok {
			return nil, fmt.Errorf("crosscodeeval: column %q is not a UTF-8 string column (got %T)", name, chunk)
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

// Load fetches (network-gated) and decodes the CrossCodeEval parquet for the
// given language at the pinned rev, returning the per-language []Task. The
// language is validatePathSegment-checked AND allow-list-checked before any
// filepath.Join or URL build (T-86-03-03). The fetch is cached under
// $HELIX_CACHE_DIR/crosscodeeval/<rev>/ (fetch.go); offline it returns an error
// the caller treats as "skip live, rely on the hermetic fixture proof". Load is
// the live entry point; LoadParquetBytes is the hermetic decode the fixture test
// drives directly over committed bytes.
func Load(ctx context.Context, language string) ([]Task, error) {
	if err := validatePathSegment(language, "language"); err != nil {
		return nil, err
	}
	if !validLanguage(language) {
		return nil, fmt.Errorf("crosscodeeval: unsupported language %q", language)
	}
	raw, err := Fetch(ctx, PinnedRev, language)
	if err != nil {
		return nil, err
	}
	return decodeParquet(ctx, raw, language)
}

// LoadParquetBytes decodes an in-memory CCE parquet payload for the given
// language WITHOUT any network — the hermetic decode path the fixture test
// drives over the committed testdata/sample.parquet. It is the sole-proof
// surface: it exercises the exact arrow-go decode + row→Task mapping Load uses,
// minus the fetch.
func LoadParquetBytes(ctx context.Context, raw []byte, language string) ([]Task, error) {
	if err := validatePathSegment(language, "language"); err != nil {
		return nil, err
	}
	return decodeParquet(ctx, raw, language)
}
