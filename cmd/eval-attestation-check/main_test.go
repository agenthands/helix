package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeEval writes a synthetic EVAL.md with the given Verified-at date to a
// temp dir and returns the path.
func writeEval(t *testing.T, dateStr string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "EVAL.md")
	body := fmt.Sprintf(`# Helix Evaluation Harness

Lorem ipsum.

## Provider Retention Attestation

**Verified at:** %s (synthetic)
**Re-verification cadence:** Quarterly
`, dateStr)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestRunFreshDate(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	path := writeEval(t, today)

	var buf bytes.Buffer
	code := run(&buf, []string{path})
	if code != 0 {
		t.Errorf("exit code = %d; want 0", code)
	}
	if buf.Len() != 0 {
		t.Errorf("stderr should be empty for fresh date; got: %q", buf.String())
	}
}

func TestRunStaleDate(t *testing.T) {
	stale := time.Now().UTC().AddDate(0, 0, -300).Format("2006-01-02")
	path := writeEval(t, stale)

	var buf bytes.Buffer
	code := run(&buf, []string{path})
	if code != 0 {
		t.Errorf("exit code = %d; want 0 (warn-only); stderr=%q", code, buf.String())
	}
	if !strings.Contains(buf.String(), "WARNING") {
		t.Errorf("stderr should contain WARNING; got: %q", buf.String())
	}
}

func TestRunMissingFile(t *testing.T) {
	var buf bytes.Buffer
	code := run(&buf, []string{"/no/such/path/EVAL.md"})
	if code != 0 {
		t.Errorf("exit code = %d; want 0 (warn-only)", code)
	}
	if !strings.Contains(buf.String(), "ERROR") {
		t.Errorf("stderr should contain ERROR; got: %q", buf.String())
	}
}

func TestRunMissingDateLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "EVAL.md")
	if err := os.WriteFile(path, []byte("# Title\n\nNo verified-at line here.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	code := run(&buf, []string{path})
	if code != 0 {
		t.Errorf("exit code = %d; want 0 (warn-only)", code)
	}
	if !strings.Contains(buf.String(), "no '**Verified at:**' line") {
		t.Errorf("stderr should mention missing line; got: %q", buf.String())
	}
}

func TestRunStrictStaleDate(t *testing.T) {
	stale := time.Now().UTC().AddDate(0, 0, -300).Format("2006-01-02")
	path := writeEval(t, stale)

	var buf bytes.Buffer
	code := run(&buf, []string{"--strict", path})
	if code != 2 {
		t.Errorf("exit code = %d; want 2 (strict + stale)", code)
	}
}

func TestRunStrictMissingFile(t *testing.T) {
	var buf bytes.Buffer
	code := run(&buf, []string{"--strict", "/no/such/file"})
	if code != 2 {
		t.Errorf("exit code = %d; want 2 (strict + missing)", code)
	}
}

func TestRunStrictFresh(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	path := writeEval(t, today)

	var buf bytes.Buffer
	code := run(&buf, []string{"--strict", path})
	if code != 0 {
		t.Errorf("exit code = %d; want 0 (strict + fresh = pass)", code)
	}
}
