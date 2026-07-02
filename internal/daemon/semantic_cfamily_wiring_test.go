package daemon

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	cextract "github.com/agenthands/helix/internal/semantic/extract/c"
	cppextract "github.com/agenthands/helix/internal/semantic/extract/cpp"
	csharpextract "github.com/agenthands/helix/internal/semantic/extract/csharp"
	javaextract "github.com/agenthands/helix/internal/semantic/extract/java"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/treesitter"
	"github.com/agenthands/helix/internal/workspace"
)

// TestLangFromExt_CFamily is the Phase 135 B1 RED gate: langFromExt must map
// the C-family extensions to their canonical provider languages so the
// production index no longer drops C/C++/C#/Java files before extraction.
// Rust/Kotlin/PHP/Ruby are now WIRED for extraction (v2.13 Phase 140 D-BREADTH).
func TestLangFromExt_CFamily(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// Existing (regression guard — must not change).
		{"a.go", "go"},
		{"a.ts", "typescript"},
		{"a.tsx", "typescript"},
		{"a.js", "javascript"},
		{"a.jsx", "javascript"},
		{"a.py", "python"},
		// B1 net-new C-family.
		{"a.c", "c"},
		{"a.h", "c"},
		{"a.cpp", "cpp"},
		{"a.cc", "cpp"},
		{"a.cxx", "cpp"},
		{"a.hpp", "cpp"},
		{"a.hh", "cpp"},
		{"a.hxx", "cpp"},
		{"a.cs", "c_sharp"},
		{"a.java", "java"},
		// Case-insensitivity (langFromExt lowercases the ext).
		{"A.C", "c"},
		{"Foo.JAVA", "java"},
		// v2.13 Phase 140 (D-BREADTH) — now WIRED for extraction.
		{"a.rs", "rust"},
		{"a.kt", "kotlin"},
		{"a.kts", "kotlin"},
		{"a.php", "php"},
		{"a.rb", "ruby"},
		// Unknown.
		{"a.txt", ""},
		{"noext", ""},
	}
	for _, tc := range cases {
		if got := langFromExt(tc.path); got != tc.want {
			t.Errorf("langFromExt(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestLangFromExt_RegistryResolves guards the classifyAndExtract gate: every
// C-family language langFromExt now returns MUST resolve to a registered
// provider, else classifyAndExtract silently skips the file at the
// `provider, ok := b.extractRegistry.Provider(lang)` check.
func TestLangFromExt_RegistryResolves(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	registry := productionExtractRegistryForTest(grammars)
	for _, ext := range []string{"a.c", "a.h", "a.cpp", "a.cs", "a.java"} {
		lang := langFromExt(ext)
		if lang == "" {
			t.Fatalf("langFromExt(%q) unexpectedly empty", ext)
		}
		if _, ok := registry.Provider(lang); !ok {
			t.Errorf("registry has no provider for lang %q (from %q) — classifyAndExtract would skip", lang, ext)
		}
	}
}

// TestProductionBuildFn_ReachesCFile is the Phase 135 B1 proof (Task 2):
// a real .c fixture driven through makeProductionBuildFn yields >= 1 C
// symbol in the committed snapshot. Before B1, langFromExt("*.c")=="" so
// the file was dropped and zero C symbols were committed.
func TestProductionBuildFn_ReachesCFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping production buildFn integration test in -short mode (opens DuckDB)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsDir := t.TempDir()
	t.Chdir(wsDir)

	cSrc := "struct Point { int x; int y; };\n" +
		"int origin(struct Point* p) { return p->x + p->y; }\n"
	if err := os.WriteFile(filepath.Join(wsDir, "pt.c"), []byte(cSrc), 0o644); err != nil {
		t.Fatalf("write pt.c: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	storeCfg := semantic.Config{
		Enabled: true,
		Store: semantic.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	metrics := obs.Noop(logger.Handler()).Metrics()
	store, err := semanticstore.Open(ctx, storeCfg, logger, metrics)
	if err != nil {
		t.Fatalf("semanticstore.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	grammars := treesitter.NewGrammarRegistry()
	registry := extract.NewExtractorRegistry(
		grammars,
		cextract.NewProvider(grammars),
	)

	cfg := loadSemanticConfig()
	b := &semanticBundle{
		cfg:             cfg,
		store:           store,
		extractRegistry: registry,
		logger:          logger,
		metrics:         metrics,
	}

	buildFn := b.makeProductionBuildFn()
	wsKey := workspace.WorkspaceKey{RepoRoot: wsDir}
	st := &testBuildState{}
	result, err := buildFn(ctx, wsKey, "full", st)
	if err != nil {
		t.Fatalf("buildFn: %v", err)
	}
	if result.SnapshotID == 0 {
		t.Fatalf("IndexResult.SnapshotID == 0 — no committed snapshot")
	}

	repoID := wsKey.Hash()
	latest, err := store.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		t.Fatalf("LatestCommittedSnapshot: %v", err)
	}
	if latest == 0 {
		t.Fatalf("no committed snapshot for repo %q after buildFn", repoID)
	}
	// The workspace holds only the .c fixture, so every committed symbol
	// originates from it. Before B1, langFromExt("*.c")=="" dropped the
	// file and this set was empty.
	names := map[string]bool{}
	if err := store.IterateCommittedSymbols(ctx, latest, func(row semanticstore.SymbolRow) bool {
		names[row.Name] = true
		return true
	}); err != nil {
		t.Fatalf("IterateCommittedSymbols: %v", err)
	}
	if len(names) == 0 {
		t.Fatalf("committed symbols = 0, want >= 1 (B1: production index must reach the .c fixture)")
	}
	for _, want := range []string{"Point", "origin"} {
		if !names[want] {
			t.Errorf("committed C symbols missing %q (got %v) — B1 extraction incomplete", want, keysOf(names))
		}
	}
}

// TestFactsFromExtracted_CStructDedup is the Phase 135 dedup regression gate.
// A C file that DEFINES a struct and USES it as a type emits two symbols that
// canonicalize to one SymbolID (signatureHash truncates at `{`). Before the
// snapshot-level dedup this aborted WriteSnapshotFacts on the PK
// (snapshot_id, symbol_id). After it, factsFromExtracted must yield exactly
// ONE "Point" struct row, and the kept row must be the DEFINITION (its
// signature carries the body `{`), not the impoverished type-use.
func TestFactsFromExtracted_CStructDedup(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := cextract.NewProvider(grammars)
	src := "struct Point { int x; int y; };\n" +
		"int origin(struct Point* p) { return p->x + p->y; }\n"
	ef, err := p.Extract(context.Background(), []byte(src), extract.SourceFile{Path: "pt.c", Language: "c"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	facts := factsFromExtracted([]*extract.ExtractedFile{ef}, "r", "", nil, nil)

	// No two symbols may share a SymbolID (the store PK invariant).
	seen := map[uint64]int{}
	for _, s := range facts.Symbols {
		seen[s.SymbolID]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("SymbolID %d emitted %d times — PK collision would abort WriteSnapshotFacts", id, n)
		}
	}

	// Exactly one struct row named Point, and it is the definition (has body).
	var pointRows []semanticstore.SymbolFact
	for _, s := range facts.Symbols {
		if s.Kind == string(extract.KindStruct) && s.Name == "Point" {
			pointRows = append(pointRows, s)
		}
	}
	if len(pointRows) != 1 {
		t.Fatalf("expected exactly 1 Point struct row, got %d: %+v", len(pointRows), pointRows)
	}
	if !strings.Contains(pointRows[0].Signature, "{") {
		t.Errorf("kept Point row is not the definition (signature %q lacks a body) — richer-symbol tiebreak regressed", pointRows[0].Signature)
	}
	// The kept definition's NodeID is what any RESOLVES_TO target would bind to.
	if pointRows[0].NodeID == 0 {
		t.Errorf("kept Point definition has NodeID 0 — cannot be a resolution target")
	}
}

// productionExtractRegistryForTest mirrors the daemon-bootstrap provider set
// (daemon.go:376-388) for the C-family languages under test.
func productionExtractRegistryForTest(grammars *treesitter.GrammarRegistry) *extract.Registry {
	return extract.NewExtractorRegistry(
		grammars,
		cextract.NewProvider(grammars),
		cppextract.NewProvider(grammars),
		csharpextract.NewProvider(grammars),
		javaextract.NewProvider(grammars),
	)
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
