//go:build windows

package container

import "syscall"

// procGroupAttr returns nil on windows: POSIX process groups (Setpgid) have no
// windows analog, and exec.CommandContext's default cancel (Process.Kill) is
// sufficient there. Keeping the seam build-tag-split lets the package compile for
// the windows release archive without importing unix-only syscall fields
// (Pitfall 6).
func procGroupAttr() *syscall.SysProcAttr {
	return nil
}
