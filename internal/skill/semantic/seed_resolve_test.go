package semantic

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// fakeSymbolByName is a recorder-free, table-injection mock for the
// SymbolByNameAccessor seam used by resolveSeed. Tests assign Candidates
// (or Err) before invoking the helper.
type fakeSymbolByName struct {
	candidates []integ.SymbolID
	err        error
	calls      int
}

func (f *fakeSymbolByName) QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]integ.SymbolID, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.candidates, nil
}

// newSkillWithSymbolByName returns a SemanticSkill with the SymbolByName
// seam injected. No other accessors are wired (resolveSeed never needs them).
func newSkillWithSymbolByName(t *testing.T, sym *fakeSymbolByName) *SemanticSkill {
	t.Helper()
	s := &SemanticSkill{}
	s.SetSymbolByName(sym)
	return s
}

func TestSeedResolve_SymbolID(t *testing.T) {
	// SymbolID present → short-circuit (accessor never invoked).
	sym := &fakeSymbolByName{}
	s := newSkillWithSymbolByName(t, sym)
	got, err := s.resolveSeed(context.Background(), workspace.WorkspaceKey{}, SeedInput{SymbolID: "sym_abc"})
	if err != nil {
		t.Fatalf("resolveSeed: %v", err)
	}
	if got.Resolution != ResolutionExact {
		t.Errorf("got Resolution=%q, want exact", got.Resolution)
	}
	if got.SymbolID != "sym_abc" {
		t.Errorf("got SymbolID=%q, want sym_abc", got.SymbolID)
	}
	if got.AmbiguousCandidates != nil {
		t.Errorf("got AmbiguousCandidates=%v, want nil", got.AmbiguousCandidates)
	}
	if sym.calls != 0 {
		t.Errorf("accessor called %d times, want 0 (short-circuit)", sym.calls)
	}
}

func TestSeedResolve_FileAndName_Exact(t *testing.T) {
	sym := &fakeSymbolByName{candidates: []integ.SymbolID{"only_one"}}
	s := newSkillWithSymbolByName(t, sym)
	got, err := s.resolveSeed(context.Background(), workspace.WorkspaceKey{},
		SeedInput{FilePath: "src/a.go", SymbolName: "Alpha"})
	if err != nil {
		t.Fatalf("resolveSeed: %v", err)
	}
	if got.Resolution != ResolutionExact {
		t.Errorf("got Resolution=%q, want exact", got.Resolution)
	}
	if got.SymbolID != "only_one" {
		t.Errorf("got SymbolID=%q, want only_one", got.SymbolID)
	}
	if got.AmbiguousCandidates != nil {
		t.Errorf("got AmbiguousCandidates=%v, want nil", got.AmbiguousCandidates)
	}
}

func TestSeedResolve_FileAndName_Ambiguous(t *testing.T) {
	sym := &fakeSymbolByName{candidates: []integ.SymbolID{"a", "b", "c"}}
	s := newSkillWithSymbolByName(t, sym)
	got, err := s.resolveSeed(context.Background(), workspace.WorkspaceKey{},
		SeedInput{FilePath: "src/a.go", SymbolName: "Foo"})
	if err != nil {
		t.Fatalf("resolveSeed: %v", err)
	}
	if got.Resolution != ResolutionAmbiguous {
		t.Errorf("got Resolution=%q, want ambiguous", got.Resolution)
	}
	if got.SymbolID != "a" {
		t.Errorf("got primary SymbolID=%q, want a (first candidate)", got.SymbolID)
	}
	if len(got.AmbiguousCandidates) != 3 {
		t.Errorf("got %d ambiguous, want 3", len(got.AmbiguousCandidates))
	}
}

func TestSeedResolve_FileAndName_AmbiguousCappedAtFive(t *testing.T) {
	// 6 candidates → resolveSeed must cap AmbiguousCandidates to 5.
	sym := &fakeSymbolByName{candidates: []integ.SymbolID{"a", "b", "c", "d", "e", "f"}}
	s := newSkillWithSymbolByName(t, sym)
	got, err := s.resolveSeed(context.Background(), workspace.WorkspaceKey{},
		SeedInput{FilePath: "src/a.go", SymbolName: "Foo"})
	if err != nil {
		t.Fatalf("resolveSeed: %v", err)
	}
	if got.Resolution != ResolutionAmbiguous {
		t.Errorf("got Resolution=%q, want ambiguous", got.Resolution)
	}
	if len(got.AmbiguousCandidates) != 5 {
		t.Errorf("got %d ambiguous, want exactly 5 (D1 cap)", len(got.AmbiguousCandidates))
	}
	if got.SymbolID != "a" {
		t.Errorf("got primary SymbolID=%q, want a", got.SymbolID)
	}
}

func TestSeedResolve_FileAndName_NotFound(t *testing.T) {
	sym := &fakeSymbolByName{candidates: nil}
	s := newSkillWithSymbolByName(t, sym)
	got, err := s.resolveSeed(context.Background(), workspace.WorkspaceKey{},
		SeedInput{FilePath: "src/missing.go", SymbolName: "Ghost"})
	if err != nil {
		t.Fatalf("resolveSeed: %v", err)
	}
	if got.Resolution != ResolutionNotFound {
		t.Errorf("got Resolution=%q, want not_found", got.Resolution)
	}
	if got.SymbolID != "" {
		t.Errorf("got SymbolID=%q, want zero value", got.SymbolID)
	}
}

func TestSeedResolve_MissingBoth(t *testing.T) {
	sym := &fakeSymbolByName{}
	s := newSkillWithSymbolByName(t, sym)
	_, err := s.resolveSeed(context.Background(), workspace.WorkspaceKey{}, SeedInput{})
	if err == nil {
		t.Fatalf("got nil error, want invalid-args error on empty SeedInput")
	}
	if !strings.Contains(err.Error(), "symbol_id") || !strings.Contains(err.Error(), "file_path") || !strings.Contains(err.Error(), "symbol_name") {
		t.Errorf("error message=%q does not mention 'symbol_id OR (file_path AND symbol_name)'", err.Error())
	}
}

func TestSeedResolve_FilePathOnly(t *testing.T) {
	// File path supplied but symbol_name missing → invalid args.
	sym := &fakeSymbolByName{}
	s := newSkillWithSymbolByName(t, sym)
	_, err := s.resolveSeed(context.Background(), workspace.WorkspaceKey{},
		SeedInput{FilePath: "src/a.go"})
	if err == nil {
		t.Fatalf("got nil error, want invalid-args error for file_path-only input")
	}
}

func TestSeedResolve_AccessorError(t *testing.T) {
	wantErr := errors.New("db blew up")
	sym := &fakeSymbolByName{err: wantErr}
	s := newSkillWithSymbolByName(t, sym)
	_, err := s.resolveSeed(context.Background(), workspace.WorkspaceKey{},
		SeedInput{FilePath: "src/a.go", SymbolName: "Alpha"})
	if err == nil {
		t.Fatalf("got nil error, want propagated accessor error")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got err=%v, want wrapping %v", err, wantErr)
	}
}
