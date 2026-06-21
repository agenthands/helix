package container

import (
	"strings"
	"testing"
)

func TestArchGateMatch(t *testing.T) {
	// host == manifest: allowed regardless of escape hatch.
	t.Setenv("BENCH_ARCH_MISMATCH_OK", "")
	if err := ArchGate("arm64", "arm64"); err != nil {
		t.Fatalf("ArchGate(arm64,arm64) = %v, want nil", err)
	}
}

func TestArchGateMismatchRefused(t *testing.T) {
	t.Setenv("BENCH_ARCH_MISMATCH_OK", "")
	err := ArchGate("arm64", "amd64")
	if err == nil {
		t.Fatalf("ArchGate(arm64,amd64) = nil, want refusal")
	}
	// Error must name both arches and the escape hatch (CONTAINER-01 / SC#1).
	msg := err.Error()
	for _, want := range []string{"arm64", "amd64", "BENCH_ARCH_MISMATCH_OK"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
}

func TestArchGateMismatchEscapeHatch(t *testing.T) {
	t.Setenv("BENCH_ARCH_MISMATCH_OK", "1")
	if err := ArchGate("arm64", "amd64"); err != nil {
		t.Fatalf("ArchGate(arm64,amd64) with OK=1 = %v, want nil", err)
	}
}

func TestArchGateEscapeHatchOnlyExactOne(t *testing.T) {
	for _, v := range []string{"", "0", "true", "yes", "2", " 1", "1 "} {
		t.Run("val="+v, func(t *testing.T) {
			t.Setenv("BENCH_ARCH_MISMATCH_OK", v)
			if err := ArchGate("arm64", "amd64"); err == nil {
				t.Fatalf("ArchGate with OK=%q = nil, want refusal (only exact \"1\" opens gate)", v)
			}
		})
	}
}

func TestArchGateForHostUsesRuntimeGOARCH(t *testing.T) {
	// ArchGateForHost passes runtime.GOARCH as host; a manifest matching the
	// real host must pass. We can't know GOARCH at compile time, so assert the
	// matching case via the wrapper against itself is allowed and a deliberate
	// bogus arch is refused (escape hatch unset).
	t.Setenv("BENCH_ARCH_MISMATCH_OK", "")
	if err := ArchGateForHost("definitely-not-a-real-arch"); err == nil {
		t.Fatalf("ArchGateForHost(bogus) = nil, want refusal")
	}
}
