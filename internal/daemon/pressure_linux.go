package daemon

import "github.com/agenthands/helix/internal/kernel/lspool"

// newPlatformPressure returns a platform-appropriate MemoryPressure implementation.
func newPlatformPressure() lspool.MemoryPressure {
	return lspool.NewLinuxMemoryPressure()
}
