// Package swebench is a LEAF Go-transform package wiring the upstream SWE-bench
// Verified evaluator (`python -m swebench.harness.run_evaluation`) into the bench
// stack via a subprocess-shellout (ADAPTER-SWE-01). It owns the fixed-argv
// builder + subprocess seam for the harness, mirroring the
// bench/container/engine.go + run.go os/exec discipline EXACTLY: a FIXED argv
// slice, every value validated BEFORE crossing os/exec, a strict env allowlist
// (never the inherited env), process-group ownership so a context cancel
// group-kills the harness + its per-instance Docker descendants, and NEVER a
// shell. The harness owns its OWN Docker per-instance (Open Q4) — this wrapper
// shells the Python CLI directly, NOT through container.Engine.Run.
package swebench

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// allowedDatasetNames is the FIXED allowlist of the two known HF dataset names
// the harness may be pointed at: the canonical SWE-bench Verified suite and the
// UTBoost-augmented Verified suite (VERIFIED-02). A dataset name not in this set
// is refused BEFORE any argv is produced (T-87-01), so a crafted --dataset_name
// value can never reach the subprocess. The UTBoost name is the value confirmed
// against upstream at the Task 4 human-verify checkpoint.
var allowedDatasetNames = map[string]struct{}{
	"princeton-nlp/SWE-bench_Verified":     {},
	"Bertsekas/SWE-Bench_Verified_UTBoost": {},
}

// allowedCacheLevels is the fixed set of swebench --cache_level values. Anything
// else fails closed before argv (defense in depth — no user value reaches exec
// as a flag value unvalidated).
var allowedCacheLevels = map[string]struct{}{
	"none":     {},
	"base":     {},
	"env":      {},
	"instance": {},
}

// errHarnessUnavailable is the sentinel returned by Detect when python (or the
// swebench package probe) is not available. Live harness tests t.Skip on this —
// it is NEVER treated as a failure (the environment simply lacks Docker+swebench).
var errHarnessUnavailable = errors.New("bench/evaluators/swebench: python with the swebench harness is not available")

// errBadHarnessArg is returned by RunArgs when any input fails its total
// validator. It fail-closes the same argv flag-smuggling / path-escape vectors
// bench/container/run.go closes: a flag-shaped value, a relative or '..'-bearing
// predictions path, a malformed run_id or instance id, an unknown dataset name,
// or an out-of-range worker count can never reach the harness argv (T-87-01).
var errBadHarnessArg = errors.New("bench/evaluators/swebench: invalid harness argument")

// errHarnessEnv is returned by Run when the subprocess environment is
// misconfigured in a way that would otherwise surface as an opaque harness crash
// (WR-04): an empty PATH (the harness cannot locate `docker`), or an invalid
// WorkDir (WR-03). Failing closed here turns a confusing run-time crash into a
// clear, actionable config refusal.
var errHarnessEnv = errors.New("bench/evaluators/swebench: invalid harness environment")

// HarnessRun is the validated config for one `python -m swebench.harness.
// run_evaluation` invocation. Every field is total-validated by RunArgs before a
// single argv element is produced.
type HarnessRun struct {
	// DatasetName must be one of allowedDatasetNames (the 2-name allowlist).
	DatasetName string
	// PredictionsPath must be a clean, absolute path with no ".." and no ':'
	// (isValidPredictionsPath) — a relative or '-'-leading value is refused.
	PredictionsPath string
	// RunID must match [A-Za-z0-9_-]+ AND not start with '-' (flag-shape refusal).
	RunID string
	// InstanceIDs each must match the SWE-bench <owner>__<repo>-<num> shape and
	// not start with '-'.
	InstanceIDs []string
	// MaxWorkers must be >= 1.
	MaxWorkers int
	// CacheLevel must be one of allowedCacheLevels.
	CacheLevel string
	// WorkDir is the controlled working directory the harness runs in (WR-03).
	// The swebench harness writes its logs/run_evaluation/<run_id>/... output tree
	// relative to CWD, so rooting it under a validated dir (clean, absolute, same
	// isValidPredictionsPath discipline) makes artifact location deterministic
	// rather than a function of the parent process's CWD. When empty, Run defaults
	// it to <HELIX_CACHE_DIR>/swebench-runs (created if absent) so output never
	// lands under an uncontrolled parent CWD.
	WorkDir string
}

// isValidRunID reports whether s is a non-empty [A-Za-z0-9_-]+ that does NOT
// start with '-'. The leading-'-' refusal closes the flag-smuggling vector (a
// run id of "-rf" would otherwise be parsed by the harness as a flag); the
// charset is explicit and total (no regexp reaching exec, mirroring run.go's
// isValidRepo discipline).
func isValidRunID(s string) bool {
	if s == "" || s[0] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}

