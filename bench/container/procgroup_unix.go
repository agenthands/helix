//go:build !windows

package container

import "syscall"

// procGroupAttr returns a SysProcAttr that places the engine subprocess in its
// own process group (Setpgid), so a context-cancel can group-kill the engine and
// any descendants. syscall.SysProcAttr.Setpgid is unix-only — this file is
// build-tagged !windows so the windows release archive still cross-compiles
// (Pitfall 6).
func procGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
