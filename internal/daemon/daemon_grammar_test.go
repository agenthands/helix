package daemon

import (
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/semantic"
	repomapSkill "github.com/agenthands/helix/internal/skill/repomap"
	"github.com/agenthands/helix/internal/treesitter"
)

// TestBootstrap_GrammarRegistrySingleton enforces the EXTRACT-05 / BUG-04
// invariant at runtime: every consumer that holds a *treesitter.GrammarRegistry
// MUST hold the exact same pointer the daemon constructed at step 6 of New().
//
// Why pointer-equality (not deep-equal): the registry caches compiled grammars
// behind unexported fields. Two registries with identical contents at construction
// time can diverge — one warm, the other cold — and produce inconsistent
// extraction. The invariant is "exactly one registry instance lives in the
// process," and pointer comparison is the most direct way to prove that.
//
// Consumers asserted (HEAD-available in this Wave-2 worktree):
//   - daemon.grammarRegistry (Daemon-owned singleton, returned via test accessor)
//   - extract.Registry.Grammars() (Phase 59 P02 — exported accessor)
//   - edit.BodyExtractor.registry (kernel edit pipeline — reflected, unexported)
//   - repomap.RepoMapSkill.registry (Phase 49 BUG-04 — reflected, unexported)
//
// Per-language providers (Phase 59 P04 — separate worktree) and the scheduler
// (Phase 59 P03 — separate worktree) are NOT yet in HEAD. When those land at
// merge time, this test gains additional pointer-equality assertions:
//   - goextract.Provider.grammars
//   - tsextract.Provider.grammars
//   - pyextract.Provider.grammars
//   - scheduler.Scheduler.<grammar-holder-field>
//
// Until then, the static source-grep test (TestNoNewGrammarRegistry in
// internal/semantic/extract/) covers what runtime cannot — any future provider
// that constructs its own registry trips the static gate.
func TestBootstrap_GrammarRegistrySingleton(t *testing.T) {
	// Reset the global repomap skill's registry pointer so this test owns the
	// SetRegistry call. The skill is a process singleton (Caddy-style init()
	// registration in internal/skill/repomap) and SetRegistry is idempotent;
	// without resetting, a previous test that constructed a daemon would
	// already have populated s.registry with a *different* GrammarRegistry,
	// turning this test's "all consumers point at the same singleton" assertion
	// into "all consumers point at a singleton, but maybe a previous test's."
	resetRepoMapSkillRegistry(t)

	wsDir := t.TempDir()
	// BL-01: store.Path must be workspace-relative; chdir into wsDir.
	t.Chdir(wsDir)
	dbPath := filepath.Join(".helix", "semantic.duckdb")

	cfg := &config.SerenaConfig{
		Profile: "full",
		Mode:    "edit",
		SemanticIndex: semantic.Config{
			Enabled: true,
			Store: semantic.StoreConfig{
				Kind:        "duckdb",
				Path:        dbPath,
				MemoryLimit: "256MiB",
				Threads:     2,
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New(cfg) error: %v", err)
	}

	daemonGrammar := d.GrammarRegistryForTest()
	if daemonGrammar == nil {
		t.Fatal("daemon.GrammarRegistryForTest() returned nil; expected the daemon-singleton")
	}

	// Consumer 1: extract.Registry exposes Grammars() exported.
	extractRegistry := d.SemanticExtractRegistryForTest()
	if extractRegistry == nil {
		t.Fatal("daemon.SemanticExtractRegistryForTest() returned nil; cfg.SemanticIndex.Enabled was true")
	}
	extractRegistryGrammar := extractRegistry.Grammars()
	if extractRegistryGrammar != daemonGrammar {
		t.Errorf("extract.Registry.Grammars() pointer mismatch: got %p, want %p (EXTRACT-05 violation: extract registry holds a different *GrammarRegistry than the daemon)",
			extractRegistryGrammar, daemonGrammar)
	}

	// Consumer 2: edit.BodyExtractor — registry is unexported; use reflection.
	bodyExtractorAny := d.BodyExtractorForTest()
	if bodyExtractorAny == nil {
		t.Fatal("daemon.BodyExtractorForTest() returned nil")
	}
	bodyExtractorGrammar := extractGrammarRegistryFieldByName(t, bodyExtractorAny, "registry")
	if bodyExtractorGrammar != daemonGrammar {
		t.Errorf("edit.BodyExtractor.registry pointer mismatch: got %p, want %p (EXTRACT-05 violation: body extractor holds a different *GrammarRegistry than the daemon)",
			bodyExtractorGrammar, daemonGrammar)
	}

	// Consumer 3: repomap.RepoMapSkill — registry is unexported, set via SetRegistry
	// during daemon bootstrap step 12a. The skill is a global singleton so we fetch
	// it via the package accessor and reflect into its registry field.
	repomapInst := repomapSkill.GetRepoMapSkill()
	if repomapInst == nil {
		t.Fatal("repomapSkill.GetRepoMapSkill() returned nil; daemon bootstrap should have initialized it")
	}
	repomapGrammar := extractGrammarRegistryFieldByName(t, repomapInst, "registry")
	if repomapGrammar != daemonGrammar {
		t.Errorf("repomap.RepoMapSkill.registry pointer mismatch: got %p, want %p (EXTRACT-05 violation: repomap skill holds a different *GrammarRegistry than the daemon)",
			repomapGrammar, daemonGrammar)
	}
}

// resetRepoMapSkillRegistry clears the global repomap skill's registry field
// via reflection so a fresh daemon-singleton injection takes effect. The
// repomap skill is a Caddy-style init() singleton and SetRegistry is
// idempotent; resetting between tests keeps the EXTRACT-05 assertion testing
// the *current* daemon's registry rather than whichever registry the first
// test happened to inject.
func resetRepoMapSkillRegistry(t *testing.T) {
	t.Helper()
	rs := repomapSkill.GetRepoMapSkill()
	if rs == nil {
		return
	}
	v := reflect.ValueOf(rs).Elem()
	for _, field := range []string{"registry", "elider", "extractor"} {
		f := v.FieldByName(field)
		if !f.IsValid() {
			continue
		}
		ptr := unsafe.Pointer(f.UnsafeAddr())
		// Reset the pointer-typed field to nil. All three are interface or
		// pointer types in RepoMapSkill, so writing a nil pointer of their
		// element type clears them.
		switch f.Kind() {
		case reflect.Ptr:
			*(*unsafe.Pointer)(ptr) = nil
		case reflect.Interface:
			// Two-word interface; zero both words.
			*(*[2]unsafe.Pointer)(ptr) = [2]unsafe.Pointer{}
		}
	}
}

// extractGrammarRegistryFieldByName uses reflection to read an unexported
// *treesitter.GrammarRegistry field by name on the supplied struct pointer.
// Used by the EXTRACT-05 regression test to inspect consumer types whose
// registry field is unexported.
//
// reflect's CanInterface() refuses unexported fields directly; we use
// unsafe.Pointer to bypass — this is acceptable in a regression-test path
// where the alternative is to break encapsulation by exporting a field
// purely for testing.
func extractGrammarRegistryFieldByName(t *testing.T, holder interface{}, fieldName string) *treesitter.GrammarRegistry {
	t.Helper()
	v := reflect.ValueOf(holder)
	if v.Kind() != reflect.Ptr {
		t.Fatalf("extractGrammarRegistryFieldByName: holder is %s, want pointer", v.Kind())
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		t.Fatalf("extractGrammarRegistryFieldByName: holder.Elem is %s, want struct", elem.Kind())
	}
	field := elem.FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("extractGrammarRegistryFieldByName: %T has no field named %q", holder, fieldName)
	}
	// Bypass the unexported-field guard via unsafe.
	ptr := unsafe.Pointer(field.UnsafeAddr())
	registryPtr := *(**treesitter.GrammarRegistry)(ptr)
	return registryPtr
}
