// Package terminalbench is a LEAF Go-transform package wiring the Terminal-Bench
// 2.0 evaluator (`tb run`) into the bench stack via a subprocess-shellout
// (ADAPTER-TERM-01). It owns the fixed-argv builder + subprocess seam for the
// harness, mirroring bench/evaluators/swebench/harness.go EXACTLY: a FIXED argv
// slice, every value validated BEFORE crossing os/exec, a strict env allowlist
// (never the inherited env), process-group ownership so a context cancel
// group-kills the harness + its per-task Docker descendants, and NEVER a shell.
//
// tb owns its OWN per-task Docker isolation via its DockerComposeManager (a fresh
// container per task — the SC#2 container-isolation invariant). This wrapper
// shells the `tb run` CLI directly and does NOT route through
// bench/container.Engine.Run (Pitfall 2 anti-pattern): the adapter imports NO
// bench/container and re-implements no isolation, so cross-task filesystem
// leakage cannot be introduced by this adapter (asserted by an import-level gate
// in ingest_test.go).
//
// O-1 / Pitfall 2 — the runnerKind seam: Terminal-Bench was rewritten as Harbor,
// so the 2.0-native path is `harbor run` while CONTEXT.md locks `tb run`. The
// binary name and the dataset-selection flag form live behind a single
// runnerKind constant so a future swap to `harbor run` is a one-line change. The
// JSON ingestion (results.json: accuracy/n_resolved/n_unresolved, per-trial
// is_resolved) is identical across kinds and is the load-bearing part.
package terminalbench

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runnerKind is the O-1 binary-name seam: it selects which CLI the adapter shells
// (`tb` for the CONTEXT-locked legacy 1.x path, `harbor` for the 2.0-native
// framework). Swapping the default from tb to harbor is a one-line change of
// defaultRunnerKind — the JSON-ingestion-bearing argv structure is unchanged.
type runnerKind int

const (
	// runnerKindTB is the CONTEXT-locked legacy 1.x CLI: `tb run` with the
	// --dataset-name / --dataset-version flag pair.
	runnerKindTB runnerKind = iota
	// runnerKindHarbor is the 2.0-native framework: `harbor run`. Documented as
	// the one-line swap (O-1); the dataset-selection flag form is the only other
	// divergence and is carried in runnerSpec.
	runnerKindHarbor
)

// defaultRunnerKind is the kind Detect/Run use. CONTEXT.md + ADAPTER-TERM-01 lock
// `tb run`, so it is runnerKindTB; flipping it to runnerKindHarbor is the entire
// O-1 swap.
const defaultRunnerKind = runnerKindTB

// runnerSpec is the per-kind divergence the seam carries: the binary name and the
// dataset-selection flag form. Everything else in the argv is identical across
// kinds (the load-bearing JSON ingestion is shape-identical).
type runnerSpec struct {
	// binary is the CLI binary name probed on PATH and exec'd ("tb" / "harbor").
	binary string
	// datasetName is the --dataset-name value the legacy tb CLI takes. Under
	// harbor the dataset selection collapses to a single --dataset flag form; that
	// one-line divergence lives here so RunArgs's structure never changes.
	datasetName string
}

// spec returns the runnerSpec for a kind — the SINGLE place the binary name and
// dataset-flag form live, so the tb→harbor swap is one constant flip.
func (k runnerKind) spec() runnerSpec {
	switch k {
	case runnerKindHarbor:
		return runnerSpec{binary: "harbor", datasetName: "terminal-bench-core"}
	default:
		return runnerSpec{binary: "tb", datasetName: "terminal-bench-core"}
	}
}

// errHarnessUnavailable is the sentinel returned by Detect when neither tb nor
// harbor is on PATH. Live harness tests t.Skip on this — it is NEVER treated as a
// failure (the environment simply lacks Docker + tb/harbor).
var errHarnessUnavailable = errors.New("bench/evaluators/terminalbench: neither tb nor harbor is on PATH")

// errBadHarnessArg is returned by RunArgs when any input fails its total
// validator. It fail-closes the same argv flag-smuggling / path-escape vectors
// bench/evaluators/swebench closes: a flag-shaped agent / task-id /
// dataset-version, or a relative / '..'-bearing / ':'-bearing output-path, can
// never reach the tb argv (T-88-02-01).
var errBadHarnessArg = errors.New("bench/evaluators/terminalbench: invalid harness argument")

// errHarnessEnv is returned by Run when the subprocess environment is
// misconfigured (WR-04: an empty PATH so tb cannot locate `docker`; WR-03: an
// invalid WorkDir). Failing closed turns a confusing run-time crash into a clear,
// actionable config refusal.
var errHarnessEnv = errors.New("bench/evaluators/terminalbench: invalid harness environment")

