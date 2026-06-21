package multiswebench

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRunArgsGolden (Task 2, ADAPTER-MULTI-01): the Multi-SWE divergence — the
// argv is config-FILE-driven, not flag-driven. RunArgs over a validated config
// path yields EXACTLY the fixed ["-m","multi_swe_bench.harness.run_evaluation",
// "--config",<path>]. Hermetic seam (no live process).
func TestRunArgsGolden(t *testing.T) {
	const cfg = "/cache/helix/multiswebench-runs/config.json"
	got, err := RunArgs(cfg)
	if err != nil {
		t.Fatalf("RunArgs on a valid config path returned error: %v", err)
	}
	want := []string{
		"-m", "multi_swe_bench.harness.run_evaluation",
		"--config", cfg,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs argv mismatch:\n got = %#v\nwant = %#v", got, want)
	}
}

// TestRunArgsFailClose (Task 2, T-88-01-02): a config path failing
// isValidPredictionsPath is refused with errBadHarnessArg AND a nil argv (never a
// partial argv that could cross os/exec).
func TestRunArgsFailClose(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
	}{
		{"empty", ""},
		{"relative", "config.json"},
		{"dotdot", "/cache/../etc/config.json"},
		{"colon", "/cache/a:b/config.json"},
		{"flag-shaped", "--config=/tmp/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RunArgs(tc.cfg)
			if err == nil {
				t.Fatalf("expected errBadHarnessArg for %q, got nil and argv %#v", tc.cfg, got)
			}
			if !errors.Is(err, errBadHarnessArg) {
				t.Errorf("expected errBadHarnessArg, got %v", err)
			}
			if got != nil {
				t.Errorf("fail-close must produce a NIL argv, got %#v", got)
			}
		})
	}
}

// TestRunShimEquivalence (Task 2): Harness.Run via the runShim seam asserts the
// EXACT same argv RunArgs produces, proving the os/exec boundary argv without a
// live python+multi_swe_bench+docker.
func TestRunShimEquivalence(t *testing.T) {
	const cfg = "/cache/helix/multiswebench-runs/config.json"
	var captured []string
	h := &Harness{
		bin:     "/usr/bin/python3",
		runShim: func(args []string) error { captured = args; return nil },
	}
	if err := h.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run via runShim returned error: %v", err)
	}
	want, err := RunArgs(cfg)
	if err != nil {
		t.Fatalf("RunArgs error: %v", err)
	}
	if !reflect.DeepEqual(captured, want) {
		t.Errorf("Run shim argv mismatch:\n got = %#v\nwant = %#v", captured, want)
	}
}

// TestRunFailClosesBeforeShim (T-88-01-02): Run returns the RunArgs error verbatim
// and NEVER calls the shim when the config path is invalid.
func TestRunFailClosesBeforeShim(t *testing.T) {
	shimCalled := false
	h := &Harness{
		bin:     "/usr/bin/python3",
		runShim: func(args []string) error { shimCalled = true; return nil },
	}
	err := h.Run(context.Background(), "relative/config.json")
	if err == nil || !errors.Is(err, errBadHarnessArg) {
		t.Fatalf("expected errBadHarnessArg from Run, got %v", err)
	}
	if shimCalled {
		t.Error("Run must fail-close BEFORE the shim — the shim must never see a bad-input argv")
	}
}

// parseEnvKV splits a "KEY=VALUE" slice into a key→value map for assertions.
func parseEnvKV(env []string) map[string]string {
	seen := map[string]string{}
	for _, kv := range env {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				seen[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return seen
}

// TestAllowlistEnvStrict (Task 2, T-88-01-03): allowlistEnv forwards ONLY the
// allowlisted keys and never an unrelated parent var (e.g. a secret). The base 3
// keys are always present; DOCKER_* keys are only forwarded when set.
func TestAllowlistEnvStrict(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("HOME", "/home/runner")
	t.Setenv("HELIX_CACHE_DIR", "/cache/helix")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "super-secret")
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")

	env, err := allowlistEnv()
	if err != nil {
		t.Fatalf("allowlistEnv returned error with PATH set: %v", err)
	}
	seen := parseEnvKV(env)
	for _, want := range []string{"PATH", "HOME", "HELIX_CACHE_DIR"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("allowlistEnv missing forwarded key %q", want)
		}
	}
	if _, leaked := seen["AWS_SECRET_ACCESS_KEY"]; leaked {
		t.Error("allowlistEnv leaked AWS_SECRET_ACCESS_KEY — only the allowlist may be forwarded")
	}
	if len(seen) != 3 {
		t.Errorf("allowlistEnv forwarded %d keys, want exactly 3 (DOCKER_* unset)", len(seen))
	}
}

// TestAllowlistEnvFailsOnEmptyPath (WR-04): allowlistEnv fails closed with
// errHarnessEnv when PATH is empty — the harness shells out to `docker`.
func TestAllowlistEnvFailsOnEmptyPath(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := allowlistEnv()
	if err == nil || !errors.Is(err, errHarnessEnv) {
		t.Fatalf("allowlistEnv with empty PATH = %v, want errHarnessEnv", err)
	}
}

// TestResolveWorkDir (WR-03): an empty WorkDir defaults under HELIX_CACHE_DIR with
// the multiswebench-runs name; an invalid one fails closed with errHarnessEnv.
func TestResolveWorkDir(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("HELIX_CACHE_DIR", cache)

	got, err := resolveWorkDir("")
	if err != nil {
		t.Fatalf("resolveWorkDir(\"\") error: %v", err)
	}
	if !strings.HasPrefix(got, cache) || !strings.Contains(got, "multiswebench-runs") {
		t.Errorf("default work dir = %q, want it under %q/multiswebench-runs", got, cache)
	}
	if fi, statErr := os.Stat(got); statErr != nil || !fi.IsDir() {
		t.Errorf("default work dir %q must exist as a directory (stat err=%v)", got, statErr)
	}

	explicit := filepath.Join(cache, "explicit-run")
	got, err = resolveWorkDir(explicit)
	if err != nil || got != explicit {
		t.Fatalf("resolveWorkDir(%q) = %q, %v; want it accepted verbatim", explicit, got, err)
	}

	for _, bad := range []string{"relative/dir", "/tmp/../etc/run", "/tmp/a:b/run"} {
		if _, err := resolveWorkDir(bad); err == nil || !errors.Is(err, errHarnessEnv) {
			t.Errorf("resolveWorkDir(%q) = %v, want errHarnessEnv", bad, err)
		}
	}
}

// TestDetectAndLiveSmoke is the live leg: Detect SKIPs cleanly on
// errHarnessUnavailable (python absent), and even with python present a live
// Multi-SWE evaluation needs Docker + multi_swe_bench + images. It is NEVER the
// sole proof — the hermetic golden-argv + fail-close + shim tests above are.
func TestDetectAndLiveSmoke(t *testing.T) {
	h, err := Detect()
	if err != nil {
		if !errors.Is(err, errHarnessUnavailable) {
			t.Fatalf("Detect failed with an unexpected error (want errHarnessUnavailable): %v", err)
		}
		t.Skipf("harness unavailable, skipping live smoke: %v", err)
	}
	if h.bin == "" {
		t.Fatal("Detect returned a Harness with an empty bin")
	}
	t.Skip("python resolved, but a live multi_swe_bench evaluation needs Docker+multi_swe_bench+images (gated); hermetic argv tests carry the proof")
}
