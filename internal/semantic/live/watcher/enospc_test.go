package watcher

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// TestIsENOSPC_DirectErrno pins the bare-syscall.ENOSPC match —
// fsnotify v1.9.0's backend_inotify.go returns the unwrapped
// syscall.Errno from inotify_add_watch (60-RESEARCH.md Pitfall 2).
func TestIsENOSPC_DirectErrno(t *testing.T) {
	if !isENOSPC(syscall.ENOSPC) {
		t.Fatal("direct ENOSPC not matched by isENOSPC")
	}
}

// TestIsENOSPC_WrappedErrno pins the errors.Is unwrap behaviour —
// future fsnotify revs that wrap ENOSPC with %w must still be caught
// by the same helper without code change.
func TestIsENOSPC_WrappedErrno(t *testing.T) {
	wrapped := fmt.Errorf("inotify_add_watch /tmp: %w", syscall.ENOSPC)
	if !isENOSPC(wrapped) {
		t.Fatal("wrapped ENOSPC not matched via errors.Is")
	}
}

// TestIsENOSPC_UnrelatedErrno regression-guards against accidentally
// matching every syscall errno (e.g., EACCES looks superficially like
// "watch limit" in casual reading but is a permissions error).
func TestIsENOSPC_UnrelatedErrno(t *testing.T) {
	if isENOSPC(syscall.EACCES) {
		t.Fatal("EACCES was matched by isENOSPC; only ENOSPC should match")
	}
	if isENOSPC(errors.New("plain string error")) {
		t.Fatal("non-syscall error matched by isENOSPC")
	}
}

// TestENOSPCFallback_StatusFlips exercises markENOSPC directly: the
// status MUST flip to Active=false / Reason=inotify_enospc / non-empty
// remediation hint, and the slog.Warn MUST fire EXACTLY ONCE no
// matter how many times markENOSPC is invoked (the sync.Once guard
// per workspace per watcher lifetime — 60-CONTEXT.md acceptance #9).
func TestENOSPCFallback_StatusFlips(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	ww := &workspaceWatcher{logger: logger}
	ww.status.Store(WatcherStatus{Active: true, Reason: "running"})

	// Two consecutive markENOSPC calls — second MUST be a no-op.
	ww.markENOSPC()
	ww.markENOSPC()

	s := ww.Status()
	if s.Active {
		t.Fatalf("expected Active=false after ENOSPC; got %+v", s)
	}
	if s.Reason != "inotify_enospc" {
		t.Fatalf("Reason = %q, want %q", s.Reason, "inotify_enospc")
	}
	if !strings.Contains(s.RemediationHint, "fs.inotify.max_user_watches") {
		t.Fatalf("RemediationHint missing sysctl key: %q", s.RemediationHint)
	}

	// Acceptance #9: exactly one Warn line must reference the ENOSPC
	// fallback. Counting "fsnotify ENOSPC" occurrences in the captured
	// log buffer is the cheapest way to prove the sync.Once gate.
	got := strings.Count(buf.String(), "fsnotify ENOSPC")
	if got != 1 {
		t.Fatalf("expected exactly 1 ENOSPC Warn line, got %d; log=%q", got, buf.String())
	}
}

// TestENOSPCFallback_ConcurrentMarkOnceOnly proves the sync.Once
// guard is not just well-ordered single-threaded but also race-safe
// under concurrent invocation (which is the actual production
// failure mode — multiple recursive-add ENOSPCs racing the
// fsnotify.Errors channel ENOSPC).
func TestENOSPCFallback_ConcurrentMarkOnceOnly(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	ww := &workspaceWatcher{logger: logger}
	ww.status.Store(WatcherStatus{Active: true, Reason: "running"})

	const N = 64
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			ww.markENOSPC()
		}()
	}
	wg.Wait()

	if got := strings.Count(buf.String(), "fsnotify ENOSPC"); got != 1 {
		t.Fatalf("concurrent markENOSPC: want 1 Warn, got %d", got)
	}
	if ww.Status().Reason != "inotify_enospc" {
		t.Fatalf("status Reason did not flip: %+v", ww.Status())
	}
}
