//go:build !windows

package terminalbench

import "syscall"

// procGroupAttr returns a SysProcAttr that places the tb subprocess in its own
// process group (Setpgid), so a context-cancel can group-kill the tb CLI AND the
// per-task Docker descendants it spawns via its DockerComposeManager.
// syscall.SysProcAttr.Setpgid is unix-only — this file is build-tagged !windows
// so the windows release archive still cross-compiles (mirrors
// bench/evaluators/swebench/procgroup_unix.go, Pitfall 6 / T-88-02-06).
func procGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
