// Command lspgen generates Go types from the LSP 3.17 metaModel.json.
//
// It reads protocol/metaModel.json and produces four files in protocol/gen/:
//   - tsprotocol.go: All struct types, const enumerations, type aliases
//   - tsclient.go:   Client-side method signatures (request senders)
//   - tsserver.go:   Server-side dispatch table (notification handling)
//   - tsjson.go:     Custom MarshalJSON/UnmarshalJSON for union types
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// MetaModel is the top-level structure of metaModel.json.
type MetaModel struct {
	MetaData      MetaData       `json:"metaData"`
	Requests      []Request      `json:"requests"`
	Notifications []Notification `json:"notifications"`
	Structures    []Structure    `json:"structures"`
	Enumerations  []Enumeration  `json:"enumerations"`
	TypeAliases   []TypeAlias    `json:"typeAliases"`
}

// MetaData contains version information.
type MetaData struct {
	Version string `json:"version"`
}

// Request is an LSP request definition.
type Request struct {
	Method            string  `json:"method"`
	Params            *Type   `json:"params,omitempty"`
	Result            *Type   `json:"result,omitempty"`
	PartialResult     *Type   `json:"partialResult,omitempty"`
	RegistrationOptions *Type `json:"registrationOptions,omitempty"`
	Documentation     string  `json:"documentation,omitempty"`
	Since             string  `json:"since,omitempty"`
	Proposed          bool    `json:"proposed,omitempty"`
	MessageDirection  string  `json:"messageDirection,omitempty"`
}

// Notification is an LSP notification definition.
type Notification struct {
	Method            string  `json:"method"`
	Params            *Type   `json:"params,omitempty"`
	RegistrationOptions *Type `json:"registrationOptions,omitempty"`
	Documentation     string  `json:"documentation,omitempty"`
	Since             string  `json:"since,omitempty"`
	Proposed          bool    `json:"proposed,omitempty"`
	MessageDirection  string  `json:"messageDirection,omitempty"`
}

// Structure is an LSP structure (struct) definition.
type Structure struct {
	Name          string     `json:"name"`
	Properties    []Property `json:"properties,omitempty"`
	Extends       []Type     `json:"extends,omitempty"`
	Mixins        []Type     `json:"mixins,omitempty"`
	Documentation string     `json:"documentation,omitempty"`
	Since         string     `json:"since,omitempty"`
	Proposed      bool       `json:"proposed,omitempty"`
}

