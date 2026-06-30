// Package cochange mines git history for file co-change relationships and
// emits FILE_CHANGES_WITH facts. It is intentionally dependency-light
// (stdlib + xxhash only): the daemon-side caller maps []CoChange into
// semantic-store EdgeFact rows keyed by file-level node IDs.
//
// A "co-change" is two files that appear together in the same commit more
// often than chance. The signal is a strong predictor of latent coupling —
// files that change together usually share a hidden dependency (a shared
// concept, a protocol pair, a view+controller) even when no static edge
// connects them. This captures exactly the coupling the symbol graph misses.
package cochange

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strings"

	"github.com/cespare/xxhash/v2"
)

// CoChange is one file-pair co-change fact: PathA and PathB (repo-relative,
// forward-slashed) appeared together in Count commits.
type CoChange struct {
	PathA string
	PathB string
	Count int
}

const (
	defaultMaxCommits = 1000
	defaultMinCount   = 2
	maxFilesPerCommit = 100 // skip bulk/refactor commits (noise)
	defaultMaxPairs   = 5000
)

var commitHashRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Mine runs `git log --name-only` over repoRoot and returns file-pair
// co-changes whose commit count meets the minimum threshold. Best-effort:
// a non-git directory or git failure yields (nil, err); callers MUST treat
// the error as non-fatal (co-change is enrichment, not a hard dependency).
func Mine(ctx context.Context, repoRoot string) ([]CoChange, error) {
	return MineWith(ctx, repoRoot, defaultMaxCommits, defaultMinCount)
}

// MineWith is the configurable entry point used by tests.
func MineWith(ctx context.Context, repoRoot string, maxCommits, minCount int) ([]CoChange, error) {
	if maxCommits <= 0 {
		maxCommits = defaultMaxCommits
	}
	if minCount < 1 {
		minCount = defaultMinCount
	}
	cmd := exec.CommandContext(ctx, "git",
		"-C", repoRoot,
		"log",
		"--no-renames",
		"--name-only",
		"--pretty=format:%H",
		"--max-count="+itoa(maxCommits),
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parse(out, minCount), nil
}

type pairKey struct{ a, b string }

// parse walks the git-log output: each commit begins with a 40-hex hash line
// (from --pretty=format:%H), followed by its changed paths until the next
// hash. It accumulates per-pair co-change counts and returns pairs meeting
// minCount, capped to defaultMaxPairs.
func parse(out []byte, minCount int) []CoChange {
	counts := make(map[pairKey]int)
	var commitFiles []string
	flush := func() {
		defer func() { commitFiles = nil }()
		if len(commitFiles) < 2 || len(commitFiles) > maxFilesPerCommit {
			return
		}
		// Dedupe within a single commit (a file changed twice counts once).
		seen := make(map[string]struct{}, len(commitFiles))
		uniq := commitFiles[:0]
		for _, f := range commitFiles {
			if f == "" {
				continue
			}
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			uniq = append(uniq, f)
		}
		for i := 0; i < len(uniq); i++ {
			for j := i + 1; j < len(uniq); j++ {
				a, b := uniq[i], uniq[j]
				if a > b {
					a, b = b, a
				}
				counts[pairKey{a, b}]++
			}
		}
	}

	for _, raw := range bytes.Split(out, []byte{'\n'}) {
		s := strings.TrimRight(string(raw), "\r")
		if commitHashRe.MatchString(s) {
			flush()
			continue
		}
		if s == "" {
			continue
		}
		commitFiles = append(commitFiles, strings.ReplaceAll(s, "\\", "/"))
	}
	flush()

	results := make([]CoChange, 0, len(counts))
	for k, c := range counts {
		if c < minCount {
			continue
		}
		results = append(results, CoChange{PathA: k.a, PathB: k.b, Count: c})
	}
	sortResults(results)
	if len(results) > defaultMaxPairs {
		results = results[:defaultMaxPairs]
	}
	return results
}

// FileNodeID derives a stable, high-bit-safe node ID for a file from the
// owning repoID + repo-relative path. File-level edges share the edge store's
// uint64 node-id space, distinguished by SrcKind/DstKind="file".
func FileNodeID(repoID, relPath string) uint64 {
	h := xxhash.Sum64String(repoID + "\x00" + relPath)
	return h & 0x7FFFFFFFFFFFFFFF
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func sortResults(r []CoChange) {
	for i := 1; i < len(r); i++ {
		for j := i; j > 0; j-- {
			if before(r[j], r[j-1]) {
				r[j], r[j-1] = r[j-1], r[j]
			} else {
				break
			}
		}
	}
}

func before(x, y CoChange) bool {
	if x.Count != y.Count {
		return x.Count > y.Count
	}
	if x.PathA != y.PathA {
		return x.PathA < y.PathA
	}
	return x.PathB < y.PathB
}
