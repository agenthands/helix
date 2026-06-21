package swebench

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// canonicalRun is the shared 5-instance smoke config the golden-argv +
// runShim-equivalence tests drive (the ADAPTER-SWE-01 "smoke run of 5 tasks"
// shape, hermetic — no live process).
func canonicalRun() HarnessRun {
	return HarnessRun{
		DatasetName:     "princeton-nlp/SWE-bench_Verified",
		PredictionsPath: "/tmp/run/predictions.jsonl",
		RunID:           "helix-smoke-1",
		InstanceIDs: []string{
			"sympy__sympy-20590",
			"django__django-12345",
			"astropy__astropy-7166",
			"scikit-learn__scikit-learn-13142",
			"matplotlib__matplotlib-23476",
		},
		MaxWorkers: 4,
		CacheLevel: "env",
	}
}

// TestHarnessArgvGolden (Phase 87 Task 3, ADAPTER-SWE-01): RunArgs produces the
// EXACT fixed argv for the canonical 5-instance smoke, byte-for-byte. This is the
// hermetically-assertable seam (no live process) — it pins the harness CLI shape
// so a refactor that reorders/renames a flag is caught immediately.
func TestHarnessArgvGolden(t *testing.T) {
	got, err := RunArgs(canonicalRun())
	if err != nil {
		t.Fatalf("RunArgs returned error on a valid config: %v", err)
	}
	want := []string{
		"-m", "swebench.harness.run_evaluation",
		"--dataset_name", "princeton-nlp/SWE-bench_Verified",
		"--predictions_path", "/tmp/run/predictions.jsonl",
		"--run_id", "helix-smoke-1",
		"--max_workers", "4",
		"--cache_level", "env",
		"--instance_ids",
		"sympy__sympy-20590",
		"django__django-12345",
		"astropy__astropy-7166",
		"scikit-learn__scikit-learn-13142",
		"matplotlib__matplotlib-23476",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs argv mismatch:\n got = %#v\nwant = %#v", got, want)
	}
}

// TestHarnessArgvUTBoostDataset (VERIFIED-02): the UTBoost augmented dataset name
// confirmed at the Task 4 checkpoint is in the allowlist and produces argv.
func TestHarnessArgvUTBoostDataset(t *testing.T) {
	r := canonicalRun()
	r.DatasetName = "Bertsekas/SWE-Bench_Verified_UTBoost"
	r.RunID = "helix-smoke-1-utboost"
	got, err := RunArgs(r)
	if err != nil {
		t.Fatalf("RunArgs rejected the allowlisted UTBoost dataset: %v", err)
	}
	if got[3] != "Bertsekas/SWE-Bench_Verified_UTBoost" {
		t.Errorf("dataset_name argv = %q, want the UTBoost suite", got[3])
	}
}

