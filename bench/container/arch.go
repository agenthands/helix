package container

import (
	"fmt"
	"os"
	"runtime"
)

// archMismatchEnv is the read-only escape hatch that lets an operator
// deliberately run an image whose architecture differs from the host. ONLY the
// exact value "1" opens the gate — "0", "", "true", " 1", etc. all keep it shut
// (avoids truthy-string ambiguity).
const archMismatchEnv = "BENCH_ARCH_MISMATCH_OK"

// ArchGate is a PURE decision function: given the host architecture and the
// image manifest architecture, it returns nil when they match, and otherwise
// REFUSES with an error unless the operator has set BENCH_ARCH_MISMATCH_OK=1.
//
// Refusing a cross-arch run prevents an arm64 host from silently qemu-emulating
// an amd64 SWE-bench image (Pitfall 5) — emulation perturbs timing and can
// change build/test outcomes, corrupting bench results. hostArch is injected as
// a parameter (rather than read from runtime.GOARCH) so this gate is
// hermetically testable both ways without depending on the runner's real arch.
//
// manifestArch is read daemon-free from crane.Config(...).Architecture in Plan
// 03's pull path; this function only string-compares, keeping arch.go free of
// the go-containerregistry dependency.
func ArchGate(hostArch, manifestArch string) error {
	if hostArch == manifestArch {
		return nil
	}
	if os.Getenv(archMismatchEnv) == "1" {
		return nil
	}
	return fmt.Errorf(
		"bench/container: host arch %s != image arch %s — refusing to run under emulation; set %s=1 to override",
		hostArch, manifestArch, archMismatchEnv,
	)
}

// ArchGateForHost is the convenience wrapper that supplies the real host arch
// via runtime.GOARCH (the internal/langregistry installer idiom — never parse
// `uname -m`).
func ArchGateForHost(manifestArch string) error {
	return ArchGate(runtime.GOARCH, manifestArch)
}