// isValidInstanceID reports whether s has the SWE-bench instance shape
// "<owner>__<repo>-<num>" (e.g. "sympy__sympy-20590"): it must contain "__",
// end with "-<digits>", carry a non-empty owner and repo, use only the
// conservative [A-Za-z0-9_.-] alphabet, and NOT start with '-' (flag-smuggling
// refusal). Explicit + total, no regexp reaches exec.
func isValidInstanceID(s string) bool {
	if s == "" || s[0] == '-' {
		return false
	}
	// Must contain the owner/repo "__" separator with a non-empty owner.
	sep := strings.Index(s, "__")
	if sep <= 0 {
		return false
	}
	// Must end with "-<digits>" (the issue number), with a non-empty repo before it.
	dash := strings.LastIndex(s, "-")
	if dash <= sep+2 || dash == len(s)-1 {
		return false
	}
	num := s[dash+1:]
	for i := 0; i < len(num); i++ {
		if num[i] < '0' || num[i] > '9' {
			return false
		}
	}
	// Conservative alphabet over the whole string.
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-' || c == '.':
		default:
			return false
		}
	}
	return true
}

// isValidPredictionsPath reports whether p is a clean, absolute path safe to
// place as the --predictions_path argv value. Mirrors bench/container/run.go's
// isValidMountPath total discipline: non-empty, no ':' (separator-smuggling),
// absolute (a relative or '-'-leading value — never absolute — is rejected,
// closing the flag-smuggling vector), and already-clean (rejects "..", trailing
// slash, redundant separators post-canonicalization). Checked against the SLASH
// form so a windows cross-compile behaves identically.
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

// RunArgs builds the FIXED argv for `python -m swebench.harness.run_evaluation`,
// the hermetically-assertable seam (mirrors bench/container/run.go runArgs). It
// fail-closes — returning errBadHarnessArg and a nil argv — on the FIRST bad
// input, BEFORE producing any argv, so a crafted dataset name, predictions path,
// run id, instance id, cache level, or worker count can NEVER cross os/exec
// (T-87-01). The argv is:
//
//	["-m", "swebench.harness.run_evaluation",
//	 "--dataset_name", <name>, "--predictions_path", <path>,
//	 "--run_id", <id>, "--max_workers", <n>, "--cache_level", <lvl>,
//	 "--instance_ids", <iid>...]
//
// Every value is validated by a total, regexp-free validator before append.
func RunArgs(r HarnessRun) ([]string, error) {
	if _, ok := allowedDatasetNames[r.DatasetName]; !ok {
		return nil, fmt.Errorf("%w: dataset_name %q not in allowlist", errBadHarnessArg, r.DatasetName)
	}
	if !isValidPredictionsPath(r.PredictionsPath) {
		return nil, fmt.Errorf("%w: predictions_path %q must be a clean absolute path without '..' or ':'", errBadHarnessArg, r.PredictionsPath)
	}
	if !isValidRunID(r.RunID) {
		return nil, fmt.Errorf("%w: run_id %q must match [A-Za-z0-9_-]+ and not start with '-'", errBadHarnessArg, r.RunID)
	}
	if r.MaxWorkers < 1 {
		return nil, fmt.Errorf("%w: max_workers %d must be >= 1", errBadHarnessArg, r.MaxWorkers)
	}
	if _, ok := allowedCacheLevels[r.CacheLevel]; !ok {
		return nil, fmt.Errorf("%w: cache_level %q not in {none,base,env,instance}", errBadHarnessArg, r.CacheLevel)
	}
	if len(r.InstanceIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one instance id is required", errBadHarnessArg)
	}
	for _, iid := range r.InstanceIDs {
		if !isValidInstanceID(iid) {
			return nil, fmt.Errorf("%w: instance id %q is not a valid <owner>__<repo>-<num>", errBadHarnessArg, iid)
		}
	}

	args := make([]string, 0, 12+len(r.InstanceIDs))
	args = append(args,
		"-m", "swebench.harness.run_evaluation",
		"--dataset_name", r.DatasetName,
		"--predictions_path", r.PredictionsPath,
		"--run_id", r.RunID,
		"--max_workers", strconv.Itoa(r.MaxWorkers),
		"--cache_level", r.CacheLevel,
		"--instance_ids",
	)
	args = append(args, r.InstanceIDs...)
	return args, nil
}

// Harness is a resolved python interpreter able to run the swebench harness. All
// invocation crosses os/exec with FIXED argv (RunArgs) and a strict env allowlist
// — never a shell, never the full inherited env (mirrors bench/container/engine.go).
type Harness struct {
	bin string
	// runShim, when non-nil, intercepts Run's os/exec invocation with the exact
	// argv RunArgs produced. It exists ONLY so the hermetic argv-equivalence test
	// can prove the argv crossing the boundary without a live python+swebench+docker.
	// Production callers leave it nil → Run crosses os/exec for real.
	runShim func(args []string) error
}

