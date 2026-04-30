//go:build integration || llm || llmjudge

package contract_test

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/test/harness"
)

// compileMetaSchema compiles the Draft 2020-12 meta-schema for validating
// tool inputSchema and outputSchema. The meta-schema is bundled with the
// library and resolved locally without network access (Pitfall 2).
func compileMetaSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	sch, err := c.Compile("https://json-schema.org/draft/2020-12/schema")
	require.NoError(t, err, "failed to compile Draft 2020-12 meta-schema")
	return sch
}

// listAllTools starts a runner with full/admin profile (all tools visible)
// and returns the complete tool list. Caller owns the runner lifecycle via
// t.Cleanup.
func listAllTools(t *testing.T) ([]*mcp.Tool, *harness.Runner) {
	t.Helper()
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err, "ListTools failed")
	require.NotEmpty(t, result.Tools, "expected at least one tool")
	return result.Tools, runner
}

// TestSchema_InputSchemaValidDraft2020 validates that every tool's inputSchema
// passes JSON Schema Draft 2020-12 meta-schema validation (CONT-02, D-08, D-09).
func TestSchema_InputSchemaValidDraft2020(t *testing.T) {
	meta := compileMetaSchema(t)
	tools, _ := listAllTools(t)

	for _, tool := range tools {
		t.Run(tool.Name+"_inputSchema", func(t *testing.T) {
			require.NotNil(t, tool.InputSchema, "inputSchema must not be nil for %s", tool.Name)
			err := meta.Validate(tool.InputSchema)
			require.NoError(t, err, "inputSchema for %s is not valid Draft 2020-12", tool.Name)
		})
	}
}

// TestSchema_OutputSchemaValidDraft2020 validates that every tool's outputSchema
// (when declared) passes JSON Schema Draft 2020-12 meta-schema validation (D-08).
func TestSchema_OutputSchemaValidDraft2020(t *testing.T) {
	meta := compileMetaSchema(t)
	tools, _ := listAllTools(t)

	for _, tool := range tools {
		if tool.OutputSchema == nil {
			continue
		}
		t.Run(tool.Name+"_outputSchema", func(t *testing.T) {
			err := meta.Validate(tool.OutputSchema)
			require.NoError(t, err, "outputSchema for %s is not valid Draft 2020-12", tool.Name)
		})
	}
}

// TestSchema_InputSchemaIsObject asserts that every tool's inputSchema defines
// an object type. Tools that accept parameters must also have a properties key.
// Tools with no parameters (e.g., list_memories) may omit properties.
func TestSchema_InputSchemaIsObject(t *testing.T) {
	tools, _ := listAllTools(t)

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			schema, ok := tool.InputSchema.(map[string]any)
			require.True(t, ok, "inputSchema for %s should be map[string]any, got %T", tool.Name, tool.InputSchema)

			typeVal, hasType := schema["type"]
			require.True(t, hasType, "inputSchema for %s must contain 'type' key", tool.Name)
			require.Equal(t, "object", typeVal, "inputSchema for %s must have type 'object'", tool.Name)

			// Tools with required fields must have properties defined.
			if _, hasRequired := schema["required"]; hasRequired {
				_, hasProps := schema["properties"]
				require.True(t, hasProps, "inputSchema for %s has 'required' but no 'properties' key", tool.Name)
			}
		})
	}
}

// TestSchema_RequiredFieldsExist cross-references declared required fields
// against actual properties in the schema (CONT-02).
func TestSchema_RequiredFieldsExist(t *testing.T) {
	tools, _ := listAllTools(t)

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			schema, ok := tool.InputSchema.(map[string]any)
			require.True(t, ok, "inputSchema for %s should be map[string]any", tool.Name)

			reqRaw, hasRequired := schema["required"]
			if !hasRequired {
				return // no required fields declared, nothing to check
			}

			reqSlice, ok := reqRaw.([]any)
			require.True(t, ok, "required for %s should be []any, got %T", tool.Name, reqRaw)

			propsRaw, hasProps := schema["properties"]
			require.True(t, hasProps, "schema has required but no properties for %s", tool.Name)

			props, ok := propsRaw.(map[string]any)
			require.True(t, ok, "properties for %s should be map[string]any", tool.Name)

			for _, r := range reqSlice {
				name, ok := r.(string)
				require.True(t, ok, "required field name should be string for %s, got %T", tool.Name, r)
				_, exists := props[name]
				require.True(t, exists, "required field %q not found in properties for tool %s", name, tool.Name)
			}
		})
	}
}
