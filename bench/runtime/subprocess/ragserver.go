// subprocess/ragserver.go wires the standalone `cmd/helix-bench-rag` MCP server
// spawn for the Phase 83 baseline_rag control arm (ABLATE-04 #3). It is the THIRD
// spawn variant in this package alongside StartDaemon (daemon.go) and StartClaude
// (claude.go).
//
// CRITICAL transport difference from StartDaemon: the Helix daemon is a SOCKETED
// server (`helix --serve --socket=<cell>/daemon.sock`) that the bench drives over
// the gRPC StreamMCP wire (forwarder.OpenSession dials the socket directly —
// drive.go:driveScript; the `helix --mode=stdio` forwarder head it used pre-Phase-94
// was deleted). The baseline_rag server is fundamentally different —
// `cmd/helix-bench-rag` IS the MCP server and speaks the MCP protocol directly
// over its OWN stdin/stdout
// (StdioTransport; cmd/helix-bench-rag/server.go Run). There is no socket, no
// forwarder, and (D-06) no TCP port. So this helper cannot delegate to the eval
// sandbox's StartDaemon (which hard-codes the helix binary + socket argv and
// returns a socket-oriented *DaemonHandle whose unexported fields are not
// constructible here). Instead it spawns the bench-rag binary directly with
// `--corpus=<repoDir>` and returns a RAGHandle the caller drives over the process
// pipes — mirroring the DaemonHandle's Pid()/Kill() lifecycle surface (the cell
// captures Pid() before Kill(), exactly like the daemon leg).
//
// Isolation invariant (ABLATE-04 #1c): the spawned binary shares NO daemon code
// (no internal/kernel, internal/semantic, or internal/mcp). That boundary is
// enforced statically (make vet's benchragleakage analyzer) and dynamically
// (cmd/helix-bench-rag/leakage_test.go) — this spawn helper only invokes the
// already-isolated binary.
package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
)

// RAGServerBinEnv is the env var that overrides the resolved helix-bench-rag
// binary path. When unset, StartRAGServer resolves the binary as a sibling of the
// helix binary (same directory, named "helix-bench-rag"). This mirrors the
// HELIX_BIN override used by the bench integration tests.
const RAGServerBinEnv = "HELIX_BENCH_RAG_BIN"

// ragKillTimeout bounds how long Kill waits for the bench-rag process group to be
// reaped before re-sending SIGKILL and blocking on the owner Wait goroutine.
const ragKillTimeout = 5 * time.Second

// RAGHandle owns one spawned cmd/helix-bench-rag process. It exposes the same
// Pid()/Kill() lifecycle surface as evalsandbox.DaemonHandle (so the cell drives
// it identically) PLUS the Stdin/Stdout pipes the MCP driver speaks over — the
// bench-rag server is the MCP endpoint directly (no forwarder), so the caller
// frames JSON-RPC over these pipes itself.
type RAGHandle struct {
	cmd    *exec.Cmd
	taskID string
	mode   string

	// Stdin/Stdout are the MCP transport: the driver writes JSON-RPC request
	// frames to Stdin and reads NDJSON response lines from Stdout. They are owned
	// by the caller for the life of the drive; Kill closes them via process reap.
	Stdin  io.WriteCloser
	Stdout io.ReadCloser

	exited   chan error
	killOnce sync.Once
}

// Pid returns the spawned bench-rag process PID (0 if not started), mirroring
// DaemonHandle.Pid so the cell can capture it before Kill (criterion #4 PID gate).
func (h *RAGHandle) Pid() int {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return 0
	}
	return h.cmd.Process.Pid
}

// Kill terminates the bench-rag process group and waits for it to exit. It is
// idempotent (safe to call once after the drive completes). Mirrors
// DaemonHandle.Kill: group SIGKILL (the process is its own group leader via
// Setpgid) reaps any descendants, then we observe the single owner Wait goroutine
// bounded by ragKillTimeout.
func (h *RAGHandle) Kill() error {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	pid := h.cmd.Process.Pid
	h.killOnce.Do(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = h.cmd.Process.Kill()
		// Closing stdin unblocks any blocked write and signals the server's stdio
		// transport to drain toward EOF.
		if h.Stdin != nil {
			_ = h.Stdin.Close()
		}
	})
	select {
	case <-h.exited:
		return nil
	case <-time.After(ragKillTimeout):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-h.exited
		return fmt.Errorf("subprocess: rag server %s/%s did not exit within %s after kill", h.taskID, h.mode, ragKillTimeout)
	}
}