// Property is a field within a structure.
type Property struct {
	Name          string `json:"name"`
	Type          Type   `json:"type"`
	Optional      bool   `json:"optional,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	Since         string `json:"since,omitempty"`
	Proposed      bool   `json:"proposed,omitempty"`
}

// Enumeration is an LSP enum type.
type Enumeration struct {
	Name               string             `json:"name"`
	Type               Type               `json:"type"`
	Values             []EnumerationEntry `json:"values"`
	SupportsCustomValues bool             `json:"supportsCustomValues,omitempty"`
	Documentation      string             `json:"documentation,omitempty"`
	Since              string             `json:"since,omitempty"`
	Proposed           bool               `json:"proposed,omitempty"`
}

// EnumerationEntry is a single value in an enumeration.
type EnumerationEntry struct {
	Name          string      `json:"name"`
	Value         interface{} `json:"value"`
	Documentation string      `json:"documentation,omitempty"`
	Since         string      `json:"since,omitempty"`
	Proposed      bool        `json:"proposed,omitempty"`
}

// TypeAlias is an LSP type alias.
type TypeAlias struct {
	Name          string `json:"name"`
	Type          Type   `json:"type"`
	Documentation string `json:"documentation,omitempty"`
	Since         string `json:"since,omitempty"`
	Proposed      bool   `json:"proposed,omitempty"`
}

// Type represents a type reference in the metamodel.
type Type struct {
	Kind    string `json:"kind"`
	Name    string `json:"name,omitempty"`
	Items   []Type `json:"items,omitempty"`
	Element *Type  `json:"element,omitempty"`
	Key     *Type  `json:"key,omitempty"`
	Value   interface{} `json:"value,omitempty"`
}

func main() {
	// Find the project root relative to the source file location.
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	modelPath := filepath.Join(projectRoot, "protocol", "metaModel.json")
	outDir := filepath.Join(projectRoot, "protocol", "gen")

	data, err := os.ReadFile(modelPath)
	if err != nil {
		log.Fatalf("reading metaModel.json: %v", err)
	}

	var model MetaModel
	if err := json.Unmarshal(data, &model); err != nil {
		log.Fatalf("parsing metaModel.json: %v", err)
	}

	log.Printf("LSP metaModel version %s: %d structures, %d enumerations, %d requests, %d notifications, %d typeAliases",
		model.MetaData.Version,
		len(model.Structures), len(model.Enumerations),
		len(model.Requests), len(model.Notifications), len(model.TypeAliases))

	g := &Generator{
		model:      &model,
		outDir:     outDir,
		orTypes:    make(map[string]*OrType),
		nameCache:  make(map[string]bool),
	}

	g.collectOrTypes()

	if err := g.generateProtocol(); err != nil {
		log.Fatalf("generating tsprotocol.go: %v", err)
	}
	if err := g.generateClient(); err != nil {
		log.Fatalf("generating tsclient.go: %v", err)
	}
	if err := g.generateServer(); err != nil {
		log.Fatalf("generating tsserver.go: %v", err)
	}
	if err := g.generateJSON(); err != nil {
		log.Fatalf("generating tsjson.go: %v", err)
	}

	log.Printf("Generated LSP protocol types in %s", outDir)
}

// OrType represents a union type (Or_X_Y) that needs custom JSON handling.
type OrType struct {
	Name    string
	Types   []Type
	GoTypes []string // resolved Go type names
}

// Generator holds state for the code generation process.
type Generator struct {
	model     *MetaModel
	outDir    string
	orTypes   map[string]*OrType // key is the Or_X_Y name
	nameCache map[string]bool
}

// collectOrTypes walks all types in the model to find union (or) types.
func (g *Generator) collectOrTypes() {
	seen := make(map[string]bool)

	var walk func(t Type, parent, field string)
	walk = func(t Type, parent, field string) {
		switch t.Kind {
		case "or":
			name := g.orTypeName(t)
			// Only register as Or_ type if the name actually starts with "Or_"
			// (i.e., it's a true union, not a simplified X|null -> X case)
			if strings.HasPrefix(name, "Or_") && !seen[name] {
				seen[name] = true
				goTypes := make([]string, len(t.Items))
				for i, item := range t.Items {
					goTypes[i] = g.goType(item)
				}
				g.orTypes[name] = &OrType{
					Name:    name,
					Types:   t.Items,
					GoTypes: goTypes,
				}
			}
			// Walk items for nested or types
			for _, item := range t.Items {
				walk(item, parent, field)
			}
		case "array":
			if t.Element != nil {
				walk(*t.Element, parent, field)
			}
		case "map":
			if t.Key != nil {
				walk(*t.Key, parent, field)
			}
			if vt, ok := t.Value.(map[string]interface{}); ok {
				var valType Type
				b, _ := json.Marshal(vt)
				_ = json.Unmarshal(b, &valType)
				walk(valType, parent, field)
			}
		case "literal":
			// literal types have properties encoded in value
		case "and":
			for _, item := range t.Items {
				walk(item, parent, field)
			}
		}
	}

	for _, s := range g.model.Structures {
		for _, p := range s.Properties {
			walk(p.Type, s.Name, p.Name)
		}
	}
	for _, r := range g.model.Requests {
		if r.Params != nil {
			walk(*r.Params, r.Method, "params")
		}
		if r.Result != nil {
			walk(*r.Result, r.Method, "result")
		}
	}
	for _, n := range g.model.Notifications {
		if n.Params != nil {
			walk(*n.Params, n.Method, "params")
		}
	}
	for _, ta := range g.model.TypeAliases {
		walk(ta.Type, ta.Name, "")
	}
}

// generateProtocol writes tsprotocol.go with all structs, enums, and type aliases.
func (g *Generator) generateProtocol() error {
	var b strings.Builder

	writeHeader(&b, "gen", g.model.MetaData.Version)

	// Enumerations
	for _, e := range g.model.Enumerations {
		writeDoc(&b, e.Documentation)
		goName := exportName(e.Name)
		baseType := g.goBaseType(e.Type)
		fmt.Fprintf(&b, "type %s %s\n\n", goName, baseType)

		if len(e.Values) > 0 {
			fmt.Fprintf(&b, "const (\n")
			for _, v := range e.Values {
				writeDoc(&b, v.Documentation)
				constName := goName + exportName(v.Name)
				switch val := v.Value.(type) {
				case string:
					fmt.Fprintf(&b, "\t%s %s = %q\n", constName, goName, val)
				case float64:
					if baseType == "uint32" || baseType == "int32" {
						fmt.Fprintf(&b, "\t%s %s = %d\n", constName, goName, int64(val))
					} else {
						fmt.Fprintf(&b, "\t%s %s = %v\n", constName, goName, val)
					}
				default:
					fmt.Fprintf(&b, "\t%s %s = %v\n", constName, goName, val)
				}
			}
			fmt.Fprintf(&b, ")\n\n")
		}
	}

	// Structures
	for _, s := range g.model.Structures {
		writeDoc(&b, s.Documentation)
		goName := exportName(s.Name)
		fmt.Fprintf(&b, "type %s struct {\n", goName)

		// Embedded extends/mixins
		for _, ext := range s.Extends {
			fmt.Fprintf(&b, "\t%s\n", g.goType(ext))
		}
		for _, mixin := range s.Mixins {
			fmt.Fprintf(&b, "\t%s\n", g.goType(mixin))
		}

		// Properties
		for _, p := range s.Properties {
			writeDoc(&b, p.Documentation)
			fieldName := goFieldName(p.Name)
			fieldType := g.goType(p.Type)
			jsonTag := p.Name
			omit := ""
			if p.Optional {
				omit = ",omitempty"
				fieldType = ptrType(fieldType)
			}
			fmt.Fprintf(&b, "\t%s %s `json:\"%s%s\"`\n", fieldName, fieldType, jsonTag, omit)
		}
		fmt.Fprintf(&b, "}\n\n")
	}

	// Type aliases
	for _, ta := range g.model.TypeAliases {
		goName := exportName(ta.Name)
		goType := g.goType(ta.Type)

		// Skip aliases where the name itself is not a valid identifier
		// (e.g., LSPAny -> interface{}, LSPObject -> map[string]interface{})
		if !isValidAliasTarget(goName) {
			fmt.Fprintf(&b, "// %s is represented as %s in Go.\n\n", ta.Name, goType)
			continue
		}
		// Skip aliases where the name resolves to the same thing
		if goName == goType {
			continue
		}

		writeDoc(&b, ta.Documentation)
		if isValidAliasTarget(goType) {
			// Standard type alias: type X = Y
			fmt.Fprintf(&b, "type %s = %s\n\n", goName, goType)
		} else {
			// Non-identifier target: use a named type instead of alias
			// e.g., type DocumentSelector []DocumentFilter
			fmt.Fprintf(&b, "type %s %s\n\n", goName, goType)
		}
	}

	// Or_ union type definitions (struct declarations only, methods in tsjson.go)
	names := make([]string, 0, len(g.orTypes))
	for name := range g.orTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ot := g.orTypes[name]
		fmt.Fprintf(&b, "// %s represents a union type: %s.\n", name, strings.Join(ot.GoTypes, " | "))
		fmt.Fprintf(&b, "type %s struct {\n", name)
		fmt.Fprintf(&b, "\tValue interface{}\n")
		fmt.Fprintf(&b, "}\n\n")
	}

	return writeFormatted(filepath.Join(g.outDir, "tsprotocol.go"), b.String())
}

