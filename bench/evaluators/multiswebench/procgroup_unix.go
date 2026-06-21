//go:build !windows

package multiswebench

import "syscall"

// procGroupAttr returns a SysProcAttr that places the harness subprocess in its
// own process group (Setpgid), so a context-cancel can group-kill the python
// harness AND the per-instance Docker descendants it spawns. syscall.SysProcAttr.
// Setpgid is unix-only — this file is build-tagged !windows so the windows
// release archive still cross-compiles (mirrors bench/evaluators/swebench/
// procgroup_unix.go, Pitfall 6 / T-87-05).
func procGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
