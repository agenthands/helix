package semantic

import (
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/mcp"
)

// RegisterAll registers all four Phase 64 MCP tools with the SerenaMCPServer:
// index_semantic_graph, refresh_semantic_graph, get_semantic_graph_status,
// get_semantic_context. Daemon (P64-08) calls this from semantic_wiring.go
// after the SemanticSkill's accessors are wired.
//
// The function is the EXPORTED registration entry point — the per-tool
// register*SemanticGraph functions are package-private so they cannot be
// invoked individually out-of-package. RegisterAll guarantees the four-tool
// set is registered as a single atomic operation; callers never have to
// remember the order or set.
func RegisterAll(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	if server == nil || s == nil {
		return
	}
	registerIndexSemanticGraph(server, s, tracer)
	registerRefreshSemanticGraph(server, s, tracer)
	registerGetSemanticGraphStatus(server, s, tracer)
	registerGetSemanticContext(server, s, tracer)
	registerExplainSymbolDeep(server, s, tracer)
	registerFindRelatedSymbols(server, s, tracer)
	registerValidateGraphEdge(server, s, tracer)
	registerGetClusterMap(server, s, tracer)
	registerExplainCluster(server, s, tracer)
	registerGetChangeImpactGraph(server, s, tracer)
	registerTraceDataFlow(server, s, tracer)
}
