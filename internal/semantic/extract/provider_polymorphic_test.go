// Package extract_test exercises the EXTRACT-public surface as an external
// consumer would — i.e. binding against the extract.Provider interface and
// the daemon-style Registry, without any concrete-type assertions on the
// per-language *Provider structs.
//
// This file is the RED gate for Phase 59 D-06: it MUST fail to compile (or
// fail at runtime) against the pre-D-06 extract.Provider interface, which
// does NOT yet declare Extract(...). After D-06 lands (Task 2), all tests
// in this file pass — proving Phase 65's locked call site at
// internal/daemon/semantic_wiring.go:687-742 can dispatch through the
// interface without per-language switch.
package extract_test

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
	pyextract "github.com/agenthands/helix/internal/semantic/extract/python"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
	tsextract "github.com/agenthands/helix/internal/semantic/extract/typescript"
)

// Compile-time interface-shape guard (Test 4 of the plan): each concrete
// *Provider must satisfy the widened extract.Provider interface. If any
// concrete provider stops satisfying the contract, this file fails to
// compile. Mirrors the pattern at internal/daemon/semantic_wiring.go.
var (
	_ extract.Provider = (*goextract.Provider)(nil)
	_ extract.Provider = (*tsextract.Provider)(nil)
	_ extract.Provider = (*pyextract.Provider)(nil)
)

// mustGetProvider does the (Provider, bool) lookup against the registry,
// t.Fatalf on missing. Returns the result typed as the *interface*, not
// the concrete struct — the entire point of D-06 is that callers never
// need a concrete type.
func mustGetProvider(t *testing.T, r *extract.Registry, lang string) extract.Provider {
	t.Helper()
	p, ok := r.Provider(lang)
	if !ok {
		t.Fatalf("registry.Provider(%q): not registered", lang)
	}
	return p
}

// buildRegistry constructs a *extract.Registry with all three first-class
// providers wired in, using the test-helper GrammarRegistry. Mirrors the
// shape Phase 59 P05 / Phase 65 daemon wiring will use in production.
func buildRegistry(t *testing.T) *extract.Registry {
	t.Helper()
	grammars := testutil.NewTestRegistry(t)
	return extract.NewExtractorRegistry(
		grammars,
		goextract.NewProvider(grammars),
		tsextract.NewProvider(grammars),
		pyextract.NewProvider(grammars),
	)
}

// TestProvider_PolymorphicExtract_Go exercises the locked Phase 65 call
// site shape: lookup → polymorphic Extract → inspect ExtractedFile.
// Pre-D-06: fails to compile (Provider interface lacks Extract).
// Post-D-06: passes.
func TestProvider_PolymorphicExtract_Go(t *testing.T) {
	registry := buildRegistry(t)

	provider, ok := registry.Provider("go")
	if !ok {
		t.Fatalf("registry.Provider(\"go\"): not registered")
	}

	src := []byte("package main\n\nfunc Hello() {}\n")
	extracted, err := provider.Extract(
		context.Background(),
		src,
		extract.SourceFile{Path: "test.go", Language: "go"},
	)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if extracted == nil {
		t.Fatalf("Extract returned nil ExtractedFile")
	}
	if extracted.File.Path != "test.go" {
		t.Errorf("File.Path = %q, want %q", extracted.File.Path, "test.go")
	}
	if extracted.File.Language != "go" {
		t.Errorf("File.Language = %q, want %q", extracted.File.Language, "go")
	}
	if len(extracted.Symbols) < 1 {
		t.Errorf("Symbols: got %d, want >= 1 (Hello function)", len(extracted.Symbols))
	}
	foundHello := false
	for _, s := range extracted.Symbols {
		if s.Name == "Hello" {
			foundHello = true
			break
		}
	}
	if !foundHello {
		t.Errorf("symbol Hello not found in extracted.Symbols: %+v", extracted.Symbols)
	}
}

