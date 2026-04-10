package kernel

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"
)

// WrapToolSpan wraps an MCP tool handler so every invocation creates a
// kernel.tool.{name} span as a child of whatever span is active on ctx.
// The helper is applied at RegisterTools time so per-handler bodies stay
// untouched.
//
// Per D-07: NO attributes are set on kernel spans — tool_name / profile /
// mode / language / outcome are the TelemetryMiddleware's responsibility.
// The kernel sub-span name alone provides sufficient identification.
//
// Usage:
//
//	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{Name: "find_references"},
//	    WrapToolSpan(k.Tracer(), "find_references", findReferencesHandler))
func WrapToolSpan[In, Out any](
	tracer trace.Tracer,
	toolName string,
	fn mcpsdk.ToolHandlerFor[In, Out],
) mcpsdk.ToolHandlerFor[In, Out] {
	spanName := "kernel.tool." + toolName
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, input In) (*mcpsdk.CallToolResult, Out, error) {
		ctx, span := tracer.Start(ctx, spanName)
		defer span.End()
		result, output, err := fn(ctx, req, input)
		if err != nil && span.IsRecording() {
			span.RecordError(err)
		}
		return result, output, err
	}
}
