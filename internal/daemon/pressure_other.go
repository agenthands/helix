//go:build !darwin && !linux

package daemon

import "github.com/agenthands/helix/internal/kernel/lspool"

// newPlatformPressure returns nil on unsupported platforms.
// The pool handles nil pressure gracefully (skips pressure checks).
func newPlatformPressure() lspool.MemoryPressure {
	return nil
}
