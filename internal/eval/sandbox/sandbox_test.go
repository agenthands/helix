package sandbox

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSandboxLayout verifies that NewSandbox creates the root tmpdir and
// that ModeDir / HomeFor / RepoFor / SocketFor / McpConfigPath return
// paths rooted inside it.
func TestSandboxLayout(t *testing.T) {
	sb, err := NewSandbox("testlayout", "/usr/bin/true")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sb.Root) })

	assert.DirExists(t, sb.Root)

	md := sb.ModeDir("task1", "baseline")
	assert.True(t, strings.HasPrefix(md, sb.Root), "ModeDir should be inside Root")

	assert.Equal(t, filepath.Join(md, "home"), sb.HomeFor("task1", "baseline"))
	assert.Equal(t, filepath.Join(md, "repo"), sb.RepoFor("task1", "baseline"))
	assert.Equal(t, filepath.Join(md, "daemon.sock"), sb.SocketFor("task1", "baseline"))
	assert.Equal(t, filepath.Join(md, "mcp-config.json"), sb.McpConfigPath("task1", "baseline"))

	// Prepare must create subdirs.
	require.NoError(t, sb.Prepare("task1", "baseline"))
	assert.DirExists(t, sb.HomeFor("task1", "baseline"))
	assert.DirExists(t, sb.RepoFor("task1", "baseline"))

	// Check mode 0700 on the sandbox root.
	info, err := os.Stat(sb.Root)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
}

// TestSandboxIsolation checks that two modes for the same task get disjoint
// sockets, HOME directories, and repo paths.
func TestSandboxIsolation(t *testing.T) {
	sb, err := NewSandbox("testisolation", "/usr/bin/true")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sb.Root) })

	sockA := sb.SocketFor("task1", "baseline")
	sockB := sb.SocketFor("task1", "native")
	assert.NotEqual(t, sockA, sockB)

	homeA := sb.HomeFor("task1", "baseline")
	homeB := sb.HomeFor("task1", "native")
	assert.NotEqual(t, homeA, homeB)

	repoA := sb.RepoFor("task1", "baseline")
	repoB := sb.RepoFor("task1", "native")
	assert.NotEqual(t, repoA, repoB)
}

// TestSandboxCloneRepo verifies that CloneRepo copies files from a source tree
// into the per-(task,mode) repo directory.
func TestSandboxCloneRepo(t *testing.T) {
	sb, err := NewSandbox("testclone", "/usr/bin/true")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sb.Root) })

	require.NoError(t, sb.Prepare("task1", "baseline"))

	// Create a small source repo fixture.
	srcRepo := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcRepo, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(srcRepo, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcRepo, "sub", "util.go"), []byte("package sub"), 0644))

	require.NoError(t, sb.CloneRepo(srcRepo, "task1", "baseline"))

	dst := sb.RepoFor("task1", "baseline")
	assert.FileExists(t, filepath.Join(dst, "main.go"))
	assert.FileExists(t, filepath.Join(dst, "sub", "util.go"))
}

