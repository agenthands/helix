//go:build cgo

package extract

import "context"

// ExtractFunc is the shared signature per-language provider packages
// (Phase 59 P04) implement. The function is documented here so the
// scheduler (P03) and provider packages converge on a single shape.
//
// Implementations are responsible for:
//   - parsing source bytes via p.TreeSitterLanguage(),
//   - running the provider's compiled Queries against the parse tree,
//   - building SymbolFact / ReferenceFact / ImportFact / TypeFact /
//     HeritageFact rows with confidence stamped to ConfidenceTSOnly,
//   - constructing the FileFact envelope with the right
//     ExtractionStatus + PartialReason on parse / query errors.
//
// Per-extraction QueryCursor objects MUST be Close()'d (Pitfall #2 in
// 59-RESEARCH.md). Provider-cached *tree_sitter.Query objects live for
// the daemon lifetime and are Close()'d on shutdown.
type ExtractFunc func(ctx context.Context, p Provider, source []byte, file SourceFile) (*ExtractedFile, error)