// HarnessRun is the validated config for one `tb run` invocation. Every field is
// total-validated by RunArgs before a single argv element is produced.
type HarnessRun struct {
	// Agent is the tb --agent value (e.g. "oracle"). Must be non-empty and not
	// start with '-' (flag-shape refusal).
	Agent string
	// DatasetVersion is the tb --dataset-version value. Must be non-empty and not
	// start with '-' (flag-shape refusal).
	DatasetVersion string
	// TaskID is the tb --task-id value. Must be non-empty and not start with '-'.
	TaskID string
	// OutputPath is the tb --output-path value: a clean, absolute path with no
	// ".." and no ':' (isValidOutputPath). A relative or '-'-leading value is
	// refused (the runs/<ts>/results.json tree is written under this dir).
	OutputPath string
	// WorkDir is the controlled working directory the harness runs in (WR-03).
	// When empty, Run defaults it to <HELIX_CACHE_DIR>/terminalbench-runs (created
	// if absent) so tb output never lands under an uncontrolled parent CWD.
	WorkDir string
}

// isValidArg reports whether s is a non-empty value that does NOT start with '-'.
// The leading-'-' refusal closes the flag-smuggling vector (a task-id of "-rf"
// would otherwise be parsed by tb as a flag). Mirrors swebench's isValidRunID
// leading-'-' refusal idiom (no regexp reaches exec).
func isValidArg(s string) bool {
	return s != "" && s[0] != '-'
}

// isValidOutputPath reports whether p is a clean, absolute path safe to place as
// the --output-path argv value. Mirrors swebench's isValidPredictionsPath total
// discipline: non-empty, no ':' (separator-smuggling), absolute (a relative or
// '-'-leading value — never absolute — is rejected, closing the flag-smuggling
// vector), and already-clean (rejects "..", trailing slash, redundant separators
// post-canonicalization). Checked against the SLASH form so a windows
// cross-compile behaves identically.
func isValidOutputPath(p string) bool {
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

// RunArgs builds the FIXED argv for `tb run`, the hermetically-assertable seam
// (mirrors swebench RunArgs). It fail-closes — returning errBadHarnessArg and a
// nil argv — on the FIRST bad input, BEFORE producing any argv, so a crafted
// agent, task-id, dataset-version, or output-path can NEVER cross os/exec
// (T-88-02-01). The argv (for kind=tb) is:
//
//	["run", "--agent", <agent>, "--dataset-name", "terminal-bench-core",
//	 "--dataset-version", <ver>, "--task-id", <id>, "--output-path", <out>]
//
// The binary name and dataset-flag form live behind the runnerKind seam: argv is
// built for the kind k actually RESOLVED by Detect (carried on Harness.kind), NOT
// for the global defaultRunnerKind. This closes the WR-01 mis-wiring where a
// harbor-resolved Harness would otherwise be invoked with tb-shaped argv the
// moment harbor's spec() diverges from tb's. The argv STRUCTURE is identical
// across kinds; only spec() values differ.
func RunArgs(k runnerKind, r HarnessRun) ([]string, error) {
	spec := k.spec()
	if !isValidArg(r.Agent) {
		return nil, fmt.Errorf("%w: agent %q must be non-empty and not start with '-'", errBadHarnessArg, r.Agent)
	}
	if !isValidArg(r.DatasetVersion) {
		return nil, fmt.Errorf("%w: dataset_version %q must be non-empty and not start with '-'", errBadHarnessArg, r.DatasetVersion)
	}
	if !isValidArg(r.TaskID) {
		return nil, fmt.Errorf("%w: task_id %q must be non-empty and not start with '-'", errBadHarnessArg, r.TaskID)
	}
	if !isValidOutputPath(r.OutputPath) {
		return nil, fmt.Errorf("%w: output_path %q must be a clean absolute path without '..' or ':'", errBadHarnessArg, r.OutputPath)
	}

	return []string{
		"run",
		"--agent", r.Agent,
		"--dataset-name", spec.datasetName,
		"--dataset-version", r.DatasetVersion,
		"--task-id", r.TaskID,
		"--output-path", r.OutputPath,
	}, nil
}

// Harness is a resolved tb/harbor binary able to run a Terminal-Bench evaluation.
// All invocation crosses os/exec with FIXED argv (RunArgs) and a strict env
// allowlist — never a shell, never the full inherited env (mirrors swebench).
type Harness struct {
	bin string
	// kind is the runnerKind Detect actually resolved (the binary that won the
	// PATH probe). Run/RunArgs build argv for THIS kind, not the global
	// defaultRunnerKind, so a harbor-resolved Harness gets harbor-shaped argv the
	// moment harbor's spec() diverges from tb's (WR-01).
	kind runnerKind
	// runShim, when non-nil, intercepts Run's os/exec invocation with the exact
	// argv RunArgs produced. It exists ONLY so the hermetic argv-equivalence test
	// can prove the argv crossing the boundary without a live tb+docker.
	// Production callers leave it nil → Run crosses os/exec for real.
	runShim func(args []string) error
}

// Detect resolves the runner binary on PATH, probing tb then harbor (the
// runnerKind seam order). Returns errHarnessUnavailable when neither is found.
// The presence of Docker + the dataset images is verified at run time; Detect's
// PATH probe is the cheap hermetic gate live tests t.Skip on.
func Detect() (*Harness, error) {
	for _, k := range []runnerKind{runnerKindTB, runnerKindHarbor} {
		if p, err := exec.LookPath(k.spec().binary); err == nil {
			return &Harness{bin: p, kind: k}, nil
		}
	}
	return nil, errHarnessUnavailable
}

// Run executes one tb evaluation via the resolved binary, mirroring swebench Run
// EXACTLY: it builds the fixed argv with RunArgs (returning that error verbatim —
// fail-close before crossing os/exec), then runs tb with a strict env allowlist
// (PATH/HOME/HELIX_CACHE_DIR + DOCKER_* when set, never the full parent env —
// T-88-02-02) and process-group ownership (procGroupAttr) so a context cancel can
// group-kill tb and its per-task Docker descendants (T-88-02-06). NO shell, NO
// docker Go SDK, NO bench/container — tb owns its own DockerComposeManager
// per-task isolation (SC#2).
func (h *Harness) Run(ctx context.Context, r HarnessRun) error {
	args, err := RunArgs(h.kind, r)
	if err != nil {
		return err
	}
	// runShim is the hermetic test seam: when set it receives the exact argv that
	// would cross os/exec, proving the boundary argv without a live process.
	if h.runShim != nil {
		return h.runShim(args)
	}
	workDir, err := resolveWorkDir(r.WorkDir)
	if err != nil {
		return err
	}
	env, err := allowlistEnv()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, h.bin, args...)
	cmd.SysProcAttr = procGroupAttr()
	// WR-02: override the context-cancel kill to signal the WHOLE process group,
	// not just the leader PID. Setpgid alone places the child in its own group, so
	// the default single-PID kill would orphan the per-task Docker descendants. On
	// windows this is a no-op (no POSIX process groups).
	setGroupKillCancel(cmd)
	cmd.Env = env
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bench/evaluators/terminalbench: %s run: %w", h.bin, err)
	}
	return nil
}

