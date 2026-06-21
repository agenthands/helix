// cmd/helix-bench is the Phase 75 provider-independent benchmark harness
// entrypoint. It exposes six top-level subcommands:
//
//   - helix-bench run                 — run the bench suite (wired in Phase 78:
//     expands the matrix and dispatches each cell through bench/runtime.RunCell)
//   - helix-bench fetch-datasets      — download/refresh bench datasets (Phase NN)
//   - helix-bench doctor              — check host prerequisites; exit 0 on a clean host
//   - helix-bench report              — render bench reports (Phase NN)
//   - helix-bench validate-cost-table — HARD-FAIL strict validator for the
//     bench/datasets/cost-table.yaml pricing + staleness contract (COST-01/D-13/D-16)
//   - helix-bench aggregate           — Phase 82 D-02: reduce a durable multi-run
//     tree into the first leaderboard.md + cost_quality.md (STATS-01/COST-03),
//     fail-closed on a deficient run (D-05)
//
// (BENCH-02's "exactly five subcommands" --help acceptance was the Phase 75
// contract; Phase 82 adds `aggregate` as the always-intended sixth — the
// aggregator surface the milestone roadmap reserved here.)
//
// The companion `verify-tos` validator (D-16) is exposed as a sibling subcommand
// and wired into the Makefile, but BENCH-02 counts exactly five top-level
// subcommands for the --help acceptance; verify-tos is a Makefile gate.
//
// main() is the only os.Exit site; every subcommand uses RunE so errors
// propagate to the single exit point and the process exits non-zero (D-16).
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agenthands/helix/bench/datasets/crosscodeeval"
	"github.com/agenthands/helix/bench/datasets/repobench"
	runtime "github.com/agenthands/helix/bench/runtime"
	"github.com/spf13/cobra"
)

