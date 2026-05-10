// Phase 63 P63-02 Task 2 / COMPACT-05: kill-mid-compact subprocess test.
//
// Proves DuckDB ACID rollback restores ALL of the compactor's single-tx
// state (snapshot pending row + overlay deletes + retention deletes)
// when the process is SIGKILLed mid-tx — i.e., before tx.Commit
// returns. This is the explicit GREEN evidence for the COMPACT-05
// "kill-mid-compact" requirement that backs threat T-63-02-02
// (retention atomicity).
//
// Strategy:
//
//  1. Build the subprocess fixture (testdata/cmd/compact_one) into a
//     tmpdir-rooted binary.
//
//  2. In the parent: open a fresh store at WS/.helix/semantic.duckdb,
//     seed N overlay rows (real BeginOverlayTx) and M prior committed
//     snapshots (real BeginSnapshot+CommitSnapshot). Record
//     before-kill counts. Close the store cleanly so the subprocess
//     can open the same file.
//
//  3. Launch the fixture pointed at the same WS dir; wait for it to
//     touch a sentinel file ("ready") which signals it has reached the
//     mid-tx point (Begin + Write + ClearOverlayLE + DeleteSnapshotsBeyond
//     all done, Commit NOT called). retain=1 + a captured epoch of 1<<60
//     forces ClearOverlayLE to mark every seeded overlay row for
//     deletion AND DeleteSnapshotsBeyond to mark every seeded prior
//     snapshot for deletion — so rollback has the maximum amount of
//     state to restore.
//
//  4. SIGKILL the subprocess (no graceful shutdown — DuckDB cannot run
//     any cleanup hooks).
//
//  5. Re-open the same DB file in the parent. Assert:
//       a) overlay row count == before-kill (rows restored),
//       b) LatestCommittedSnapshot still resolves to the highest seeded
//          prior id (retention DELETE rolled back; killed tx never
//          committed).
//
// If DuckDB's ACID guarantees fail under SIGKILL the test fails with
// concrete diffs. If the production code shape changes such that
// Commit happens before the sentinel write, that's a real bug — the
// test escalates rather than weakening assertions.

//go:build !windows

package compact_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/store"
)

func TestCompactor_KillMidCompact(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGKILL semantics not portable to Windows")
	}

	wsDir := t.TempDir()

	// Build the fixture binary. We compile it from its source path so
	// it picks up the current module. The testdata/ subtree is excluded
	// from `go build ./...` package discovery.
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "compact_one")
	srcDir := filepath.Join("testdata", "cmd", "compact_one")
	buildCmd := exec.Command("go", "build", "-o", bin, "./"+srcDir)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, string(out))
	}

	const (
		repo            = "kill-test"
		nOverlayRows    = 7
		nPriorSnapshots = 4
	)

	// Seed the DB in the parent process. Must run in wsDir so the
	// workspace-relative store path resolves.
	prevDir := changeWD(t, wsDir)
	priorIDs := seedKillFixture(t, repo, nOverlayRows, nPriorSnapshots)
	prevDir() // restore parent's cwd before launching subprocess

	if len(priorIDs) != nPriorSnapshots {
		t.Fatalf("seed sanity: priorIDs len = %d, want %d", len(priorIDs), nPriorSnapshots)
	}
	highestPrior := priorIDs[len(priorIDs)-1]

	// Sanity: re-open and capture before-kill counts.
	beforeOverlay, beforeLatest := openAndProbeKill(t, wsDir, repo)
	if beforeOverlay != nOverlayRows {
		t.Fatalf("seed sanity: overlay rows = %d, want %d", beforeOverlay, nOverlayRows)
	}
	if beforeLatest != highestPrior {
		t.Fatalf("seed sanity: LatestCommittedSnapshot = %d, want %d", beforeLatest, highestPrior)
	}

	// Spawn the fixture. captured = 1<<60 ⇒ ClearOverlayLE will mark
	// EVERY seeded overlay row for deletion. retain=1 ⇒
	// DeleteSnapshotsBeyond will mark EVERY prior committed snapshot
	// for deletion (the only "kept" slot is the not-yet-committed
	// pending one, which the killed tx will never commit). Both
	// DELETE sets are the maximum possible, so rollback has the most
	// to restore.
	readyPath := filepath.Join(wsDir, "ready.flag")
	cmd := exec.Command(bin,
		"-wsdir", wsDir,
		"-repo", repo,
		"-captured", "1152921504606846976", // 1<<60
		"-retain", "1",
		"-ready", readyPath,
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start fixture: %v", err)
	}
	// Always reap the subprocess.
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// Wait for the fixture to touch the sentinel.
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if exited, _ := processExited(cmd); exited {
			t.Fatalf("fixture exited before sentinel was written; check stderr above")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err := os.Stat(readyPath); err != nil {
		t.Fatalf("timed out waiting for ready sentinel: %v", err)
	}

	// SIGKILL — no clean shutdown, no DuckDB driver hooks, no chance
	// to commit. This is the guarantee being validated.
	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL: %v", err)
	}
	state, waitErr := cmd.Process.Wait()
	if waitErr != nil {
		t.Logf("Wait after SIGKILL (expected for killed process): %v", waitErr)
	}
	if state != nil && state.ExitCode() == 0 {
		t.Fatalf("fixture exited cleanly with 0 — it should have been killed mid-tx")
	}

	// Reopen the DB and assert ACID rollback restored everything.
	afterOverlay, afterLatest := openAndProbeKill(t, wsDir, repo)

	if afterOverlay != beforeOverlay {
		t.Errorf("overlay rows after SIGKILL: got %d, want %d (ClearOverlayLE rollback should have restored them)",
			afterOverlay, beforeOverlay)
	}
	if afterLatest != highestPrior {
		t.Errorf("LatestCommittedSnapshot after SIGKILL: got %d, want %d (retention DELETE rollback should have restored every prior snapshot; killed tx must not have committed)",
			afterLatest, highestPrior)
	}
}

