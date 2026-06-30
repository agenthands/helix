package extract

import (
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/classifier"
	"github.com/agenthands/helix/internal/semantic/minhash"
	"github.com/agenthands/helix/internal/semantic/relatedidx"
)

// SymbolKind enumerates the symbol categories Phase 59's tree-sitter
// providers emit. Closed enum (D-01b): consumers may rely on exhaustive
// case discrimination.
type SymbolKind string

const (
	KindFunction       SymbolKind = "function"
	KindMethod         SymbolKind = "method"
	KindStruct         SymbolKind = "struct"
	KindClass          SymbolKind = "class"
	KindInterface      SymbolKind = "interface"
	KindEnum           SymbolKind = "enum"
	KindType           SymbolKind = "type"
	KindVariable       SymbolKind = "variable"
	KindConstant       SymbolKind = "constant"
	KindParameter      SymbolKind = "parameter"
	KindField          SymbolKind = "field"
	KindRoute          SymbolKind = "route"
	KindResource       SymbolKind = "resource"
	KindExternalModule SymbolKind = "external_module"
)

// ReferenceKind enumerates the reference categories Phase 59 emits.
// (CALL|REFERENCES|USES_TYPE|READS|WRITES|...). Concrete values land
// alongside per-language providers in Phase 59 P04; the type alias is
// the load-bearing contract.
type ReferenceKind string

// ExtractionStatus is the per-file extraction-outcome enum (D-05).
type ExtractionStatus string

const (
	ExtractionStatusReady       ExtractionStatus = "ready"
	ExtractionStatusPartial     ExtractionStatus = "partial"
	ExtractionStatusUnsupported ExtractionStatus = "unsupported"
	ExtractionStatusFailed      ExtractionStatus = "failed"
)

// PartialReason classifies why a file extraction landed in partial /
// unsupported / failed state. Closed enum.
type PartialReason string

const (
	PartialReasonUnsupportedLanguage PartialReason = "unsupported_language"
	PartialReasonParseError          PartialReason = "parse_error"
	PartialReasonQueryError          PartialReason = "query_error"
	PartialReasonTimeout             PartialReason = "timeout"
	PartialReasonFileTooLarge        PartialReason = "file_too_large"
	PartialReasonBinaryOrGenerated   PartialReason = "binary_or_generated"
	PartialReasonPermissionDenied    PartialReason = "permission_denied"
	PartialReasonExtractorBug        PartialReason = "extractor_bug"
)

// FileSemanticAvailability is the consumer-side classification (D-05).
// Mirrors ExtractionStatus plus a "missing" rung for files that were
// never extracted.
type FileSemanticAvailability string

const (
	AvailabilityReady       FileSemanticAvailability = "ready"
	AvailabilityPartial     FileSemanticAvailability = "partial"
	AvailabilityUnsupported FileSemanticAvailability = "unsupported"
	AvailabilityFailed      FileSemanticAvailability = "failed"
	AvailabilityMissing     FileSemanticAvailability = "missing"
)

// Position is a (zero-based line, zero-based column UTF-16 code-unit)
// pair. Phase 59 keeps this tiny and local rather than re-using
// protocol/gen LSP types so internal/semantic/extract has no LSP coupling.
type Position struct {
	Line   uint32
	Column uint32
}

// Range is a half-open [Start, End) source-range pair.
type Range struct {
	Start Position
	End   Position
}

// SymbolFact is the in-memory shape per CONTEXT.md D-01b. Persistence
// mapping to DuckDB columns lives in internal/semantic/store/.
type SymbolFact struct {
	ID               semantic.SymbolID // xxhash64(CanonicalizeStableSymbolKey)
	StableKey        StableSymbolKey
	Language         string
	Kind             SymbolKind
	Name             string
	QualifiedName    string
	File             string
	Range            Range
	SelectionRange   Range
	ContainerID      *semantic.SymbolID // owner symbol (e.g., enclosing class)
	Signature        string
	SignatureHash    string
	Receiver         *ReceiverFact // Go methods, Python instance methods
	Visibility       string        // exported|private|package|...
	Doc              string        // leading-comment block, if extracted
	Confidence       float32       // 0.70 ts-only baseline; raised on merge
	ExtractionSource string        // "tree_sitter" in this phase
	Partial          bool
	PartialReason    string

	// Fingerprint carries structural near-clone (MinHash), structural-
	// profile (ASTProfile), and semantic-vocabulary (Random-Indexing context
	// vector) signatures computed at extraction time over the symbol's body
	// AST. Only populated for function/method symbols with a body large enough
	// to fingerprint; nil otherwise. The tree-sitter tree is closed when
	// Extract returns, so these MUST be computed inside the provider while the
	// AST is alive (see FingerprintBody). factsFromExtracted consumes them to
	// emit SIMILAR_TO (MinHash, structure), DATA_FLOWS (ASTProfile, structure),
	// and SEMANTICALLY_RELATED (ContextVec, vocabulary) edges. MinHash/Profile
	// capture SHAPE; ContextVec captures VOCABULARY — orthogonal by design.
	MinHash    *minhash.Signature
	Profile    *classifier.ASTProfile
	ContextVec *relatedidx.Vector
}

