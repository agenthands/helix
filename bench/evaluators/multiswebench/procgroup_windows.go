//go:build windows

package multiswebench

import "syscall"

// procGroupAttr returns nil on windows: POSIX process groups (Setpgid) have no
// windows analog, so cancellation falls back to exec.CommandContext's default
// Process.Kill. Keeping the seam build-tag-split lets the package compile for the
// windows release archive without importing unix-only syscall fields (mirrors
// bench/evaluators/swebench/procgroup_windows.go, Pitfall 6).
//
// KNOWN LIMITATION: unlike the unix path (Setpgid → group-kill of the whole
// harness+Docker subtree), the windows default Process.Kill terminates ONLY the
// direct python process, NOT its Docker descendants. On a context cancel the
// per-instance image-build/test workers the harness spawns can be orphaned. The
// proper fix is a Win32 Job Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE; that
// is deferred and tracked, not claimed as parity with the unix group-kill here.
func procGroupAttr() *syscall.SysProcAttr {
	return nil
}
