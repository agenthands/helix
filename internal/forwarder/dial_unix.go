//go:build !windows

package forwarder

import (
	"os/exec"
	"syscall"
)

// detachFromProcessGroup configures cmd to start in its own process group so
// the daemon survives the forwarder exiting. Unix-only; Windows uses a
// different (default) detach model.
func detachFromProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
