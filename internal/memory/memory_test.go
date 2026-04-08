package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherCreateAndDelete(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "project")
	globalDir := filepath.Join(dir, "global")
	dbPath := filepath.Join(dir, "index.db")

	os.MkdirAll(projectDir, 0o755)
	os.MkdirAll(globalDir, 0o755)

	idx, err := NewIndex(dbPath)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	defer idx.Close()

	w, err := NewWatcher(idx, projectDir, globalDir, nil)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// Write a .md file directly to filesystem (not through store).
	filePath := filepath.Join(projectDir, "watcher_test.md")
	if err := os.WriteFile(filePath, []byte("# Watcher Test\n\nContent for watcher test with unique_watcher_token."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait for debounce + margin.
	time.Sleep(800 * time.Millisecond)

	// Verify index.Search finds the new file.
	entries, err := idx.Search("unique_watcher_token", "")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected watcher to index the new file, but search returned no results")
	}
	if entries[0].Name != "watcher_test" {
		t.Errorf("expected name 'watcher_test', got %q", entries[0].Name)
	}

	// Delete the file.
	if err := os.Remove(filePath); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Wait for debounce + margin.
	time.Sleep(800 * time.Millisecond)

	// Verify index no longer has it.
	entries, err = idx.Search("unique_watcher_token", "")
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(entries))
	}
}

func TestWatcherGlobalDir(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "project")
	globalDir := filepath.Join(dir, "global")
	dbPath := filepath.Join(dir, "index.db")

	os.MkdirAll(projectDir, 0o755)
	os.MkdirAll(globalDir, 0o755)

	idx, err := NewIndex(dbPath)
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	defer idx.Close()

	w, err := NewWatcher(idx, projectDir, globalDir, nil)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Run(ctx)

	// Write a .md file to global dir.
	filePath := filepath.Join(globalDir, "global_test.md")
	if err := os.WriteFile(filePath, []byte("# Global Note\n\nGlobal content."), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	time.Sleep(800 * time.Millisecond)

	entries, err := idx.List("global", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected watcher to index global file")
	}
	if entries[0].Name != "global/global_test" {
		t.Errorf("expected name 'global/global_test', got %q", entries[0].Name)
	}
}
