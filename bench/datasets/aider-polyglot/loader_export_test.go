package aiderpolyglot

import (
	"path/filepath"
	"testing"
)

// fixtureGoWordy is the committed Go fixture exercise dir.
func fixtureGoWordy(t *testing.T) string {
	t.Helper()
	return filepath.Join("fixtures", "go", "exercises", "practice", "wordy")
}

// TestLoadExerciseExported proves the exported LoadExercise wrapper delegates to
// loadExercise verbatim: it loads the committed Go fixture, sets SrcDir to the
// passed dir (load-bearing for WR-01 restorePristineTests — RunExercise:243-249),
// and surfaces the config.json solution/example file lists. Hermetic: no network.
func TestLoadExerciseExported(t *testing.T) {
	dir := fixtureGoWordy(t)
	ex, err := LoadExercise(dir, "go")
	if err != nil {
		t.Fatalf("LoadExercise: %v", err)
	}
	if ex.SrcDir != dir {
		t.Fatalf("SrcDir = %q, want %q (load-bearing for WR-01)", ex.SrcDir, dir)
	}
	if ex.Language != "go" {
		t.Fatalf("Language = %q, want go", ex.Language)
	}
	if got := ex.Config.Files.Solution; len(got) != 1 || got[0] != "wordy.go" {
		t.Fatalf("Files.Solution = %v, want [wordy.go]", got)
	}
	if got := ex.Config.Files.Example; len(got) != 1 || got[0] != ".meta/example.go" {
		t.Fatalf("Files.Example = %v, want [.meta/example.go]", got)
	}
}

// TestNativeTestCommandExported proves the exported NativeTestCommand wrapper
// delegates verbatim: it returns the dataset's native per-language argv for
// known languages and fail-closes (non-nil error) for an unknown one.
func TestNativeTestCommandExported(t *testing.T) {
	goArgv, err := NativeTestCommand("go")
	if err != nil {
		t.Fatalf("NativeTestCommand(go): %v", err)
	}
	if len(goArgv) != 3 || goArgv[0] != "go" || goArgv[1] != "test" || goArgv[2] != "./..." {
		t.Fatalf("NativeTestCommand(go) = %v, want [go test ./...]", goArgv)
	}

	pyArgv, err := NativeTestCommand("python")
	if err != nil {
		t.Fatalf("NativeTestCommand(python): %v", err)
	}
	if len(pyArgv) != 1 || pyArgv[0] != "pytest" {
		t.Fatalf("NativeTestCommand(python) = %v, want [pytest]", pyArgv)
	}

	if _, err := NativeTestCommand("cobol"); err == nil {
		t.Fatal("NativeTestCommand(cobol) = nil error, want fail-closed for unknown language")
	}
}
