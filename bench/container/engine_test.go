package container

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakePATH builds a temp dir, drops 0755 stub executables named after each of
// bins into it, and points $PATH at exactly that dir (nothing else) for the
// duration of the test. Returns the dir. Fully hermetic — no real docker/podman
// is ever invoked because the tests only probe LookPath / build argv, never run
// the stub.
func fakePATH(t *testing.T, bins ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, b := range bins {
		name := b
		if runtime.GOOS == "windows" {
			name = b + ".exe"
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write stub %s: %v", p, err)
		}
	}
	t.Setenv("PATH", dir)
	return dir
}

func TestDetectFindsDocker(t *testing.T) {
	fakePATH(t, "docker")
	e, err := Detect()
	if err != nil {
		t.Fatalf("Detect() err = %v, want nil", err)
	}
	if got := filepath.Base(e.bin); got != "docker" && got != "docker.exe" {
		t.Fatalf("engine bin basename = %q, want docker", got)
	}
}

func TestDetectFindsPodmanWhenNoDocker(t *testing.T) {
	fakePATH(t, "podman")
	e, err := Detect()
	if err != nil {
		t.Fatalf("Detect() err = %v, want nil", err)
	}
	if got := filepath.Base(e.bin); got != "podman" && got != "podman.exe" {
		t.Fatalf("engine bin basename = %q, want podman", got)
	}
}

func TestDetectPrefersDocker(t *testing.T) {
	fakePATH(t, "docker", "podman")
	e, err := Detect()
	if err != nil {
		t.Fatalf("Detect() err = %v, want nil", err)
	}
	if got := filepath.Base(e.bin); got != "docker" && got != "docker.exe" {
		t.Fatalf("with both present, engine bin = %q, want docker (probed first)", got)
	}
}

func TestDetectNoneAvailable(t *testing.T) {
	fakePATH(t) // empty PATH dir, no stubs
	e, err := Detect()
	if e != nil {
		t.Fatalf("Detect() engine = %v, want nil", e)
	}
	if !errors.Is(err, errEngineUnavailable) {
		t.Fatalf("Detect() err = %v, want errEngineUnavailable", err)
	}
}

const validHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestPullByDigestUsesAtSha256NeverTag(t *testing.T) {
	e := &Engine{bin: "docker"}
	argv, err := e.pullArgs("ghcr.io/x/y", validHex)
	if err != nil {
		t.Fatalf("pullArgs err = %v, want nil", err)
	}
	// argv = ["pull", "--", "<repo>@sha256:<hex>"] — the "--" terminates option
	// parsing so a crafted ref can never be smuggled as a docker/podman flag.
	if len(argv) != 3 || argv[0] != "pull" || argv[1] != "--" {
		t.Fatalf("pullArgs = %v, want [pull -- <ref>]", argv)
	}
	want := "ghcr.io/x/y@sha256:" + validHex
	if argv[2] != want {
		t.Fatalf("ref = %q, want %q", argv[2], want)
	}
	// A tag-shaped ref (":latest", or no @sha256:) must never be produced.
	for _, bad := range []string{":latest", ":v1", ":main"} {
		if argv[2] == "ghcr.io/x/y"+bad {
			t.Fatalf("ref %q is tag-shaped, must be digest-pinned", argv[2])
		}
	}
}

func TestPullByDigestRejectsFlagSmugglingRepo(t *testing.T) {
	e := &Engine{bin: "docker"}
	// A repo beginning with '-' or carrying flag/shell metacharacters must
	// fail-close with errBadRepo BEFORE any argv reaches the engine (argv
	// flag-smuggling, T-84-01-03).
	for _, bad := range []string{
		"-x/y",                 // leading dash → would parse as a docker flag
		"--privileged",         // a real docker flag
		"ghcr.io/x/y;rm -rf /", // shell metacharacters
		"ghcr.io/x y",          // space
		"GHCR.io/x/y",          // uppercase (not a valid lowercase OCI ref)
		"",                     // empty
		"/leading-slash",       // leading separator
	} {
		if _, err := e.pullArgs(bad, validHex); !errors.Is(err, errBadRepo) {
			t.Fatalf("pullArgs(repo=%q) err = %v, want errBadRepo", bad, err)
		}
	}
	// A normal lowercase OCI ref with host:port still validates.
	if _, err := e.pullArgs("localhost:5000/owner/name", validHex); err != nil {
		t.Fatalf("pullArgs(valid host:port repo) err = %v, want nil", err)
	}
}

func TestPullByDigestRejectsNonHexDigest(t *testing.T) {
	e := &Engine{bin: "docker"}
	for _, bad := range []string{
		"",                                 // empty
		"sha256:" + validHex,               // includes prefix → not 64 hex
		validHex[:63],                      // too short
		validHex + "0",                     // too long
		"g" + validHex[1:],                 // non-hex char
		"0123456789ABCDEF" + validHex[16:], // uppercase
	} {
		if _, err := e.pullArgs("ghcr.io/x/y", bad); !errors.Is(err, errBadDigest) {
			t.Fatalf("pullArgs(%q) err = %v, want errBadDigest", bad, err)
		}
	}
	// And PullByDigest fail-closes before exec on a bad digest.
	if err := e.PullByDigest(context.Background(), "ghcr.io/x/y", "nothex"); !errors.Is(err, errBadDigest) {
		t.Fatalf("PullByDigest bad digest err = %v, want errBadDigest", err)
	}
}

func TestIsHexSHA256(t *testing.T) {
	if !isHexSHA256(validHex) {
		t.Fatalf("isHexSHA256(valid) = false, want true")
	}
	for _, bad := range []string{"", validHex[:63], validHex + "0", "G" + validHex[1:]} {
		if isHexSHA256(bad) {
			t.Fatalf("isHexSHA256(%q) = true, want false", bad)
		}
	}
}
