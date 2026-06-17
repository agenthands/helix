// Package sandbox manages per-(task, mode) filesystem and environment isolation
// for the Phase 67 evaluation harness. Each mode receives a private HOME
// directory, daemon socket, and helix_config.yml so no cross-mode contamination
// occurs. Isolation mechanics follow the subprocess design in 67-RESEARCH.md
// §"Subprocess Isolation Design". Real implementation lands in Wave 1+.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Sandbox holds the per-run root tmpdir and the path to the helix binary used
// for daemon subprocess spawning.
type Sandbox struct {
	Root     string
	helixBin string

	mu      sync.Mutex
	handles []*DaemonHandle
}

// NewSandbox creates a new evaluation sandbox rooted at a unique tmpdir under
// /tmp (not the OS default tmpdir, which may be very long on macOS).
// Unix domain socket paths are limited to 104 bytes on macOS; using /tmp
// keeps the sandbox root path short enough for daemon.sock to fit within that
// limit even with task/mode path components appended.
// The sandbox root is mode 0700 and is rejected if it is itself a symbolic link
// (T-67-01 mitigation).
func NewSandbox(runID string, helixBin string) (*Sandbox, error) {
	root, err := os.MkdirTemp("/tmp", "helix-eval-"+runID+"-")
	if err != nil {
		return nil, fmt.Errorf("sandbox: create tmpdir: %w", err)
	}
	// EvalSymlinks resolves /tmp -> /private/tmp on macOS so downstream
	// os.Stat calls use the canonical path.
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("sandbox: eval symlinks on root: %w", err)
	}

	// Enforce 0700 on the root (MkdirTemp may apply a more permissive umask).
	if err := os.Chmod(root, 0700); err != nil {
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("sandbox: chmod root: %w", err)
	}

	// T-67-01: reject if the root resolves through a symlink.
	if err := checkNotSymlink(root); err != nil {
		_ = os.RemoveAll(root)
		return nil, err
	}

	return &Sandbox{Root: root, helixBin: helixBin}, nil
}

// checkNotSymlink returns an error if the path itself is a symbolic link.
// Uses os.Lstat so it does not follow the final component. This detects the
// T-67-01 threat of an attacker placing a symlink at a predictable tmpdir path
// without triggering false positives from OS-level symlinks (e.g., macOS
// /var -> /private/var) that are in the parent directories, not the node itself.
func checkNotSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("sandbox: lstat %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("sandbox: path %q is a symbolic link (T-67-01 mitigation)", path)
	}
	return nil
}

// ModeDir returns the per-(task, mode) directory: <root>/<task>/<mode>/.
func (s *Sandbox) ModeDir(taskID, mode string) string {
	return filepath.Join(s.Root, taskID, mode)
}

// HomeFor returns the HOME directory for a given (task, mode) pair.
func (s *Sandbox) HomeFor(taskID, mode string) string {
	return filepath.Join(s.ModeDir(taskID, mode), "home")
}

// RepoFor returns the repo directory for a given (task, mode) pair.
func (s *Sandbox) RepoFor(taskID, mode string) string {
	return filepath.Join(s.ModeDir(taskID, mode), "repo")
}

// SocketFor returns the daemon Unix socket path for a given (task, mode) pair.
func (s *Sandbox) SocketFor(taskID, mode string) string {
	return filepath.Join(s.ModeDir(taskID, mode), "daemon.sock")
}

// McpConfigPath returns the MCP config JSON path for a given (task, mode) pair.
func (s *Sandbox) McpConfigPath(taskID, mode string) string {
	return filepath.Join(s.ModeDir(taskID, mode), "mcp-config.json")
}

// Prepare creates the directory tree for a (task, mode) pair. This includes
// home/.helix/ and repo/ under the mode dir, all with mode 0700.
func (s *Sandbox) Prepare(taskID, mode string) error {
	md := s.ModeDir(taskID, mode)
	dirs := []string{
		filepath.Join(md, "home", ".helix"),
		filepath.Join(md, "repo"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("sandbox: prepare %q: %w", d, err)
		}
	}
	return nil
}

// CloneRepo recursively copies srcRepo into the per-(task,mode) repo directory.
// Symlinks inside srcRepo are NOT followed beyond the repo boundary.
func (s *Sandbox) CloneRepo(srcRepo, taskID, mode string) error {
	dst := s.RepoFor(taskID, mode)
	return filepath.WalkDir(srcRepo, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks — do not follow out of srcRepo.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		rel, err := filepath.Rel(srcRepo, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}

		return copyFile(path, target)
	})
}

// copyFile copies src to dst, preserving the source file mode.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// DaemonHandle represents a running daemon subprocess.
type DaemonHandle struct {
	cmd    *exec.Cmd
	taskID string
	mode   string
}

// Pid returns the OS process ID of the daemon subprocess, or 0 if the process
// has not been started or the handle is nil. Used by trace.TapDaemonLog to
// gate log events to the correct daemon process (T-67-04 mitigation).
func (h *DaemonHandle) Pid() int {
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return 0
	}
	return h.cmd.Process.Pid
}