// pythonCandidates is the fixed probe order for the interpreter on PATH.
var pythonCandidates = []string{"python3", "python"}

// Detect resolves the first python interpreter on PATH (python3 then python).
// Returns errHarnessUnavailable when none is found. The presence of the swebench
// package itself is verified at run time (the harness emits a clear import error
// the wrapper surfaces); Detect's PATH probe is the cheap hermetic gate live
// tests t.Skip on.
func Detect() (*Harness, error) {
	for _, name := range pythonCandidates {
		if p, err := exec.LookPath(name); err == nil {
			return &Harness{bin: p}, nil
		}
	}
	return nil, errHarnessUnavailable
}

// Run executes one harness evaluation via the resolved python, mirroring
// bench/container/engine.go Run EXACTLY: it builds the fixed argv with RunArgs
// (returning that error verbatim — fail-close before crossing os/exec), then runs
// python with a strict env allowlist (PATH/HOME/HELIX_CACHE_DIR only, never the
// full parent env — T-87-02) and process-group ownership (procGroupAttr) so a
// context cancel can group-kill the harness and its per-instance Docker
// descendants (T-87-05). NO shell, NO docker Go SDK.
func (h *Harness) Run(ctx context.Context, r HarnessRun) error {
	args, err := RunArgs(r)
	if err != nil {
		return err
	}
	// runShim is the hermetic test seam: when set it receives the exact argv that
	// would cross os/exec, proving the boundary argv without a live process.
	if h.runShim != nil {
		return h.runShim(args)
	}
	// Resolve and validate the controlled working dir (WR-03) so the harness
	// output tree is rooted deterministically, never under the parent CWD.
	workDir, err := resolveWorkDir(r.WorkDir)
	if err != nil {
		return err
	}
	// Build the strict env and fail closed on a misconfigured one (WR-04) — an
	// empty PATH would otherwise surface as an opaque "docker not found" crash.
	env, err := allowlistEnv()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, h.bin, args...)
	cmd.SysProcAttr = procGroupAttr()
	// WR-02: override the context-cancel kill to signal the WHOLE process group,
	// not just the leader PID. Setpgid alone places the child in its own group, so
	// the default single-PID kill would orphan the per-instance Docker descendants.
	// On windows this is a no-op (no POSIX process groups).
	setGroupKillCancel(cmd)
	cmd.Env = env
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bench/evaluators/swebench: %s run_evaluation: %w", h.bin, err)
	}
	return nil
}

// resolveWorkDir validates an explicit WorkDir or derives a controlled default
// under HELIX_CACHE_DIR (WR-03). An explicit dir MUST pass the same
// isValidPredictionsPath discipline (clean, absolute, no ':' / '..'); the default
// is <cacheRoot>/swebench-runs, created if absent. The returned dir is guaranteed
// to exist so exec.Cmd.Dir is always a real directory.
func resolveWorkDir(workDir string) (string, error) {
	if workDir == "" {
		workDir = filepath.Join(harnessCacheRoot(), "swebench-runs")
	}
	if !isValidPredictionsPath(workDir) {
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

// envAllowlist is the strict set of keys forwarded to the harness subprocess,
// never the full parent environment (T-87-02). PATH/HOME/HELIX_CACHE_DIR are the
// base set; the DOCKER_* keys (WR-04) are forwarded WHEN SET so a non-default
// Docker daemon (custom socket / TLS) is reachable — the swebench harness shells
// out to `docker`, so dropping these silently breaks a non-default setup. It is
// still an explicit allowlist: an UNLISTED key is never forwarded.
var envAllowlist = []string{
	"PATH", "HOME", "HELIX_CACHE_DIR",
	"DOCKER_HOST", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH",
}

// allowlistEnv builds the strict env passed to the harness subprocess: only the
// envAllowlist keys are forwarded (set keys only), never the full parent
// environment. Mirrors bench/container/engine.go allowlistEnv (T-87-02).
//
// It FAILS CLOSED (WR-04) when PATH resolves empty: the harness shells out to
// `docker`, which it locates via PATH, so a missing PATH would otherwise surface
// as an opaque subprocess crash rather than a clear config refusal.
func allowlistEnv() ([]string, error) {
	if os.Getenv("PATH") == "" {
		return nil, fmt.Errorf("%w: PATH is empty — the harness needs it to locate `docker`", errHarnessEnv)
	}
	env := make([]string, 0, len(envAllowlist))
	for _, k := range envAllowlist {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env, nil
}