// generateClient writes tsclient.go with client method signatures.
func (g *Generator) generateClient() error {
	var b strings.Builder

	writeHeader(&b, "gen", g.model.MetaData.Version)

	fmt.Fprintf(&b, "// ClientInterface defines the methods a client can send to a server.\n")
	fmt.Fprintf(&b, "type ClientInterface interface {\n")
	for _, r := range g.model.Requests {
		if r.MessageDirection == "clientToServer" || r.MessageDirection == "" {
			methodName := methodToGoName(r.Method)
			writeDoc(&b, r.Documentation)
			paramType := "interface{}"
			if r.Params != nil {
				paramType := g.goType(*r.Params)
				_ = paramType
			}
			resultType := "interface{}"
			if r.Result != nil {
				resultType = g.goType(*r.Result)
			}
			_ = paramType
			if r.Params != nil {
				fmt.Fprintf(&b, "\t%s(params *%s) (*%s, error)\n", methodName, g.goType(*r.Params), resultType)
			} else {
				fmt.Fprintf(&b, "\t%s() (*%s, error)\n", methodName, resultType)
			}
		}
	}
	fmt.Fprintf(&b, "}\n\n")

	// Server-to-client requests
	fmt.Fprintf(&b, "// ServerToClientInterface defines methods the server can send to the client.\n")
	fmt.Fprintf(&b, "type ServerToClientInterface interface {\n")
	for _, r := range g.model.Requests {
		if r.MessageDirection == "serverToClient" {
			methodName := methodToGoName(r.Method)
			writeDoc(&b, r.Documentation)
			resultType := "interface{}"
			if r.Result != nil {
				resultType = g.goType(*r.Result)
			}
			if r.Params != nil {
				fmt.Fprintf(&b, "\t%s(params *%s) (*%s, error)\n", methodName, g.goType(*r.Params), resultType)
			} else {
				fmt.Fprintf(&b, "\t%s() (*%s, error)\n", methodName, resultType)
			}
		}
	}
	fmt.Fprintf(&b, "}\n\n")

	// Request method constants
	fmt.Fprintf(&b, "// LSP request method constants.\n")
	fmt.Fprintf(&b, "const (\n")
	for _, r := range g.model.Requests {
		constName := "Method" + methodToGoName(r.Method)
		fmt.Fprintf(&b, "\t%s = %q\n", constName, r.Method)
	}
	fmt.Fprintf(&b, ")\n\n")

	return writeFormatted(filepath.Join(g.outDir, "tsclient.go"), b.String())
}

