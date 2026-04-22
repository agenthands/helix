package help

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ParamDoc holds documentation for a single tool parameter.
type ParamDoc struct {
	Name        string
	Type        string
	Description string
	Required    bool
	EnumValues  []string
}

// ExtractParamDocs extracts parameter documentation from a tool's InputSchema.
// Uses the same json.Marshal -> map pattern proven in suggest.go.
func ExtractParamDocs(inputSchema any) []ParamDoc {
	data, err := json.Marshal(inputSchema)
	if err != nil {
		return nil
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil
	}

	required := map[string]bool{}
	if reqArr, ok := schema["required"].([]any); ok {
		for _, r := range reqArr {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}

	var docs []ParamDoc
	for name, propVal := range props {
		doc := ParamDoc{Name: name, Required: required[name]}
		if propMap, ok := propVal.(map[string]any); ok {
			if t, ok := propMap["type"].(string); ok {
				doc.Type = t
			}
			if d, ok := propMap["description"].(string); ok {
				doc.Description = d
			}
			if enumVals, ok := propMap["enum"].([]any); ok {
				for _, v := range enumVals {
					if s, ok := v.(string); ok {
						doc.EnumValues = append(doc.EnumValues, s)
					}
				}
			}
		}
		docs = append(docs, doc)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Name < docs[j].Name })
	return docs
}

// FormatHelp formats comprehensive help text for a tool.
// Combines full description, parameter docs from schema, and co-located helpText.
func FormatHelp(toolName, fullDescription string, params []ParamDoc, helpText string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", toolName))
	b.WriteString(fullDescription)
	b.WriteString("\n\n")

	if len(params) > 0 {
		b.WriteString("## Parameters\n\n")
		for _, p := range params {
			reqStr := "optional"
			if p.Required {
				reqStr = "required"
			}
			b.WriteString(fmt.Sprintf("- **%s** (%s, %s)", p.Name, p.Type, reqStr))
			if p.Description != "" {
				b.WriteString(fmt.Sprintf(": %s", p.Description))
			}
			if len(p.EnumValues) > 0 {
				b.WriteString(fmt.Sprintf(" [values: %s]", strings.Join(p.EnumValues, ", ")))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if helpText != "" {
		b.WriteString(helpText)
		b.WriteString("\n")
	}

	return b.String()
}
