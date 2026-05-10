// Phase 63 P63-02 Task 2 / COMPACT-05 — kill-mid-compact subprocess fixture.
//
// This program opens the workspace's DuckDB store (relative path
// ".helix/semantic.duckdb"), executes the compactor's single-tx
// sequence (Begin → WriteSnapshotFacts → ClearOverlayLE →
// DeleteSnapshotsBeyond) up to but NOT INCLUDING tx.Commit, writes a
// sentinel "ready" file so the parent test knows the program is mid-tx,
// and then blocks on stdin until SIGKILL.
//
// Because Commit is never reached, when the parent SIGKILLs this
// process DuckDB's ACID rollback restores:
//   - the pending semantic_snapshots row (vanishes),
//   - the deleted overlay rows (re-appear),
//   - the deleted prior snapshots (re-appear).
//
// Verifying that recovery is the test's whole point.
//
// This file lives under testdata/cmd/ so `go build ./...` skips it (the
// `testdata/` directory is excluded from default package discovery).
// The parent test compiles it explicitly via `go build -o`.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/store"
)

func main() {
	wsDir := flag.String("wsdir", "", "workspace directory (must contain .helix/semantic.duckdb)")
	repo := flag.String("repo", "kill-test", "repo id")
	capturedEpoch := flag.Uint64("captured", 0, "captured overlay epoch for ClearOverlayLE")
	retain := flag.Int("retain", 1, "DeleteSnapshotsBeyond retain")
	ready := flag.String("ready", "", "path to sentinel file to touch when mid-tx")
	flag.Parse()

	if *wsDir == "" || *ready == "" {
		fmt.Fprintln(os.Stderr, "compact_one: -wsdir and -ready required")
		os.Exit(2)
	}

	if err := os.Chdir(*wsDir); err != nil {
		fmt.Fprintf(os.Stderr, "compact_one: chdir %s: %v\n", *wsDir, err)
		os.Exit(2)
	}

	ctx := context.Background()
	cfg := semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	provider := obs.Noop(logger.Handler())

	s, err := store.Open(ctx, cfg, logger, provider.Metrics())
	if err != nil {
		fmt.Fprintf(os.Stderr, "compact_one: store.Open: %v\n", err)
		os.Exit(2)
	}
	// NOTE: intentionally NOT closing s on the kill path — the SIGKILL
	// terminates the process while the tx is open, which is the whole
	// point. DuckDB's WAL/ACID rollback restores prior state on the
	// next Open in the parent.

	// Mirror the production compactor's single-tx sequence
	// (compactor.go:275-318): Begin → Write({}) → ClearOverlayLE →
	// DeleteSnapshotsBeyond, but DO NOT Commit.
	snap, err := s.BeginSnapshot(ctx, store.SnapshotMeta{
		RepoID:        *repo,
		CapturedEpoch: *capturedEpoch,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "compact_one: BeginSnapshot: %v\n", err)
		os.Exit(2)
	}

	if err := s.WriteSnapshotFacts(ctx, snap, store.Facts{}); err != nil {
		fmt.Fprintf(os.Stderr, "compact_one: WriteSnapshotFacts: %v\n", err)
		os.Exit(2)
	}

	if err := snap.ClearOverlayLE(ctx, *repo, *capturedEpoch); err != nil {
		fmt.Fprintf(os.Stderr, "compact_one: ClearOverlayLE: %v\n", err)
		os.Exit(2)
	}

	if err := snap.DeleteSnapshotsBeyond(ctx, *retain); err != nil {
		fmt.Fprintf(os.Stderr, "compact_one: DeleteSnapshotsBeyond: %v\n", err)
		os.Exit(2)
	}

	// At this point the tx holds:
	//   - pending insert into semantic_snapshots,
	//   - deletes against semantic_live_overlay_*,
	//   - deletes against prior committed semantic_snapshots + cascade.
	// None of it is committed yet. Touch the sentinel to signal "mid-tx".
	f, err := os.Create(*ready)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compact_one: create ready: %v\n", err)
		os.Exit(2)
	}
	_ = f.Close()

	// Block until the parent SIGKILLs us. A long sleep is fine; the
	// parent's test timeout is the upper bound.
	time.Sleep(10 * time.Minute)
	// If we somehow get here, exit non-zero so the parent notices.
	os.Exit(3)
}
