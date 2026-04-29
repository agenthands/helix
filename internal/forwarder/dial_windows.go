//go:build windows

package forwarder

import "os/exec"

// detachFromProcessGroup is a no-op on Windows. Detached daemon spawning on
// Windows uses CREATE_NEW_PROCESS_GROUP via different SysProcAttr fields if
// needed in the future; for now the default behavior is acceptable since
// goreleaser only ships Windows binaries as placeholder archives (DEF-51-02).
func detachFromProcessGroup(_ *exec.Cmd) {}
