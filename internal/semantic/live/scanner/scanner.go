package scanner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/workspace"
)

// Producer is the consumer side of the manifest scan: a single
// `OnWorkspaceChanged` call per scan cycle that surfaces a non-empty
// changed-paths slice. In production this is `*service.Service` from the
// 60-04 spine; tests inject a recording stub. Producer mirrors the
// `live.OnWorkspaceChanged` shape exactly so the daemon (60-05B) can wire
// the live service in without an adapter.
type Producer interface {
	OnWorkspaceChanged(ctx context.Context, sig live.WorkspaceChangeSignal) error
}

// FileHashLookup is the read-side store API the scanner consults to learn
// the snapshot's known file set for diff. Returns map[absPath]contentHash
// for the given repoID (snapshot only — overlay rows are layered on by
// the consumer side, the scanner doesn't need overlay-merged state at
// scan time).
//
// The interface is declared here (not in `internal/semantic/live`) so the
// scanner can be wired without dragging the live package's classifier
// FileHashLookup (which has a single-path shape) — the scanner needs the
// whole-repo set to detect deletions.
type FileHashLookup interface {
	KnownFiles(ctx context.Context, repoID semantic.RepoID) (map[string]string, error)
}

// Config tunes the scanner ticker. All fields default to safe values when
// zero; the daemon (60-05B) populates Interval from
// `cfg.SemanticIndex.LiveUpdates.ManifestScanInterval`.
type Config struct {
	// Interval bounds one full scan cycle. Default 10s (matches the
	// SPEC §25 default for `manifest_scan_interval`).
	Interval time.Duration

	// MaxParallelFiles caps the per-scan hashing fan-out. Default 4.
	// Reserved for a future tunable; the current impl is sequential.
	MaxParallelFiles int
}

// Scanner is a per-workspace ticker. Run blocks until ctx is cancelled.
// Construct via New; the Manager (manager.go) handles per-workspace
// goroutine ownership.
type Scanner struct {
	ws       workspace.WorkspaceKey
	repoID   semantic.RepoID
	producer Producer
	lookup   FileHashLookup
	cfg      Config
	logger   *slog.Logger

	// hasher is overridable for tests. Production uses HashFile (xxhash64).
	hasher func(absPath string) (string, error)

	// now is overridable for tests. Production uses time.Now.
	now func() time.Time
}

// New constructs a Scanner. logger may be nil (slog.Default is substituted).
// All other arguments are required; passing a nil producer or lookup will
// panic at the first scanOnce call.
func New(ws workspace.WorkspaceKey, repoID semantic.RepoID, p Producer, l FileHashLookup, cfg Config, logger *slog.Logger) *Scanner {
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.MaxParallelFiles <= 0 {
		cfg.MaxParallelFiles = 4
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scanner{
		ws:       ws,
		repoID:   repoID,
		producer: p,
		lookup:   l,
		cfg:      cfg,
		logger:   logger,
		hasher:   HashFile,
		now:      time.Now,
	}
}

// Run drains until ctx is cancelled. Performs an immediate first scan so
// tests do not have to wait Interval before observing a missed event,
// then ticks at cfg.Interval.
//
// Returns ctx.Err() on cancellation; never returns nil.
func (s *Scanner) Run(ctx context.Context) error {
	s.scanOnce(ctx)
	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.scanOnce(ctx)
		}
	}
}

// scanOnce performs one diff cycle: walk + hash + compare + emit. Per-file
// errors are logged at warn level and skipped; a top-level lookup error
// short-circuits the cycle (next tick will retry). Empty diff sets do
// NOT call the producer — this preserves the 60-04 acceptance #8 invariant
// that no-op flushes do not advance overlay_epoch.
func (s *Scanner) scanOnce(ctx context.Context) {
	known, err := s.lookup.KnownFiles(ctx, s.repoID)
	if err != nil {
		s.logger.Warn("scanner: KnownFiles failed",
			"repo_id", s.repoID, "err", err)
		return
	}
	seen := make(map[string]bool, len(known))
	var changed []string
	var mu sync.Mutex

	if walkErr := Walk(s.ws.RepoRoot, func(absPath string) error {
		// Bail mid-walk on cancellation — large workspaces should not
		// block the goroutine teardown.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		seen[absPath] = true
		hash, err := s.hasher(absPath)
		if err != nil {
			// Transient read errors (file deleted between WalkDir and
			// open) — skip the entry. Other walk entries are unaffected.
			return nil
		}
		prev, ok := known[absPath]
		if !ok || prev != hash {
			mu.Lock()
			changed = append(changed, absPath)
			mu.Unlock()
		}
		return nil
	}); walkErr != nil && ctx.Err() == nil {
		// Real walk error (not cancellation) — log and stop this cycle.
		s.logger.Warn("scanner: Walk failed",
			"repo_id", s.repoID, "root", s.ws.RepoRoot, "err", walkErr)
		return
	}

	// Detect deletions: paths present in the known set but absent on disk.
	for path := range known {
		if !seen[path] {
			changed = append(changed, path)
		}
	}
	if len(changed) == 0 {
		return
	}
	if err := s.producer.OnWorkspaceChanged(ctx, live.WorkspaceChangeSignal{
		WorkspaceID: s.ws,
		Paths:       changed,
		Source:      live.ChangeSourceManifestScan,
		ObservedAt:  s.now(),
	}); err != nil {
		// Service.OnWorkspaceChanged is fire-and-forget at the kernel
		// boundary, but a real producer error here is worth surfacing.
		s.logger.Warn("scanner: producer error",
			"repo_id", s.repoID, "err", err)
	}
}

// HashFile computes the xxhash64 of the file at absPath, returning the
// 16-character lowercase-hex digest. Matches the Phase 59 stable-ID hash
// (D-05 invariant). Exported so the daemon (60-05B) can wire the same
// hasher into the live classifier without a separate import.
func HashFile(absPath string) (string, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := xxhash.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%016x", h.Sum64()), nil
}
