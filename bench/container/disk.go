package container

import "fmt"

// MinFreeBytes is the minimum available disk space the bench run requires before
// it will pull any image (CONTAINER-04). 50 GiB binary (A3: GiB, not GB, per
// RESEARCH) — a full SWE-bench image set can exhaust a contributor's laptop, so
// the pre-flight guard refuses below this watermark before any network pull.
const MinFreeBytes uint64 = 50 * 1024 * 1024 * 1024

// DiskGuard performs a pre-flight available-disk check. availFn is an injectable
// seam: production wires it to the build-tagged availBytes (unix.Statfs /
// GetDiskFreeSpaceEx), while tests inject a synthetic function so the low-disk
// refusal is 100% hermetic and never depends on a real full volume.
type DiskGuard struct {
	availFn func(path string) (uint64, error)
}

// NewDiskGuard returns a DiskGuard wired to the platform's real available-bytes
// implementation (availBytes is provided by disk_unix.go / disk_windows.go).
func NewDiskGuard() DiskGuard {
	return DiskGuard{availFn: availBytes}
}

// Check refuses the run when the available disk at path is below MinFreeBytes.
// A syscall failure from availFn is propagated verbatim (no "refused" framing,
// since it is not a budget violation). A budget violation returns a single-line
// error naming the free amount, the path, the 50 GiB requirement, and a one-line
// remediation (CONTAINER-04).
func (g DiskGuard) Check(path string) error {
	free, err := g.availFn(path)
	if err != nil {
		return err
	}
	if free < MinFreeBytes {
		return fmt.Errorf("bench refused: %d GiB free at %s, need 50 GiB — free space or set $HELIX_CACHE_DIR to a larger volume", free/(1<<30), path)
	}
	return nil
}
