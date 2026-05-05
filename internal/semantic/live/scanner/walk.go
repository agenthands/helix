// Package scanner ships the periodic content-hash manifest scanner that
// catches missed fsnotify events (LIVE-03) and serves as the ENOSPC
// fallback (LIVE-04) when the watcher in 60-05A reports inotify exhaustion.
//
// The scanner walks the workspace root every interval, hashes each file
// with xxhash64 (matching the Phase 59 stable-ID hash invariant, D-05),
// compares against the known set returned by the FileHashLookup, and emits
// one `live.WorkspaceChangeSignal{Source: manifest_scan}` per scan cycle
// containing the union of (modified, created, deleted) paths.
//
// 60-CONTEXT.md D-05 invariant: this package does NOT import
// internal/repomap/* — the walker shape mirrors repomap's `walkAndExtract`
// but is net-new code. The skipDirs list is re-declared here so the
// kernel→semantic vet boundary (60-01) is preserved.
package scanner

import (
	"io/fs"
	"path/filepath"
)

// skipDirs mirrors repomap's exclusion list (`internal/skill/repomap/skill.go`)
// but is declared verbatim here to honor the 60-01 cascade — no import from
// internal/repomap is allowed inside internal/semantic/live/*.
var skipDirs = map[string]bool{
	".git":         true,
	".helix":       true,
	"__pycache__":  true,
	".venv":        true,
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"coverage":     true,
}

// Walk visits every regular file under root that is not below a skipped
// directory and is not a symlink. fn is called with the absolute path. A
// nil error from fn continues; a non-nil error stops the walk.
//
// Stat-level errors on individual entries (permission denied, mid-walk
// deletion) are silently skipped so a transient FS error never aborts a
// whole scan cycle.
//
// Symlink rejection (T-60-04-03 mirror): any DirEntry whose Type() reports
// fs.ModeSymlink is ignored. The classifier in `internal/semantic/live`
// uses os.Lstat for the same reason; this walker enforces the same policy
// at enumeration time so the classifier never sees a symlinked path.
func Walk(root string, fn func(absPath string) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		return fn(path)
	})
}