// ReceiverFact carries method-receiver metadata (Go-style methods,
// Python instance methods). Nil on free functions.
type ReceiverFact struct {
	Type string
	Name string
}

// ReferenceFact is the in-memory shape per CONTEXT.md D-01b.
type ReferenceFact struct {
	ID               semantic.ReferenceID
	Language         string
	Kind             ReferenceKind // CALL|REFERENCES|USES_TYPE|READS|WRITES|...
	Name             string
	File             string
	Range            Range
	ContainerID      *semantic.SymbolID
	ReceiverText     string             // pre-resolution literal, if any
	ResolvedTarget   *semantic.SymbolID // unset in Phase 59; Phase 61 fills via LSP
	ResolutionSource string             // "" until enrichment lands
	ValidationState  string             // "syntactic" baseline in Phase 59
	Confidence       float32
	Reason           string
	Partial          bool
	PartialReason    string
}

// ImportFact is the in-memory shape per CONTEXT.md D-01b.
type ImportFact struct {
	ID       semantic.ImportID
	Language string
	Source   string // module path / package name
	Alias    string
	Symbols  []string // for explicit named imports
	File     string
	Range    Range
}

// TypeFact carries a type-annotation fact attached to a symbol (D-01b).
type TypeFact struct {
	ID         semantic.TypeFactID
	Language   string
	SubjectID  semantic.SymbolID // symbol the annotation is attached to
	Annotation string            // raw annotation text
	Range      Range
}

// HeritageFact carries an extends/implements/embeds edge (D-01b).
type HeritageFact struct {
	ID        semantic.HeritageID
	Language  string
	SubjectID semantic.SymbolID
	Relation  string // "extends"|"implements"|"embeds"
	Target    string // target qualified name (resolution is later)
	Range     Range
}

// FileFact is the per-file extraction outcome row (D-05). Mirrors the
// semantic_files columns added by P01's applyMigration002.
type FileFact struct {
	Path              string
	Language          string
	ExtractionStatus  ExtractionStatus
	ExtractionPartial bool
	PartialReason     PartialReason
	ExtractorName     string
	ExtractorVersion  string
	ErrorMessage      string
}

// SourceFile is the provider-input envelope. The scheduler hands the
// provider (path, language) so the provider can build FileFact /
// ExtractedFile without re-classifying language by extension.
type SourceFile struct {
	Path     string
	Language string
}

// RouteFact captures an HTTP route registration detected in source: a
// (method, path, handler) triple synthesized from framework call sites /
// decorators — Go net/http + gin/echo/gorilla/chi, Python Flask/FastAPI,
// TS/JS Express/NestJS. factsFromExtracted promotes each RouteFact into a
// synthetic Route SymbolFact plus a HANDLES edge from the handler symbol.
type RouteFact struct {
	Language string
	Method   string // "GET","POST",... "" when the framework does not fix one
	Path     string // "/users/:id"
	Handler  string // handler symbol name (best-effort; "" when unresolved)
	File     string
	Range    Range
}

// ResourceFact captures a data-resource / ORM-entity definition detected in
// source — a class/struct that maps to a persistence table (Go GORM, Python
// SQLAlchemy, TS TypeORM, Java JPA, C# EF, Rust Diesel, …). factsFromExtracted
// promotes each into a synthetic Resource SymbolFact so the graph has typed
// data-entity nodes alongside Route nodes.
type ResourceFact struct {
	Language string
	Name     string // entity class/struct name
	Table    string // mapped table name (best-effort; "" / same as Name)
	ORM      string // "gorm","sqlalchemy","typeorm","jpa","ef","diesel","eloquent","activerecord"
	File     string
	Range    Range
}

// ExtractedFile bundles the per-file extraction output. The provider's
// ExtractFile (in P04 per-language packages) returns one of these per
// successful or partial extraction.
type ExtractedFile struct {
	File          FileFact
	Symbols       []SymbolFact
	References    []ReferenceFact
	Imports       []ImportFact
	Types         []TypeFact
	Heritage      []HeritageFact
	Routes        []RouteFact
	Resources     []ResourceFact
	Partial       bool
	PartialReason string
}
