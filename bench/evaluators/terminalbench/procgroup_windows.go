//go:build windows

package terminalbench

import (
	"os/exec"
	"syscall"
)

// procGroupAttr returns nil on windows: POSIX process groups (Setpgid) have no
// windows analog, so cancellation falls back to exec.CommandContext's default
// Process.Kill. Keeping the seam build-tag-split lets the package compile for the
// windows release archive without importing unix-only syscall fields (mirrors
// bench/evaluators/swebench/procgroup_windows.go, Pitfall 6).
//
// KNOWN LIMITATION: unlike the unix path (Setpgid → group-kill of the whole
// tb+Docker subtree), the windows default Process.Kill terminates ONLY the
// direct tb process, NOT its Docker descendants. On a context cancel the
// per-task containers tb spawns can be orphaned. The proper fix is a Win32 Job
// Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE; that is deferred and tracked,
// not claimed as parity with the unix group-kill here.
func procGroupAttr() *syscall.SysProcAttr {
	return nil
}

// setGroupKillCancel is a no-op on windows (WR-02): POSIX process-group kill has
// no analog, so cancellation keeps exec.CommandContext's default Process.Kill of
// the direct child. The same Win32 Job Object limitation documented on
// procGroupAttr applies — descendants can be orphaned on cancel. Keeping the seam
// build-tag-split so the windows release archive still cross-compiles without
// the unix-only negative-pid syscall.Kill.
func setGroupKillCancel(cmd *exec.Cmd) {}
