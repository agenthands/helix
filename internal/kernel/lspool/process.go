// Package lspool manages Language Server child processes as warm workers
// with lifecycle management, share-until-dirty policy, and circuit breaking.
package lspool

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/agenthands/helix/internal/kernel/jsonrpc"
)

// stderrRingSize is the number of last stderr lines captured for crash reports.
const stderrRingSize = 10

// ProcessHandle manages an LS child process with safe pipe I/O goroutine separation.
// Per research Pattern 1 (Pitfall 1): never do pipe I/O from the Wait/reaper goroutine.
// Separate goroutines for reading, writing, and waiting.
type ProcessHandle struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	conn    *jsonrpc.Conn // JSON-RPC over stdin/stdout
	done    chan struct{} // closed when process exits
	exitErr error
	mu      sync.Mutex
	logger  *slog.Logger

	// stderrRing captures the last N lines of stderr for crash diagnostics.
	stderrRing []string
	stderrMu   sync.Mutex

	// tracer is forwarded to the jsonrpc.Conn created in Start so every LS
	// frame emits a child span. Phase 55 D-01: nil → noop fallback.
	tracer trace.Tracer
}

// NewProcessHandle creates a new ProcessHandle for the given LS command.
//
// tracer is propagated to the jsonrpc.Conn created during Start. A nil tracer
// falls back to a noop tracer (Phase 55 D-01).
func NewProcessHandle(command string, args []string, workDir string, env []string, logger *slog.Logger, tracer trace.Tracer) *ProcessHandle {
	cmd := exec.Command(command, args...)
	cmd.Dir = workDir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	if tracer == nil {
		tracer = tracenoop.NewTracerProvider().Tracer("process-noop")
	}
	return &ProcessHandle{
		cmd:    cmd,
		done:   make(chan struct{}),
		logger: logger,
		tracer: tracer,
	}
}

// Start spawns the LS process and creates the jsonrpc.Conn, but does NOT
// begin reading frames. Callers MUST invoke StartListen(ctx) AFTER setting
// Conn().OnNotification (if needed) to begin dispatch. Phase 56 D-02.
//
// The sessionPrefix is used for JSON-RPC request ID generation to avoid collisions.
func (p *ProcessHandle) Start(ctx context.Context, sessionPrefix string) error {
	var err error

	// Set up pipes before starting.
	p.stdin, err = p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	p.stdout, err = p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	p.stderr, err = p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	// Start the process.
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start process %s: %w", p.cmd.Path, err)
	}

	// Create JSON-RPC connection wrapping stdin/stdout.
	rwc := &stdinStdoutRWC{stdin: p.stdin, stdout: p.stdout}
	p.conn = jsonrpc.NewConn(rwc, sessionPrefix, p.tracer)

	// Launch stderr drain goroutine — captures last N lines for crash reports.
	go p.drainStderr()

	// Phase 56 D-02: do NOT launch the JSON-RPC dispatch loop here. Callers
	// must invoke StartListen(ctx) AFTER wiring Conn().OnNotification, otherwise
	// notifications received before assignment are silently dropped.

	// Launch reaper goroutine: waits for process exit.
	go p.reap()

	// Launch context cancellation handler for graceful shutdown.
	go p.watchContext(ctx)

	return nil
}

// Conn returns the JSON-RPC connection. Only valid after Start.
func (p *ProcessHandle) Conn() *jsonrpc.Conn {
	return p.conn
}

// StartListen begins the JSON-RPC dispatch loop. Caller MUST set
// Conn().OnNotification (if needed) before invoking StartListen — otherwise
// notifications received before assignment are silently dropped (this is the
// exact bug Phase 56 fixes; see jsonrpc.Conn.Listen comment).
func (p *ProcessHandle) StartListen(ctx context.Context) {
	go p.conn.Listen(ctx)
}

// Wait blocks until the process exits.
func (p *ProcessHandle) Wait() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exitErr
}

// Done returns a channel that is closed when the process exits.
func (p *ProcessHandle) Done() <-chan struct{} {
	return p.done
}

// Pid returns the process ID, or -1 if not started.
func (p *ProcessHandle) Pid() int {
	if p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return -1
}

// Stop performs graceful shutdown: LSP shutdown request -> exit notification -> SIGTERM -> SIGKILL.
func (p *ProcessHandle) Stop(ctx context.Context) error {
	// Step 1: Send LSP shutdown request (3s timeout).
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 3*time.Second)
	defer shutdownCancel()

	var result interface{}
	shutdownErr := p.conn.Call(shutdownCtx, "shutdown", nil, &result)
	if shutdownErr != nil {
		p.logger.Debug("LSP shutdown request failed", "error", shutdownErr)
	}

	// Step 2: Send exit notification.
	exitCtx, exitCancel := context.WithTimeout(ctx, 2*time.Second)
	defer exitCancel()
	_ = p.conn.Notify(exitCtx, "exit", nil)

	// Step 3: Wait briefly for clean exit.
	select {
	case <-p.done:
		return nil
	case <-time.After(2 * time.Second):
	}

	// Step 4: SIGTERM.
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Step 5: Wait 3s for SIGTERM, then SIGKILL.
	select {
	case <-p.done:
		return nil
	case <-time.After(3 * time.Second):
	}

	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}

	<-p.done
	return nil
}

// LastStderr returns the last captured stderr lines for crash diagnostics.
func (p *ProcessHandle) LastStderr() []string {
	p.stderrMu.Lock()
	defer p.stderrMu.Unlock()
	out := make([]string, len(p.stderrRing))
	copy(out, p.stderrRing)
	return out
}

// drainStderr reads stderr and captures the last N lines.
func (p *ProcessHandle) drainStderr() {
	buf := make([]byte, 4096)
	var partial string
	for {
		n, err := p.stderr.Read(buf)
		if n > 0 {
			text := partial + string(buf[:n])
			lines := strings.Split(text, "\n")
			// Last element is partial line (or empty if ends with \n).
			partial = lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			for _, line := range lines {
				if line == "" {
					continue
				}
				p.logger.Debug("LS stderr", "line", line)
				p.stderrMu.Lock()
				p.stderrRing = append(p.stderrRing, line)
				if len(p.stderrRing) > stderrRingSize {
					p.stderrRing = p.stderrRing[len(p.stderrRing)-stderrRingSize:]
				}
				p.stderrMu.Unlock()
			}
		}
		if err != nil {
			return
		}
	}
}

// reap waits for the process to exit and records the result.
// Per Pitfall 1: this goroutine ONLY calls cmd.Wait(), no pipe I/O.
func (p *ProcessHandle) reap() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.exitErr = err
	p.mu.Unlock()
	close(p.done)
}

// watchContext handles context cancellation with SIGTERM -> SIGKILL escalation.
func (p *ProcessHandle) watchContext(ctx context.Context) {
	select {
	case <-ctx.Done():
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-p.done:
			case <-time.After(3 * time.Second):
				if p.cmd.Process != nil {
					_ = p.cmd.Process.Kill()
				}
			}
		}
	case <-p.done:
		// Process already exited.
	}
}

// stdinStdoutRWC adapts separate stdin (writer) and stdout (reader) into io.ReadWriteCloser.
type stdinStdoutRWC struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (s *stdinStdoutRWC) Read(p []byte) (int, error) {
	return s.stdout.Read(p)
}

func (s *stdinStdoutRWC) Write(p []byte) (int, error) {
	return s.stdin.Write(p)
}

func (s *stdinStdoutRWC) Close() error {
	err1 := s.stdin.Close()
	err2 := s.stdout.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
