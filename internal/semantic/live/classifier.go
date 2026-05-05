package live

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/agenthands/helix/internal/semantic"
)

// FileHashLookup is the read-side store API the classifier consults to
// decide whether a path was previously known. The classifier MUST NOT
// import store directly to keep the dependency arrow one-way (semantic
// store wires the implementation in via the live.Service constructor).
//
// The (hash, ok, err) tuple distinguishes:
//   - (h, true, nil)     — path is known with content_hash == h
//   - ("", false, nil)   — path is unknown to the store
//   - ("", false, err)   — store lookup failed; classifier propagates
type FileHashLookup interface {
	EffectiveContentHash(ctx context.Context, repoID semantic.RepoID, path string) (string, bool, error)
}

// FileHasher computes the content hash for a path. xxhash64 in production
// (matches Phase 59 stable-ID hash, D-05 invariant).
//
// Phase 60 P04 does NOT call the hasher in the classifier — the EXECUTOR
// DECISION (60-04 PLAN RED 2 ALTERNATIVE) is to return ChangeFileModified
// regardless of hash equality and let the handler short-circuit if the
// hash hasn't changed. The hasher parameter is reserved for a future
// optimization that prunes touch-but-unchanged events at classify time.
type FileHasher func(absPath string) (string, error)

// ClassifyPathChange decides the SourceChangeKind for a single path. Single
// owner of kind decisions per 60-CONTEXT.md D-01; sources only emit paths.
//
// Decision table:
//
//	source=helix_edit                              → ChangeHelixEdit, true
//	path exists,  unknown to store                 → ChangeFileCreated, true
//	path exists,  known to store                   → ChangeFileModified, true
//	path missing, known to store                   → ChangeFileDeleted, true
//	path missing, unknown to store                 → no-op (false)
//	dangling symlink, unknown to store             → no-op (false) (T-60-04-03)
//
// The path-existence check uses os.Lstat (NOT os.Stat) so dangling symlinks
// are classified as missing rather than chased. This matches the existing
// repomap walker's symlink-rejection invariant and mitigates T-60-04-03
// (symlink traversal).
func ClassifyPathChange(
	ctx context.Context,
	repoID semantic.RepoID,
	absPath string,
	lookup FileHashLookup,
	_ FileHasher, // reserved for future hash-equality optimization
	source ChangeSource,
) (SourceChangeKind, bool, error) {
	if source == ChangeSourceHelixEdit {
		return ChangeHelixEdit, true, nil
	}

	info, statErr := os.Lstat(absPath)
	pathExists := statErr == nil && info != nil
	pathMissing := statErr != nil && errors.Is(statErr, fs.ErrNotExist)
	if !pathExists && !pathMissing {
		// some other stat error (EACCES etc.) — propagate
		return "", false, statErr
	}

	// Symlink rejection (T-60-04-03): treat any symlink as missing for
	// classification purposes. The watcher upstream (60-05A) is also
	// expected to filter out symlinks at enumeration time, but we
	// double-check here so a manifest scan that surfaces a symlink does
	// not silently bypass the policy.
	if pathExists && info.Mode()&os.ModeSymlink != 0 {
		pathExists = false
		pathMissing = true
	}

	_, known, err := lookup.EffectiveContentHash(ctx, repoID, absPath)
	if err != nil {
		return "", false, err
	}
	switch {
	case pathExists && !known:
		return ChangeFileCreated, true, nil
	case pathExists && known:
		return ChangeFileModified, true, nil
	case pathMissing && known:
		return ChangeFileDeleted, true, nil
	default:
		// path missing, never known → no-op
		return "", false, nil
	}
}
