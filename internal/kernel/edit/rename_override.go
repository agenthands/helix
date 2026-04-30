package edit

import (
	"context"
	"sort"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/kernel/symbols"
	gen "github.com/agenthands/helix/protocol/gen"
)

// RenameStrategy enumerates the path a successful rename took.
// String values are a public contract: they appear in rename_symbol tool
// responses AND in the serena_rename_strategy_total metric label set.
// MUST NOT change without a coordinated telemetry-schema update.
type RenameStrategy string

const (
	// StrategyLSPNative indicates textDocument/rename succeeded via the LS.
	StrategyLSPNative RenameStrategy = "lsp-native"
	// StrategyRustClientSide indicates the rust-analyzer quirk override was
	// used: references were gathered via textDocument/references and text
	// edits were applied client-side. See BUG-DEFER-02 for semantic limits.
	StrategyRustClientSide RenameStrategy = "rust-client-side"
)

// RenameOverrider is an optional interface that lspool.QuirkAdapter
// implementations may satisfy (directly or via a wrapper in package edit)
// to bypass textDocument/rename. Called by edit.RenameSymbol when the
// native rename attempt fails.
//
// Semantic-accuracy note: overrides are NOT required to match native LSP
// rename fidelity (cross-crate trait-impl discovery, macro expansion).
// Implementations MUST document their limits in their doc comment AND in
// USAGE.md. See BUG-DEFER-02.
//
// On success returns (*RenameResult, nil); the caller sets Strategy.
// On failure returns (nil, serr.*); the caller wraps in serr.Unsupported
// per D-06 if the native path also failed.
type RenameOverrider interface {
	RenameOverride(
		ctx context.Context,
		lease *lspool.WorkerLease,
		uri string,
		line, col int,
		newName string,
	) (*RenameResult, error)
}

// RustClientSideRename implements the references-driven fallback used by
// RustAnalyzerRenameOverride. Kept in package edit to avoid the
// lspool -> edit import cycle (applyTextEdits is package-private here).
//
// Contract assumptions (rust-analyzer only — DO NOT generalize):
//  1. textDocument/references (includeDeclaration=true) returns ranges
//     that exactly span the identifier to be renamed.
//  2. The symbol has at least one reference (including the declaration).
//
// Failure modes:
//   - zero references        -> serr.NotFound
//   - references RPC error   -> serr.Internal (wrap)
//   - text-edit apply error  -> serr.Internal (wrap, with file URI detail)
func RustClientSideRename(
	ctx context.Context,
	lease *lspool.WorkerLease,
	uri string,
	line, col int,
	newName string,
) (*RenameResult, error) {
	if lease == nil {
		return nil, serr.New(serr.Internal, "rust-client-side rename: nil lease")
	}
	locs, err := symbols.FindReferences(ctx, lease, uri, line, col, true)
	if err != nil {
		return nil, serr.Wrap(serr.Internal, "rust-client-side rename: find references", err).WithDetail(err.Error())
	}
	if len(locs) == 0 {
		return nil, serr.New(serr.NotFound, "rust-client-side rename: no references at position")
	}
	byFile := make(map[string][]gen.TextEdit)
	for _, loc := range locs {
		byFile[loc.URI] = append(byFile[loc.URI], gen.TextEdit{Range: loc.Range, NewText: newName})
	}
	res := &RenameResult{}
	for fileURI, edits := range byFile {
		if err := applyTextEdits(fileURI, edits); err != nil {
			return nil, serr.Wrap(serr.Internal, "rust-client-side rename: apply edits", err).WithDetail(fileURI)
		}
		res.FilesChanged++
		res.EditsApplied += len(edits)
		res.Files = append(res.Files, fileURI)
	}
	sort.Strings(res.Files)
	return res, nil
}

// RustAnalyzerRenameOverride wraps a *lspool.RustAnalyzerAdapter with a
// RenameOverrider implementation. Lives in package edit so the method can
// return *RenameResult without forcing lspool -> edit import (RESEARCH
// Pitfall 1 resolution). The dispatcher in rename.go constructs this
// wrapper inline after type-asserting the adapter for *lspool.RustAnalyzerAdapter.
//
// Semantic-accuracy note (D-05, BUG-DEFER-02): the references-driven
// fallback does NOT match native rust-analyzer rename fidelity on
// cross-crate trait-impl discovery, macro-expansion sites, or re-exports
// that rename at the re-export site. See USAGE.md Troubleshooting.
type RustAnalyzerRenameOverride struct {
	Inner *lspool.RustAnalyzerAdapter
}

// RenameOverride satisfies the RenameOverrider interface. Best-effort
// waits for rust-analyzer quiescence (readiness is an optimization; the
// references-driven fallback works even when the server is not yet
// quiescent because textDocument/references succeeds at the failing
// position per RCA §1).
func (w *RustAnalyzerRenameOverride) RenameOverride(
	ctx context.Context,
	lease *lspool.WorkerLease,
	uri string,
	line, col int,
	newName string,
) (*RenameResult, error) {
	if w.Inner != nil {
		_ = w.Inner.WaitUntilRenameReady(ctx) // best-effort
	}
	return RustClientSideRename(ctx, lease, uri, line, col, newName)
}
