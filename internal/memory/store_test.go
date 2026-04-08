package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *MemoryStore {
	t.Helper()
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "project", "memories")
	globalDir := filepath.Join(dir, "global", "memories")
	dbPath := filepath.Join(dir, "index.db")

	store, err := NewMemoryStore(projectDir, globalDir, dbPath, nil)
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestWriteReadProject(t *testing.T) {
	store := newTestStore(t)

	content := "# Project Notes\n\nSome important info."
	if err := store.Write("notes", content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := store.Read("notes")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != content {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestWriteReadGlobal(t *testing.T) {
	store := newTestStore(t)

	content := "# Global Style Guide\n\nUse tabs."
	if err := store.Write("global/style_guide", content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := store.Read("global/style_guide")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != content {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestWriteReadWithMDExtension(t *testing.T) {
	store := newTestStore(t)

	if err := store.Write("readme.md", "# Hello"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := store.Read("readme.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "# Hello" {
		t.Errorf("content mismatch: got %q", got)
	}
}

func TestListAll(t *testing.T) {
	store := newTestStore(t)

	store.Write("foo", "content foo")
	store.Write("bar", "content bar")
	store.Write("global/baz", "content baz")

	entries, err := store.List("", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestListByScope(t *testing.T) {
	store := newTestStore(t)

	store.Write("local1", "content")
	store.Write("local2", "content")
	store.Write("global/g1", "content")

	entries, err := store.List("project", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 project entries, got %d", len(entries))
	}

	entries, err = store.List("global", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 global entry, got %d", len(entries))
	}
}

func TestSearchByContent(t *testing.T) {
	store := newTestStore(t)

	store.Write("auth_notes", "# Authentication\n\nJWT tokens with refresh rotation.")
	store.Write("db_notes", "# Database\n\nPostgreSQL with pgx driver.")

	entries, err := store.Search("JWT tokens", "")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected search results, got none")
	}
	if entries[0].Name != "auth_notes" {
		t.Errorf("expected auth_notes, got %q", entries[0].Name)
	}
}

func TestSearchByTitle(t *testing.T) {
	store := newTestStore(t)

	store.Write("guide", "# Deployment Guide\n\nSteps to deploy.")
	store.Write("other", "# Other\n\nUnrelated content.")

	entries, err := store.Search("Deployment", "")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected search results")
	}
	found := false
	for _, e := range entries {
		if e.Name == "guide" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'guide' in search results")
	}
}

func TestRename(t *testing.T) {
	store := newTestStore(t)

	store.Write("old_name", "# Old\n\nContent")

	if err := store.Rename("old_name", "new_name"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// Old name should not be readable.
	if _, err := store.Read("old_name"); err == nil {
		t.Error("expected error reading old name after rename")
	}

	got, err := store.Read("new_name")
	if err != nil {
		t.Fatalf("Read new_name: %v", err)
	}
	if !strings.Contains(got, "Content") {
		t.Errorf("renamed memory content missing expected text")
	}
}

func TestDelete(t *testing.T) {
	store := newTestStore(t)

	store.Write("to_delete", "# Delete Me")

	if err := store.Delete("to_delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := store.Read("to_delete"); err == nil {
		t.Error("expected error reading deleted memory")
	}

	entries, err := store.List("", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(entries))
	}
}

func TestEdit(t *testing.T) {
	store := newTestStore(t)

	store.Write("editable", "# Title\n\nOriginal content.")

	err := store.Edit("editable", func(content string) string {
		return strings.Replace(content, "Original", "Updated", 1)
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	got, err := store.Read("editable")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(got, "Updated content.") {
		t.Errorf("expected 'Updated content.' in %q", got)
	}
}

func TestRebuildIndex(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "project", "memories")
	globalDir := filepath.Join(dir, "global", "memories")
	dbPath := filepath.Join(dir, "index.db")

	// Create store and write some memories.
	store, err := NewMemoryStore(projectDir, globalDir, dbPath, nil)
	if err != nil {
		t.Fatalf("NewMemoryStore: %v", err)
	}
	store.Write("alpha", "# Alpha\n\nAlpha content with unique_marker_xyz.")
	store.Write("global/beta", "# Beta\n\nBeta content.")
	store.Close()

	// Open a fresh index and rebuild from files.
	idx, err := NewIndex(filepath.Join(dir, "index2.db"))
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	defer idx.Close()

	if err := idx.Rebuild(projectDir, globalDir); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	entries, err := idx.Search("unique_marker_xyz", "")
	if err != nil {
		t.Fatalf("Search after rebuild: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected to find 'alpha' after rebuild")
	}
}

func TestPathResolution(t *testing.T) {
	store := newTestStore(t)

	// "global/x" -> globalDir
	store.Write("global/x", "global content")
	_, gpath := store.resolve("global/x")
	if !strings.Contains(gpath, "global") {
		t.Errorf("global path should contain 'global': %s", gpath)
	}

	// "x" -> projectDir
	store.Write("x", "project content")
	scope, ppath := store.resolve("x")
	if scope != ScopeProject {
		t.Errorf("expected project scope, got %s", scope)
	}
	if strings.Contains(ppath, "global") {
		t.Errorf("project path should not contain 'global': %s", ppath)
	}

	// "a/b/c" -> nested projectDir
	store.Write("a/b/c", "nested content")
	_, npath := store.resolve("a/b/c")
	if !strings.HasSuffix(npath, filepath.Join("a", "b", "c.md")) {
		t.Errorf("nested path incorrect: %s", npath)
	}

	// Verify the file actually exists on disk.
	if _, err := os.Stat(npath); err != nil {
		t.Errorf("nested file not found on disk: %v", err)
	}
}