// generateServer writes tsserver.go with server dispatch tables.
func (g *Generator) generateServer() error {
	var b strings.Builder

	writeHeader(&b, "gen", g.model.MetaData.Version)

	// ServerInterface for handling client-to-server requests
	fmt.Fprintf(&b, "// ServerInterface defines the methods a server must implement.\n")
	fmt.Fprintf(&b, "type ServerInterface interface {\n")
	for _, r := range g.model.Requests {
		if r.MessageDirection == "clientToServer" || r.MessageDirection == "" {
			methodName := methodToGoName(r.Method)
			writeDoc(&b, r.Documentation)
			resultType := "interface{}"
			if r.Result != nil {
				resultType = g.goType(*r.Result)
			}
			if r.Params != nil {
				fmt.Fprintf(&b, "\t%s(params *%s) (*%s, error)\n", methodName, g.goType(*r.Params), resultType)
			} else {
				fmt.Fprintf(&b, "\t%s() (*%s, error)\n", methodName, resultType)
			}
		}
	}
	fmt.Fprintf(&b, "}\n\n")

	// Notification method constants
	fmt.Fprintf(&b, "// LSP notification method constants.\n")
	fmt.Fprintf(&b, "const (\n")
	for _, n := range g.model.Notifications {
		constName := "MethodNotification" + methodToGoName(n.Method)
		fmt.Fprintf(&b, "\t%s = %q\n", constName, n.Method)
	}
	fmt.Fprintf(&b, ")\n\n")

	// NotificationHandler interface
	fmt.Fprintf(&b, "// NotificationHandler handles LSP notifications.\n")
	fmt.Fprintf(&b, "type NotificationHandler interface {\n")
	for _, n := range g.model.Notifications {
		methodName := methodToGoName(n.Method)
		writeDoc(&b, n.Documentation)
		if n.Params != nil {
			fmt.Fprintf(&b, "\t%s(params *%s) error\n", methodName, g.goType(*n.Params))
		} else {
			fmt.Fprintf(&b, "\t%s() error\n", methodName)
		}
	}
	fmt.Fprintf(&b, "}\n\n")

	// Dispatch function
	fmt.Fprintf(&b, "// NotificationDispatch maps method names to dispatch functions.\n")
	fmt.Fprintf(&b, "var NotificationDispatch = map[string]string{\n")
	for _, n := range g.model.Notifications {
		goName := methodToGoName(n.Method)
		fmt.Fprintf(&b, "\t%q: %q,\n", n.Method, goName)
	}
	fmt.Fprintf(&b, "}\n\n")

	return writeFormatted(filepath.Join(g.outDir, "tsserver.go"), b.String())
}

