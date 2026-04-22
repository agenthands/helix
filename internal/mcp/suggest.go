package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolParamInfo holds known parameter names and enum values for a single tool.
type ToolParamInfo struct {
	ValidParams []string            // known parameter names for this tool
	EnumValues  map[string][]string // param_name -> valid enum values (nil if no enums)
}

// ToolSchemaMap maps tool names to their parameter information,
// used by SuggestionMiddleware to provide "did you mean" suggestions.
type ToolSchemaMap struct {
	tools      map[string]*ToolParamInfo
	maxDistance int // Levenshtein threshold (default 2 per D-01)
}

// BuildToolSchemaMap constructs a ToolSchemaMap from registered tool definitions.
// It extracts parameter names and enum constraints from each tool's InputSchema.
// Tools whose schema fails to parse are skipped gracefully.
func BuildToolSchemaMap(tools []*mcpsdk.Tool) *ToolSchemaMap {
	m := &ToolSchemaMap{
		tools:      make(map[string]*ToolParamInfo),
		maxDistance: 2,
	}
	for _, t := range tools {
		info := &ToolParamInfo{}

		// Marshal InputSchema -> JSON -> unmarshal to map to extract properties.
		data, err := json.Marshal(t.InputSchema)
		if err != nil {
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			continue
		}

		props, ok := schema["properties"].(map[string]any)
		if !ok {
			continue
		}

		for name, propVal := range props {
			info.ValidParams = append(info.ValidParams, name)
			// Extract enum values if present.
			if propMap, ok := propVal.(map[string]any); ok {
				if enumVals, ok := propMap["enum"].([]any); ok {
					strs := make([]string, 0, len(enumVals))
					for _, v := range enumVals {
						if s, ok := v.(string); ok {
							strs = append(strs, s)
						}
					}
					if len(strs) > 0 {
						if info.EnumValues == nil {
							info.EnumValues = make(map[string][]string)
						}
						info.EnumValues[name] = strs
					}
				}
			}
		}
		m.tools[t.Name] = info
	}
	return m
}

// enrichProtocolError intercepts SDK schema validation errors (protocol errors)
// and enriches them with "did you mean" suggestions. When a suggestion is found,
// the protocol error is converted to a tool error (CallToolResult with IsError=true)
// per Pitfall 3 -- agents handle tool errors better than protocol errors.
func (sm *ToolSchemaMap) enrichProtocolError(req mcpsdk.Request, result mcpsdk.Result, err error) (mcpsdk.Result, error) {
	toolName := extractToolName(req)
	info, ok := sm.tools[toolName]
	if !ok {
		return result, err
	}

	badParams := extractBadParams(err.Error())
	if len(badParams) == 0 {
		return result, err
	}

	// Find the single best suggestion across all bad params (D-06).
	bestMatch := ""
	bestBadParam := ""
	bestDist := sm.maxDistance + 1

	for _, badParam := range badParams {
		match, dist := bestParamSuggestion(badParam, info.ValidParams, sm.maxDistance)
		if match != "" && dist < bestDist {
			bestMatch = match
			bestBadParam = badParam
			bestDist = dist
		}
	}

	if bestMatch == "" {
		return result, err
	}

	// Convert protocol error to tool error with suggestion appended (D-04, Pitfall 3).
	enriched := fmt.Sprintf("%s\nDid you mean: %s (instead of %s)?", err.Error(), bestMatch, bestBadParam)
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: enriched},
		},
		IsError: true,
	}, nil
}

