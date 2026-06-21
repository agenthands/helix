package swebenchutboost

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cacheDirEnv overrides the cache root verbatim when set (mirrors
// crosscodeeval/fetch.go EXACTLY so the adapter shares the operator's one
// HELIX_CACHE_DIR convention).
const cacheDirEnv = "HELIX_CACHE_DIR"

// fetchSubdir is the fixed segment under the cache root that holds the pinned
// UTBoost augmented-suite cache: <cacheDir>/swebench-utboost/<rev>/<file>.
const fetchSubdir = "swebench-utboost"

// maxDatasetBytes caps a single downloaded UTBoost payload (T-87-04): the HF
// bytes are untrusted, so io.LimitReader bounds the body to defend against an
// oversized / zip-bomb payload OOM-ing the loader. 256 MiB comfortably covers
// the augmented Verified suite while refusing a hostile multi-GB body.
const maxDatasetBytes = 256 << 20

// fetchTimeout is the explicit per-fetch deadline threaded through
// http.NewRequestWithContext so a stalled HF connection cannot hang the loader
// (defense in depth alongside the caller's own ctx).
const fetchTimeout = 120 * time.Second

// cacheDir resolves the cache root with the SAME three-step precedence as
// crosscodeeval/fetch.go (HELIX_CACHE_DIR → os.UserCacheDir()/helix →
// ~/.helix/cache), so the UTBoost cache lands beside the rag-index +
// aider-polyglot + CCE caches.
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

// validatePathSegment rejects names that could escape a join root once they
// become a path segment — a clone of crosscodeeval.validatePathSegment (T-87-04),
// kept in this leaf package so it carries no bench/runtime import. It rejects "",
// anything filepath.Clean rewrites ("..", "a//b"), any embedded separator, and a
// leading dot. MUST run BEFORE any filepath.Join.
func validatePathSegment(name, kind string) error {
	if name == "" {
		return fmt.Errorf("swebench-utboost: %s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("swebench-utboost: %s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

// cachePath returns the on-disk cache path for the augmented suite file at rev:
// <cacheDir>/swebench-utboost/<rev>/<file>. BOTH rev and file are
// validatePathSegment-checked before any filepath.Join so the joined path cannot
// escape the cache root (T-87-04) — a traversal or separator segment is rejected
// up front.
func cachePath(rev, file string) (string, error) {
	if err := validatePathSegment(rev, "rev"); err != nil {
		return "", err
	}
	if err := validatePathSegment(file, "file"); err != nil {
		return "", err
	}
	return filepath.Join(cacheDir(), fetchSubdir, rev, file), nil
}

// resolveURL builds the HF resolve URL for the augmented suite file at rev from
// the pinned Host + DatasetID constants and the VALIDATED rev + file ONLY — never
// a caller-supplied URL (T-87-04 SSRF). The rev must be an immutable 40-hex
// commit (isHexSHA1) and the host must be a valid https origin (isValidHTTPSHost,
// rejects a leading '-' / a non-https downgrade). The file path segment is
// validatePathSegment-checked. The result is
// https://huggingface.co/datasets/<DatasetID>/resolve/<rev>/<file>.
func resolveURL(rev, file string) (string, error) {
	if !isValidHTTPSHost(Host) {
		return "", fmt.Errorf("swebench-utboost: pinned host %q is not a valid https origin", Host)
	}
	if !isHexSHA1(rev) {
		return "", fmt.Errorf("swebench-utboost: rev %q is not a 40-hex immutable commit (refuse mutable ref)", rev)
	}
	if err := validatePathSegment(file, "file"); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/datasets/%s/resolve/%s/%s", Host, DatasetID, rev, file), nil
}

// Fetch downloads (or reuses the cached) UTBoost augmented-suite file at rev,
// returning the raw bytes. The URL is built ONLY from the pinned Host+DatasetID
// constants + the validated rev+file (resolveURL, SSRF-safe T-87-04); the body is
// size-capped via io.LimitReader (T-87-04); the rev and file are
// validatePathSegment-checked before the cache filepath.Join. It is the LIVE leg:
// the network-gated TestLiveFetch drives it. On a cache hit the cached bytes are
// returned with no network. A non-hex rev is refused BEFORE any network call.
func Fetch(ctx context.Context, rev, file string) ([]byte, error) {
	// Refuse a mutable ref up front, before touching the cache OR the network
	// (T-87-03 / Pitfall 5) — a caller cannot smuggle a branch/tag in.
	if !isHexSHA1(rev) {
		return nil, fmt.Errorf("swebench-utboost: rev %q is not a 40-hex immutable commit (refuse mutable ref)", rev)
	}
	dst, err := cachePath(rev, file)
	if err != nil {
		return nil, err
	}
	if b, err := readCacheCapped(dst); err == nil {
		return b, nil
	} else if !os.IsNotExist(err) {
		// A present-but-oversized (or otherwise unreadable) cache file must NOT
		// silently fall through to a re-fetch: the cap is a total invariant
		// (WR-02), so surface the refusal. Only a genuine cache MISS
		// (os.IsNotExist) proceeds to the network leg.
		return nil, err
	}
	url, err := resolveURL(rev, file)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("swebench-utboost: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("swebench-utboost: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("swebench-utboost: fetch %s: HTTP %d", url, resp.StatusCode)
	}

	// Cap the untrusted body. Read one byte past the cap to detect an oversized
	// payload (T-87-04) rather than silently truncating it.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDatasetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("swebench-utboost: read body %s: %w", url, err)
	}
	if int64(len(body)) > maxDatasetBytes {
		return nil, fmt.Errorf("swebench-utboost: payload %s exceeds %d-byte cap", url, int64(maxDatasetBytes))
	}

	if err := writeCacheAtomic(dst, body); err != nil {
		return nil, err
	}
	return body, nil
}

// readCacheCapped reads a cache-hit file under the SAME maxDatasetBytes cap the
// network leg enforces (WR-02), so a poisoned/oversized cache file (cosmic-ray
// growth, a future writer bug, or a manually planted file under the predictable
// <cacheDir>/swebench-utboost/<rev>/<file> path) is REFUSED rather than read
// unbounded into memory. A genuine cache miss returns an os.IsNotExist error so
// the caller falls through to the network fetch; an oversized file returns a cap
// error. It stat-checks first (cheap, catches the common case) and ALSO reads
// via io.LimitReader so a file whose size races the read still cannot exceed the
// cap.
func readCacheCapped(dst string) ([]byte, error) {
	fi, err := os.Stat(dst)
	if err != nil {
		return nil, err // os.IsNotExist(err) ⇒ genuine miss; other errors surface.
	}
	if fi.Size() > maxDatasetBytes {
		return nil, fmt.Errorf("swebench-utboost: cache file %s is %d bytes, exceeds %d-byte cap (refuse poisoned/oversized cache)", dst, fi.Size(), int64(maxDatasetBytes))
	}
	f, err := os.Open(dst)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Read one byte past the cap to detect a file that grew between the stat and
	// the read (TOCTOU), symmetric with the network body's +1 overflow probe.
	body, err := io.ReadAll(io.LimitReader(f, maxDatasetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("swebench-utboost: read cache %s: %w", dst, err)
	}
	if int64(len(body)) > maxDatasetBytes {
		return nil, fmt.Errorf("swebench-utboost: cache file %s exceeds %d-byte cap (refuse poisoned/oversized cache)", dst, int64(maxDatasetBytes))
	}
	return body, nil
}

// writeCacheAtomic writes the fetched payload to a temp file in the SAME
// directory as dst and atomically renames it into place (mirrors crosscodeeval/
// fetch.go writeCacheAtomic). A non-atomic write interrupted mid-flight would
// leave a truncated cache file that os.ReadFile then serves forever as a "hit";
// the temp+rename makes a partial write never become a cache hit.
func writeCacheAtomic(dst string, body []byte) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("swebench-utboost: mkdir cache: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(dst)+"-*")
	if err != nil {
		return fmt.Errorf("swebench-utboost: create temp cache: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("swebench-utboost: write temp cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("swebench-utboost: close temp cache: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("swebench-utboost: rename temp cache into %s: %w", dst, err)
	}
	return nil
}
