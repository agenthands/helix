// Package sandbox provides the bench-side per-(task, mode) sandbox. It is a thin
// wrapper that EMBEDS internal/eval/sandbox.Sandbox (D-07: reuse, never fork) and
// adds ONLY the bench-specific DURABLE artifact paths.
//
// Division of responsibility (D-08):
//   - Ephemeral scratch (HOME, repo working copy, the Unix daemon socket) lives
//     in the embedded eval sandbox under /tmp; it is deleted on success and
//     PRESERVED on failure for debugging. That delete-vs-preserve gating is the
//     responsibility of the cell orchestrator's call site — this package does NOT
//     override the embedded Cleanup().
//   - Durable artifacts (result.v2.json, the merged-trace JSON) live under the
//     out dir: bench/reports/<run_id>/<task>/<mode>/<run_index>/. The durable
//     path layout is computed by the cell orchestrator's cellDurablePaths
//     (which threads the <run_index> segment); this package only records the
//     durable artifact root via OutDir().
//
// All filesystem hardening (lstat-symlink reject, 0700, macOS short-socket-root)
// is inherited verbatim from the embedded eval sandbox; it is intentionally not
// reimplemented here.
package sandbox

import (
	"fmt"

	evalsandbox "github.com/agenthands/helix/internal/eval/sandbox"
)

// Sandbox embeds the eval sandbox and adds the durable bench artifact root.
//
// The embedded *evalsandbox.Sandbox supplies the full per-(task, mode) isolation
// surface: NewSandbox/Prepare/CloneRepo/StartDaemon/ModeDir/HomeFor/RepoFor/
// SocketFor/McpConfigPath/Cleanup. None of those is reimplemented here.
type Sandbox struct {
	*evalsandbox.Sandbox

	// outDir is the durable artifact root for this run. Default layout:
	// bench/reports/<run_id>/<task>/<mode>/ (D-08); the run-id segment is the
	// caller's responsibility (it passes the per-run out dir here).
	outDir string
}

// New constructs a bench sandbox. It creates the embedded eval sandbox (which
// owns the ephemeral /tmp scratch root) and records outDir as the durable
// artifact root.
//
//   - runID    — the run identifier (eval shape: e.g. "20060102T150405Z").
//   - helixBin — path to the helix binary used to spawn the daemon subprocess.
//   - outDir   — durable artifact root for this run (e.g. bench/reports/<run_id>).
func New(runID, helixBin, outDir string) (*Sandbox, error) {
	es, err := evalsandbox.NewSandbox(runID, helixBin)
	if err != nil {
		return nil, fmt.Errorf("bench sandbox: %w", err)
	}
	return &Sandbox{Sandbox: es, outDir: outDir}, nil
}

// OutDir returns the durable artifact root recorded at construction.
func (s *Sandbox) OutDir() string { return s.outDir }
