package aiderpolyglot

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cacheDirEnv overrides the cache root verbatim when set (mirrors
// bench/ragindex/cache.go exactly so the adapter shares the operator's one
// HELIX_CACHE_DIR convention).
const cacheDirEnv = "HELIX_CACHE_DIR"

// cloneSubdir is the fixed segment under the cache root that holds the pinned
// dataset clone: <cacheDir>/aider-polyglot/<pinned_sha>/.
const cloneSubdir = "aider-polyglot"

// cacheDir resolves the cache root with the SAME three-step precedence as
// bench/ragindex/cache.go (HELIX_CACHE_DIR → os.UserCacheDir()/helix →
// ~/.helix/cache), so the clone cache lands beside the rag-index cache.
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

// clonePath returns the on-disk clone dir for the pinned sha:
// <cacheDir>/aider-polyglot/<sha>/. The sha is isHexSHA1-validated (40 hex, no
// separators) so the joined path cannot escape the cache root (T-85-07-01 /
// T-85-07-02) — a non-sha is rejected before any Join.
func clonePath(sha string) (string, error) {
	if !isHexSHA1(sha) {
		return "", fmt.Errorf("aiderpolyglot: clonePath: %q is not a 40-hex sha", sha)
	}
	return filepath.Join(cacheDir(), cloneSubdir, sha), nil
}

// isValidGitURL is a conservative guard mirroring container.isValidRepo's
// leading-'-' rejection: the URL must NOT begin with '-' (the argv flag-
// smuggling vector, T-85-07-05) and must be an http(s) upstream. It is total and
// regexp-free.
func isValidGitURL(u string) bool {
	if u == "" || u[0] == '-' {
		return false
	}
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}

// cloneArgs builds the FIXED git argv for a --depth 1 clone pinned EXACTLY at
// sha — never a branch/tag (T-85-07-01). It returns three steps run in order:
//
//	git clone --no-checkout --depth 1 -- <repoURL> <destDir>
//	git -C <destDir> fetch --depth 1 origin -- <sha>
//	git -C <destDir> checkout --detach <sha>
//
// It fail-closes via isHexSHA1(sha), isValidGitURL(repoURL), and
// validatePathSegment on the dest leaf BEFORE producing any argv, so a mutable
// ref, a flag-shaped URL, or a traversal dest can never cross os/exec
// (T-85-07-01 / T-85-07-02 / T-85-07-05). The "--" terminator guards the clone
// (URL/dest) and fetch (refspec) steps (mirror bench/container engine.go
// discipline). The checkout step omits "--" because `git checkout --detach`
// REJECTS a "-- <pathspec>" form ("--detach does not take a path argument"); the
// sha is already isHexSHA1-validated (40 hex, no leading '-'), so it can never be
// a flag — flag-smuggling is closed by the validator, not the terminator, here.
func cloneArgs(repoURL, sha, destDir string) ([][]string, error) {
	if !isHexSHA1(sha) {
		return nil, fmt.Errorf("aiderpolyglot: cloneArgs: %q is not a 40-hex sha (refuse mutable ref)", sha)
	}
	if !isValidGitURL(repoURL) {
		return nil, fmt.Errorf("aiderpolyglot: cloneArgs: invalid repo URL %q", repoURL)
	}
	if destDir == "" || filepath.Clean(destDir) != destDir {
		return nil, fmt.Errorf("aiderpolyglot: cloneArgs: unclean dest dir %q", destDir)
	}
	// The dest leaf must be a safe path segment: no traversal token, no embedded
	// separator, and no leading dot (".git", ".ssh"). A caller-supplied absolute
	// temp dir (e.g. t.TempDir()) has a generated leaf that satisfies all three,
	// so it is still permitted. We now PROPAGATE validatePathSegment's rejection
	// rather than re-deriving a narrower predicate and swallowing most of it
	// (WR-03): the previous form silently accepted a leading-dot leaf despite the
	// validator rejecting it, so the guard did not enforce what its name claimed.
	if err := validatePathSegment(filepath.Base(destDir), "clone dest"); err != nil {
		return nil, err
	}
	return [][]string{
		{"git", "clone", "--no-checkout", "--depth", "1", "--", repoURL, destDir},
		{"git", "-C", destDir, "fetch", "--depth", "1", "origin", "--", sha},
		{"git", "-C", destDir, "checkout", "--detach", sha},
	}, nil
}

// gitEnv builds the strict env passed to every git child: only PATH, HOME, and
// HELIX_CACHE_DIR are forwarded (never the full parent env, T-85-07-04), plus
// GIT_TERMINAL_PROMPT=0 so a missing credential can never hang on an interactive
// prompt. Mirrors bench/container.allowlistEnv.
func gitEnv() []string {
	env := make([]string, 0, 4)
	for _, k := range []string{"PATH", "HOME", "HELIX_CACHE_DIR"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	return env
}

// Clone performs the pinned-sha shallow clone into destDir, running the three
// cloneArgs steps in order with exec.CommandContext (so a context cancel kills
// the git child) and the strict gitEnv allowlist. It is the LIVE leg: the
// network-gated TestLiveCloneAtPinnedSha drives it; the hermetic
// TestCloneArgsFixedArgv is the authoritative argv proof. Callers should pass a
// destDir under clonePath(sha) to reuse the cache.
func Clone(ctx context.Context, sha, destDir string) error {
	steps, err := cloneArgs(RepoURL, sha, destDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return fmt.Errorf("aiderpolyglot: mkdir clone parent: %w", err)
	}
	for _, args := range steps {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Env = gitEnv()
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("aiderpolyglot: %s: %w: %s", strings.Join(args, " "), err, stderr.String())
		}
	}
	return nil
}

// headSHA returns the resolved HEAD commit of a clone dir via `git -C <dir>
// rev-parse HEAD`, used by the live test to assert the checkout pinned exactly
// at PinnedSHA. Strict gitEnv allowlist; no network.
func headSHA(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("aiderpolyglot: rev-parse HEAD in %q: %w", dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}
