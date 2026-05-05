package watcher

import (
	"errors"
	"syscall"
)

// ErrInotifyENOSPC is returned by newWorkspaceWatcher when fsnotify.
// NewWatcher() itself fails with ENOSPC (Linux inotify_init1 watch-
// limit exhaustion at construction time, distinct from the more
// common fsnotify.Add() ENOSPC during recursive walk).
//
// Callers (the Manager) MAY surface this as an early degraded-mode
// signal; the existing addRecursive() ENOSPC path handles the more
// frequent case of the user filling fs.inotify.max_user_watches mid-
// walk.
var ErrInotifyENOSPC = errors.New("inotify ENOSPC: fs.inotify.max_user_watches exhausted")

// isENOSPC matches the unwrapped syscall error fsnotify v1.9.0 returns
// from inotify_add_watch (see 60-RESEARCH.md Pitfall 2 — the upstream
// backend_inotify.go returns the syscall.Errno directly without
// wrapping). errors.Is correctly unwraps both bare syscall.ENOSPC and
// any caller-wrapping with %w.
func isENOSPC(err error) bool {
	return errors.Is(err, syscall.ENOSPC)
}
