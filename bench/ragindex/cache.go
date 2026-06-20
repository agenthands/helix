// Package ragindex builds and caches a per-corpus embedding index backed by
// chromem-go. It is a LEAF package: it imports only chromem-go + the Go
// standard library, never internal/kernel, internal/semantic, or bench/runtime.
// This leaf invariant is what the standalone cmd/helix-bench-rag server (and its
// no-kernel/no-semantic vet gate) relies on transitively.
package ragindex

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cacheDirEnv is the (new this phase) env var that, when set, overrides the
// cache root verbatim. No prior usage exists in the tree.
const cacheDirEnv = "HELIX_CACHE_DIR"

// indexSubdir is the fixed segment under the cache root that holds per-corpus
// embedding indexes: <cacheDir>/bench-rag-index/<corpus_sha>/.
const indexSubdir = "bench-rag-index"

// cacheDir resolves the cache root with a three-step precedence (mirrors the
// repo's ~/.helix home convention in internal/config/loader.go):
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

// CorpusSHA returns a deterministic, content-only fingerprint of the corpus
// rooted at root. It hashes ONLY the sorted set of (relative-path,
// sha256(content)) pairs — never mtime and never directory-walk order — so the
// same logical corpus always maps to the same sha and a single byte of content
// change flips it (Pitfall 4). The sort mirrors result.go's fairnessBlock
// sort-for-byte-stability discipline.
func CorpusSHA(root string) (string, error) {
	var entries []string
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		// Normalize to forward slashes so the sha is OS-independent.
		rel = filepath.ToSlash(rel)
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(b)
		entries = append(entries, rel+":"+hex.EncodeToString(sum[:]))
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	sort.Strings(entries)
	h := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(h[:]), nil
}

// IndexPath returns the on-disk cache path for the corpus rooted at root:
// <cacheDir>/bench-rag-index/<corpus_sha>/. The corpus_sha is a hex sha256 with
// no path separators, so the joined path cannot escape the cache root
// (T-83-01-03 tampering mitigation).
func IndexPath(root string) (string, error) {
	sha, err := CorpusSHA(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir(), indexSubdir, sha), nil
}
