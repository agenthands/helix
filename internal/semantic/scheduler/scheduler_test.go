package scheduler_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/scheduler"
)

// newTestScheduler constructs a Scheduler with a nil registry — the Wave 2
// orchestration surface does not exercise the registry, and the daemon
// (P05) injects the real one.
func newTestScheduler(t *testing.T) *scheduler.Scheduler {
	t.Helper()
	return scheduler.NewScheduler(nil)
}

// TestScheduler_Idempotent verifies that two ScheduleInitialExtraction calls
// for the same workspace return the same JobID — the D-04 idempotency
// invariant.
func TestScheduler_Idempotent(t *testing.T) {
	s := newTestScheduler(t)
	ws := semantic.WorkspaceID("ws-1")

	a := s.ScheduleInitialExtraction(ws, scheduler.InitialExtraction{Reason: "workspace_activation"})
	b := s.ScheduleInitialExtraction(ws, scheduler.InitialExtraction{Reason: "workspace_activation"})

	if a == "" {
		t.Fatalf("expected non-empty JobID")
	}
	if a != b {
		t.Fatalf("expected idempotent JobID, got a=%q b=%q", a, b)
	}
}

// TestScheduler_OneJobPerWorkspace verifies that:
//   - Two different workspaces get two different JobIDs.
//   - The same workspace returns the FIRST call's JobID even when the second
//     call uses a different Reason (in-flight wins; D-04 idempotency).
func TestScheduler_OneJobPerWorkspace(t *testing.T) {
	s := newTestScheduler(t)

	a := s.ScheduleInitialExtraction(semantic.WorkspaceID("ws-a"), scheduler.InitialExtraction{Reason: "workspace_activation"})
	b := s.ScheduleInitialExtraction(semantic.WorkspaceID("ws-b"), scheduler.InitialExtraction{Reason: "workspace_activation"})
	if a == b {
		t.Fatalf("expected distinct JobIDs across workspaces, got %q == %q", a, b)
	}

	a2 := s.ScheduleInitialExtraction(semantic.WorkspaceID("ws-a"), scheduler.InitialExtraction{Reason: "require_ready_cold"})
	if a != a2 {
		t.Fatalf("expected first-wins idempotency for in-flight job, got %q != %q", a, a2)
	}
}

// TestScheduler_StateTransitions verifies the publish path:
//   - Initial state for an unknown workspace is SemanticNotStarted.
//   - After ScheduleInitialExtraction state moves to SemanticIndexing.
//   - A Subscribe channel receives the transition.
//
// Subscribers must be installed BEFORE ScheduleInitialExtraction to observe
// the indexing transition, mirroring the live wiring (callers Subscribe up
// front, then RequireReady kicks the scheduler).
func TestScheduler_StateTransitions(t *testing.T) {
	s := newTestScheduler(t)
	ws := semantic.WorkspaceID("ws-trans")

	st0 := s.Status(ws)
	if st0.State != scheduler.SemanticNotStarted {
		t.Fatalf("expected initial state %q, got %q", scheduler.SemanticNotStarted, st0.State)
	}

	sub := s.Subscribe(ws)
	_ = s.ScheduleInitialExtraction(ws, scheduler.InitialExtraction{Reason: "workspace_activation"})

	st1 := s.Status(ws)
	if st1.State != scheduler.SemanticIndexing {
		t.Fatalf("expected post-schedule state %q, got %q", scheduler.SemanticIndexing, st1.State)
	}

	select {
	case got := <-sub:
		if got.State != scheduler.SemanticIndexing {
			t.Fatalf("subscriber expected %q, got %q", scheduler.SemanticIndexing, got.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("subscriber timeout — expected SemanticIndexing transition")
	}
}

// TestScheduler_NoTimeSleepPolling is a static-analysis test: scan the
// scheduler package's non-test .go sources and refuse any occurrence of
// `time.Sleep` outside comments. D-04 acceptance #9 enforces channel-based
// waits only.
func TestScheduler_NoTimeSleepPolling(t *testing.T) {
	pkgDir := schedulerPackageDir(t)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read scheduler dir: %v", err)
	}
	var offenders []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || name == "export_test.go" {
			continue
		}
		full := filepath.Join(pkgDir, name)
		f, err := os.Open(full)
		if err != nil {
			t.Fatalf("open %s: %v", full, err)
		}
		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			// Strip a trailing line comment so `x // time.Sleep notes` doesn't trip.
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = line[:idx]
			}
			if strings.Contains(line, "time.Sleep") {
				offenders = append(offenders, name+":"+itoa(lineNo))
			}
		}
		_ = f.Close()
		if err := scanner.Err(); err != nil {
			t.Fatalf("scan %s: %v", full, err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("D-04 acceptance #9 violation: time.Sleep found in scheduler package non-test sources: %v", offenders)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// schedulerPackageDir resolves the on-disk directory of the scheduler
// package. We rely on `go test` running with CWD = the package directory,
// which is the standard go test invariant.
func schedulerPackageDir(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return cwd
}
