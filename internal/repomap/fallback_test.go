package repomap

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gen "github.com/postfix/serena/protocol/gen"
)

// mockRequester implements SymbolRequester for testing without a real LSP server.
type mockRequester struct {
	symbols []gen.DocumentSymbol
	err     error
}

func (m *mockRequester) Request(_ context.Context, _ string, _ interface{}, result interface{}) error {
	if m.err != nil {
		return m.err
	}
	ptr := result.(*[]gen.DocumentSymbol)
	*ptr = m.symbols
	return nil
}

func TestFallbackExtract_FlatSymbols(t *testing.T) {
	mock := &mockRequester{
		symbols: []gen.DocumentSymbol{
			{
				Name: "doSomething",
				Kind: 12, // Function
				SelectionRange: gen.Range{
					Start: gen.Position{Line: 5, Character: 4},
				},
			},
			{
				Name: "myVar",
				Kind: 13, // Variable
				SelectionRange: gen.Range{
					Start: gen.Position{Line: 10, Character: 0},
				},
			},
		},
	}

	ext := NewFallbackExtractor()
	tags, err := ext.Extract(context.Background(), mock, "test.java", "file:///test.java")

	require.NoError(t, err)
	assert.Len(t, tags, 2)

	assert.Equal(t, "doSomething", tags[0].Name)
	assert.Equal(t, TagDef, tags[0].Kind)
	assert.Equal(t, "test.java", tags[0].File)
	assert.Equal(t, 5, tags[0].Line)
	assert.Equal(t, 4, tags[0].Column)

	assert.Equal(t, "myVar", tags[1].Name)
	assert.Equal(t, TagDef, tags[1].Kind)
	assert.Equal(t, 10, tags[1].Line)
}

func TestFallbackExtract_NestedSymbols(t *testing.T) {
	mock := &mockRequester{
		symbols: []gen.DocumentSymbol{
			{
				Name: "MyClass",
				Kind: 5, // Class
				SelectionRange: gen.Range{
					Start: gen.Position{Line: 1, Character: 6},
				},
				Children: []gen.DocumentSymbol{
					{
						Name: "method1",
						Kind: 6, // Method
						SelectionRange: gen.Range{
							Start: gen.Position{Line: 3, Character: 8},
						},
					},
					{
						Name: "method2",
						Kind: 6, // Method
						SelectionRange: gen.Range{
							Start: gen.Position{Line: 7, Character: 8},
						},
					},
				},
			},
		},
	}

	ext := NewFallbackExtractor()
	tags, err := ext.Extract(context.Background(), mock, "test.java", "file:///test.java")

	require.NoError(t, err)
	assert.Len(t, tags, 3)

	assert.Equal(t, "MyClass", tags[0].Name)
	assert.Equal(t, TagDef, tags[0].Kind)
	assert.Equal(t, 1, tags[0].Line)

	assert.Equal(t, "MyClass.method1", tags[1].Name)
	assert.Equal(t, TagDef, tags[1].Kind)
	assert.Equal(t, 3, tags[1].Line)

	assert.Equal(t, "MyClass.method2", tags[2].Name)
	assert.Equal(t, TagDef, tags[2].Kind)
	assert.Equal(t, 7, tags[2].Line)
}

func TestFallbackExtract_NoReferences(t *testing.T) {
	mock := &mockRequester{
		symbols: []gen.DocumentSymbol{
			{
				Name: "SomeClass",
				Kind: 5,
				SelectionRange: gen.Range{
					Start: gen.Position{Line: 0, Character: 0},
				},
				Children: []gen.DocumentSymbol{
					{
						Name: "field1",
						Kind: 8, // Field
						SelectionRange: gen.Range{
							Start: gen.Position{Line: 1, Character: 4},
						},
					},
				},
			},
			{
				Name: "helperFunc",
				Kind: 12, // Function
				SelectionRange: gen.Range{
					Start: gen.Position{Line: 5, Character: 0},
				},
			},
		},
	}

	ext := NewFallbackExtractor()
	tags, err := ext.Extract(context.Background(), mock, "test.java", "file:///test.java")

	require.NoError(t, err)
	for i, tag := range tags {
		assert.Equal(t, TagDef, tag.Kind, "tag %d (%s) should be def, not ref (D-07)", i, tag.Name)
		assert.NotEqual(t, TagRef, tag.Kind, "tag %d (%s) must not be ref (D-07)", i, tag.Name)
	}
}

func TestFallbackExtract_Error(t *testing.T) {
	mock := &mockRequester{
		err: errors.New("LSP connection failed"),
	}

	ext := NewFallbackExtractor()
	tags, err := ext.Extract(context.Background(), mock, "test.java", "file:///test.java")

	assert.Error(t, err)
	assert.Nil(t, tags)
	assert.Contains(t, err.Error(), "fallback documentSymbol")
}

func TestFallbackExtract_Empty(t *testing.T) {
	mock := &mockRequester{
		symbols: []gen.DocumentSymbol{},
	}

	ext := NewFallbackExtractor()
	tags, err := ext.Extract(context.Background(), mock, "test.java", "file:///test.java")

	require.NoError(t, err)
	assert.Len(t, tags, 0)
}
