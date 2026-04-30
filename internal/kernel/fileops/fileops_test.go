package fileops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
)

// --- ValidatePath tests ---

func TestValidatePathRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	_, err := ValidatePath(root, "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for path outside root")
	}
	if !errors.Is(err, serr.ErrInvalidArgs) {
		t.Fatalf("expected InvalidArgs error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "outside workspace root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	_, err := ValidatePath(root, "../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestValidatePathAcceptsValidPath(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0o644)

	resolved, err := ValidatePath(root, "test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// On macOS, t.TempDir() may return /var/... while EvalSymlinks gives /private/var/...
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	if !strings.HasPrefix(resolved, resolvedRoot) {
		t.Fatalf("resolved path %q not inside root %q", resolved, resolvedRoot)
	}
}

func TestValidatePathEmptyRoot(t *testing.T) {
	_, err := ValidatePath("", "test.txt")
	if err == nil {
		t.Fatal("expected error for empty root")
	}
	if !errors.Is(err, serr.ErrNoWorkspace) {
		t.Fatalf("expected NoWorkspace error, got: %v", err)
	}
}

// --- ReadFile tests ---

func TestReadFileExisting(t *testing.T) {
	root := t.TempDir()
	content := "hello world\nline 2\n"
	os.WriteFile(filepath.Join(root, "test.txt"), []byte(content), 0o644)

	got, err := ReadFile(root, "test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Fatalf("got %q, want %q", got, content)
	}
}

func TestReadFileNotFound(t *testing.T) {
	root := t.TempDir()
	_, err := ReadFile(root, "nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !errors.Is(err, serr.ErrNotFound) {
		t.Fatalf("expected NotFound error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadFileDirectory(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "subdir"), 0o755)

	_, err := ReadFile(root, "subdir")
	if err == nil {
		t.Fatal("expected error for directory")
	}
	if !errors.Is(err, serr.ErrInvalidArgs) {
		t.Fatalf("expected InvalidArgs error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ReadFileRange tests ---

func TestReadFileRangeValid(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(filepath.Join(root, "test.txt"), []byte(content), 0o644)

	got, err := ReadFileRange(root, "test.txt", 2, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "line2") || !strings.Contains(got, "line4") {
		t.Fatalf("expected lines 2-4, got: %s", got)
	}
	if strings.Contains(got, "line1") || strings.Contains(got, "line5") {
		t.Fatalf("should not contain lines outside range, got: %s", got)
	}
}

func TestReadFileRangeZeroEnd(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\nline3\n"
	os.WriteFile(filepath.Join(root, "test.txt"), []byte(content), 0o644)

	got, err := ReadFileRange(root, "test.txt", 2, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "line2") || !strings.Contains(got, "line3") {
		t.Fatalf("expected lines 2 to end, got: %s", got)
	}
	if strings.Contains(got, "line1") {
		t.Fatalf("should not contain line1, got: %s", got)
	}
}

func TestReadFileRangeOutOfBounds(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\n"
	os.WriteFile(filepath.Join(root, "test.txt"), []byte(content), 0o644)

	_, err := ReadFileRange(root, "test.txt", 100, 200)
	if err == nil {
		t.Fatal("expected error for out-of-bounds range")
	}
	if !errors.Is(err, serr.ErrInvalidArgs) {
		t.Fatalf("expected InvalidArgs error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "beyond end of file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- CreateFile tests ---

func TestCreateFileCreatesDirectories(t *testing.T) {
	root := t.TempDir()
	err := CreateFile(root, "a/b/c/test.txt", "content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "a", "b", "c", "test.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("got %q, want %q", string(data), "content")
	}
}

func TestCreateFileErrorsOnExisting(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "exists.txt"), []byte("old"), 0o644)

	err := CreateFile(root, "exists.txt", "new")
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !errors.Is(err, serr.ErrInvalidArgs) {
		t.Fatalf("expected InvalidArgs error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- OverwriteFile tests ---

func TestOverwriteFileAtomicWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "test.txt")
	os.WriteFile(path, []byte("old content"), 0o644)

	err := OverwriteFile(root, "test.txt", "new content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != "new content" {
		t.Fatalf("got %q, want %q", string(data), "new content")
	}
}

func TestOverwriteFileCreatesNew(t *testing.T) {
	root := t.TempDir()
	err := OverwriteFile(root, "new.txt", "content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "new.txt"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(data) != "content" {
		t.Fatalf("got %q, want %q", string(data), "content")
	}
}

// --- ListDirectory tests ---

func TestListDirectoryOrdering(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "zdir"), 0o755)
	os.MkdirAll(filepath.Join(root, "adir"), 0o755)
	os.WriteFile(filepath.Join(root, "zebra.txt"), []byte("z"), 0o644)
	os.WriteFile(filepath.Join(root, "apple.txt"), []byte("a"), 0o644)

	entries, err := ListDirectory(root, ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}

	// Dirs first, alphabetically
	if entries[0].Name != "adir" || !entries[0].IsDir {
		t.Fatalf("expected adir first, got %s (isDir=%v)", entries[0].Name, entries[0].IsDir)
	}
	if entries[1].Name != "zdir" || !entries[1].IsDir {
		t.Fatalf("expected zdir second, got %s", entries[1].Name)
	}
	// Files second, alphabetically
	if entries[2].Name != "apple.txt" || entries[2].IsDir {
		t.Fatalf("expected apple.txt third, got %s", entries[2].Name)
	}
	if entries[3].Name != "zebra.txt" || entries[3].IsDir {
		t.Fatalf("expected zebra.txt fourth, got %s", entries[3].Name)
	}
}

func TestListDirectoryNotFound(t *testing.T) {
	root := t.TempDir()
	_, err := ListDirectory(root, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
	if !errors.Is(err, serr.ErrNotFound) {
		t.Fatalf("expected NotFound error, got: %v", err)
	}
}

// --- FindFiles tests ---

func TestFindFilesGlob(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src", "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("pkg"), 0o644)
	os.WriteFile(filepath.Join(root, "src", "sub", "util.go"), []byte("pkg"), 0o644)
	os.WriteFile(filepath.Join(root, "readme.md"), []byte("#"), 0o644)

	results, err := FindFiles(root, "*.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 .go files, got %d: %v", len(results), results)
	}
}

func TestFindFilesDoublestar(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "a", "b", "c"), 0o755)
	os.WriteFile(filepath.Join(root, "top.go"), []byte("pkg"), 0o644)
	os.WriteFile(filepath.Join(root, "a", "mid.go"), []byte("pkg"), 0o644)
	os.WriteFile(filepath.Join(root, "a", "b", "c", "deep.go"), []byte("pkg"), 0o644)
	os.WriteFile(filepath.Join(root, "a", "b", "c", "deep.txt"), []byte("txt"), 0o644)

	results, err := FindFiles(root, "**/*.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should find mid.go, deep.go (** matches at least one segment in "**/*.go")
	goCount := 0
	for _, r := range results {
		if strings.HasSuffix(r, ".go") {
			goCount++
		}
	}
	if goCount < 2 {
		t.Fatalf("expected at least 2 .go files in subdirs, got %d: %v", goCount, results)
	}
}

func TestFindFilesSkipsGitDir(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(root, ".git", "config"), []byte("cfg"), 0o644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("pkg"), 0o644)

	results, err := FindFiles(root, "*.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range results {
		if strings.Contains(r, ".git") {
			t.Fatalf("should not include .git files: %s", r)
		}
	}
}

// --- SearchPattern tests ---

func TestSearchPatternRegex(t *testing.T) {
	root := t.TempDir()
	content := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
	os.WriteFile(filepath.Join(root, "main.go"), []byte(content), 0o644)

	results, err := SearchPattern(root, `fmt\.Println`, SearchOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}
	if results[0].Line != 2 {
		t.Fatalf("expected match on line 2, got line %d", results[0].Line)
	}
}

func TestSearchPatternContextLines(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\nTARGET\nline4\nline5\n"
	os.WriteFile(filepath.Join(root, "test.txt"), []byte(content), 0o644)

	results, err := SearchPattern(root, "TARGET", SearchOpts{ContextLines: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}
	if len(results[0].ContextBefore) != 1 || results[0].ContextBefore[0] != "line2" {
		t.Fatalf("expected 1 context before line, got %v", results[0].ContextBefore)
	}
	if len(results[0].ContextAfter) != 1 || results[0].ContextAfter[0] != "line4" {
		t.Fatalf("expected 1 context after line, got %v", results[0].ContextAfter)
	}
}

func TestSearchPatternSkipsBinary(t *testing.T) {
	root := t.TempDir()
	// Create a binary file with null bytes
	binaryContent := []byte("hello\x00world\x00binary")
	os.WriteFile(filepath.Join(root, "binary.bin"), binaryContent, 0o644)
	// Create a text file
	os.WriteFile(filepath.Join(root, "text.txt"), []byte("hello world"), 0o644)

	results, err := SearchPattern(root, "hello", SearchOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only find in text.txt, not binary.bin
	for _, r := range results {
		if strings.Contains(r.Path, "binary") {
			t.Fatalf("should not match in binary file: %s", r.Path)
		}
	}
}

func TestSearchPatternIncludeGlob(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.go"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(root, "main.py"), []byte("hello"), 0o644)

	results, err := SearchPattern(root, "hello", SearchOpts{IncludeGlob: "*.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 match (only .go), got %d", len(results))
	}
	if !strings.HasSuffix(results[0].Path, ".go") {
		t.Fatalf("expected .go file, got %s", results[0].Path)
	}
}

func TestSearchPatternInvalidRegex(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0o644)

	_, err := SearchPattern(root, "[invalid", SearchOpts{})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !errors.Is(err, serr.ErrInvalidArgs) {
		t.Fatalf("expected InvalidArgs error, got: %v", err)
	}
}

// --- ReplaceInFile tests ---

func TestReplaceInFileLiteral(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("foo bar foo baz foo"), 0o644)

	count, err := ReplaceInFile(root, "test.txt", "foo", "qux", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 replacements, got %d", count)
	}

	data, _ := os.ReadFile(filepath.Join(root, "test.txt"))
	if string(data) != "qux bar qux baz qux" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestReplaceInFileRegex(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("foo123 bar456 baz789"), 0o644)

	count, err := ReplaceInFile(root, "test.txt", `[a-z]+(\d+)`, "NUM$1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 replacements, got %d", count)
	}

	data, _ := os.ReadFile(filepath.Join(root, "test.txt"))
	if string(data) != "NUM123 NUM456 NUM789" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestReplaceInFileNoMatch(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello world"), 0o644)

	count, err := ReplaceInFile(root, "test.txt", "zzz", "xxx", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 replacements, got %d", count)
	}
}

func TestReplaceInFileInvalidRegex(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.txt"), []byte("hello"), 0o644)

	_, err := ReplaceInFile(root, "test.txt", "[invalid", "x", true)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !errors.Is(err, serr.ErrInvalidArgs) {
		t.Fatalf("expected InvalidArgs error, got: %v", err)
	}
}
