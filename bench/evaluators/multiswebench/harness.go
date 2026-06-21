package multiswebench

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// errHarnessUnavailable is the sentinel returned by Detect when python is not on
// PATH. Live harness tests t.Skip on this — it is NEVER a failure (the
// environment simply lacks Docker+multi_swe_bench). Mirrors swebench.
var errHarnessUnavailable = errors.New("bench/evaluators/multiswebench: python with the multi_swe_bench harness is not available")

// errBadHarnessArg is returned by RunArgs when the config path fails its total
// validator. It fail-closes the path-escape / flag-smuggling vectors a crafted
// --config value would otherwise open (a relative or '..'-bearing path, a
// ':'-bearing path, or a flag-shaped value can never reach the harness argv —
// T-88-01-02). Mirrors swebench's errBadHarnessArg.
var errBadHarnessArg = errors.New("bench/evaluators/multiswebench: invalid harness argument")

// errHarnessEnv is returned by Run when the subprocess environment is
// misconfigured (WR-04: an empty PATH so the harness cannot locate `docker`;
// WR-03: an invalid WorkDir). Failing closed turns a confusing run-time crash
// into a clear, actionable config refusal. Mirrors swebench.
var errHarnessEnv = errors.New("bench/evaluators/multiswebench: invalid harness environment")

// RunArgs builds the FIXED argv for `python -m multi_swe_bench.harness.
// run_evaluation --config <path>` — the Multi-SWE divergence from the SWE-bench
// template: the harness is CONFIG-FILE-DRIVEN, not flag-driven (Pitfall 4), so
// the only argv input is the config path. It fail-closes (errBadHarnessArg, nil
// argv) when cfgPath fails isValidPredictionsPath, BEFORE producing any argv, so
// a crafted config path can never cross os/exec (T-88-01-02). The argv is:
//
//	["-m", "multi_swe_bench.harness.run_evaluation", "--config", <cfgPath>]
//
// The config itself is produced + path-validated by WriteConfig (config.go); this
// validator is the defense-in-depth check at the argv boundary.
func RunArgs(cfgPath string) ([]string, error) {
	if !isValidPredictionsPath(cfgPath) {
		return nil, fmt.Errorf("%w: config path %q must be a clean absolute path without '..' or ':'", errBadHarnessArg, cfgPath)
	}
	return []string{
		"-m", "multi_swe_bench.harness.run_evaluation",
		"--config", cfgPath,
	}, nil
}

// Harness is a resolved python interpreter able to run the multi_swe_bench
// harness. All invocation crosses os/exec with FIXED argv (RunArgs) and a strict
// env allowlist — never a shell, never the full inherited env (mirrors
// bench/evaluators/swebench/harness.go).
type Harness struct {
	bin string
	// runShim, when non-nil, intercepts Run's os/exec invocation with the exact
	// argv RunArgs produced. It exists ONLY so the hermetic argv-equivalence test
	// can prove the boundary argv without a live python+multi_swe_bench+docker.
	// Production callers leave it nil → Run crosses os/exec for real.
	runShim func(args []string) error
}

// pythonCandidates is the fixed probe order for the interpreter on PATH.
var pythonCandidates = []string{"python3", "python"}

// Detect resolves the first python interpreter on PATH (python3 then python).
// Returns errHarnessUnavailable when none is found. The presence of the
// multi_swe_bench package itself is verified at run time (the harness emits a
// clear import error the wrapper surfaces); Detect's PATH probe is the cheap
// hermetic gate live tests t.Skip on.
func Detect() (*Harness, error) {
	for _, name := range pythonCandidates {
		if p, err := exec.LookPath(name); err == nil {
			return &Harness{bin: p}, nil
		}
	}
	return nil, errHarnessUnavailable
}

// Run executes one harness evaluation via the resolved python, mirroring
// bench/evaluators/swebench/harness.go Run EXACTLY: it builds the fixed argv with
// RunArgs (returning that error verbatim — fail-close before crossing os/exec),
// then runs python with a strict env allowlist (PATH/HOME/HELIX_CACHE_DIR +
// DOCKER_* when set, never the full parent env — T-88-01-03) and process-group
// ownership (procGroupAttr) so a context cancel can group-kill the harness and
// its per-instance Docker descendants. NO shell, NO docker Go SDK.
func (h *Harness) Run(ctx context.Context, cfgPath string) error {
	args, err := RunArgs(cfgPath)
	if err != nil {
		return err
	}
	if h.runShim != nil {
		return h.runShim(args)
	}
	workDir, err := resolveWorkDir("")
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
	// the default single-PID kill would orphan the Docker descendants. On windows
	// this is a no-op (no POSIX process groups).
	setGroupKillCancel(cmd)
	cmd.Env = env
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bench/evaluators/multiswebench: %s run_evaluation: %w", h.bin, err)
	}
	return nil
}

// resolveWorkDir validates an explicit WorkDir or derives a controlled default
// under HELIX_CACHE_DIR (WR-03). An explicit dir MUST pass the same
// isValidPredictionsPath discipline; the default is <cacheRoot>/multiswebench-runs,
// created if absent. The returned dir is guaranteed to exist.
func resolveWorkDir(workDir string) (string, error) {
	if workDir == "" {
		workDir = filepath.Join(harnessCacheRoot(), "multiswebench-runs")
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
// never the full parent environment (T-88-01-03). PATH/HOME/HELIX_CACHE_DIR are
// the base set; the DOCKER_* keys are forwarded WHEN SET so a non-default Docker
// daemon (custom socket / TLS) is reachable — multi_swe_bench shells out to
// `docker` too. It is still an explicit allowlist: an UNLISTED key is never
// forwarded.
var envAllowlist = []string{
	"PATH", "HOME", "HELIX_CACHE_DIR",
	"DOCKER_HOST", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH",
}

// allowlistEnv builds the strict env passed to the harness subprocess: only the
// envAllowlist keys are forwarded (set keys only), never the full parent
// environment. It FAILS CLOSED (WR-04) when PATH resolves empty: the harness
// shells out to `docker`, which it locates via PATH.
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
