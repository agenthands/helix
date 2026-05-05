// Package nosemantic2kernel provides a go/analysis Analyzer that fails the
// build if any package whose import path begins with internal/semantic/lspenrich/
// imports a package whose import path begins with internal/kernel/, EXCEPT
// when the imported path is rooted under internal/kernel/lspool (the
// carve-out allow-list).
//
// Enforces Phase 61 ENRICH-01 acceptance #1 (61-CONTEXT.md "Acceptance
// criteria" item #1): "internal/semantic/lspenrich/ (or wherever the worker
// lives) does NOT import internal/kernel" — pinned by the plan's must_haves
// truth: "internal/semantic/lspenrich does not import internal/kernel (only
// internal/kernel/lspool + internal/workspace) and a vet analyzer
// mechanically enforces this".
//
// Scope rationale: the analyzer's checkedPkgPrefix is intentionally narrow
// (`internal/semantic/lspenrich`), NOT the broader `internal/semantic`. The
// broader semantic tree pre-dates Phase 61 and contains established
// kernel-importing packages (e.g. internal/semantic/live/service which
// implements kernel.EditNotifier — Phase 60 architecture). The ENRICH-01
// invariant pins the lspenrich seam specifically; broadening the analyzer
// would falsely flag pre-existing Phase 60 design.
//
// `internal/workspace` is outside the forbidden prefix (`internal/kernel/...`)
// entirely, so no dedicated carve-out is required for it.
//
// This is the symmetric sibling of internal/lint/nokernel2semantic; the
// pair together pin the kernel↔semantic boundary at the two seams that
// matter:
//
//	nokernel2semantic:  internal/kernel/*           MUST NOT import internal/semantic/*
//	nosemantic2kernel:  internal/semantic/lspenrich MUST NOT import internal/kernel/*
//	                    (carve-out: internal/kernel/lspool)
//
// Trace:
//
//	Acceptance #1 (ENRICH-01)  →  this analyzer  →  cmd/vet-nosemantic2kernel
//	                              (`make vet` runs the singlechecker)
//
// CI note (carried over from nokernel2semantic): .github/workflows/go-test.yml
// currently runs plain `go vet ./...` (without `-vettool=`). The
// singlechecker fires on local `make vet` / `make test` invocations today; a
// follow-up wiring step (out of scope for the introducing plan) can swap CI
// to `make vet` to make the ENRICH-01 #1 gate apply on every PR.
//
// The forbiddenImportPrefix string is intentionally a prefix (no trailing
// slash) so it matches any sub-package of internal/kernel/ without per-
// package allowlisting; the carve-out is checked separately (full path equal
// to lspoolPkgPath OR HasPrefix lspoolSubPkgPrefix).
package nosemantic2kernel

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	// checkedPkgPrefix is intentionally narrow — only lspenrich and its
	// sub-packages are gated. See package doc comment for rationale.
	checkedPkgPrefix      = "github.com/agenthands/helix/internal/semantic/lspenrich"
	forbiddenImportPrefix = "github.com/agenthands/helix/internal/kernel"

	// lspoolPkgPath is the carve-out allow-list path. An import equal to
	// this path OR rooted under lspoolSubPkgPrefix is permitted even though
	// it shares the forbidden prefix.
	lspoolPkgPath      = "github.com/agenthands/helix/internal/kernel/lspool"
	lspoolSubPkgPrefix = "github.com/agenthands/helix/internal/kernel/lspool/"
)

// Analyzer enforces ENRICH-01 acceptance #1: internal/semantic/lspenrich/*
// must not import internal/kernel/*, with a carve-out for
// internal/kernel/lspool.
var Analyzer = &analysis.Analyzer{
	Name: "nosemantic2kernel",
	Doc:  "fails if internal/semantic/lspenrich/* imports internal/kernel/* (carve-out: internal/kernel/lspool)",
	Run: func(pass *analysis.Pass) (interface{}, error) {
		if !strings.HasPrefix(pass.Pkg.Path(), checkedPkgPrefix) {
			return nil, nil
		}
		for _, file := range pass.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				if !strings.HasPrefix(path, forbiddenImportPrefix) {
					continue
				}
				// Carve-out: internal/kernel/lspool and its sub-packages.
				if path == lspoolPkgPath || strings.HasPrefix(path, lspoolSubPkgPrefix) {
					continue
				}
				pass.Reportf(imp.Pos(),
					"internal/semantic/lspenrich/* must not import internal/kernel/* (got import %q in %s)",
					path, pass.Pkg.Path())
			}
		}
		return nil, nil
	},
}
