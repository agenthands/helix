package forwarder

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inProcLocker is an in-process, mutex-backed daemonLocker used ONLY by the
// synctest race tests. It serializes goroutines deterministically under the
// virtual clock without touching a real OS file lock (real syscall.Flock would
// durably-block synctest, which only virtualizes Go-runtime blocking — see
// RESEARCH Pitfall 4). One *sync.Mutex is shared per lockfile path so distinct
// sockets get independent locks, mirroring per-inode OS flock semantics.
type inProcLocker struct {
	mu *sync.Mutex
}

func (l *inProcLocker) TryLock() (bool, error) { return l.mu.TryLock(), nil }
func (l *inProcLocker) Lock() error            { l.mu.Lock(); return nil }
func (l *inProcLocker) Unlock() error          { l.mu.Unlock(); return nil }

// newInProcLockerFactory returns a locker factory whose lockers share one
// *sync.Mutex per path, so calls on the same path serialize and calls on
// distinct paths do not.
func newInProcLockerFactory() func(path string) daemonLocker {
	var mu sync.Mutex
	mutexes := map[string]*sync.Mutex{}
	return func(path string) daemonLocker {
		mu.Lock()
		defer mu.Unlock()
		m, ok := mutexes[path]
		if !ok {
			m = &sync.Mutex{}
			mutexes[path] = m
		}
		return &inProcLocker{mu: m}
	}
}

// fakeDaemon models a daemon that becomes connectable only after spawnDaemon
// has been called for its socket. It records spawn invocations so the tests can
// assert exactly-one-spawn semantics.
type fakeDaemon struct {
	mu      sync.Mutex
	up      map[string]bool // socket -> daemon up (connectable)
	spawns  int64
	preWarm map[string]bool // socket -> already up before any spawn (warm reuse)
}

func newFakeDaemon() *fakeDaemon {
	return &fakeDaemon{up: map[string]bool{}, preWarm: map[string]bool{}}
}

// connect mirrors tryConnect's contract: nil error means "daemon reachable".
func (d *fakeDaemon) connect(socketPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.up[socketPath] || d.preWarm[socketPath] {
		return nil
	}
	return assert.AnError
}

// spawn mirrors startDaemon: it increments the spawn counter and (synchronously,
// as the real daemon's net.Listen would eventually) marks the socket up so that
// the post-spawn waitForDaemon-equivalent connect succeeds.
func (d *fakeDaemon) spawn(socketPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	atomic.AddInt64(&d.spawns, 1)
	d.up[socketPath] = true
	return nil
}

func (d *fakeDaemon) spawnCount() int64 { return atomic.LoadInt64(&d.spawns) }

// runGuard drives the lock/double-check algorithm via startupGuard with the
// injected seams. It returns nil on a successful (connected) outcome.
func runGuard(ctx context.Context, d *fakeDaemon, lf func(string) daemonLocker, socketPath string) error {
	return startupGuard(ctx, socketPath, seams{
		newLocker: lf,
		connect:   d.connect,
		spawn:     d.spawn,
		// waitUp mirrors waitForDaemon: once spawn marked the socket up, a single
		// connect probe succeeds. No real polling/sleep needed for the algorithm test.
		waitUp: d.connect,
	})
}

// TestRace_OneDaemon_Synctest pins the core CLI-03 guarantee: N concurrent cold
// callers against the SAME socket cause exactly one spawn. Virtual clock; no real
// exec, no real sleep, no real OS lock.
func TestRace_OneDaemon_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := newFakeDaemon()
		lf := newInProcLockerFactory()
		const N = 16
		const socket = "/tmp/helix-race/daemon.sock"

		var wg sync.WaitGroup
		errs := make([]error, N)
		for i := 0; i < N; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = runGuard(t.Context(), d, lf, socket)
			}(i)
		}
		wg.Wait()
		synctest.Wait()

		for i, err := range errs {
			require.NoError(t, err, "caller %d should end connected", i)
		}
		assert.Equal(t, int64(1), d.spawnCount(),
			"N concurrent cold callers must cause exactly one daemon spawn")
	})
}

// TestRace_LockLoserReuse_Synctest pins the lock-loser reuse path: a caller that
// loses TryLock blocks on Lock until the winner's daemon is up, then reuses it via
// tryConnect WITHOUT spawning a second daemon.
func TestRace_LockLoserReuse_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := newFakeDaemon()
		lf := newInProcLockerFactory()
		const socket = "/tmp/helix-loser/daemon.sock"

		// Winner takes the lock first and holds the goroutine scheduling so the
		// loser must wait on Lock(). With synctest's deterministic scheduler both
		// run; the assertion is on the spawn count, which is order-independent.
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.NoError(t, runGuard(t.Context(), d, lf, socket))
			}()
		}
		wg.Wait()
		synctest.Wait()

		assert.Equal(t, int64(1), d.spawnCount(),
			"lock loser must reuse the winner's daemon, not spawn a second")
	})
}

// TestRace_DoubleCheck_TOCTOU pins the double-checked connect: a caller whose
// pre-lock probe failed but whose post-lock probe succeeds (a peer finished
// between probe and lock) must NOT spawn.
func TestRace_DoubleCheck_TOCTOU(t *testing.T) {
	d := newFakeDaemon()
	lf := newInProcLockerFactory()
	const socket = "/tmp/helix-toctou/daemon.sock"

	// Simulate the TOCTOU window: the pre-lock probe fails, then a peer brings the
	// daemon up while this caller is acquiring the lock. We model that by injecting
	// a connect seam that fails once (pre-lock) then succeeds (post-lock double-check).
	var calls int64
	connect := func(socketPath string) error {
		if atomic.AddInt64(&calls, 1) == 1 {
			return assert.AnError // pre-lock probe miss
		}
		return nil // peer finished; double-check hits
	}
	err := startupGuard(context.Background(), socket, seams{
		newLocker: lf,
		connect:   connect,
		spawn:     d.spawn,
		waitUp:    d.connect,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), d.spawnCount(),
		"double-check must prevent the redundant spawn after a TOCTOU win by a peer")
}

// TestRace_PerSocketIndependent pins per-socket isolation: two callers on DISTINCT
// sockets each spawn once (the locks are independent — bench per-cell isolation).
func TestRace_PerSocketIndependent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := newFakeDaemon()
		lf := newInProcLockerFactory()
		sockets := []string{"/tmp/helix-a/daemon.sock", "/tmp/helix-b/daemon.sock"}

		var wg sync.WaitGroup
		for _, s := range sockets {
			wg.Add(1)
			go func(s string) {
				defer wg.Done()
				assert.NoError(t, runGuard(t.Context(), d, lf, s))
			}(s)
		}
		wg.Wait()
		synctest.Wait()

		assert.Equal(t, int64(2), d.spawnCount(),
			"distinct sockets must spawn independently (per-socket lock, no serialization)")
	})
}

// TestLockfilePath pins the lockfile derivation: <socketPath>.lock (a regular
// file, never bind()ed — RESEARCH Pitfall 2).
func TestLockfilePath(t *testing.T) {
	assert.Equal(t, "/tmp/helix-x/daemon.sock.lock", lockfilePath("/tmp/helix-x/daemon.sock"))
}

// guard against unused import when iterating
var _ = time.Second
