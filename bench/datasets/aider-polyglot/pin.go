package aiderpolyglot

// RepoURL is the upstream Aider Polyglot benchmark repository. The adapter ONLY
// ever clones this URL, pinned by RepoURL @ PinnedSHA — never a fork, never a
// mutable ref (T-85-07-01).
const RepoURL = "https://github.com/Aider-AI/polyglot-benchmark"

// PinnedSHA is the exact 40-hex commit the dataset is pinned to. It is a REAL
// commit (refs/heads/main HEAD of Aider-AI/polyglot-benchmark resolved at plan
// time, 2026-06-21), NOT a placeholder: the clone fetches THIS sha with
// --depth 1 and checks it out, so a malicious upstream cannot swap content under
// us by moving a branch/tag (T-85-07-01 / Pitfall: mutable-ref tampering). To
// re-pin, resolve `git ls-remote https://github.com/Aider-AI/polyglot-benchmark
// main` and update this constant (and re-capture the per-language baseline).
const PinnedSHA = "7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f"

// isHexSHA1 reports whether s is exactly 40 lowercase hex characters — the shape
// of a git sha1 commit with no ref decoration. Mirrors bench/container's
// isHexSHA256 guard discipline (explicit, total, no regexp): a branch/tag name,
// a short sha, an uppercase sha, or any non-hex byte is rejected, so a mutable
// ref can never reach the git child argv (T-85-07-01 / T-85-07-05).
func isHexSHA1(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