// Kill terminates the daemon process and waits for it to exit.
func (h *DaemonHandle) Kill() error {
	if h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	pid := h.cmd.Process.Pid
	// WR-01: the daemon is its own group leader (Setpgid in StartDaemon), so
	// SIGKILL the whole group on the normal path — not just the leader — to reap
	// any descendants (language servers, a `go test` child). Doing this only in
	// the 5s-timeout fallback leaked children on the common happy path under
	// --parallel. The group kill is best-effort (ESRCH once the group is gone).
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	if err := h.cmd.Process.Kill(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			// Process already finished — reap the zombie if not already reaped.
			_ = h.cmd.Wait()
			return nil
		}
		return fmt.Errorf("sandbox: kill daemon %s/%s: %w", h.taskID, h.mode, err)
	}

	// Wait with a 5s deadline so we don't hang.
	done := make(chan error, 1)
	go func() { done <- h.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Re-send SIGKILL to the entire process group, then WR-02: drain the
		// in-flight Wait() goroutine so the process is confirmed reaped before we
		// return. The done channel is buffered (cap 1) so the goroutine's send
		// never blocks, and Wait() returns once the group SIGKILL lands.
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-done
		return fmt.Errorf("sandbox: daemon %s/%s did not exit within 5s after kill", h.taskID, h.mode)
	}
	return nil
}

// StartDaemon spawns the helix daemon subprocess with an isolated environment
// and waits up to 10 seconds for the Unix socket to become available.
//
// Environment allowlist (T-67-Pitfall-1 mitigation):
//   - PATH   (inherited from host)
//   - HOME   (set to HomeFor(taskID, mode) — isolated per mode)
//   - HELIX_LOG_LEVEL=info
//
// The daemon stderr is redirected to <modeDir>/daemon.log.
//
// Optional DaemonOptions tune the spawn additively (P77 D-07: extend, do not
// fork). Existing 5-arg callers compile unchanged.
//
// daemonOpts holds the resolved optional spawn settings.
type daemonOpts struct {
	workDir string
}

// DaemonOption configures an optional StartDaemon behavior.
type DaemonOption func(*daemonOpts)

// WithWorkingDir sets the daemon subprocess's working directory (cmd.Dir). The
// bench harness uses this for per-cell store isolation (D-03): pointing cmd.Dir
// at the per-cell repo makes the eager .helix/semantic.duckdb open resolve
// per-cell, avoiding a shared-lock deadlock under --parallel. An empty dir is a
// no-op (cmd.Dir stays the default).
func WithWorkingDir(dir string) DaemonOption {
	return func(o *daemonOpts) { o.workDir = dir }
}

func (s *Sandbox) StartDaemon(ctx context.Context, taskID, mode, profileName, cfgPath string, opts ...DaemonOption) (*DaemonHandle, error) {
	var o daemonOpts
	for _, opt := range opts {
		opt(&o)
	}

	sockPath := s.SocketFor(taskID, mode)
	homePath := s.HomeFor(taskID, mode)
	modeDir := s.ModeDir(taskID, mode)

	args := []string{"--serve", "--socket=" + sockPath, "--http-addr=", "--json"}
	if profileName != "" {
		args = append(args, "--profile="+profileName)
	}
	if cfgPath != "" {
		args = append(args, "--config="+cfgPath)
	}

	cmd := exec.CommandContext(ctx, s.helixBin, args...)

	// D-03: per-cell store isolation. When set, run the daemon with cwd at the
	// per-cell repo so the eager .helix/semantic.duckdb open resolves per-cell.
	if o.workDir != "" {
		cmd.Dir = o.workDir
	}

	// WR-05: put the daemon in its OWN process group so the Kill 5s-timeout
	// fallback's syscall.Kill(-pid, SIGKILL) targets exactly the daemon and its
	// descendants (language servers, `go test`). Without Setpgid the daemon shares
	// the harness's group, so -pid keys a group the daemon does not lead, the kill
	// returns ESRCH, and the children it claims to reap survive — a real leak under
	// --parallel.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Env allowlist.
	env := []string{
		"HOME=" + homePath,
		"HELIX_LOG_LEVEL=info",
	}
	if p := os.Getenv("PATH"); p != "" {
		env = append(env, "PATH="+p)
	}
	cmd.Env = env

	// Redirect daemon stderr to a log file. Keep the file open until the daemon
	// is confirmed up (or killed on failure) so that startup error output is not
	// lost on platforms where closing the parent fd before the child flushes its
	// kernel buffer causes silent data loss.
	logPath := filepath.Join(modeDir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("sandbox: open daemon log: %w", err)
	}
	defer logFile.Close() // closed when StartDaemon returns

	cmd.Stderr = logFile
	cmd.Stdout = logFile

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sandbox: start daemon %s/%s: %w", taskID, mode, err)
	}

	// Poll until socket appears or ctx/deadline expires.
	if err := waitSocket(ctx, sockPath, 10*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("sandbox: daemon %s/%s socket did not appear: %w", taskID, mode, err)
	}

	h := &DaemonHandle{cmd: cmd, taskID: taskID, mode: mode}

	s.mu.Lock()
	s.handles = append(s.handles, h)
	s.mu.Unlock()

	return h, nil
}

// Cleanup kills any held daemon handles and removes the entire sandbox root.
func (s *Sandbox) Cleanup() error {
	s.mu.Lock()
	handles := s.handles
	s.handles = nil
	s.mu.Unlock()

	for _, h := range handles {
		_ = h.Kill()
	}

	return os.RemoveAll(s.Root)
}

// waitSocket polls sockPath until the file exists or the timeout/ctx expires.
// The loop pattern mirrors internal/forwarder/dial.go waitForDaemon.
func waitSocket(ctx context.Context, sockPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if _, err := os.Stat(sockPath); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			if t.After(deadline) {
				return fmt.Errorf("socket %q did not appear within %s", sockPath, timeout)
			}
		}
	}
}
