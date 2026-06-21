//go:build windows

package container

import "syscall"

// procGroupAttr returns nil on windows: POSIX process groups (Setpgid) have no
// windows analog, so cancellation falls back to exec.CommandContext's default
// Process.Kill. Keeping the seam build-tag-split lets the package compile for
// the windows release archive without importing unix-only syscall fields
// (Pitfall 6).
//
// KNOWN LIMITATION (WR-04): unlike the unix path (Setpgid → group-kill of the
// whole engine subtree), the windows default Process.Kill terminates ONLY the
// direct docker/podman process, NOT its descendants. On a context cancel or
// timeout, child pull/transfer workers a CLI spawns can be orphaned and keep
// consuming disk/network. The proper fix is to attach the child to a Win32 Job
// Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE (via golang.org/x/sys/windows)
// so a cancel tears down the whole tree; that is deferred and tracked, not
// claimed as parity with the unix group-kill here.
func procGroupAttr() *syscall.SysProcAttr {
	return nil
}
