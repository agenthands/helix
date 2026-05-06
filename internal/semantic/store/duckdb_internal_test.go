//go:build !(windows && arm64)

package store

import (
	"errors"
	"syscall"
	"testing"
)

// TestClassifyReopenError exercises the WR-03 classifier helper: transient
// filesystem-contention errors are a worth-one-retry signal; corruption
// signatures route directly to quarantine.
func TestClassifyReopenError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want reopenErrClass
	}{
		{"ebusy", syscall.EBUSY, reopenTransient},
		{"eintr", syscall.EINTR, reopenTransient},
		{"eagain", syscall.EAGAIN, reopenTransient},
		{"checksum", errors.New("page checksum mismatch"), reopenCorruption},
		{"corrupt-msg", errors.New("file is corrupt"), reopenCorruption},
		{"header", errors.New("invalid header"), reopenCorruption},
		{"malformed", errors.New("malformed b-tree node"), reopenCorruption},
		{"unknown", errors.New("some random other error"), reopenUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyReopenError(tc.err); got != tc.want {
				t.Fatalf("classifyReopenError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestClassifyReopenError_ETXTBSY exercises ETXTBSY separately. It is a
// POSIX-only constant but syscall.ETXTBSY is defined on linux and darwin,
// the only platforms shipping CGO=1 in v1.10. The build tag at the top of
// this file already excludes windows/arm64, so the constant resolves.
func TestClassifyReopenError_ETXTBSY(t *testing.T) {
	if got := classifyReopenError(syscall.ETXTBSY); got != reopenTransient {
		t.Fatalf("classifyReopenError(ETXTBSY) = %v, want reopenTransient", got)
	}
}

// TestDecideReopenRetry_PersistentTransientHardFails locks WR-NEW-02: when
// the SECOND open attempt is still transient (persistent EBUSY etc), the
// retry decision must be HardFail — refusing to quarantine a (likely
// clean) DB so the supervisor can restart and try again. Only an actual
// corruption-class error on the second attempt should quarantine.
func TestDecideReopenRetry_PersistentTransientHardFails(t *testing.T) {
	tests := []struct {
		name      string
		secondErr error
		want      reopenRetryDecision
	}{
		{"persistent_ebusy", syscall.EBUSY, reopenRetryHardFail},
		{"persistent_eintr", syscall.EINTR, reopenRetryHardFail},
		{"persistent_eagain", syscall.EAGAIN, reopenRetryHardFail},
		{"persistent_etxtbsy", syscall.ETXTBSY, reopenRetryHardFail},
		{"corruption_on_retry", errors.New("page checksum mismatch"), reopenRetryQuarantine},
		{"unknown_on_retry", errors.New("some other error"), reopenRetryQuarantine},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decideReopenRetry(tc.secondErr)
			if got != tc.want {
				t.Fatalf("decideReopenRetry(%v) = %v, want %v", tc.secondErr, got, tc.want)
			}
		})
	}
}
