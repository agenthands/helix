// Package jdtlscache resolves the warm jdtls -data directory used by the
// Java integration tests. The directory is keyed on (fixture content hash,
// jdtls binary identity hash) under os.UserCacheDir() so that:
//
//   - Repeat test runs reuse jdtls's index across invocations (warm start).
//   - Fixture edits invalidate the cache automatically (different hash → new dir).
//   - jdtls upgrades invalidate the cache automatically (different binary → new dir).
//
// The package uses only the Go standard library (no CGO, no extra deps) so it
// can run under default `go test ./...` without any build tag.
package jdtlscache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// ResolveDataDir returns the absolute path to the warm jdtls -data directory
// for a given fixture and resolved jdtls binary. It creates the directory if
// missing.
//
// Path layout: $UserCacheDir/helix-test/jdtls/<fixtureName>-<fixtureHash[:12]>-<jdtlsHash[:12]>/
func ResolveDataDir(fixtureName, fixtureRoot, jdtlsPath string) (string, error) {
	fh, err := hashTree(fixtureRoot)
	if err != nil {
		return "", fmt.Errorf("fixture hash: %w", err)
	}
	jh, err := hashBinary(jdtlsPath)
	if err != nil {
		return "", fmt.Errorf("jdtls hash: %w", err)
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}

	dir := filepath.Join(cache, "helix-test", "jdtls",
		fmt.Sprintf("%s-%s-%s", fixtureName, fh, jh))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir warm jdtls dir: %w", err)
	}
	return dir, nil
}

// hashTree computes a deterministic SHA-256 prefix over the sorted file list
// and contents under root. Returns the first 12 hex chars.
func hashTree(root string) (string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)

	h := sha256.New()
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		h.Write([]byte(rel))
		h.Write([]byte{0})
		data, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		h.Write(data)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

// hashBinary computes a deterministic SHA-256 prefix over (path || file
// contents) so that either a content change or a path change invalidates the
// cache. Returns the first 12 hex chars.
func hashBinary(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	h.Write([]byte(path))
	h.Write([]byte{0})
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}
