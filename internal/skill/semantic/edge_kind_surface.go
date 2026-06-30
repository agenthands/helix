package semantic

// edge_kind_surface.go declares the closed lowercase MCP-surface enum used by
// Phase 71 read tools to classify edges, plus a one-way mapper from the
// extractor/resolver internal kind strings into the surface enum.
//
// Extended 2026-06-28 with borrowed edge types from codebase-memory-mcp:
// IMPORTS, DEFINES, DATA_FLOWS, HTTP_CALLS, ASYNC_CALLS, EMITS, LISTENS_ON,
// SIMILAR_TO, SEMANTICALLY_RELATED, plus the structural/call-classification
// kinds HANDLES, CONFIGURES, WRITES, MEMBER_OF, TESTS, and the git-mined
// FILE_CHANGES_WITH (co-change coupling).

type EdgeKindSurface string

const (
	EdgeKindCalls               EdgeKindSurface = "calls"
	EdgeKindReferences          EdgeKindSurface = "references"
	EdgeKindImplements          EdgeKindSurface = "implements"
	EdgeKindExtends             EdgeKindSurface = "extends"
	EdgeKindHasType             EdgeKindSurface = "has_type"
	EdgeKindUsesType            EdgeKindSurface = "uses_type"
	EdgeKindContains            EdgeKindSurface = "contains"
	EdgeKindImports             EdgeKindSurface = "imports"
	EdgeKindDefines             EdgeKindSurface = "defines"
	EdgeKindDataFlows           EdgeKindSurface = "data_flows"      // interprocedural caller.param -> callee.param flow (v2.9; emitted by dataFlowEdges)
	EdgeKindStructuralTwin      EdgeKindSurface = "structural_twin" // control-flow/expression-shape profile cosine
	EdgeKindHTTPCalls           EdgeKindSurface = "http_calls"
	EdgeKindAsyncCalls          EdgeKindSurface = "async_calls"
	EdgeKindEmits               EdgeKindSurface = "emits"
	EdgeKindListensOn           EdgeKindSurface = "listens_on"
	EdgeKindSimilarTo           EdgeKindSurface = "similar_to"
	EdgeKindSemanticallyRelated EdgeKindSurface = "semantically_related"
	EdgeKindHandles             EdgeKindSurface = "handles"
	EdgeKindConfigures          EdgeKindSurface = "configures"
	EdgeKindWrites              EdgeKindSurface = "writes"
	EdgeKindMemberOf            EdgeKindSurface = "member_of"
	EdgeKindTests               EdgeKindSurface = "tests"
	EdgeKindFileChangesWith     EdgeKindSurface = "file_changes_with"
	EdgeKindCrossImports        EdgeKindSurface = "cross_imports"
	EdgeKindCrossCalls          EdgeKindSurface = "cross_calls"
	EdgeKindOther               EdgeKindSurface = "other"
)

func MapInternalKind(internal string) EdgeKindSurface {
	switch internal {
	case "CALLS":
		return EdgeKindCalls
	case "REFERENCES":
		return EdgeKindReferences
	case "IMPLEMENTS":
		return EdgeKindImplements
	case "EXTENDS":
		return EdgeKindExtends
	case "RESOLVES_TO":
		return EdgeKindHasType
	case "USES_TYPE":
		return EdgeKindUsesType
	case "CONTAINS":
		return EdgeKindContains
	case "DEFINED_IN":
		return EdgeKindContains
	case "IMPORTS":
		return EdgeKindImports
	case "DEFINES":
		return EdgeKindDefines
	case "DATA_FLOWS":
		return EdgeKindDataFlows
	case "STRUCTURAL_TWIN":
		return EdgeKindStructuralTwin
	case "HTTP_CALLS":
		return EdgeKindHTTPCalls
	case "ASYNC_CALLS":
		return EdgeKindAsyncCalls
	case "EMITS":
		return EdgeKindEmits
	case "LISTENS_ON":
		return EdgeKindListensOn
	case "SIMILAR_TO":
		return EdgeKindSimilarTo
	case "SEMANTICALLY_RELATED":
		return EdgeKindSemanticallyRelated
	case "HANDLES":
		return EdgeKindHandles
	case "CONFIGURES":
		return EdgeKindConfigures
	case "WRITES":
		return EdgeKindWrites
	case "MEMBER_OF":
		return EdgeKindMemberOf
	case "TESTS":
		return EdgeKindTests
	case "FILE_CHANGES_WITH":
		return EdgeKindFileChangesWith
	case "CROSS_IMPORTS":
		return EdgeKindCrossImports
	case "CROSS_CALLS":
		return EdgeKindCrossCalls
	default:
		return EdgeKindOther
	}
}
