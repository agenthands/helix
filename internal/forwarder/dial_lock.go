package forwarder

import (
	"context"
	"fmt"

	"github.com/gofrs/flock"
)

// daemonLocker is the minimal advisory-lock surface the startup guard needs.
// Production uses a gofrs/flock-backed implementation (portable: Unix flock +
// Windows LockFileEx, no CGO — RESEARCH Pitfall 6). The synctest race test
// injects an in-process mutex implementation because a real OS file lock would
// durably-block synctest's virtual clock (RESEARCH Pitfall 4).
type daemonLocker interface {
	// TryLock attempts to acquire the lock without blocking. It reports whether
	// the lock was acquired.
	TryLock() (bool, error)
	// Lock blocks until the lock is acquired.
	Lock() error
	// Unlock releases the lock.
	Unlock() error
}

// flockLocker adapts *flock.Flock to daemonLocker.
type flockLocker struct{ fl *flock.Flock }

func (l *flockLocker) TryLock() (bool, error) { return l.fl.TryLock() }
func (l *flockLocker) Lock() error            { return l.fl.Lock() }
func (l *flockLocker) Unlock() error          { return l.fl.Unlock() }

// lockfilePath derives the per-socket startup lockfile path. It is a plain
// regular file that is never bind()ed, so it is NOT subject to the AF_UNIX
// sun_path length limit that constrains the socket itself (RESEARCH Pitfall 2).
// The file lives in the existing per-uid 0700 socket dir (daemon/socket.go
// createSocketDir), so it inherits those permissions — no new permission code
// (threat T-90-01).
func lockfilePath(socketPath string) string {
	return socketPath + ".lock"
}

// seams bundles the injectable dependencies of the startup guard so the
// lock/double-check ALGORITHM can be unit-tested deterministically under
// testing/synctest without real exec, real sleep, or a real OS file lock.
type seams struct {
	// newLocker builds a per-socket advisory locker for the given lockfile path.
	newLocker func(lockfilePath string) daemonLocker
	// connect probes for a reachable daemon at socketPath; nil error == reachable.
	connect func(socketPath string) error
	// spawn starts a daemon process for socketPath.
	spawn func(socketPath string) error
	// waitUp blocks until the just-spawned daemon at socketPath is reachable,
	// returning nil once connectable (or an error on timeout).
	waitUp func(socketPath string) error
}

// startupGuard runs the race-free spawn algorithm (CLI-03):
//
//  1. Pre-lock probe: if the daemon is already reachable, return (warm reuse).
//     (In production the caller does this fast-path probe before invoking the
//     guard; the guard repeats it harmlessly for the locked path's correctness.)
//  2. Acquire the per-socket startup lock. TryLock first; if another caller is
//     mid-spawn (lock held), block on Lock() until it releases (its daemon is up),
//     then probe and return on success (lock-loser reuse).
//  3. Double-checked probe AFTER acquiring the lock: a peer may have finished
//     between the pre-lock probe and lock acquisition (TOCTOU — RESEARCH Pitfall 1),
//     so re-probe and return on success WITHOUT spawning.
//  4. Only if still unreachable, spawn the daemon and wait for it to come up.
//
// The lock is per-socket (RESEARCH Open-Q 2) so distinct sockets — e.g. bench
// per-cell sockets — do not serialize. The daemon's own net.Listen("unix") bind
// remains the defense-in-depth backstop and is not touched here.
func startupGuard(ctx context.Context, socketPath string, s seams) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	locker := s.newLocker(lockfilePath(socketPath))

	// WR-04: distinguish a genuine TryLock error (lockfile dir gone, EACCES, I/O
	// error from gofrs/flock) from ordinary contention. A real error must be
	// surfaced, not silently reclassified as "lock held by a peer" — otherwise the
	// caller falls into a blocking Lock() (or hangs) and the root cause is lost.
	locked, err := locker.TryLock()
	if err != nil {
		return fmt.Errorf("startup lock tryLock: %w", err)
	}
	if !locked {
		// A peer holds the startup lock and is spawning. Block until it releases
		// (its daemon's socket is up by then), then reuse via the post-lock probe.
		if err := locker.Lock(); err != nil {
			return fmt.Errorf("startup lock: %w", err)
		}
	}
	defer func() { _ = locker.Unlock() }()

	// Double-checked locking: whether we won TryLock or waited on Lock, a peer may
	// have brought the daemon up in the meantime. Re-probe before spawning.
	if err := s.connect(socketPath); err == nil {
		return nil
	}

	// Still no daemon — we own the lock, so we are the single spawner.
	if err := s.spawn(socketPath); err != nil {
		return err
	}
	return s.waitUp(socketPath)
}
