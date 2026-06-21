// Package multiswebench is a LEAF Go-transform package wiring the upstream
// Multi-SWE-bench evaluator (`python -m multi_swe_bench.harness.run_evaluation
// --config <config.json>`) into the bench stack via a subprocess-shellout
// (ADAPTER-MULTI-01). It clones the bench/evaluators/swebench/ template; the one
// structural divergence is that Multi-SWE-bench is CONFIG-FILE-DRIVEN, not
// flag-driven — so this file owns a config.json PRODUCER that validates every
// path field BEFORE marshal, and the harness argv is the trivial fixed
// ["-m","multi_swe_bench.harness.run_evaluation","--config",<path>].
//
// Multi-SWE-bench has NO per-instance `language` JSON field (Pitfall 1): the
// language is the dataset-file's directory (go/, java/, ts/, ...). The cell
// orchestrator stamps it into Cell.Language; Ingest carries it through verbatim.
package multiswebench

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// errBadConfig is returned by WriteConfig when any path field fails its total
// validator. It fail-closes the same path-escape / flag-smuggling vectors the
// SWE-bench argv validator closes (a relative or '..'-bearing path, a ':'-bearing
// path, or a flag-shaped value can never reach the config the harness reads —
// T-88-01-01). Mirrors swebench's errBadHarnessArg.
var errBadConfig = errors.New("bench/evaluators/multiswebench: invalid config field")

// Config is the validated config.json the Multi-SWE-bench harness consumes via
// `--config`. The struct-field declaration order is the JSON key order
// json.MarshalIndent emits, so the producer is byte-stable (no map iteration).
// The 18 field names are the verbatim upstream spellings (88-RESEARCH Pattern 1 /
// A1) — pinned here as the hermetic golden contract Plan 04's human-verify
// confirms against the live README.
//
// CITED: github.com/multi-swe-bench/multi-swe-bench README config.json schema.
type Config struct {
	Mode                  string   `json:"mode"`           // "evaluation" for our use
	Workdir               string   `json:"workdir"`        // path (validated)
	PatchFiles            []string `json:"patch_files"`    // agent patches, JSONL (each path validated)
	DatasetFiles          []string `json:"dataset_files"`  // per-language dataset JSONL (each path validated)
	ForceBuild            bool     `json:"force_build"`
	OutputDir             string   `json:"output_dir"`     // final_report.json lands here (validated)
	Specifics             []string `json:"specifics"`      // [] = all
	Skips                 []string `json:"skips"`
	RepoDir               string   `json:"repo_dir"`       // path (validated)
	NeedClone             bool     `json:"need_clone"`
	GlobalEnv             []string `json:"global_env"`     // ["KEY=VALUE"]
	ClearEnv              bool     `json:"clear_env"`
	StopOnError           bool     `json:"stop_on_error"`
	MaxWorkers            int      `json:"max_workers"`
	MaxWorkersBuildImage  int      `json:"max_workers_build_image"`
	MaxWorkersRunInstance int      `json:"max_workers_run_instance"`
	LogDir                string   `json:"log_dir"`        // path (validated)
	LogLevel              string   `json:"log_level"`      // "INFO"
}

// isValidPredictionsPath reports whether p is a clean, absolute path safe to
// place into the config the harness reads. Copied VERBATIM from
// bench/evaluators/swebench/harness.go:162-177: non-empty, no ':'
// (separator-smuggling), absolute (a relative or '-'-leading value — never
// absolute — is rejected, closing the flag-smuggling vector), and already-clean
// (rejects "..", trailing slash, redundant separators post-canonicalization).
// Checked against the SLASH form so a windows cross-compile behaves identically.
func isValidPredictionsPath(p string) bool {
	if p == "" {
		return false
	}
	if strings.ContainsRune(p, ':') {
		return false
	}
	s := filepath.ToSlash(p)
	if !strings.HasPrefix(s, "/") {
		return false
	}
	if filepath.ToSlash(filepath.Clean(s)) != s {
		return false
	}
	return true
}

// WriteConfig validates every path field of c and, only if ALL pass, marshals c
// to stable-key-order JSON and writes it atomically to path. It fail-closes
// (errBadConfig, NO file written) on the FIRST bad path field — Workdir,
// OutputDir, RepoDir, LogDir, and each PatchFiles[]/DatasetFiles[] entry must be a
// clean absolute path with no '..'/':' and no leading '-' (T-88-01-01). The write
// is a temp+rename (writeConfigAtomic) so a crash mid-write never leaves a torn
// config a harness reads (T-88-01-05). json.MarshalIndent emits the struct-field
// declaration order, so re-running WriteConfig is byte-identical (deterministic).
func WriteConfig(path string, c Config) error {
	// Validate every path field BEFORE marshal — fail-closed on the first bad one.
	for _, f := range []struct {
		name string
		val  string
	}{
		{"workdir", c.Workdir},
		{"output_dir", c.OutputDir},
		{"repo_dir", c.RepoDir},
		{"log_dir", c.LogDir},
	} {
		if !isValidPredictionsPath(f.val) {
			return fmt.Errorf("%w: %s %q must be a clean absolute path without '..' or ':'", errBadConfig, f.name, f.val)
		}
	}
	for i, p := range c.PatchFiles {
		if !isValidPredictionsPath(p) {
			return fmt.Errorf("%w: patch_files[%d] %q must be a clean absolute path without '..' or ':'", errBadConfig, i, p)
		}
	}
	for i, p := range c.DatasetFiles {
		if !isValidPredictionsPath(p) {
			return fmt.Errorf("%w: dataset_files[%d] %q must be a clean absolute path without '..' or ':'", errBadConfig, i, p)
		}
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("bench/evaluators/multiswebench: marshal config: %w", err)
	}
	if err := writeConfigAtomic(path, b); err != nil {
		return err
	}
	return nil
}

// writeConfigAtomic writes body to a temp file in the SAME directory as dst and
// atomically renames it into place. Copied from bench/datasets/swebench-utboost/
// fetch.go:232-256 (writeCacheAtomic), renamed, so this leaf package does NOT
// import bench/runtime (whose writeDurable is unexported). A non-atomic write
// interrupted mid-flight would leave a truncated config the harness then reads as
// a valid (but torn) config; the temp+rename makes a partial write never become a
// readable config (T-88-01-05).
func writeConfigAtomic(dst string, body []byte) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("bench/evaluators/multiswebench: mkdir config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(dst)+"-*")
	if err != nil {
		return fmt.Errorf("bench/evaluators/multiswebench: create temp config: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("bench/evaluators/multiswebench: write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("bench/evaluators/multiswebench: close temp config: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("bench/evaluators/multiswebench: rename temp config into %s: %w", dst, err)
	}
	return nil
}
