package container

import (
	"os"
	"path/filepath"
)

// cacheDirEnv is the env var that, when set, overrides the cache root verbatim.
// Copied byte-for-byte from bench/ragindex so the bench image cache mirrors the
// rag-index $HELIX_CACHE_DIR convention exactly (CONTAINER-02).
const cacheDirEnv = "HELIX_CACHE_DIR"

// imagesSubdir is the fixed segment under the cache root that holds per-digest
// image caches: <cacheDir>/bench-images/<sha>/.
const imagesSubdir = "bench-images"

// cacheOKMarker is the sentinel file written into a published cache dir to mark
// a successful, fully-verified fill. Its presence is the cache-hit signal: a
// dir without the marker is treated as cold (and re-filled), so a crash between
// rename and a future write can never be mistaken for a complete cache.
const cacheOKMarker = ".container-cache-ok"

// cacheDir resolves the cache root with a three-step precedence. This function
// is copied VERBATIM from bench/ragindex/cache.go so the image cache and the
// rag-index cache share an identical root convention:
//
//  1. $HELIX_CACHE_DIR (verbatim) if set
//  2. os.UserCacheDir()/helix   (~/.cache/helix on Linux, ~/Library/Caches/helix on macOS)
//  3. ~/.helix/cache            (last-resort fallback)
func cacheDir() string {
	if d := os.Getenv(cacheDirEnv); d != "" {
		return d
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "helix")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".helix", "cache")
}

// ImagePath returns the on-disk cache path for a digest-pinned image:
// <cacheDir>/bench-images/<sha>/. sha MUST be a bare lowercase hex sha256 (64
// chars, no "sha256:" prefix); ImagePath fail-closes with errBadDigest BEFORE
// any filepath.Join, so a crafted key ("../etc", "sha256:..") cannot escape the
// bench-images/ root (T-84-02-01 tampering mitigation, mirroring ragindex
// T-83-01-03). The guard is load-bearing because, unlike ragindex's computed
// corpus sha, this sha is caller-supplied.
func ImagePath(sha string) (string, error) {
	if !isHexSHA256(sha) {
		return "", errBadDigest
	}
	return filepath.Join(cacheDir(), imagesSubdir, sha), nil
}

// Ensure resolves the cache dir for sha and fills it on a miss via verified-
// then-rename. If the final dir already carries the sentinel marker, Ensure
// returns hit=true without calling fetch. Otherwise it fetches into a sibling
// temp staging dir, writes the sentinel on success, and os.Renames the staging
// dir to the final path atomically — so a half-filled or unverified dir is
// never visible at bench-images/<sha>/ (T-84-02-02 / Pitfall 3 TOCTOU). The
// caller's fetch closure does the actual verify-then-pull INTO stageDir, which
// keeps Ensure independent of the pull/verify machinery and hermetically
// testable.
func Ensure(sha string, fetch func(stageDir string) error) (hit bool, dir string, err error) {
	finalDir, perr := ImagePath(sha)
	if perr != nil {
		return false, "", perr
	}

	if _, statErr := os.Stat(filepath.Join(finalDir, cacheOKMarker)); statErr == nil {
		return true, finalDir, nil
	}

	imagesRoot := filepath.Join(cacheDir(), imagesSubdir)
	if mkErr := os.MkdirAll(imagesRoot, 0o755); mkErr != nil {
		return false, "", mkErr
	}

	stageDir, tmpErr := os.MkdirTemp(imagesRoot, ".staging-"+sha+"-")
	if tmpErr != nil {
		return false, "", tmpErr
	}

	if fetchErr := fetch(stageDir); fetchErr != nil {
		os.RemoveAll(stageDir)
		return false, "", fetchErr
	}

	if markErr := os.WriteFile(filepath.Join(stageDir, cacheOKMarker), nil, 0o644); markErr != nil {
		os.RemoveAll(stageDir)
		return false, "", markErr
	}

	if renErr := os.Rename(stageDir, finalDir); renErr != nil {
		os.RemoveAll(stageDir)
		return false, "", renErr
	}

	return false, finalDir, nil
}
