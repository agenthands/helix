package terminalbench

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// validRun is a well-formed config used as the base for the argv-golden and
// fail-close tests (the per-field bad values are substituted per case).
func validRun() HarnessRun {
	return HarnessRun{
		Agent:          "oracle",
		DatasetVersion: "0.1.0",
		TaskID:         "hello-world",
		OutputPath:     "/cache/helix/terminalbench-runs/out",
	}
}

// TestRunArgsGolden (Task 1, ADAPTER-TERM-01, default runnerKind=tb): RunArgs over
// a validated config yields EXACTLY the fixed tb run argv (A3/A7 [ASSUMED] flag
// shapes pinned as the hermetic contract Plan 04's human-verify confirms).
// Hermetic seam — no live process.
func TestRunArgsGolden(t *testing.T) {
	r := validRun()
	got, err := RunArgs(r)
	if err != nil {
		t.Fatalf("RunArgs on a valid config returned error: %v", err)
	}
	want := []string{
		"run",
		"--agent", "oracle",
		"--dataset-name", "terminal-bench-core",
		"--dataset-version", "0.1.0",
		"--task-id", "hello-world",
		"--output-path", "/cache/helix/terminalbench-runs/out",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs argv mismatch:\n got = %#v\nwant = %#v", got, want)
	}
}

// TestRunnerKindSeam (Task 1, O-1 / Pitfall 2): the binary name + dataset-flag
// form live behind a SINGLE runnerKind seam so swapping tb→harbor is a one-line
// change. Switching the kind changes ONLY the binary name (and the documented
// dataset-flag form), never RunArgs's structure. The seam is a single
// constant/field, not scattered string literals.
func TestRunnerKindSeam(t *testing.T) {
	// The default kind is tb with the legacy binary name.
	tb := runnerKindTB.spec()
	if tb.binary != "tb" {
		t.Errorf("runnerKindTB binary = %q, want tb", tb.binary)
	}
	// The harbor kind is the 2.0-native one-line swap: ONLY the binary name (and
	// the documented dataset-flag form) differ — the seam exists as one constant.
	harbor := runnerKindHarbor.spec()
	if harbor.binary != "harbor" {
		t.Errorf("runnerKindHarbor binary = %q, want harbor", harbor.binary)
	}
	if tb.binary == harbor.binary {
		t.Error("the runnerKind seam must distinguish tb from harbor by binary name")
	}
	// The default kind used by Detect/Run is tb (CONTEXT-locked).
	if defaultRunnerKind != runnerKindTB {
		t.Errorf("defaultRunnerKind = %v, want runnerKindTB (CONTEXT locks tb run)", defaultRunnerKind)
	}
}

