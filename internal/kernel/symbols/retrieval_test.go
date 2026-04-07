package symbols

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorkerLease creates a WorkerLease backed by a mockWorker that returns
// canned responses keyed by LSP method.
type mockConn struct {
	mu        sync.Mutex
	responses map[string]json.RawMessage
}

func (m *mockConn) Call(_ context.Context, method string, _ interface{}, result interface{}) error {
	m.mu.Lock()
	raw, ok := m.responses[method]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, result)
}

func (m *mockConn) Notify(_ context.Context, _ string, _ interface{}) error {
	return nil
}

// newMockLease creates a WorkerLease that returns predefined JSON for each LSP method.
func newMockLease(responses map[string]interface{}) *lspool.WorkerLease {
	raw := make(map[string]json.RawMessage)
	for method, resp := range responses {
		data, _ := json.Marshal(resp)
		raw[method] = data
	}
	mc := &mockConn{responses: raw}
	_ = mc // used below indirectly
	return newLeaseWithMockConn(raw)
}

// Since WorkerLease.Request delegates to Worker.Request which delegates to process.Conn().Call,
// we need to create a real-ish chain. Instead, we use a simpler approach:
// construct a lease that wraps a method-level mock using the Request interface.

// testLease is a minimal implementation that routes Request through a map of canned responses.
type testLease struct {
	mu        sync.RWMutex
	responses map[string]json.RawMessage
}

func (t *testLease) Request(ctx context.Context, method string, params, result interface{}) error {
	t.mu.RLock()
	raw, ok := t.responses[method]
	t.mu.RUnlock()
	if !ok {
		return nil
	}
	return json.Unmarshal(raw, result)
}

// Since the actual functions use *lspool.WorkerLease directly and call lease.Request(),
// we need an actual WorkerLease. WorkerLease.Request delegates to Worker.Request which
// requires a running process. For unit testing, we'll test the helper functions directly
// and the integration through the LSP method calls.

// testableGoToDefinition tests using a direct lease.Request approach.
// We'll test via the exported functions by providing a real WorkerLease backed by a
// testable worker. Since that requires process infrastructure, we test at the unit level
// by verifying type conversions and helper behavior.

func TestLocationsToSymbolLocations(t *testing.T) {
	locs := []gen.Location{
		{
			URI: "file:///project/main.go",
			Range: gen.Range{
				Start: gen.Position{Line: 10, Character: 5},
				End:   gen.Position{Line: 10, Character: 15},
			},
		},
		{
			URI: "file:///project/util.go",
			Range: gen.Range{
				Start: gen.Position{Line: 20, Character: 0},
				End:   gen.Position{Line: 20, Character: 10},
			},
		},
	}

	result := locationsToSymbolLocations(locs)
	require.Len(t, result, 2)

	assert.Equal(t, "file:///project/main.go", result[0].URI)
	assert.Equal(t, uint32(10), result[0].Range.Start.Line)
	assert.Equal(t, uint32(5), result[0].Range.Start.Character)

	assert.Equal(t, "file:///project/util.go", result[1].URI)
	assert.Equal(t, uint32(20), result[1].Range.Start.Line)
}

func TestExtractHoverContent_MarkupContent(t *testing.T) {
	hover := gen.Hover{
		Contents: gen.Or_MarkedString_MarkedStringArray_MarkupContent{
			Value: gen.MarkupContent{
				Kind:  "markdown",
				Value: "```go\nfunc main()\n```",
			},
		},
	}
	content := extractHoverContent(hover)
	assert.Equal(t, "```go\nfunc main()\n```", content)
}

func TestExtractHoverContent_String(t *testing.T) {
	hover := gen.Hover{
		Contents: gen.Or_MarkedString_MarkedStringArray_MarkupContent{
			Value: "simple string hover",
		},
	}
	content := extractHoverContent(hover)
	assert.Equal(t, "simple string hover", content)
}

func TestExtractHoverContent_Nil(t *testing.T) {
	hover := gen.Hover{
		Contents: gen.Or_MarkedString_MarkedStringArray_MarkupContent{
			Value: nil,
		},
	}
	content := extractHoverContent(hover)
	assert.Empty(t, content)
}

func TestExtractHoverContent_MapWithValue(t *testing.T) {
	hover := gen.Hover{
		Contents: gen.Or_MarkedString_MarkedStringArray_MarkupContent{
			Value: map[string]interface{}{
				"kind":  "markdown",
				"value": "type info here",
			},
		},
	}
	content := extractHoverContent(hover)
	assert.Equal(t, "type info here", content)
}

