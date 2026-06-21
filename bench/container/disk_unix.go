//go:build !windows

package container

import "golang.org/x/sys/unix"

// availBytes returns the bytes available to an unprivileged caller on the
// filesystem containing path, via statfs(2): Bavail (free blocks available to
// non-root) × Bsize (fundamental block size). golang.org/x/sys/unix is imported
// ONLY in this build-tagged file — importing it in an untagged file would break
// the windows release archives (Pitfall 6).
func availBytes(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}
