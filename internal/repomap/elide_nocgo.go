//go:build !cgo

package repomap

import (
	"github.com/agenthands/helix/internal/treesitter"
)

// ElisionRenderer under !cgo is a stub; the daemon refuses to start before any
// instance is exercised at runtime (see internal/daemon/daemon.go).
type ElisionRenderer struct{}

// NewElisionRenderer under !cgo returns an empty stub. Unreachable at runtime
// because the daemon refuses to start.
func NewElisionRenderer(_ *treesitter.GrammarRegistry) *ElisionRenderer {
	return &ElisionRenderer{}
}

// RenderFile under !cgo returns the empty string. Unreachable at runtime.
func (*ElisionRenderer) RenderFile(_ []byte, _ string, _ []Tag) string {
	return ""
}

// ElideSingle preserves the package-level function symbol from the cgo half so
// any external caller compiles. Unreachable at runtime.
func ElideSingle(_ []byte, _ Tag, _, _ uint) string {
	return ""
}
