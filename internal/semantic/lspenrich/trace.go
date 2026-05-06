package lspenrich

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// tracerName is the canonical instrumentation name for lspenrich spans.
// Phase 61 SPEC-DRAFT.md §28.2 line 2761 anchors `semantic.lsp_enrich_file`
// as the per-file root span name; child spans use `semantic.lsp_enrich.<step>`.
const tracerName = "github.com/agenthands/helix/internal/semantic/lspenrich"

// StartFileSpan opens the per-file root span around a single cascade
// invocation.  The span name is the canonical "semantic.lsp_enrich_file"
// (SPEC §28.2).  Attributes:
//
//   - "semantic.repo_id" — the repo identifier this file belongs to.
//   - "semantic.path"    — the file path (relative to the repo root).
//   - "semantic.language" — language label (e.g. "go", "java").
//
// The returned context carries the new span; callers MUST defer span.End().
func StartFileSpan(ctx context.Context, repoID, path, lang string) (context.Context, trace.Span) {
	tr := otel.Tracer(tracerName)
	return tr.Start(ctx, "semantic.lsp_enrich_file",
		trace.WithAttributes(
			attribute.String("semantic.repo_id", repoID),
			attribute.String("semantic.path", path),
			attribute.String("semantic.language", lang),
		),
	)
}

// StartStepSpan opens a child span for one §14.4 cascade step.  step values
// are closed-enum: "documentSymbol", "diagnostics", "hover", "callHierarchy",
// "typeHierarchy", "implementation", "definition".  Caller MUST defer
// span.End() and may set additional attributes / status before End.
func StartStepSpan(ctx context.Context, step string) (context.Context, trace.Span) {
	tr := otel.Tracer(tracerName)
	return tr.Start(ctx, "semantic.lsp_enrich."+step,
		trace.WithAttributes(
			attribute.String("semantic.step", step),
		),
	)
}