// TestProvider_PolymorphicExtract_NoTypeAssertion is the RED gate proper:
// the variable `p` is typed against the *interface* extract.Provider, not
// the concrete *goextract.Provider. Pre-D-06 the interface has no Extract
// method, so the line `p.Extract(...)` fails to compile with
//
//	p.Extract undefined (type extract.Provider has no field or method Extract)
//
// Post-D-06 it compiles and runs successfully.
func TestProvider_PolymorphicExtract_NoTypeAssertion(t *testing.T) {
	registry := buildRegistry(t)

	// Bind explicitly against the *interface*, not the concrete type.
	// This is the load-bearing line for the RED gate — concrete-type
	// assertion would defeat the test.
	var p extract.Provider = mustGetProvider(t, registry, "go")

	src := []byte("package main\n\nfunc World() {}\n")
	extracted, err := p.Extract(
		context.Background(),
		src,
		extract.SourceFile{Path: "world.go", Language: "go"},
	)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if extracted == nil {
		t.Fatalf("Extract returned nil")
	}
	if extracted.File.Language != "go" {
		t.Errorf("Language = %q, want go", extracted.File.Language)
	}
}

// TestProvider_PolymorphicExtract_AllFirstClass table-tests the
// polymorphic dispatch across go, typescript, python — all three
// first-class providers must respond to .Extract through the interface.
func TestProvider_PolymorphicExtract_AllFirstClass(t *testing.T) {
	registry := buildRegistry(t)

	cases := []struct {
		lang   string
		path   string
		source string
	}{
		{lang: "go", path: "test.go", source: "package main\n\nfunc Hello() {}\n"},
		{lang: "typescript", path: "test.ts", source: "export function hello(): void {}\n"},
		{lang: "python", path: "test.py", source: "def hello():\n    pass\n"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.lang, func(t *testing.T) {
			// Interface-typed lookup — no concrete assertion.
			var p extract.Provider = mustGetProvider(t, registry, tc.lang)

			extracted, err := p.Extract(
				context.Background(),
				[]byte(tc.source),
				extract.SourceFile{Path: tc.path, Language: tc.lang},
			)
			if err != nil {
				t.Fatalf("Extract(%s): %v", tc.lang, err)
			}
			if extracted == nil {
				t.Fatalf("Extract(%s) returned nil", tc.lang)
			}
			if extracted.File.Language != tc.lang {
				t.Errorf("File.Language = %q, want %q", extracted.File.Language, tc.lang)
			}
			if extracted.File.Path != tc.path {
				t.Errorf("File.Path = %q, want %q", extracted.File.Path, tc.path)
			}
		})
	}
}

// TestProvider_InterfaceShape is a runtime-side anchor for the
// compile-time `var _ extract.Provider = ...` declarations at the top of
// this file. The compile-time guards do the real enforcement; this test
// merely exists so a failure mode (struct removed / renamed) surfaces
// with a discoverable test name in addition to the compile error.
func TestProvider_InterfaceShape(t *testing.T) {
	grammars := testutil.NewTestRegistry(t)

	// Each NewProvider returns extract.Provider (interface). The fact
	// that these lines compile is the contract. The runtime-level
	// non-nil check below catches a constructor that silently returns
	// nil (which would not be caught by the compile-time guards).
	providers := []struct {
		lang string
		p    extract.Provider
	}{
		{lang: "go", p: goextract.NewProvider(grammars)},
		{lang: "typescript", p: tsextract.NewProvider(grammars)},
		{lang: "python", p: pyextract.NewProvider(grammars)},
	}

	for _, pr := range providers {
		if pr.p == nil {
			t.Errorf("NewProvider(%s) returned nil interface value", pr.lang)
			continue
		}
		// Each provider must self-identify with the language passed
		// to its registry key. Hits LanguageMetadata.Language() via
		// the interface, not the concrete struct.
		if pr.p.Language() != pr.lang {
			t.Errorf("provider %s: Language() = %q, want %q",
				pr.lang, pr.p.Language(), pr.lang)
		}
	}
}