// ResolveRAGServerBin resolves the helix-bench-rag binary path. It prefers the
// HELIX_BENCH_RAG_BIN override (so a freshly-built binary can be pointed at
// without polluting PATH), then falls back to a sibling of helixBin named
// "helix-bench-rag" in the same directory. The returned path is validated to
// exist; an empty/missing binary is a clean configuration error.
func ResolveRAGServerBin(helixBin string) (string, error) {
	if env := os.Getenv(RAGServerBinEnv); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
		return "", fmt.Errorf("subprocess: %s=%q does not exist", RAGServerBinEnv, env)
	}
	if helixBin == "" {
		return "", errors.New("subprocess: cannot resolve helix-bench-rag binary (empty helix bin and no " + RAGServerBinEnv + ")")
	}
	sibling := filepath.Join(filepath.Dir(helixBin), "helix-bench-rag")
	if _, err := os.Stat(sibling); err == nil {
		return sibling, nil
	}
	return "", fmt.Errorf("subprocess: helix-bench-rag binary not found next to %q (set %s)", helixBin, RAGServerBinEnv)
}

// StartRAGServer spawns the standalone cmd/helix-bench-rag MCP server over its own
// stdio (D-06: no socket, no TCP port) with the corpus rooted at corpusRoot (the
// cloned per-cell repo working copy). It returns a RAGHandle whose Pid()/Kill()
// the caller drives — exactly like the daemon leg drives a *DaemonHandle — plus
// the Stdin/Stdout MCP transport pipes.
//
// The embedding index is built/loaded OUT-OF-BAND by the caller BEFORE spawning
// (ABLATE-04 #4 / Pitfall 3): the server's RunE calls ragindex.Open(corpus), which
// hits the warm per-corpus cache, so no embedding-API tokens are charged to the
// timed agent span. ragServerBin is the resolved helix-bench-rag binary path
// (see ResolveRAGServerBin).
//
// embedderID is the embedder_id the parent recorded when it built the index
// out-of-band; it is forwarded as HELIX_RAG_EMBEDDER so the server selects the
// SAME embedder deterministically on warm reopen (fail-closed if that embedder
// is no longer usable) rather than re-probing availability — see WR-01. An empty
// embedderID leaves the env unset (the server then re-derives selection, the
// pre-WR-01 behavior).
func StartRAGServer(ctx context.Context, sb *benchsandbox.Sandbox, ragServerBin, taskID, mode, corpusRoot, embedderID string) (*RAGHandle, error) {
	if sb == nil {
		return nil, fmt.Errorf("subprocess: nil sandbox")
	}
	if ragServerBin == "" {
		return nil, fmt.Errorf("subprocess: empty helix-bench-rag binary path")
	}
	if corpusRoot == "" {
		return nil, fmt.Errorf("subprocess: empty corpus root for rag server %s/%s", taskID, mode)
	}

	cmd := exec.CommandContext(ctx, ragServerBin, "--corpus="+corpusRoot)

	// Own process group so Kill's group SIGKILL targets exactly the server and any
	// descendants (mirrors StartDaemon's Setpgid / WR-05 discipline).
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Strict env: PATH (so the embedder/Ollama probe can resolve hosts) + the
	// embedding credentials the index build already consumed out-of-band. We do NOT
	// log or echo OPENAI_API_KEY (T-83-01-01); it is forwarded only so a warm-cache
	// reopen that re-supplies the same embedder func has the same availability.
	// HELIX_RAG_FORCE_STUB is forwarded so a hermetic-CI cell (which built the index
	// with the deterministic stub) makes the server re-supply the SAME stub embedder
	// on reopen — keeping the recorded embedder_id consistent between the
	// out-of-band build and the warm-cache server reopen (Pitfall 1).
	env := []string{}
	for _, k := range []string{"PATH", "HOME", "OPENAI_API_KEY", "HELIX_CACHE_DIR", "HELIX_RAG_FORCE_STUB"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	// Pin the parent-selected embedder so the warm-reopen server cannot silently
	// diverge from the embedder_id recorded against the on-disk vectors (WR-01).
	// The server fails closed if the pinned embedder is no longer usable rather
	// than answering queries from a different embedder space.
	if embedderID != "" {
		env = append(env, "HELIX_RAG_EMBEDDER="+embedderID)
	}
	env = append(env, "HELIX_LOG_LEVEL=info")
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("subprocess: rag server %s/%s stdin pipe: %w", taskID, mode, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("subprocess: rag server %s/%s stdout pipe: %w", taskID, mode, err)
	}
	// Server diagnostics (index-open errors, etc.) go to the cell's daemon.log
	// sibling so a failed spawn is debuggable on a preserved scratch.
	logPath := filepath.Join(sb.ModeDir(taskID, mode), "rag-server.log")
	logFile, lerr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if lerr != nil {
		return nil, fmt.Errorf("subprocess: open rag-server log: %w", lerr)
	}
	defer logFile.Close()
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("subprocess: start rag server %s/%s: %w", taskID, mode, err)
	}

	// Own the single cmd.Wait() for the life of the process (mirrors StartDaemon's
	// CR-01 discipline): send the result then close so every Kill returns at once.
	exited := make(chan error, 1)
	go func() {
		exited <- cmd.Wait()
		close(exited)
	}()

	return &RAGHandle{
		cmd:    cmd,
		taskID: taskID,
		mode:   mode,
		Stdin:  stdin,
		Stdout: stdout,
		exited: exited,
	}, nil
}