// TestRunArgsFailClose (Task 1, T-88-02-01): a task-id, dataset-version, or
// output-path that is empty / leading-'-' / (output-path) relative or
// '..'-bearing is refused with errBadHarnessArg AND a nil argv — never a partial
// argv that could cross os/exec.
func TestRunArgsFailClose(t *testing.T) {
	cases := []struct {
		name  string
		mutate func(*HarnessRun)
	}{
		{"empty agent", func(r *HarnessRun) { r.Agent = "" }},
		{"flag-shaped agent", func(r *HarnessRun) { r.Agent = "-rf" }},
		{"empty task-id", func(r *HarnessRun) { r.TaskID = "" }},
		{"flag-shaped task-id", func(r *HarnessRun) { r.TaskID = "-rf" }},
		{"empty dataset-version", func(r *HarnessRun) { r.DatasetVersion = "" }},
		{"flag-shaped dataset-version", func(r *HarnessRun) { r.DatasetVersion = "-1.0" }},
		{"empty output-path", func(r *HarnessRun) { r.OutputPath = "" }},
		{"relative output-path", func(r *HarnessRun) { r.OutputPath = "out" }},
		{"dotdot output-path", func(r *HarnessRun) { r.OutputPath = "/cache/../etc/out" }},
		{"colon output-path", func(r *HarnessRun) { r.OutputPath = "/cache/a:b/out" }},
		{"flag-shaped output-path", func(r *HarnessRun) { r.OutputPath = "--output-path=/tmp/x" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validRun()
			tc.mutate(&r)
			got, err := RunArgs(r)
			if err == nil {
				t.Fatalf("expected errBadHarnessArg, got nil and argv %#v", got)
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

// TestRunShimEquivalence (Task 1): Harness.Run via the runShim seam asserts the
// EXACT same argv RunArgs produces, proving the os/exec boundary argv without a
// live tb+docker.
func TestRunShimEquivalence(t *testing.T) {
	r := validRun()
	var captured []string
	h := &Harness{
		bin:     "/usr/bin/tb",
		runShim: func(args []string) error { captured = args; return nil },
	}
	if err := h.Run(context.Background(), r); err != nil {
		t.Fatalf("Run via runShim returned error: %v", err)
	}
	want, err := RunArgs(r)
	if err != nil {
		t.Fatalf("RunArgs error: %v", err)
	}
	if !reflect.DeepEqual(captured, want) {
		t.Errorf("Run shim argv mismatch:\n got = %#v\nwant = %#v", captured, want)
	}
}

// TestRunFailClosesBeforeShim (T-88-02-01): Run returns the RunArgs error verbatim
// and NEVER calls the shim when an input is invalid.
func TestRunFailClosesBeforeShim(t *testing.T) {
	shimCalled := false
	h := &Harness{
		bin:     "/usr/bin/tb",
		runShim: func(args []string) error { shimCalled = true; return nil },
	}
	r := validRun()
	r.OutputPath = "relative/out"
	err := h.Run(context.Background(), r)
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

// TestAllowlistEnvStrict (Task 1, T-88-02-02): allowlistEnv forwards ONLY the
// allowlisted keys and never an unrelated parent var. The base 3 keys are always
// present; DOCKER_* keys are only forwarded when set.
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
// errHarnessEnv when PATH is empty — tb shells out to `docker`.
func TestAllowlistEnvFailsOnEmptyPath(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := allowlistEnv()
	if err == nil || !errors.Is(err, errHarnessEnv) {
		t.Fatalf("allowlistEnv with empty PATH = %v, want errHarnessEnv", err)
	}
}

// TestResolveWorkDir (WR-03): an empty WorkDir defaults under HELIX_CACHE_DIR with
// the terminalbench-runs name; an invalid one fails closed with errHarnessEnv.
func TestResolveWorkDir(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("HELIX_CACHE_DIR", cache)

	got, err := resolveWorkDir("")
	if err != nil {
		t.Fatalf("resolveWorkDir(\"\") error: %v", err)
	}
	if !strings.HasPrefix(got, cache) || !strings.Contains(got, "terminalbench-runs") {
		t.Errorf("default work dir = %q, want it under %q/terminalbench-runs", got, cache)
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
// errHarnessUnavailable (neither tb nor harbor on PATH), and even with tb present
// a live ≥5-task evaluation needs Docker + the tb dataset images. It is NEVER the
// sole proof — the hermetic golden-argv + fail-close + shim tests above are.
func TestDetectAndLiveSmoke(t *testing.T) {
	h, err := Detect()
	if err != nil {
		if !errors.Is(err, errHarnessUnavailable) {
			t.Fatalf("Detect failed with an unexpected error (want errHarnessUnavailable): %v", err)
		}
		t.Skipf("tb/harbor unavailable, skipping live smoke: %v", err)
	}
	if h.bin == "" {
		t.Fatal("Detect returned a Harness with an empty bin")
	}
	t.Skip("tb/harbor resolved, but a live ≥5-task tb evaluation needs Docker+the dataset images (gated); hermetic argv tests carry the proof")
}
