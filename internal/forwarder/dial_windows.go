//go:build windows

package forwarder

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// detachFromProcessGroup configures the daemon child to start in its own
// process group, detached from the forwarder's console. Without this, a
// Ctrl-C in the calling shell would propagate via CTRL_C_EVENT to the
// daemon and kill it alongside the forwarder, defeating the detach intent
// in dial.go.
//
// CREATE_NEW_PROCESS_GROUP isolates the daemon from console signals; the
// daemon's own signal handlers (see internal/daemon) can still terminate it
// via cross-process gRPC shutdown. DETACHED_PROCESS additionally severs the
// inherited console so the daemon does not appear as a child of the
// forwarder's console window.
func detachFromProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
}
