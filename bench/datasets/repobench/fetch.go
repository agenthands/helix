package repobench

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// cacheDirEnv overrides the cache root verbatim when set (mirrors
// bench/ragindex/cache.go + bench/datasets/crosscodeeval/fetch.go EXACTLY so the
// adapter shares the operator's one HELIX_CACHE_DIR convention).
const cacheDirEnv = "HELIX_CACHE_DIR"

// fetchSubdir is the fixed segment under the cache root that holds the pinned
// RepoBench parquet cache: <cacheDir>/repobench/<rev>/<language>.parquet.
const fetchSubdir = "repobench"

// maxParquetBytes caps a single downloaded parquet payload (T-86-04-04): the HF
// bytes are untrusted, so io.LimitReader bounds the body to defend against an
// oversized / zip-bomb parquet OOM-ing the loader. 256 MiB comfortably covers a
// per-language RepoBench parquet while refusing a hostile multi-GB body.
const maxParquetBytes = 256 << 20

// fetchTimeout is the explicit per-fetch deadline threaded through
// http.NewRequestWithContext so a stalled HF connection cannot hang the loader
// (defense in depth alongside the caller's own ctx).
const fetchTimeout = 120 * time.Second

// cacheDir resolves the cache root with the SAME three-step precedence as
// bench/ragindex/cache.go (HELIX_CACHE_DIR → os.UserCacheDir()/helix →
// ~/.helix/cache), so the RepoBench parquet cache lands beside the rag-index,
// aider-polyglot, and crosscodeeval caches.
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

// cachePath returns the on-disk cache path for a language's parquet at rev:
// <cacheDir>/repobench/<rev>/<language>.parquet. BOTH rev and language are
// validatePathSegment-checked before any filepath.Join so the joined path cannot
// escape the cache root (T-86-04-03) — a traversal or separator segment is
// rejected up front.
func cachePath(rev, language string) (string, error) {
	if err := validatePathSegment(rev, "rev"); err != nil {
		return "", err
	}
	if err := validatePathSegment(language, "language"); err != nil {
		return "", err
	}
	return filepath.Join(cacheDir(), fetchSubdir, rev, language+".parquet"), nil
}

// resolveURL builds the HF resolve URL for a language's parquet at rev from the
// pinned Host + the per-language pinned Repo and the VALIDATED rev + language
// ONLY — never a caller-supplied URL (T-86-04-02 SSRF). The rev must be an
// immutable 40-hex commit (isHexSHA1) and the host must be a valid https origin
// (isValidHTTPSHost, rejects a leading '-' / a non-https downgrade). The language
// path segment is validatePathSegment-checked and must resolve to a pinned repo.
// The result is
// https://huggingface.co/datasets/<repo>/resolve/<rev>/<language>.parquet.
func resolveURL(rev, language string) (string, error) {
	if !isValidHTTPSHost(Host) {
		return "", fmt.Errorf("repobench: pinned host %q is not a valid https origin", Host)
	}
	if !isHexSHA1(rev) {
		return "", fmt.Errorf("repobench: rev %q is not a 40-hex immutable commit (refuse mutable ref)", rev)
	}
	if err := validatePathSegment(language, "language"); err != nil {
		return "", err
	}
	repo := PinnedRepo(language)
	if repo == "" {
		return "", fmt.Errorf("repobench: unsupported language %q", language)
	}
	return fmt.Sprintf("%s/datasets/%s/resolve/%s/%s.parquet", Host, repo, rev, language), nil
}

// Fetch downloads (or reuses the cached) RepoBench parquet for the given language
// at rev, returning the raw bytes. The URL is built ONLY from the pinned
// Host + per-language Repo constants + the validated rev+language (resolveURL,
// SSRF-safe T-86-04-02); the body is size-capped via io.LimitReader
// (T-86-04-04); the rev and language are validatePathSegment-checked before the
// cache filepath.Join (T-86-04-03). It is the LIVE leg: the network-gated
// TestLiveFetch drives it; the hermetic decode test drives LoadParquetBytes over
// committed bytes instead. On a cache hit the cached bytes are returned with no
// network.
func Fetch(ctx context.Context, rev, language string) ([]byte, error) {
	dst, err := cachePath(rev, language)
	if err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(dst); err == nil {
		return b, nil
	}
	url, err := resolveURL(rev, language)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("repobench: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("repobench: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("repobench: fetch %s: HTTP %d", url, resp.StatusCode)
	}

	// Cap the untrusted body. Read one byte past the cap to detect an oversized
	// payload (T-86-04-04) rather than silently truncating it.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxParquetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("repobench: read body %s: %w", url, err)
	}
	if int64(len(body)) > maxParquetBytes {
		return nil, fmt.Errorf("repobench: parquet %s exceeds %d-byte cap", url, int64(maxParquetBytes))
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, fmt.Errorf("repobench: mkdir cache: %w", err)
	}
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		return nil, fmt.Errorf("repobench: write cache %s: %w", dst, err)
	}
	return body, nil
}
