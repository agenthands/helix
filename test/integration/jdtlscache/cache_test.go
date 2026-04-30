package jdtlscache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFixture writes a small set of files into root and returns root.
func writeFixture(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "pom.xml"), []byte("<project/>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "Main.java"), []byte("class Main {}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "Greeter.java"), []byte("class Greeter {}"), 0o644))
}

func TestFixtureHashStable(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root)

	h1, err := hashTree(root)
	require.NoError(t, err)
	h2, err := hashTree(root)
	require.NoError(t, err)

	assert.Equal(t, h1, h2, "hashTree must be deterministic on an unchanged tree")
	assert.Len(t, h1, 12, "hash prefix must be 12 hex chars")
}

func TestFixtureHashInvalidation(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root)

	before, err := hashTree(root)
	require.NoError(t, err)

	// Mutate one file's contents.
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "Main.java"), []byte("class Main { /* changed */ }"), 0o644))

	after, err := hashTree(root)
	require.NoError(t, err)

	assert.NotEqual(t, before, after, "hashTree must change when a tracked file's contents change")
}

func TestJdtlsHashInvalidation(t *testing.T) {
	dir := t.TempDir()

	binA := filepath.Join(dir, "jdtls-a")
	binB := filepath.Join(dir, "jdtls-b")
	require.NoError(t, os.WriteFile(binA, []byte("fake-jdtls-content-a"), 0o755))
	require.NoError(t, os.WriteFile(binB, []byte("fake-jdtls-content-b"), 0o755))

	hA, err := hashBinary(binA)
	require.NoError(t, err)
	hB, err := hashBinary(binB)
	require.NoError(t, err)
	assert.NotEqual(t, hA, hB, "different binary contents must produce different hashes")

	// Same content, different path → still different hash (path is part of identity).
	dir2 := t.TempDir()
	binC := filepath.Join(dir2, "jdtls-a")
	require.NoError(t, os.WriteFile(binC, []byte("fake-jdtls-content-a"), 0o755))

	hC, err := hashBinary(binC)
	require.NoError(t, err)
	assert.NotEqual(t, hA, hC, "same content at different path must still produce different hashes")
	assert.Len(t, hA, 12, "binary hash must be 12 hex chars")
}

func TestResolveDataDir_CreatesUnderUserCache(t *testing.T) {
	cacheRoot := t.TempDir()
	// os.UserCacheDir consults different env vars per OS.
	t.Setenv("XDG_CACHE_HOME", cacheRoot) // Linux
	t.Setenv("HOME", cacheRoot)           // darwin / *nix fallback
	t.Setenv("LocalAppData", cacheRoot)   // Windows

	fixtureRoot := t.TempDir()
	writeFixture(t, fixtureRoot)

	jdtlsPath := filepath.Join(t.TempDir(), "jdtls")
	require.NoError(t, os.WriteFile(jdtlsPath, []byte("fake-jdtls"), 0o755))

	dir, err := ResolveDataDir("java", fixtureRoot, jdtlsPath)
	require.NoError(t, err)

	// Path must contain the helix-test/jdtls segments somewhere.
	wantSegment := filepath.Join("helix-test", "jdtls")
	assert.True(t, strings.Contains(dir, wantSegment),
		"returned path %q must contain segment %q", dir, wantSegment)

	// Final directory name must start with the fixture name.
	assert.True(t, strings.HasPrefix(filepath.Base(dir), "java-"),
		"directory name %q must start with fixture-name prefix", filepath.Base(dir))
}

func TestResolveDataDir_DirectoryExists(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheRoot)
	t.Setenv("HOME", cacheRoot)
	t.Setenv("LocalAppData", cacheRoot)

	fixtureRoot := t.TempDir()
	writeFixture(t, fixtureRoot)

	jdtlsPath := filepath.Join(t.TempDir(), "jdtls")
	require.NoError(t, os.WriteFile(jdtlsPath, []byte("fake-jdtls"), 0o755))

	dir, err := ResolveDataDir("java", fixtureRoot, jdtlsPath)
	require.NoError(t, err)

	info, err := os.Stat(dir)
	require.NoError(t, err, "ResolveDataDir must create the directory")
	assert.True(t, info.IsDir(), "returned path must be a directory")
}
