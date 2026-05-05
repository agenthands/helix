//go:build integration

package nokernel2semantic_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAnalyzer_RealTreeIsClean shells out to `go vet -vettool=...` against
// the actual ./internal/kernel/... package set and asserts exit-zero. This
// is the regression net for LIVE-07 invariant #1 ("internal/kernel/ does not
// import internal/semantic/...") on the live source tree, complementing the
// fixture-driven analysistest suite.
//
// Build-tag-gated (`integration`) so the unit-test path stays fast; runs
// under `go test -tags=integration ./internal/lint/nokernel2semantic/...`.
func TestAnalyzer_RealTreeIsClean(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}

	vettool := exec.Command("go", "env", "GOPATH")
	gopathBytes, err := vettool.Output()
	if err != nil {
		t.Fatalf("go env GOPATH: %v", err)
	}
	gopath := strings.TrimSpace(string(gopathBytes))

	binName := "vet-nokernel2semantic"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	vetPath := filepath.Join(gopath, "bin", binName)

	// Ensure the singlechecker is installed and current.
	install := exec.Command("go", "install", "./cmd/vet-nokernel2semantic")
	install.Dir = repoRoot
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("go install ./cmd/vet-nokernel2semantic failed: %v\n%s", err, out)
	}

	if _, err := os.Stat(vetPath); err != nil {
		t.Fatalf("singlechecker not at %s after install: %v", vetPath, err)
	}

	cmd := exec.Command("go", "vet", "-vettool="+vetPath, "./internal/kernel/...")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go vet -vettool=%s ./internal/kernel/... reported violations (LIVE-07 #1 broken):\n%s", vetPath, out)
	}
}

// findRepoRoot walks up from this file's directory until it finds a go.mod
// file, returning that directory.
func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
