// Phase 71-01 Task 3: seed-resolution helper shared by 71-03 / 71-04 / 71-05.
//
// resolveSeed translates a SeedInput (either a direct SymbolID or a
// (file_path, symbol_name) tuple) to a ResolvedSeed carrying the primary
// SymbolID plus a closed-enum Resolution and (for the ambiguous path) up to
// five candidate alternates.
//
// Contract (D1 from 71-CONTEXT.md):
//   - SymbolID present → short-circuit, Resolution=exact, accessor NOT called.
//   - (FilePath, SymbolName) both present → SymbolByNameAccessor lookup.
//     0 matches → Resolution=not_found (no error).
//     1 match  → Resolution=exact, SymbolID=match[0].
//     >1 match → Resolution=ambiguous, SymbolID=match[0],
//                AmbiguousCandidates=match[:min(5, len)] (D1 cap).
//   - Neither variant valid → serr.InvalidArgs error mentioning both forms.

package semantic

import (
	"context"
	"fmt"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// SeedInput is the JSON input variant accepted by every Phase 71 single-symbol
// tool. Exactly one of (SymbolID) or (FilePath + SymbolName) must be set;
// resolveSeed validates the combination.
type SeedInput struct {
	SymbolID   string `json:"symbol_id,omitempty"   jsonschema:"graph-internal stable key"`
	FilePath   string `json:"file_path,omitempty"   jsonschema:"workspace-relative path; pair with symbol_name"`
	SymbolName string `json:"symbol_name,omitempty" jsonschema:"qualified symbol name; pair with file_path"`
}

// Resolution is the closed-enum disposition of a seed lookup. Mirrors the
// envelope.go:14-32 Freshness convention for downstream JSON serialization
// stability.
type Resolution string

const (
	// ResolutionExact — a single graph symbol matched (or SymbolID was used
	// directly without an accessor round-trip).
	ResolutionExact Resolution = "exact"
	// ResolutionAmbiguous — multiple symbols matched (file_path, symbol_name);
	// resolveSeed returns the first match as primary and lists up to 5 in
	// AmbiguousCandidates.
	ResolutionAmbiguous Resolution = "ambiguous"
	// ResolutionNotFound — no symbol matched the input tuple.
	ResolutionNotFound Resolution = "not_found"
)

// ResolvedSeed is resolveSeed's return shape. SymbolID is the primary match
// (zero value when Resolution=not_found). AmbiguousCandidates is non-nil only
// when Resolution=ambiguous.
type ResolvedSeed struct {
	SymbolID            integ.SymbolID
	Resolution          Resolution
	AmbiguousCandidates []integ.SymbolID
}

// ambiguousCap is the D1 hard cap on AmbiguousCandidates length. The
// *Store.QuerySymbolByName accessor LIMITs at 6 (one above the cap so the
// caller can detect "more than 5" before truncating to 5).
const ambiguousCap = 5

// resolveSeed translates SeedInput to ResolvedSeed. See package-level comment
// for the full contract. Returns a wrapped serr.InvalidArgs on malformed
// input; propagates accessor errors via fmt.Errorf with %w.
func (s *SemanticSkill) resolveSeed(ctx context.Context, ws workspace.WorkspaceKey, in SeedInput) (ResolvedSeed, error) {
	// Short-circuit when SymbolID is supplied directly.
	if in.SymbolID != "" {
		return ResolvedSeed{
			SymbolID:   integ.SymbolID(in.SymbolID),
			Resolution: ResolutionExact,
		}, nil
	}

	// Both file_path AND symbol_name must be set for the tuple variant.
	if in.FilePath == "" || in.SymbolName == "" {
		return ResolvedSeed{}, serr.New(
			serr.InvalidArgs,
			"seed requires symbol_id OR (file_path AND symbol_name)",
		)
	}

	s.mu.Lock()
	acc := s.symbolByName
	s.mu.Unlock()
	if acc == nil {
		return ResolvedSeed{}, serr.New(
			serr.InvalidArgs,
			"seed resolver: SymbolByNameAccessor not wired",
		)
	}

	candidates, err := acc.QuerySymbolByName(ctx, ws.Hash(), in.FilePath, in.SymbolName)
	if err != nil {
		return ResolvedSeed{}, fmt.Errorf("resolveSeed(%q,%q): %w", in.FilePath, in.SymbolName, err)
	}

	switch {
	case len(candidates) == 0:
		return ResolvedSeed{Resolution: ResolutionNotFound}, nil
	case len(candidates) == 1:
		return ResolvedSeed{
			SymbolID:   candidates[0],
			Resolution: ResolutionExact,
		}, nil
	default:
		amb := candidates
		if len(amb) > ambiguousCap {
			amb = amb[:ambiguousCap]
		}
		return ResolvedSeed{
			SymbolID:            candidates[0],
			Resolution:          ResolutionAmbiguous,
			AmbiguousCandidates: amb,
		}, nil
	}
}
