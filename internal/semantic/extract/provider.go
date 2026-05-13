package extract

import (
	"context"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// LanguageMetadata is the lookup-side surface — language identifier,
// file extensions, and LSP-enrichment opt-in. Carved off Provider so
// future helper sub-types can attach without churning every consumer.
// Phase 65 callers should type against Provider not LanguageMetadata
// unless they have a concrete reason to narrow.
type LanguageMetadata interface {
	// Language returns the canonical language identifier (e.g. "go",
	// "typescript", "python"). Used as the Registry key. MUST be stable
	// across daemon restarts.
	Language() string

	// Extensions lists the file extensions claimed by this provider
	// (e.g. ".go" or ".ts"). Used by the scheduler to dispatch source
	// files to the right provider when language is not pre-classified.
	Extensions() []string

	// SupportsLSPEnrichment indicates whether Phase 61's enrichment
	// worker may attempt to merge this language's facts with LSP
	// callbacks. Languages whose LSP coverage is unstable can return
	// false to opt out.
	SupportsLSPEnrichment() bool
}

// ExtractionPipeline is the per-file extraction surface. Promoted to
// interface level in the 2026-05-08 update (D-06) so Phase 65's
// production buildFn at internal/daemon/semantic_wiring.go can call
// provider.Extract(...) polymorphically without per-language switch.
//
// The signature mirrors the existing concrete Extract methods in
// internal/semantic/extract/{golang,typescript,python}/provider.go
// byte-for-byte; promotion is a contract widening, not a behavioral
// change.
type ExtractionPipeline interface {
	// Extract parses the source bytes for `file` and returns the
	// resulting *ExtractedFile. Errors are reserved for unrecoverable
	// extraction-engine failures; per-file partial outcomes are
	// signalled via ExtractedFile.Partial / PartialReason instead so
	// Phase 65's production buildFn can record file-level partials
	// without losing the rest of the workspace walk.
	Extract(ctx context.Context, source []byte, file SourceFile) (*ExtractedFile, error)

	// ExtractFile reads `path` from disk and returns the per-file
	// *ExtractedFile. Phase 68 D-03 contract: synchronous in-tx
	// invocation from the live event handler — called AFTER
	// UpsertOverlayFile and BEFORE tx.Commit so the precise
	// FileFactDiff is populated against the same connection-bound
	// transaction that holds the overlay snapshot.
	//
	// Per-file I/O and partial conditions surface via
	// ExtractedFile.File.ExtractionStatus / PartialReason, mirroring
	// Extract's existing contract — the returned error is reserved
	// for unrecoverable extraction-engine failures.
	ExtractFile(ctx context.Context, repoID, path string) (*ExtractedFile, error)
}

// Provider mirrors SPEC §13.3 LanguageProvider — the surface a
// per-language extraction provider exposes to the daemon-owned
// Registry. Composes LanguageMetadata + ExtractionPipeline plus the
// tree-sitter and queries accessors that are intimate to the
// provider's internals.
//
// Phase 59 Wave 1 froze the lookup-side surface (Language, Extensions,
// TreeSitterLanguage, Queries, SupportsLSPEnrichment). The 2026-05-08
// update (D-06) promotes Extract from a concrete-only method to an
// interface method so Phase 65's locked call site at
// internal/daemon/semantic_wiring.go:687-742 can dispatch
// polymorphically through registry.Provider(lang).Extract(...).
//
// The deferred helper sub-types from the original P02 design
// (ImportResolver, ScopeBuilder, SymbolNormalizer,
// ReferenceClassifier, QueryBundle) stay deferred — they do not
// exist as concrete types yet and adding them speculatively would
// lock in shapes ahead of need.
type Provider interface {
	LanguageMetadata
	ExtractionPipeline

	// TreeSitterLanguage returns the tree-sitter language pointer the
	// provider parses with. The pointer is owned by the daemon-injected
	// *treesitter.GrammarRegistry singleton (BUG-04 invariant); the
	// provider does NOT construct or cache its own grammar registry.
	TreeSitterLanguage() *tree_sitter.Language

	// Queries returns the raw embedded queries.scm text for this
	// language. Compilation to *tree_sitter.Query is done once at
	// provider construction time and cached on the concrete struct
	// (Pitfall #2 in 59-RESEARCH.md).
	Queries() string
}