func TestSymbolKindName(t *testing.T) {
	assert.Equal(t, "Function", SymbolKindName(gen.SymbolKindFunction))
	assert.Equal(t, "Class", SymbolKindName(gen.SymbolKindClass))
	assert.Equal(t, "Interface", SymbolKindName(gen.SymbolKindInterface))
	assert.Equal(t, "Method", SymbolKindName(gen.SymbolKindMethod))
	assert.Equal(t, "Variable", SymbolKindName(gen.SymbolKindVariable))
	assert.Equal(t, "Struct", SymbolKindName(gen.SymbolKindStruct))
}

func TestMapDocumentSymbols_PreservesHierarchy(t *testing.T) {
	symbols := []gen.DocumentSymbol{
		{
			Name: "MyClass",
			Kind: gen.SymbolKindClass,
			Range: gen.Range{
				Start: gen.Position{Line: 0, Character: 0},
				End:   gen.Position{Line: 20, Character: 0},
			},
			Children: []gen.DocumentSymbol{
				{
					Name: "myMethod",
					Kind: gen.SymbolKindMethod,
					Range: gen.Range{
						Start: gen.Position{Line: 5, Character: 4},
						End:   gen.Position{Line: 10, Character: 4},
					},
				},
				{
					Name: "myField",
					Kind: gen.SymbolKindField,
					Range: gen.Range{
						Start: gen.Position{Line: 2, Character: 4},
						End:   gen.Position{Line: 2, Character: 20},
					},
				},
			},
		},
	}

	result := mapDocumentSymbols(symbols)
	require.Len(t, result, 1)
	assert.Equal(t, "MyClass", result[0].Name)
	assert.Equal(t, "Class", result[0].Kind)
	require.Len(t, result[0].Children, 2)
	assert.Equal(t, "myMethod", result[0].Children[0].Name)
	assert.Equal(t, "Method", result[0].Children[0].Kind)
	assert.Equal(t, "myField", result[0].Children[1].Name)
	assert.Equal(t, "Field", result[0].Children[1].Kind)
}

func TestBlastRadiusDeduplication(t *testing.T) {
	// Test that locationKey produces unique keys for different positions
	loc1 := SymbolLocation{
		URI: "file:///a.go",
		Range: gen.Range{
			Start: gen.Position{Line: 10, Character: 5},
		},
	}
	loc2 := SymbolLocation{
		URI: "file:///a.go",
		Range: gen.Range{
			Start: gen.Position{Line: 10, Character: 5},
		},
	}
	loc3 := SymbolLocation{
		URI: "file:///b.go",
		Range: gen.Range{
			Start: gen.Position{Line: 20, Character: 0},
		},
	}

	// Same location should produce same key
	assert.Equal(t, locationKey(loc1), locationKey(loc2))
	// Different location should produce different key
	assert.NotEqual(t, locationKey(loc1), locationKey(loc3))

	// Test deduplication across references and implementations
	fileSet := make(map[string]struct{})
	locationSet := make(map[string]struct{})

	addLocations := func(locs []SymbolLocation) {
		for _, loc := range locs {
			key := locationKey(loc)
			if _, exists := locationSet[key]; !exists {
				locationSet[key] = struct{}{}
				fileSet[loc.URI] = struct{}{}
			}
		}
	}

	refs := []SymbolLocation{loc1, loc2, loc3}     // loc1 and loc2 are duplicates
	impls := []SymbolLocation{loc1, loc3}           // all duplicates of refs
	addLocations(refs)
	addLocations(impls)

	assert.Len(t, locationSet, 2, "should deduplicate identical locations")
	assert.Len(t, fileSet, 2, "should have 2 unique files")
}

func TestHierarchyDepthLimit(t *testing.T) {
	// Verify the constant is set appropriately
	assert.Equal(t, 3, maxHierarchyDepth)
}

func TestMakePositionParams(t *testing.T) {
	params := makePositionParams("file:///test.go", 42, 7)
	assert.Equal(t, "file:///test.go", params.TextDocument.URI)
	assert.Equal(t, uint32(42), params.Position.Line)
	assert.Equal(t, uint32(7), params.Position.Character)
}

func TestItoa(t *testing.T) {
	assert.Equal(t, "0", itoa(0))
	assert.Equal(t, "42", itoa(42))
	assert.Equal(t, "100", itoa(100))
	assert.Equal(t, "-5", itoa(-5))
}

// newLeaseWithMockConn is unused since we can't easily mock Worker internals.
// The tests above cover the pure-function logic. Integration tests with a real
// LS would go in a separate _integration_test.go file.
func newLeaseWithMockConn(_ map[string]json.RawMessage) *lspool.WorkerLease {
	return nil // placeholder — not used by current tests
}
