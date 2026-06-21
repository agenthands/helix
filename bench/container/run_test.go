package container

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// TestRunArgsNetworkNoneFixedArgv is the authoritative SC#3 proof: it asserts
// the EXACT fixed argv runArgs produces, WITHOUT a live engine. The presence of
// "--network=none" in this argv IS the network-isolation enforcement point for
// untrusted Exercism test code (T-85-02-02).
func TestRunArgsNetworkNoneFixedArgv(t *testing.T) {
	image := "ghcr.io/x/y@sha256:" + validHex
	// A single mount keeps the golden deterministic; multi-mount ordering is
	// covered separately below.
	argv, err := runArgs(image, []string{"pytest", "-q"}, map[string]string{"/srv/repo": "/work"}, true)
	if err != nil {
		t.Fatalf("runArgs err = %v, want nil", err)
	}
	want := []string{
		"run", "--rm", "--network=none",
		"-v", "/srv/repo:/work",
		"--", image, "pytest", "-q",
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("runArgs argv =\n  %v\nwant\n  %v", argv, want)
	}
}

// TestRunArgsNetworkFlagIsExactlyThePolicy proves --network=none is present iff
// netNone — the flag is the network policy, not always-on.
func TestRunArgsNetworkFlagIsExactlyThePolicy(t *testing.T) {
	image := "ghcr.io/x/y@sha256:" + validHex
	argv, err := runArgs(image, []string{"true"}, map[string]string{"/srv/repo": "/work"}, false)
	if err != nil {
		t.Fatalf("runArgs(netNone=false) err = %v, want nil", err)
	}
	for _, a := range argv {
		if a == "--network=none" {
			t.Fatalf("runArgs(netNone=false) leaked --network=none: %v", argv)
		}
	}
	// netNone=true must add it back.
	argv2, err := runArgs(image, []string{"true"}, map[string]string{"/srv/repo": "/work"}, true)
	if err != nil {
		t.Fatalf("runArgs(netNone=true) err = %v, want nil", err)
	}
	found := false
	for _, a := range argv2 {
		if a == "--network=none" {
			found = true
		}
	}
	if !found {
		t.Fatalf("runArgs(netNone=true) missing --network=none: %v", argv2)
	}
}

// TestRunArgsDeterministicMountOrder proves multiple mounts emit in a stable
// (sorted-by-source) order so the argv is byte-reproducible.
func TestRunArgsDeterministicMountOrder(t *testing.T) {
	image := "ghcr.io/x/y@sha256:" + validHex
	mounts := map[string]string{
		"/b/src": "/work/b",
		"/a/src": "/work/a",
		"/c/src": "/work/c",
	}
	argv, err := runArgs(image, []string{"true"}, mounts, true)
	if err != nil {
		t.Fatalf("runArgs err = %v, want nil", err)
	}
	want := []string{
		"run", "--rm", "--network=none",
		"-v", "/a/src:/work/a",
		"-v", "/b/src:/work/b",
		"-v", "/c/src:/work/c",
		"--", image, "true",
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("runArgs argv =\n  %v\nwant\n  %v", argv, want)
	}
}

// TestRunArgsRejectsFlagSmugglingImage proves an image with a leading '-' (or
// otherwise malformed ref) fails-close with errBadRepo BEFORE any argv is
// produced (T-85-02-01), mirroring pullArgs.
func TestRunArgsRejectsFlagSmugglingImage(t *testing.T) {
	for _, bad := range []string{
		"-x/y@sha256:" + validHex, // leading dash → would parse as a docker flag
		"--privileged",            // a real docker flag
		"ghcr.io/x/y;rm -rf /",    // shell metacharacters
		"GHCR.io/x/y",             // uppercase
		"",                        // empty
		"/leading-slash",          // leading separator
	} {
		if _, err := runArgs(bad, []string{"true"}, map[string]string{"/srv/repo": "/work"}, true); !errors.Is(err, errBadRepo) {
			t.Fatalf("runArgs(image=%q) err = %v, want errBadRepo", bad, err)
		}
	}
}

// TestRunArgsRejectsBadMount proves a mount source containing ".." or an
// absolute-escape / flag-smuggling shape fails-close with errBadMount BEFORE
// any argv is produced.
func TestRunArgsRejectsBadMount(t *testing.T) {
	image := "ghcr.io/x/y@sha256:" + validHex
	for _, bad := range []map[string]string{
		{"/srv/../etc": "/work"},  // .. in source
		{"/srv/repo": "/work/../"}, // .. in destination
		{"-rm": "/work"},          // flag-smuggling source
		{"/srv/repo": "-rm"},      // flag-smuggling destination
		{"relative/path": "/work"}, // not absolute source
		{"/srv/repo": "relative"},  // not absolute destination
		{"": "/work"},             // empty source
		{"/srv/repo": ""},         // empty destination
		{"/srv/repo:x": "/work"},  // ':' in source would break the -v pair
		{"/srv/repo": "/work:y"},  // ':' in destination would break the -v pair
	} {
		if _, err := runArgs(image, []string{"true"}, bad, true); !errors.Is(err, errBadMount) {
			t.Fatalf("runArgs(mount=%v) err = %v, want errBadMount", bad, err)
		}
	}
}

// TestRunInvokesRunArgsArgv is the hermetic Run proof: it asserts Run hands the
// engine bin EXACTLY the runArgs output, captured via a recorded-argv shim — no
// real engine is invoked. Fail-close on bad input is verified too.
func TestRunInvokesRunArgsArgv(t *testing.T) {
	e := &Engine{bin: "docker"}
	image := "ghcr.io/x/y@sha256:" + validHex
	mounts := map[string]string{"/srv/repo": "/work"}

	want, err := runArgs(image, []string{"pytest", "-q"}, mounts, true)
	if err != nil {
		t.Fatalf("runArgs err = %v, want nil", err)
	}

	var got []string
	e.runShim = func(args []string) error {
		got = append([]string(nil), args...)
		return nil
	}
	if err := e.Run(context.Background(), image, []string{"pytest", "-q"}, mounts, true); err != nil {
		t.Fatalf("Run err = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Run argv =\n  %v\nwant\n  %v", got, want)
	}

	// Fail-close: a bad image must return errBadRepo and NEVER reach the shim.
	got = nil
	if err := e.Run(context.Background(), "-x/y", []string{"true"}, mounts, true); !errors.Is(err, errBadRepo) {
		t.Fatalf("Run(bad image) err = %v, want errBadRepo", err)
	}
	if got != nil {
		t.Fatalf("Run(bad image) reached the engine shim with argv %v, must fail-close", got)
	}
}

// TestRunLiveSkipsWithoutEngine is the live-gated leg: it is NOT the authoritative
// proof (the argv assertions above are). When no docker/podman is on PATH it
// t.Skips cleanly rather than failing or passing vacuously.
func TestRunLiveSkipsWithoutEngine(t *testing.T) {
	e, err := Detect()
	if errors.Is(err, errEngineUnavailable) {
		t.Skip("no docker/podman on PATH — live container Run skipped (argv assertions are the authoritative SC#3 proof)")
	}
	if err != nil {
		t.Fatalf("Detect() err = %v, want nil or errEngineUnavailable", err)
	}
	// An engine IS present: a bad image must still fail-close before exec.
	if err := e.Run(context.Background(), "-x/y", []string{"true"}, map[string]string{"/srv/repo": "/work"}, true); !errors.Is(err, errBadRepo) {
		t.Fatalf("Run(bad image) with live engine err = %v, want errBadRepo", err)
	}
}
