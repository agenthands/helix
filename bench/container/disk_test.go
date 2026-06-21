package container

import (
	"errors"
	"strings"
	"testing"
)

const gib = uint64(1) << 30

func TestDiskGuardRefusesBelowThreshold(t *testing.T) {
	g := DiskGuard{availFn: func(string) (uint64, error) { return 10 * gib, nil }}
	err := g.Check("/some/path")
	if err == nil {
		t.Fatalf("Check with 10 GiB free returned nil, want refusal")
	}
	msg := err.Error()
	for _, want := range []string{"10", "/some/path", "50 GiB"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal message %q missing %q", msg, want)
		}
	}
	if !strings.Contains(msg, "HELIX_CACHE_DIR") {
		t.Errorf("refusal message %q missing remediation pointer to $HELIX_CACHE_DIR", msg)
	}
}

func TestDiskGuardAllowsAtOrAboveThreshold(t *testing.T) {
	for _, free := range []uint64{MinFreeBytes, 60 * gib} {
		g := DiskGuard{availFn: func(string) (uint64, error) { return free, nil }}
		if err := g.Check("/p"); err != nil {
			t.Errorf("Check with %d bytes free returned %v, want nil", free, err)
		}
	}
}

func TestDiskGuardPropagatesAvailErr(t *testing.T) {
	sentinel := errors.New("statfs boom")
	g := DiskGuard{availFn: func(string) (uint64, error) { return 0, sentinel }}
	err := g.Check("/p")
	if !errors.Is(err, sentinel) {
		t.Fatalf("Check err = %v, want the propagated syscall error", err)
	}
	if strings.Contains(err.Error(), "refused") {
		t.Errorf("syscall failure should not be framed as a budget refusal: %q", err.Error())
	}
}

func TestDiskGuardRemediationIsOneLine(t *testing.T) {
	g := DiskGuard{availFn: func(string) (uint64, error) { return 1 * gib, nil }}
	err := g.Check("/p")
	if err == nil {
		t.Fatal("expected refusal")
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("refusal message must be a single line (no newline): %q", err.Error())
	}
}

func TestMinFreeBytesIs50GiB(t *testing.T) {
	if MinFreeBytes != 50*gib {
		t.Fatalf("MinFreeBytes = %d, want %d (50 GiB binary)", MinFreeBytes, 50*gib)
	}
}