// generateJSON writes tsjson.go with custom JSON marshal/unmarshal for union types.
func (g *Generator) generateJSON() error {
	var b strings.Builder

	writeHeader(&b, "gen", g.model.MetaData.Version)
	fmt.Fprintf(&b, "import (\n")
	fmt.Fprintf(&b, "\t\"encoding/json\"\n")
	fmt.Fprintf(&b, "\t\"fmt\"\n")
	fmt.Fprintf(&b, ")\n\n")

	names := make([]string, 0, len(g.orTypes))
	for name := range g.orTypes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		ot := g.orTypes[name]

		// Deduplicate Go types (e.g., multiple literal types all resolve to map[string]interface{})
		uniqueGoTypes := deduplicateStrings(ot.GoTypes)

		// MarshalJSON
		fmt.Fprintf(&b, "func (t %s) MarshalJSON() ([]byte, error) {\n", name)
		fmt.Fprintf(&b, "\tif t.Value == nil {\n")
		fmt.Fprintf(&b, "\t\treturn []byte(\"null\"), nil\n")
		fmt.Fprintf(&b, "\t}\n")
		fmt.Fprintf(&b, "\treturn json.Marshal(t.Value)\n")
		fmt.Fprintf(&b, "}\n\n")

		// UnmarshalJSON - try each unique type
		fmt.Fprintf(&b, "func (t *%s) UnmarshalJSON(data []byte) error {\n", name)
		fmt.Fprintf(&b, "\tif string(data) == \"null\" {\n")
		fmt.Fprintf(&b, "\t\tt.Value = nil\n")
		fmt.Fprintf(&b, "\t\treturn nil\n")
		fmt.Fprintf(&b, "\t}\n")

		for i, goType := range uniqueGoTypes {
			varName := fmt.Sprintf("v%d", i)
			fmt.Fprintf(&b, "\tvar %s %s\n", varName, goType)
			fmt.Fprintf(&b, "\tif err := json.Unmarshal(data, &%s); err == nil {\n", varName)
			fmt.Fprintf(&b, "\t\tt.Value = %s\n", varName)
			fmt.Fprintf(&b, "\t\treturn nil\n")
			fmt.Fprintf(&b, "\t}\n")
		}

		fmt.Fprintf(&b, "\treturn fmt.Errorf(\"%s: cannot unmarshal %%s\", string(data))\n", name)
		fmt.Fprintf(&b, "}\n\n")

		// Typed accessors: As<Type>() and Set<Type>() - deduplicated
		seenAccessor := make(map[string]bool)
		for _, goType := range uniqueGoTypes {
			shortName := accessorName(goType)
			if seenAccessor[shortName] {
				continue
			}
			seenAccessor[shortName] = true

			fmt.Fprintf(&b, "func (t %s) As%s() (%s, bool) {\n", name, shortName, goType)
			fmt.Fprintf(&b, "\tv, ok := t.Value.(%s)\n", goType)
			fmt.Fprintf(&b, "\treturn v, ok\n")
			fmt.Fprintf(&b, "}\n\n")

			fmt.Fprintf(&b, "func (t *%s) Set%s(v %s) {\n", name, shortName, goType)
			fmt.Fprintf(&b, "\tt.Value = v\n")
			fmt.Fprintf(&b, "}\n\n")
		}
	}

	return writeFormatted(filepath.Join(g.outDir, "tsjson.go"), b.String())
}