// TestSandboxStartDaemon tests that StartDaemon blocks until the socket appears
// and that the returned handle can be killed.
func TestSandboxStartDaemon(t *testing.T) {
	fakeHelixBin := buildFakeHelixBin(t)

	sb, err := NewSandbox("testdaemon", fakeHelixBin)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(sb.Root) })

	require.NoError(t, sb.Prepare("t1", "baseline"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := sb.StartDaemon(ctx, "t1", "baseline", "full", "")
	require.NoError(t, err, "StartDaemon should succeed once socket appears")
	require.NotNil(t, handle)

	// Socket should exist now.
	sockPath := sb.SocketFor("t1", "baseline")
	_, statErr := os.Stat(sockPath)
	assert.NoError(t, statErr, "socket file should exist after StartDaemon returns")

	require.NoError(t, handle.Kill())
}

// TestSandboxCleanup verifies that Cleanup removes the root tmpdir.
func TestSandboxCleanup(t *testing.T) {
	sb, err := NewSandbox("testcleanup", "/usr/bin/true")
	require.NoError(t, err)

	root := sb.Root
	assert.DirExists(t, root)

	require.NoError(t, sb.Cleanup())
	assert.NoDirExists(t, root)
}

// TestSandboxConcurrent proves that two parallel StartDaemon calls for different
// (task, mode) pairs do not collide on socket paths.
func TestSandboxConcurrent(t *testing.T) {
	fakeHelixBin := buildFakeHelixBin(t)

	sb, err := NewSandbox("testconcurrent", fakeHelixBin)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sb.Cleanup() })

	require.NoError(t, sb.Prepare("t1", "baseline"))
	require.NoError(t, sb.Prepare("t2", "native"))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	handles := make([]*DaemonHandle, 2)

	keys := []struct{ task, mode string }{{"t1", "baseline"}, {"t2", "native"}}
	for i, key := range keys {
		i, key := i, key
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, e := sb.StartDaemon(ctx, key.task, key.mode, "full", "")
			handles[i] = h
			errs[i] = e
		}()
	}
	wg.Wait()

	for i, e := range errs {
		assert.NoError(t, e, "goroutine %d should not error", i)
	}
	for _, h := range handles {
		if h != nil {
			_ = h.Kill()
		}
	}

	// Sockets must be distinct.
	assert.NotEqual(t, sb.SocketFor("t1", "baseline"), sb.SocketFor("t2", "native"))
}

// TestSandboxRefusesSymlinkedTmpdir verifies that checkNotSymlink rejects a
// path that is itself a symbolic link and accepts a real directory.
func TestSandboxRefusesSymlinkedTmpdir(t *testing.T) {
	realDir := t.TempDir()
	symlinkPath := filepath.Join(t.TempDir(), "symlink")
	require.NoError(t, os.Symlink(realDir, symlinkPath))

	err := checkNotSymlink(symlinkPath)
	assert.Error(t, err, "checkNotSymlink should reject a symlinked path")

	// A real directory must pass.
	assert.NoError(t, checkNotSymlink(realDir))
}

// buildFakeHelixBin compiles a small Go program that opens a Unix socket at the
// path specified via --socket=<path> arg (scanned manually to skip unknown
// flags like --serve, --profile), then blocks until killed.
func buildFakeHelixBin(t *testing.T) string {
	t.Helper()

	src := `package main

import (
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	// Scan argv manually to extract --socket= value; unknown flags are ignored.
	var socket string
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--socket=") {
			socket = strings.TrimPrefix(arg, "--socket=")
		}
	}

	if socket == "" {
		os.Exit(1)
	}

	_ = os.Remove(socket)

	l, err := net.Listen("unix", socket)
	if err != nil {
		os.Exit(1)
	}
	defer l.Close()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch
}
`
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	binFile := filepath.Join(dir, "fakehelix")

	require.NoError(t, os.WriteFile(srcFile, []byte(src), 0644))

	cmd := exec.Command("go", "build", "-o", binFile, srcFile)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	require.NoError(t, cmd.Run(), "build fake helix: %s", buf.String())

	return binFile
}

// TestFakeHelixBin verifies the fake helix binary can bind to a Unix socket.
func TestFakeHelixBin(t *testing.T) {
	fakeHelixBin := buildFakeHelixBin(t)

	sockPath := filepath.Join(t.TempDir(), "test.sock")
	cmd := exec.Command(fakeHelixBin, "--socket="+sockPath)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	// Wait for socket to appear.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	conn, err := net.DialTimeout("unix", sockPath, 1*time.Second)
	require.NoError(t, err)
	conn.Close()
}

// TestWithWorkingDirSetsWorkDir asserts the additive WithWorkingDir option folds
// into daemonOpts.workDir (D-03). A full daemon spawn is heavyweight, so this
// unit exercises the option-application contract directly; an empty dir is a
// no-op handled by StartDaemon's `if o.workDir != ""` guard.
func TestWithWorkingDirSetsWorkDir(t *testing.T) {
	var o daemonOpts
	WithWorkingDir("/x")(&o)
	assert.Equal(t, "/x", o.workDir)

	// Zero value stays empty when no option is applied.
	var z daemonOpts
	assert.Equal(t, "", z.workDir)
}
