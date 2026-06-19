package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDiscoverTasksSkipsHiddenDirs locks the leading-dot filter in discoverTasks
// (WR-05). A .hidden dir seeded alongside a real task dir must be skipped — not
// surfaced into the task set — because a leading-dot id would otherwise reach
// ExpandMatrix's validateMatrixID and hard-fail the ENTIRE expansion (it returns
// on the first bad id) before any legitimate task runs. A regression that
// dropped the !strings.HasPrefix(name, ".") filter would silently convert a
// benign .git/.DS_Store dir into a full-run abort; this test catches that.
func TestDiscoverTasksSkipsHiddenDirs(t *testing.T) {
	datasetsRoot := t.TempDir()
	const benchmark = "internal-toolbench"
	const language = "go"
	langDir := filepath.Join(datasetsRoot, benchmark, language)
	require.NoError(t, os.MkdirAll(langDir, 0o755))

	// A real task dir plus a leading-dot artifact dir that must be skipped.
	require.NoError(t, os.MkdirAll(filepath.Join(langDir, "IT-go-patch-apply-1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(langDir, ".hidden"), 0o755))

	tasks, err := discoverTasks(datasetsRoot, benchmark, []string{language})
	require.NoError(t, err)
	require.Equal(t, []string{"IT-go-patch-apply-1"}, tasks,
		"the leading-dot dir must be skipped and only the real task returned")
}