// processExited returns true if cmd has already terminated.
func processExited(cmd *exec.Cmd) (bool, error) {
	if cmd.ProcessState != nil {
		return true, nil
	}
	if cmd.Process == nil {
		return true, nil
	}
	// Non-blocking poll: signal 0 tests liveness without delivering anything.
	err := cmd.Process.Signal(syscall.Signal(0))
	if err != nil {
		return true, err
	}
	return false, nil
}

// seedKillFixture seeds N overlay rows + M committed snapshots for repoID
// at WS/.helix/semantic.duckdb. Caller must already have chdir'd into wsDir.
// Returns the snapshot_ids of the M committed priors in insertion order.
func seedKillFixture(t *testing.T, repoID string, nOverlay, nSnapshots int) []uint64 {
	t.Helper()
	ctx := context.Background()
	cfg := killFixtureConfig()
	provider := obs.Noop(nil)
	s, err := store.Open(ctx, cfg, nil, provider.Metrics())
	if err != nil {
		t.Fatalf("seed: store.Open: %v", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Fatalf("seed: store.Close: %v", err)
		}
	}()

	for i := 0; i < nOverlay; i++ {
		tx, err := s.BeginOverlayTx(ctx, repoID)
		if err != nil {
			t.Fatalf("seed BeginOverlayTx %d: %v", i, err)
		}
		if err := tx.UpsertOverlayFile(ctx, killFilePath(i), "h"); err != nil {
			t.Fatalf("seed Upsert %d: %v", i, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("seed Commit overlay %d: %v", i, err)
		}
	}

	ids := make([]uint64, 0, nSnapshots)
	for i := 0; i < nSnapshots; i++ {
		snap, err := s.BeginSnapshot(ctx, store.SnapshotMeta{RepoID: repoID})
		if err != nil {
			t.Fatalf("seed BeginSnapshot %d: %v", i, err)
		}
		if err := s.CommitSnapshot(ctx, snap, store.SnapshotSummary{}); err != nil {
			t.Fatalf("seed CommitSnapshot %d: %v", i, err)
		}
		ids = append(ids, snap.ID)
		// Tiny sleep so created_at strictly monotonically increases
		// (`now()` is DB-side; sub-millisecond ties would break
		// LatestCommittedSnapshot ordering).
		time.Sleep(2 * time.Millisecond)
	}
	return ids
}

// openAndProbeKill opens the store at WS/.helix/semantic.duckdb and
// returns (overlay row count over all epochs, LatestCommittedSnapshot id).
// Does its own chdir+restore so it is safe to call from a parent test
// that does not want to permanently change cwd.
func openAndProbeKill(t *testing.T, wsDir, repoID string) (int, uint64) {
	t.Helper()
	ctx := context.Background()
	prevDir := changeWD(t, wsDir)
	defer prevDir()

	cfg := killFixtureConfig()
	provider := obs.Noop(nil)
	s, err := store.Open(ctx, cfg, nil, provider.Metrics())
	if err != nil {
		t.Fatalf("openAndProbeKill: store.Open: %v", err)
	}
	defer s.Close()

	overlayCount, err := s.OverlayRowCount(ctx, repoID, 1<<62)
	if err != nil {
		t.Fatalf("OverlayRowCount: %v", err)
	}

	latest, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		t.Fatalf("LatestCommittedSnapshot: %v", err)
	}
	return overlayCount, latest
}

func killFixtureConfig() semantic.Config {
	return semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
}

func killFilePath(i int) string {
	return "kill_seed_" + itoaKill(i) + ".go"
}

func itoaKill(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	n := i
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