// resolveWorkDir validates an explicit WorkDir or derives a controlled default
// under HELIX_CACHE_DIR (WR-03). An explicit dir MUST pass the same
// isValidOutputPath discipline; the default is <cacheRoot>/terminalbench-runs,
// created if absent. The returned dir is guaranteed to exist.
func resolveWorkDir(workDir string) (string, error) {
	if workDir == "" {
		workDir = filepath.Join(harnessCacheRoot(), "terminalbench-runs")
	}
	if !isValidOutputPath(workDir) {
		return "", fmt.Errorf("%w: work_dir %q must be a clean absolute path without '..' or ':'", errHarnessEnv, workDir)
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return "", fmt.Errorf("%w: create work_dir %q: %v", errHarnessEnv, workDir, err)
	}
	return workDir, nil
}

// harnessCacheRoot resolves the cache root the default WorkDir is rooted under,
// with the same HELIX_CACHE_DIR → os.UserCacheDir()/helix → ~/.helix/cache
// precedence the dataset fetchers use, so the run output lands beside the caches.
func harnessCacheRoot() string {
	if d := os.Getenv("HELIX_CACHE_DIR"); d != "" {
		return d
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "helix")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".helix", "cache")
}

// envAllowlist is the strict set of keys forwarded to the tb subprocess, never
// the full parent environment (T-88-02-02). PATH/HOME/HELIX_CACHE_DIR are the
// base set; the DOCKER_* keys are forwarded WHEN SET so a non-default Docker
// daemon (custom socket / TLS) is reachable — tb shells out to `docker` via its
// DockerComposeManager. It is still an explicit allowlist: an UNLISTED key is
// never forwarded.
var envAllowlist = []string{
	"PATH", "HOME", "HELIX_CACHE_DIR",
	"DOCKER_HOST", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH",
}

// allowlistEnv builds the strict env passed to the tb subprocess: only the
// envAllowlist keys are forwarded (set keys only), never the full parent
// environment. It FAILS CLOSED (WR-04) when PATH resolves empty: tb shells out to
// `docker`, which it locates via PATH.
func allowlistEnv() ([]string, error) {
	if os.Getenv("PATH") == "" {
		return nil, fmt.Errorf("%w: PATH is empty — tb needs it to locate `docker`", errHarnessEnv)
	}
	env := make([]string, 0, len(envAllowlist))
	for _, k := range envAllowlist {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env, nil
}
