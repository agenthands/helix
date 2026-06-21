//go:build !windows

package terminalbench

import (
	"os/exec"
	"syscall"
	"time"
)

// procGroupAttr returns a SysProcAttr that places the tb subprocess in its own
// process group (Setpgid), so a context-cancel can group-kill the tb CLI AND the
// per-task Docker descendants it spawns via its DockerComposeManager.
// syscall.SysProcAttr.Setpgid is unix-only — this file is build-tagged !windows
// so the windows release archive still cross-compiles (mirrors
// bench/evaluators/swebench/procgroup_unix.go, Pitfall 6 / T-88-02-06).
func procGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// setGroupKillCancel wires cmd.Cancel to SIGKILL the WHOLE process group on a
// context cancel (WR-02). Without this override, exec.CommandContext's default
// cancel sends SIGKILL to the leader PID only; because procGroupAttr puts that
// child in its own new group (Setpgid), the tb/Docker descendants are orphaned
// (re-parented to init) — the opposite of the documented group-kill. The
// negative-pid syscall.Kill(-pgid) targets the group, mirroring the proven
// primitive at bench/runtime/subprocess/ragserver.go. WaitDelay bounds the wait
// after the streams close so a wedged descendant cannot hang the harness Wait.
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
