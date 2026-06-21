//go:build !windows

package swebench

import (
	"os/exec"
	"syscall"
	"time"
)

// procGroupAttr returns a SysProcAttr that places the harness subprocess in its
// own process group (Setpgid), so a context-cancel can group-kill the python
// harness AND the per-instance Docker descendants it spawns. syscall.SysProcAttr.
// Setpgid is unix-only — this file is build-tagged !windows so the windows
// release archive still cross-compiles (mirrors bench/container/procgroup_unix.go,
// Pitfall 6 / T-87-05).
func procGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// setGroupKillCancel wires cmd.Cancel to SIGKILL the WHOLE process group on a
// context cancel (Phase-88 WR-02; the same defect lives here in the Phase-87
// template). Without this override, exec.CommandContext's default cancel sends
// SIGKILL to the leader PID only; because procGroupAttr puts that child in its
// own new group (Setpgid), the python/Docker descendants are orphaned — the
// opposite of the documented group-kill. The negative-pid syscall.Kill(-pgid)
// targets the group, mirroring bench/runtime/subprocess/ragserver.go. WaitDelay
// bounds the wait after the streams close so a wedged descendant cannot hang Wait.
func setGroupKillCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative pid → the entire process group (the Setpgid leader's group).
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 10 * time.Second
}
