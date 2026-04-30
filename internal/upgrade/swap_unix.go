//go:build !windows

package upgrade

import (
	"fmt"
	"os"
	"syscall"
)

// swap atomically replaces currentPath with newPath via the POSIX
// rename(2) syscall (RESEARCH.md Pattern 3). The running process keeps
// its old executable inode alive through its open file descriptor until
// exit, so the rename does NOT crash the running binary — the next
// invocation reads the new file content. Atomicity is guaranteed iff
// newPath and currentPath are on the same filesystem; the stage-dir
// design (sibling of install path, per Pitfall 3 + stage.go) ensures
// this invariant is upheld.
func swap(currentPath, newPath string) error {
	if err := os.Rename(newPath, currentPath); err != nil {
		return fmt.Errorf("rename %s → %s: %w", newPath, currentPath, err)
	}
	return nil
}

// relaunch replaces the current process image with binPath via
// syscall.Exec. On success this function never returns — the new
// binary inherits the current PID, stdio, and tty, so the user sees
// the new version inline in the same shell session.
//
// args[0] should be the program name (conventionally binPath); the
// caller is responsible for stripping the "upgrade" verb from the
// original os.Args so the relaunched binary doesn't immediately try
// to upgrade again.
func relaunch(binPath string, args, env []string) error {
	return syscall.Exec(binPath, args, env)
}