// TestHarnessArgvFailClose (Phase 87 Task 3, T-87-01): every total validator
// fail-closes — a bad input returns errBadHarnessArg AND a nil argv (never a
// partial argv that could cross os/exec).
func TestHarnessArgvFailClose(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*HarnessRun)
	}{
		{"unknown dataset name", func(r *HarnessRun) { r.DatasetName = "evil/dataset" }},
		{"forked dataset name", func(r *HarnessRun) { r.DatasetName = "attacker/SWE-bench_Verified" }},
		{"flag-shaped run_id", func(r *HarnessRun) { r.RunID = "-rf" }},
		{"empty run_id", func(r *HarnessRun) { r.RunID = "" }},
		{"run_id with space", func(r *HarnessRun) { r.RunID = "bad id" }},
		{"relative predictions path", func(r *HarnessRun) { r.PredictionsPath = "predictions.jsonl" }},
		{"dotdot predictions path", func(r *HarnessRun) { r.PredictionsPath = "/tmp/../etc/predictions.jsonl" }},
		{"colon predictions path", func(r *HarnessRun) { r.PredictionsPath = "/tmp/a:b/predictions.jsonl" }},
		{"flag-shaped predictions path", func(r *HarnessRun) { r.PredictionsPath = "--output=/tmp/x" }},
		{"flag-leading instance id", func(r *HarnessRun) { r.InstanceIDs = []string{"-rf__x-1"} }},
		{"malformed instance id (no __)", func(r *HarnessRun) { r.InstanceIDs = []string{"sympy-20590"} }},
		{"malformed instance id (no num)", func(r *HarnessRun) { r.InstanceIDs = []string{"sympy__sympy"} }},
		{"empty instance ids", func(r *HarnessRun) { r.InstanceIDs = nil }},
		{"zero max workers", func(r *HarnessRun) { r.MaxWorkers = 0 }},
		{"negative max workers", func(r *HarnessRun) { r.MaxWorkers = -1 }},
		{"unknown cache level", func(r *HarnessRun) { r.CacheLevel = "aggressive" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := canonicalRun()
			tc.mutate(&r)
			got, err := RunArgs(r)
			if err == nil {
				t.Fatalf("expected errBadHarnessArg for %s, got nil error and argv %#v", tc.name, got)
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

// TestHarnessRunShimEquivalence (Phase 87 Task 3): Harness.Run via the runShim
// seam asserts the EXACT same argv RunArgs produces, proving the os/exec boundary
// argv without a live python+swebench+docker. allowlistEnv is asserted separately
// (it never reaches the shim — the shim short-circuits before exec).
func TestHarnessRunShimEquivalence(t *testing.T) {
	var captured []string
	h := &Harness{
		bin:     "/usr/bin/python3",
		runShim: func(args []string) error { captured = args; return nil },
	}
	r := canonicalRun()
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

// TestHarnessRunFailClosesBeforeShim (T-87-01): Run returns the RunArgs error
// verbatim and NEVER calls the shim (so no argv crosses the boundary) when an
// input is invalid.
func TestHarnessRunFailClosesBeforeShim(t *testing.T) {
	shimCalled := false
	h := &Harness{
		bin:     "/usr/bin/python3",
		runShim: func(args []string) error { shimCalled = true; return nil },
	}
	r := canonicalRun()
	r.DatasetName = "evil/dataset"
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

// TestAllowlistEnvStrict (Phase 87 Task 3, T-87-02): allowlistEnv forwards ONLY
// the allowlisted keys and never an unrelated parent var (e.g. a secret). The
// base 3 keys are always present; DOCKER_* keys are only forwarded when set.
func TestAllowlistEnvStrict(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("HOME", "/home/runner")
	t.Setenv("HELIX_CACHE_DIR", "/cache/helix")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "super-secret")
	// Ensure DOCKER_* are unset so this test asserts exactly the base 3.
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
		t.Errorf("allowlistEnv forwarded %d keys, want exactly 3 (PATH/HOME/HELIX_CACHE_DIR; DOCKER_* unset)", len(seen))
	}
}

// TestAllowlistEnvFailsOnEmptyPath (WR-04): allowlistEnv fails closed with
// errHarnessEnv when PATH is empty — the harness shells out to `docker` and
// locates it via PATH, so a missing PATH must be a validated refusal, never an
// opaque subprocess crash.
func TestAllowlistEnvFailsOnEmptyPath(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := allowlistEnv()
	if err == nil || !errors.Is(err, errHarnessEnv) {
		t.Fatalf("allowlistEnv with empty PATH = %v, want errHarnessEnv", err)
	}
}

// TestAllowlistEnvForwardsDocker (WR-04): the DOCKER_* keys are forwarded WHEN
// SET so a non-default Docker daemon (custom socket / TLS) is reachable — still
// an explicit allowlist, never the inherited env.
func TestAllowlistEnvForwardsDocker(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("HOME", "/home/runner")
	t.Setenv("HELIX_CACHE_DIR", "")
	t.Setenv("DOCKER_HOST", "tcp://10.0.0.1:2376")
	t.Setenv("DOCKER_TLS_VERIFY", "1")
	t.Setenv("DOCKER_CERT_PATH", "/certs")

	env, err := allowlistEnv()
	if err != nil {
		t.Fatalf("allowlistEnv error: %v", err)
	}
	seen := parseEnvKV(env)
	for k, want := range map[string]string{
		"DOCKER_HOST":       "tcp://10.0.0.1:2376",
		"DOCKER_TLS_VERIFY": "1",
		"DOCKER_CERT_PATH":  "/certs",
	} {
		if got, ok := seen[k]; !ok || got != want {
			t.Errorf("allowlistEnv %s = %q (present=%v), want %q forwarded", k, got, ok, want)
		}
	}
}

// TestResolveWorkDir (WR-03): an explicit WorkDir must pass the clean-absolute
// discipline and is created if absent; an invalid one fails closed with
// errHarnessEnv; an empty one defaults under HELIX_CACHE_DIR.
func TestResolveWorkDir(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("HELIX_CACHE_DIR", cache)

	// Empty -> default under cache root, created.
	got, err := resolveWorkDir("")
	if err != nil {
		t.Fatalf("resolveWorkDir(\"\") error: %v", err)
	}
	if !strings.HasPrefix(got, cache) || !strings.Contains(got, "swebench-runs") {
		t.Errorf("default work dir = %q, want it under %q/swebench-runs", got, cache)
	}
	if fi, statErr := os.Stat(got); statErr != nil || !fi.IsDir() {
		t.Errorf("default work dir %q must exist as a directory (stat err=%v)", got, statErr)
	}

	// Explicit valid absolute dir -> accepted and created.
	explicit := filepath.Join(cache, "explicit-run")
	got, err = resolveWorkDir(explicit)
	if err != nil || got != explicit {
		t.Fatalf("resolveWorkDir(%q) = %q, %v; want it accepted verbatim", explicit, got, err)
	}

	// Invalid (relative / traversal) -> fail closed.
	for _, bad := range []string{"relative/dir", "/tmp/../etc/run", "/tmp/a:b/run"} {
		if _, err := resolveWorkDir(bad); err == nil || !errors.Is(err, errHarnessEnv) {
			t.Errorf("resolveWorkDir(%q) = %v, want errHarnessEnv", bad, err)
		}
	}
}

// TestLiveSmoke is the live leg: it SKIPs cleanly on errHarnessUnavailable
// (python absent) — it is NEVER the sole proof; the hermetic golden-argv +
// fail-close + shim tests above are. A live run additionally needs Docker + the
// swebench package + per-instance images, none of which are guaranteed here.
func TestLiveSmoke(t *testing.T) {
	h, err := Detect()
	if err != nil {
		t.Skipf("harness unavailable, skipping live smoke: %v", err)
	}
	// Even with python present, a live evaluation needs Docker + the swebench
	// package + network; this wrapper does not assert a live grade here (that is
	// the Docker-gated smoke). We only prove Detect resolved an interpreter.
	if h.bin == "" {
		t.Fatal("Detect returned a Harness with an empty bin")
	}
	t.Skip("python resolved, but a live swebench evaluation needs Docker+swebench+images (gated); hermetic argv tests carry the proof")
}
