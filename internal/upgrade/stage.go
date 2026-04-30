package upgrade

import (
	"os"
	"path/filepath"

	serr "github.com/agenthands/helix/internal/errors"
)

// stageDirName is the directory leaf used for upgrade staging. Always a
// sibling of the install path so os.Rename(stage/helix, install/helix)
// is guaranteed to be same-filesystem and therefore atomic on POSIX
// systems. See RESEARCH.md Pitfall 3 for the full rationale.
const stageDirName = ".helix-upgrade-stage"

// NewStageDir creates a stage directory as a sibling of installPath and
// returns its path plus a cleanup function. The cleanup function is
// idempotent and best-effort (multiple calls are safe; a failure to
// remove the dir is swallowed because cleanup runs in defer paths and
// must not propagate).
//
// The stage dir is intentionally a sibling of the install path, NOT
// os.TempDir(): /tmp is often on tmpfs while /usr/local/bin lives on
// the root fs, and os.Rename across filesystems falls back to a
// non-atomic copy + unlink (Pitfall 3). The permission probe (D-09)
// already guarantees the install directory is writable, so the stage
// dir is automatically writable.
//
// On signature-verification failure the orchestrator deliberately does
// NOT call cleanup — the stage dir stays on disk for postmortem
// inspection (VALIDATION.md State row).
func NewStageDir(installPath string) (string, func(), error) {
	dir := filepath.Join(filepath.Dir(installPath), stageDirName)
	// Best-effort clean of any stale stage from a previous failed upgrade.
	// This is safe because the dir name is deterministic and Helix-owned.
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, serr.Wrap(serr.Internal, "creating stage dir", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	return dir, cleanup, nil
}