// enrichToolError enriches a tool error (IsError=true) with "did you mean"
// suggestions for enum values or parameter names. The original text is preserved;
// the suggestion is appended as a new line (D-04).
func (sm *ToolSchemaMap) enrichToolError(req mcpsdk.Request, ctr *mcpsdk.CallToolResult) {
	toolName := extractToolName(req)
	info, ok := sm.tools[toolName]
	if !ok {
		return
	}

	if len(ctr.Content) == 0 {
		return
	}
	tc, ok := ctr.Content[0].(*mcpsdk.TextContent)
	if !ok || tc == nil {
		return
	}
	errText := tc.Text

	// Try enum value correction: scan for patterns mentioning param names with enums.
	if info.EnumValues != nil {
		bestMatch := ""
		bestBadValue := ""
		bestDist := sm.maxDistance + 1

		for paramName, validValues := range info.EnumValues {
			if !strings.Contains(errText, paramName) {
				continue
			}
			// Try to extract bad value from error text: look for quoted strings.
			badValue := extractQuotedValue(errText, paramName)
			if badValue == "" {
				continue
			}
			match, dist := bestValueSuggestion(badValue, validValues, sm.maxDistance)
			if match != "" && dist < bestDist {
				bestMatch = match
				bestBadValue = badValue
				bestDist = dist
			}
		}

		if bestMatch != "" {
			tc.Text = fmt.Sprintf("%s\nDid you mean: %s (instead of %s)?", errText, bestMatch, bestBadValue)
			return
		}
	}

	// Try parameter name suggestions from error text.
	badParams := extractBadParams(errText)
	if len(badParams) == 0 {
		return
	}

	bestMatch := ""
	bestBadParam := ""
	bestDist := sm.maxDistance + 1

	for _, badParam := range badParams {
		match, dist := bestParamSuggestion(badParam, info.ValidParams, sm.maxDistance)
		if match != "" && dist < bestDist {
			bestMatch = match
			bestBadParam = badParam
			bestDist = dist
		}
	}

	if bestMatch != "" {
		tc.Text = fmt.Sprintf("%s\nDid you mean: %s (instead of %s)?", errText, bestMatch, bestBadParam)
	}
}

// extractQuotedValue extracts a quoted value that appears before a field name mention.
// Handles patterns like: invalid value "incming" for field "direction"
func extractQuotedValue(errText, fieldName string) string {
	// Look for pattern: "value" ... "fieldName"
	fieldIdx := strings.Index(errText, `"`+fieldName+`"`)
	if fieldIdx < 0 {
		return ""
	}
	// Search backwards from fieldIdx for the previous quoted string.
	prefix := errText[:fieldIdx]
	lastQuote := strings.LastIndex(prefix, `"`)
	if lastQuote < 0 {
		return ""
	}
	beforeLast := prefix[:lastQuote]
	openQuote := strings.LastIndex(beforeLast, `"`)
	if openQuote < 0 {
		return ""
	}
	return beforeLast[openQuote+1:]
}

// SuggestionMiddleware creates MCP middleware that enriches error responses
// with "did you mean" parameter and enum value suggestions. It intercepts
// both protocol errors (from SDK schema validation) and tool errors (IsError=true),
// looking up the correct parameter names from the pre-built ToolSchemaMap.
//
// Only tools/call requests are intercepted; all other methods pass through.
// Per D-09, suggestions only reference parameters from the same tool.
// Per D-10, no new error kinds are created.
func SuggestionMiddleware(schemaMap *ToolSchemaMap, logger *slog.Logger) mcpsdk.Middleware {
	return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			result, err := next(ctx, method, req)

			// Case 1: Protocol error from SDK schema validation.
			if err != nil {
				enrichedResult, enrichedErr := schemaMap.enrichProtocolError(req, result, err)
				if enrichedErr == nil && enrichedResult != nil {
					// Suggestion found and protocol error converted to tool error.
					toolName := extractToolName(req)
					logger.Debug("suggestion appended", "tool", toolName, "type", "protocol_error")
				}
				return enrichedResult, enrichedErr
			}

			// Case 2: Tool error (IsError=true).
			if ctr, ok := result.(*mcpsdk.CallToolResult); ok && ctr != nil && ctr.IsError {
				schemaMap.enrichToolError(req, ctr)
			}

			return result, err
		}
	}
}

// InstallSuggestionMiddleware wires the suggestion middleware onto the MCP SDK server.
func InstallSuggestionMiddleware(server *mcpsdk.Server, schemaMap *ToolSchemaMap, logger *slog.Logger) {
	server.AddReceivingMiddleware(SuggestionMiddleware(schemaMap, logger))
}
