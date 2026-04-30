package upgrade

import "os"

// daemonEnvVar is the env var the daemon supervisor sets on its own
// process so that any descendant `helix upgrade` invocation can detect
// it is running inside the daemon's process tree and refuse to swap the
// running binary in place (which would tear out the daemon's executable
// from under it). See CONTEXT.md D-08 and `internal/daemon/daemon.go`
// where the var is set during daemon startup.
const daemonEnvVar = "HELIX_RUNNING_AS_DAEMON"

// RunningInDaemon reports whether the calling process is running inside
// the helix daemon's process tree. Returns true iff HELIX_RUNNING_AS_DAEMON=1
// in the current process environment.
//
// The daemon supervisor sets this env var on its own os.Setenv at the
// start of `(*Daemon).Run`, so any forked child (LS worker, hypothetical
// upgrade exec) inherits it. The upgrade subcommand short-circuits with
// a manual-restart hint when this returns true (CONTEXT.md D-08).
//
// Tests scope the var with `t.Setenv` to avoid leaking state between
// test cases.
func RunningInDaemon() bool {
	return os.Getenv(daemonEnvVar) == "1"
}