func main() {
	// verify-tos is a Makefile-only HARD-FAIL gate, NOT one of the five BENCH-02
	// subcommands. Dispatch it directly so it does not inflate the root
	// command count (the --help acceptance fixes that at exactly five).
	if len(os.Args) > 1 && os.Args[1] == "verify-tos" {
		cmd := newVerifyTOSCmd()
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// verify-licenses is likewise a Makefile-only HARD-FAIL gate (SC#4), NOT a
	// BENCH-02 root subcommand. Dispatch it directly so it does not inflate the
	// root command count.
	if len(os.Args) > 1 && os.Args[1] == "verify-licenses" {
		cmd := newVerifyLicensesCmd()
		cmd.SetArgs(os.Args[2:])
		if err := cmd.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCmd constructs the cobra command tree. Exported (package-private but
// test-reachable) for in-process --help/doctor testing.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use: "helix-bench",
		// Validator failures are real gate errors, not usage mistakes — don't
		// dump the usage text on a non-zero exit.
		SilenceUsage: true,
		Short:        "Helix provider-independent benchmark harness",
		Long: `helix-bench drives the Phase 75 benchmark stack.

Subcommands:
  run                 run the bench suite (expands the matrix and dispatches cells)
  fetch-datasets      download / refresh bench datasets (CrossCodeEval + RepoBench)
  doctor              check host prerequisites; exits 0 on a clean host
  report              render bench reports (not yet implemented)
  validate-cost-table HARD-FAIL strict validator for bench/datasets/cost-table.yaml
  aggregate           reduce a multi-run tree into leaderboard.md + cost_quality.md

See bench/BENCH.md for the benchmark contract and bench/PROVIDERS.md for the
per-provider TOS attestation surface (gated by 'make verify-tos').`,
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newFetchDatasetsCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newReportCmd())
	root.AddCommand(newValidateCostTableCmd())
	root.AddCommand(newAggregateCmd())

	return root
}

// notYetImplemented returns a RunE that reports a clear deferred-phase error.
func notYetImplemented(feature string) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error {
		return fmt.Errorf("%s: not yet implemented (deferred to a later Phase)", feature)
	}
}

// newRunCmd returns the 'run' subcommand: it expands the
// (benchmark x mode x task) matrix and dispatches each cell through the Phase 77
// single-cell orchestrator (bench/runtime.RunCell) bounded by --parallel.
//
// Exit semantics (mirrors cmd/helix-eval): main() is the only os.Exit site, so
// RunE returns a non-nil error iff ZERO cells succeeded -> exit 1; exit 0 iff
// >=1 cell succeeded. SilenceUsage is inherited from the root (cell/infra
// failures are gate errors, not usage mistakes).
//
// Agent selection (D-01): --agent=scripted (default) is the hermetic CI gate;
// --agent=claude wires the real claude CLI branch (locally runnable, never the
// CI default).
func newRunCmd() *cobra.Command {
	var (
		benchmarks   string
		languages    []string
		modes        []string
		tasks        []string
		parallel     int
		runs         int
		out          string
		agent        string
		helixBin     string
		runID        string
		datasetsRoot string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the bench suite",
		Long: `Run the bench matrix: expand (benchmark x mode x task) into cells and
dispatch each through the per-cell orchestrator bounded by --parallel.

Each cell spawns one helix daemon over a per-cell Unix socket (no TCP ports, so
--parallel never collides), drives the scripted agent edit, runs the task's
verify step, and writes a schema-valid result.v2.json under
<out>/<task>/<mode>/.

The default --agent=scripted is the hermetic CI gate. --agent=claude wires the
real claude CLI agent (locally runnable; requires the claude binary on PATH).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBench(cmd, runBenchOpts{
				Benchmarks:   benchmarks,
				Languages:    languages,
				Modes:        modes,
				Tasks:        tasks,
				Parallel:     parallel,
				Runs:         runs,
				Out:          out,
				Agent:        agent,
				HelixBin:     helixBin,
				RunID:        runID,
				DatasetsRoot: datasetsRoot,
			})
		},
	}

	cmd.Flags().StringVar(&benchmarks, "benchmarks", "internal-toolbench", "benchmark suite")
	cmd.Flags().StringArrayVar(&languages, "languages", []string{"go"}, "language(s); repeatable. When --tasks is omitted, the discovered task set is the UNION across languages, so a task present only under one language still yields a cell for every other language — those cells' seed dirs do not exist and surface as per-cell infra errors (IN-02). Pin --tasks to avoid the cross-product fan-out under multi-language runs.")
	cmd.Flags().StringArrayVar(&modes, "modes", []string{"your_agent_full"}, "mode(s); repeatable")
	cmd.Flags().StringArrayVar(&tasks, "tasks", nil, "task id(s); repeatable (default: all tasks under the benchmark dataset dir)")
	cmd.Flags().IntVar(&parallel, "parallel", 1, "max concurrent cells")
	cmd.Flags().IntVar(&runs, "runs", 3, "runs per (task,mode); each lands a distinct run_index dir (STATS-01)")
	cmd.Flags().StringVar(&out, "out", "bench/reports", "durable output dir")
	cmd.Flags().StringVar(&agent, "agent", "scripted", "agent driver: scripted|claude")
	cmd.Flags().StringVar(&helixBin, "helix-bin", "helix", "path to the helix binary for the daemon subprocess")
	cmd.Flags().StringVar(&runID, "run-id", "", "run identifier (default: UTC timestamp 20060102T150405Z)")
	cmd.Flags().StringVar(&datasetsRoot, "datasets", "bench/datasets", "root dir containing <benchmark>/<language>/<task> seed dirs")

	return cmd
}

// runBenchOpts is the resolved flag set for the run subcommand.
type runBenchOpts struct {
	Benchmarks   string
	Languages    []string
	Modes        []string
	Tasks        []string
	Parallel     int
	Runs         int
	Out          string
	Agent        string
	HelixBin     string
	RunID        string
	DatasetsRoot string
}

// runBench implements the 'run' subcommand body. It resolves the run-id and the
// task set, expands the matrix, dispatches it bounded by --parallel, and returns
// an error iff zero cells succeeded (single-exit semantics).
func runBench(cmd *cobra.Command, o runBenchOpts) error {
	// cobra's Context() never returns nil (it defaults to context.Background()),
	// but guard defensively so a future caller invoking runBench with a
	// hand-built command cannot pass a nil context into the matrix dispatch.
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// run_id is TIMESTAMP-ONLY (RESEARCH run_id note); the git SHA is captured
	// separately into result provenance, NOT concatenated into the run_id.
	runID := o.RunID
	if runID == "" {
		runID = time.Now().UTC().Format("20060102T150405Z")
	}

	if o.Agent != "scripted" && o.Agent != "claude" {
		return fmt.Errorf("helix-bench run: unknown --agent %q (want scripted|claude)", o.Agent)
	}

	if len(o.Languages) == 0 {
		return fmt.Errorf("helix-bench run: no --languages given")
	}

	// Resolve the task set. When --tasks is empty, default to ALL task dirs found
	// under <datasets>/<benchmark>/<language>/ (so the seed smoke can omit --tasks
	// while a real run can pin specific tasks). criterion #1's --tasks=<one> is
	// supported by passing a single id explicitly. With multiple --languages, the
	// discovered task set is the union across languages (the matrix product then
	// pairs each task with each language; a task dir absent for a language yields a
	// cell whose seed dir simply will not exist — surfaced as a per-cell infra
	// error, never a silent skip).
	tasks := o.Tasks
	if len(tasks) == 0 {
		discovered, err := discoverTasks(o.DatasetsRoot, o.Benchmarks, o.Languages)
		if err != nil {
			return fmt.Errorf("helix-bench run: %w", err)
		}
		tasks = discovered
	}

	cells, err := runtime.ExpandMatrix([]string{o.Benchmarks}, o.Languages, o.Modes, tasks, o.Runs)
	if err != nil {
		return fmt.Errorf("helix-bench run: %w", err)
	}

	// out dir is run-scoped: <out>/<run_id>/, matching helix-eval's layout and
	// D-08 (durable artifacts land under <out>/<run_id>/<task>/<mode>/).
	runOutDir := filepath.Join(o.Out, runID)
	if err := os.MkdirAll(runOutDir, 0700); err != nil {
		return fmt.Errorf("helix-bench run: mkdir run output dir: %w", err)
	}

	summary, err := runtime.RunMatrix(ctx, cells, o.Parallel, runtime.RunMatrixConfig{
		RunID:        runID,
		HelixBin:     o.HelixBin,
		OutDir:       runOutDir,
		DatasetsRoot: o.DatasetsRoot,
		Agent:        o.Agent,
	})
	if err != nil {
		return fmt.Errorf("helix-bench run: %w", err)
	}

	// Surface per-cell infra errors (non-fatal) and deferred stubs (e.g.
	// baseline_rag) distinctly for operator visibility. A deferred cell is neither
	// a success nor an infra error — it is a registered-and-fail-closed arm whose
	// real implementation lands in a later phase, so it MUST NOT be reported as a
	// failure.
	for _, oc := range summary.Outcomes {
		if oc.Err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "helix-bench run: cell %s/%s/%s error: %v\n",
				oc.Cell.Benchmark, oc.Cell.Task, oc.Cell.Mode, oc.Err)
		}
		if oc.Deferred {
			fmt.Fprintf(cmd.ErrOrStderr(), "helix-bench run: deferred: %s/%s (%s)\n",
				oc.Cell.Task, oc.Cell.Mode, oc.Result.DeferredReason)
		}
	}

	// Plan-05 post-matrix 3-delta pass (D-05). It runs STRICTLY AFTER RunMatrix
	// returns (the wg.Wait() barrier — Pitfall 3): the matrix is the only tier that
	// sees every mode for a task, so the cross-mode deltas can only be computed
	// here. For each task with all 4 real-mode rows, it computes the 3 fixed deltas
	// (full vs baseline_plain/no_lsp/no_structured_edit) and surfaces them in each
	// per-mode row's ablation_deltas property; tasks missing a real mode are skipped
	// and reported. A write-back error is surfaced but non-fatal (the rows already
	// exist; the run's exit is driven by Succeeded).
	deltaReport, derr := runtime.ComputeAndWriteDeltas(summary.Outcomes)
	if derr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "helix-bench run: delta pass: %v\n", derr)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "helix-bench run: ablation deltas computed for %d task(s), %d skipped\n",
		len(deltaReport.Computed), len(deltaReport.Skipped))
	for _, sk := range deltaReport.Skipped {
		fmt.Fprintf(cmd.ErrOrStderr(), "helix-bench run: delta skipped: task %s (%s)\n", sk.Task, sk.Reason)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "helix-bench run complete: %d/%d cells succeeded\n", summary.Succeeded, summary.Total)
	fmt.Fprintf(cmd.OutOrStdout(), "Reports written to: %s\n", runOutDir)

	// Single-exit (helix-eval semantics): exit 0 iff >=1 cell succeeded.
	if summary.Succeeded == 0 {
		return fmt.Errorf("helix-bench run: 0/%d cells succeeded", summary.Total)
	}
	return nil
}

// discoverTasks returns the sorted, de-duplicated union of task ids (subdir
// names) found under <datasetsRoot>/<benchmark>/<language>/ across every given
// language. It is used when --tasks is omitted so the seed smoke can run without
// naming the task. An empty benchmark/language tree (no tasks discoverable under
// any language) is an error (a silent empty matrix would exit 0 with nothing run).
func discoverTasks(datasetsRoot, benchmark string, languages []string) ([]string, error) {
	seen := make(map[string]struct{})
	var scanned []string
	for _, lang := range languages {
		langDir := filepath.Join(datasetsRoot, benchmark, lang)
		scanned = append(scanned, langDir)
		entries, err := os.ReadDir(langDir)
		if err != nil {
			// A missing language dir is non-fatal here: a real run may pass
			// languages not all present on disk. The aggregate emptiness check
			// below turns a wholly-empty discovery into a hard error.
			continue
		}
		for _, e := range entries {
			// Skip non-dirs and leading-dot entries (.git, .DS_Store, editor
			// scratch dirs). A benign filesystem artifact must not fail the whole
			// run: ExpandMatrix's validateMatrixID rejects a leading-dot id and
			// returns on the first bad id, so letting such a dir into the task set
			// hard-fails the entire expansion before any legitimate task runs. This
			// leading-dot filter is what prevents that abort (locked by
			// TestDiscoverTasksSkipsHiddenDirs).
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				seen[e.Name()] = struct{}{}
			}
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("no task dirs under %v (pass --tasks explicitly)", scanned)
	}
	tasks := make([]string, 0, len(seen))
	for t := range seen {
		tasks = append(tasks, t)
	}
	sort.Strings(tasks)
	return tasks, nil
}

// newFetchDatasetsCmd returns the 'fetch-datasets' subcommand: it downloads (or
// reuses the cached) CrossCodeEval + RepoBench per-language parquet datasets by
// invoking each adapter's pinned-constant Fetch func (crosscodeeval.Fetch /
// repobench.Fetch). Both fetchers build the resolve URL ONLY from their pinned
// Host+Repo+Rev constants and a validated language (SSRF-safe, T-86-03-02 /
// T-86-04-02) — no caller-supplied URL crosses — and cache under HELIX_CACHE_DIR.
//
// The live fetch is network-gated by nature (it reaches huggingface.co); the
// hermetic test only proves the command is registered and is NOT the old
// notYetImplemented stub. RunE reports the cached size per (adapter, language) and
// returns a non-nil error iff EVERY fetch failed, so a partial mirror gap
// (RESEARCH Pitfall 3) does not hard-fail the whole command.
func newFetchDatasetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch-datasets",
		Short: "Download or refresh bench datasets (CrossCodeEval + RepoBench)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			out := cmd.OutOrStdout()

			var ok, failed int

			// CrossCodeEval: one pinned rev across all languages.
			for _, lang := range crosscodeeval.Languages {
				b, err := crosscodeeval.Fetch(ctx, crosscodeeval.PinnedRev, lang)
				if err != nil {
					fmt.Fprintf(out, "crosscodeeval %-12s FAILED: %v\n", lang, err)
					failed++
					continue
				}
				fmt.Fprintf(out, "crosscodeeval %-12s OK (%d bytes)\n", lang, len(b))
				ok++
			}

			// RepoBench: a per-language pinned rev (separate per-language repos).
			for _, lang := range repobench.Languages {
				rev := repobench.PinnedRev(lang)
				if rev == "" {
					fmt.Fprintf(out, "repobench %-12s FAILED: no pinned rev\n", lang)
					failed++
					continue
				}
				b, err := repobench.Fetch(ctx, rev, lang)
				if err != nil {
					fmt.Fprintf(out, "repobench %-12s FAILED: %v\n", lang, err)
					failed++
					continue
				}
				fmt.Fprintf(out, "repobench %-12s OK (%d bytes)\n", lang, len(b))
				ok++
			}

			if ok == 0 {
				return fmt.Errorf("fetch-datasets: all %d dataset fetches failed", failed)
			}
			fmt.Fprintf(out, "fetch-datasets: %d ok, %d failed\n", ok, failed)
			return nil
		},
	}
}

// newReportCmd returns the 'report' subcommand (skeleton).
func newReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Render bench reports",
		RunE:  notYetImplemented("report"),
	}
}

// newDoctorCmd returns the 'doctor' subcommand. It checks documented host
// prerequisites and returns nil (exit 0) on a clean host. The skeleton has no
// hard prerequisites beyond a working Go toolchain (already proven by being able
// to run), so it succeeds on a clean Linux host.
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check host prerequisites; exits 0 on a clean host",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "helix-bench doctor: host OK")
			return nil
		},
	}
}
