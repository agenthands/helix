package workspace

import (
	"crypto/sha256"
	"fmt"
)

// WorkspaceKey uniquely identifies a workspace by repo root, language, and toolchain.
// Per DMN-05: workspace state keyed by repo root + language + toolchain fingerprint.
type WorkspaceKey struct {
	RepoRoot  string
	Language  string
	Toolchain string
}

// String returns a human-readable representation.
func (k WorkspaceKey) String() string {
	return fmt.Sprintf("%s:%s:%s", k.RepoRoot, k.Language, k.Toolchain)
}

// Hash returns a deterministic hash for map keying.
func (k WorkspaceKey) Hash() string {
	h := sha256.Sum256([]byte(k.String()))
	return fmt.Sprintf("%x", h[:8])
}
