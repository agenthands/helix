package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestHelp asserts the cobra root runs `--help` in-process with no error and
// prints usage text (criterion #1a).
func TestHelp(t *testing.T) {
	root := newRootCmd()
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "helix-bench-rag") {
		t.Fatalf("--help output missing command name; got:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "usage") {
		t.Fatalf("--help output missing usage section; got:\n%s", out)
	}
}

// TestToolListIsExactlyFour asserts the server registers EXACTLY the 4 named
// tools — no ping/echo/activate_project (Pitfall 5 / criterion #1b).
func TestToolListIsExactlyFour(t *testing.T) {
	root := t.TempDir()
	srv, err := buildServer(newStubIndex(), root)
	if err != nil {
		t.Fatalf("buildServer: %v", err)
	}
	got := srv.ToolNames()
	sort.Strings(got)
	want := []string{"grep", "rag_read_chunk", "rag_search", "read_file"}
	if len(got) != len(want) {
		t.Fatalf("tool count = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tool set = %v, want %v", got, want)
		}
	}
}

// TestPathTraversalRejected asserts read_file/grep/rag_read_chunk reject `..`
// and absolute-path escapes outside the corpus root (T-83-02-01).
func TestPathTraversalRejected(t *testing.T) {
	root := t.TempDir()
	// A legitimate in-corpus file so the failure is the path check, not absence.
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	bad := []string{
		"../etc/passwd",
		"../../etc/passwd",
		"/etc/passwd",
		"a/../../etc/passwd",
	}

	srv := &handlers{idx: newStubIndex(), root: root}
	ctx := context.Background()

	for _, p := range bad {
		if _, err := srv.readFile(p); err == nil {
			t.Errorf("read_file(%q) = nil error, want rejection", p)
		}
		if _, err := srv.grep("package", p); err == nil {
			t.Errorf("grep(_, %q) = nil error, want rejection", p)
		}
		// rag_read_chunk takes a chunk id "<relPath>#<ordinal>"; an escaping
		// relPath must be rejected the same way.
		if _, err := srv.readChunk(ctx, p+"#0"); err == nil {
			t.Errorf("rag_read_chunk(%q#0) = nil error, want rejection", p)
		}
	}

	// A legitimate relative path must NOT be rejected by the validator.
	if _, err := srv.readFile("a.go"); err != nil {
		t.Errorf("read_file(\"a.go\") = %v, want success", err)
	}
}
