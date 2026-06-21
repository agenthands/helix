package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"golang.org/x/tools/go/packages"
)

// argField describes one field of a tool's *Args struct, recovered from the
// struct's json/jsonschema tags.
type argField struct {
	// jsonKey is the wire/schema key from the `json:"..."` tag (the part before
	// any comma). When the json tag is absent the lowercased field name is used.
	jsonKey string
	// required is true when the json tag has NO `,omitempty` option.
	required bool
	// help is the `jsonschema:"..."` tag value (flag help text); falls back to
	// the json key when absent.
	help string
	// goType records the field's underlying kind so render can pick a flag kind:
	// "string" | "int" | "bool" | "[]string" | "" (opaque/non-scalar -> JSON).
	goType string
}

// argInfo binds a tool name to the fields recovered from its *Args struct.
type argInfo struct {
	toolName string
	fields   []argField
}

// scanToolArgs loads the given package patterns (e.g. "./internal/...") and
// walks every `AddTool(...)` call expression to recover, for each registered
// tool, the (toolName, *Args struct fields) binding the runtime drops (VERB-04).
//
// Two registration shapes are handled, both yielding the same (name, argsType):
//
//	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name:"x"}, kernel.WrapToolSpan(tr,"x", func(ctx, req, args XxxArgs)...))
//	mcpsdk.AddTool(srv, &mcpsdk.Tool{Name:"y"}, func(ctx, req, args YyyArgs)...)
//
// The 2nd argument's &mcpsdk.Tool{Name:"..."} composite literal supplies the
// name; the 3rd argument's func literal (directly, or descended into a
// WrapToolSpan call) supplies the *Args struct as its LAST parameter type.
//
// Registrations whose handler takes `args map[string]any` (the AddSkillTool
// dynamic fallback) have no struct to derive flags from; they are skipped here
// (recorded as empty-field tools by the caller's intersection step).
func scanToolArgs(patterns ...string) (map[string]*argInfo, error) {
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedName | packages.NeedDeps | packages.NeedImports,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("package load reported errors")
	}

	out := make(map[string]*argInfo)
	handle := func(call *ast.CallExpr, info *types.Info, toolVars map[string]*ast.CompositeLit) {
		name, argsType, ok := recoverNameAndArgs(call, info, toolVars)
		if !ok || argsType == nil {
			return // unrecognized or map[string]any dynamic handler
		}
		st, ok := underlyingStruct(argsType)
		if !ok {
			return
		}
		fields := structFields(st)
		if len(fields) == 0 {
			return
		}
		out[name] = &argInfo{toolName: name, fields: fields}
	}
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for _, file := range pkg.Syntax {
			// Resolve `tool := &mcpsdk.Tool{Name:"x"}` vars PER FUNCTION BODY so a
			// variable reused across functions (server.go uses `tool` in three
			// register* methods) resolves to the literal in its own scope.
			ast.Inspect(file, func(n ast.Node) bool {
				var body *ast.BlockStmt
				switch fn := n.(type) {
				case *ast.FuncDecl:
					body = fn.Body
				case *ast.FuncLit:
					body = fn.Body
				default:
					return true
				}
				if body == nil {
					return true
				}
				toolVars := collectToolVars(body)
				ast.Inspect(body, func(m ast.Node) bool {
					if call, ok := m.(*ast.CallExpr); ok && isAddToolCall(call) {
						handle(call, pkg.TypesInfo, toolVars)
					}
					return true
				})
				return true
			})
		}
	}
	return out, nil
}

// isAddToolCall reports whether the call expression is `<pkg>.AddTool(...)`.
func isAddToolCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "AddTool" && len(call.Args) >= 3
}

// collectToolVars walks a scope (a function body) for `name := &mcpsdk.Tool{...}`
// (and var-spec) assignments, returning a map from the local identifier name to
// the composite literal so AddTool calls that pass the tool by variable can
// resolve the Name. Scoping per function body avoids a same-named var in another
// function clobbering the binding.
func collectToolVars(scope ast.Node) map[string]*ast.CompositeLit {
	vars := make(map[string]*ast.CompositeLit)
	record := func(lhs ast.Expr, rhs ast.Expr) {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			return
		}
		if u, ok := rhs.(*ast.UnaryExpr); ok {
			rhs = u.X
		}
		if cl, ok := rhs.(*ast.CompositeLit); ok {
			vars[id.Name] = cl
		}
	}
	ast.Inspect(scope, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) == len(s.Rhs) {
				for i := range s.Lhs {
					record(s.Lhs[i], s.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			if len(s.Names) == len(s.Values) {
				for i := range s.Names {
					record(s.Names[i], s.Values[i])
				}
			}
		}
		return true
	})
	return vars
}

// recoverNameAndArgs extracts the tool name (2nd arg composite literal Name:)
// and the *Args struct type (3rd arg func literal's last param). A nil argsType
// with ok=true signals a recognized map[string]any dynamic handler to skip.
func recoverNameAndArgs(call *ast.CallExpr, info *types.Info, toolVars map[string]*ast.CompositeLit) (name string, argsType types.Type, ok bool) {
	name, ok = toolNameFromArg(call.Args[1], toolVars)
	if !ok || name == "" {
		return "", nil, false
	}
	argsType, ok = argsTypeFromHandler(call.Args[2], info)
	if !ok {
		return "", nil, false
	}
	return name, argsType, true
}

// toolNameFromArg pulls the Name string literal out of a &mcpsdk.Tool{Name:"..."}
// composite literal (the 2nd AddTool argument), resolving the argument through a
// local variable when AddTool was passed the tool by identifier.
func toolNameFromArg(arg ast.Expr, toolVars map[string]*ast.CompositeLit) (string, bool) {
	if u, ok := arg.(*ast.UnaryExpr); ok {
		arg = u.X
	}
	if id, ok := arg.(*ast.Ident); ok {
		if cl, found := toolVars[id.Name]; found {
			arg = cl
		}
	}
	cl, ok := arg.(*ast.CompositeLit)
	if !ok {
		return "", false
	}
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Name" {
			continue
		}
		lit, ok := kv.Value.(*ast.BasicLit)
		if !ok {
			return "", false
		}
		return strings.Trim(lit.Value, "`\""), true
	}
	return "", false
}

// argsTypeFromHandler resolves the *Args struct type from the 3rd AddTool
// argument. The handler is either a direct func literal, or a
// WrapToolSpan(tracer, "name", funcLit) call whose 3rd argument is the func
// literal. In both cases the func literal's LAST parameter type is the args
// struct. A map[string]any last parameter returns (nil, true) to signal a
// dynamic handler the caller should skip.
func argsTypeFromHandler(arg ast.Expr, info *types.Info) (types.Type, bool) {
	fn := funcLitFromHandler(arg)
	if fn == nil {
		return nil, false
	}
	params := fn.Type.Params
	if params == nil || len(params.List) == 0 {
		return nil, false
	}
	last := params.List[len(params.List)-1]
	t := info.TypeOf(last.Type)
	if t == nil {
		return nil, false
	}
	if _, isMap := t.Underlying().(*types.Map); isMap {
		return nil, true // dynamic map[string]any handler — skip
	}
	return t, true
}

// funcLitFromHandler returns the handler func literal, descending into a
// WrapToolSpan(...) wrapper when present.
func funcLitFromHandler(arg ast.Expr) *ast.FuncLit {
	switch e := arg.(type) {
	case *ast.FuncLit:
		return e
	case *ast.CallExpr:
		// WrapToolSpan(tracer, "name", funcLit) — the func literal is the last arg.
		if len(e.Args) == 0 {
			return nil
		}
		if fl, ok := e.Args[len(e.Args)-1].(*ast.FuncLit); ok {
			return fl
		}
	}
	return nil
}

// underlyingStruct unwraps pointer/named types to the underlying *types.Struct.
func underlyingStruct(t types.Type) (*types.Struct, bool) {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	st, ok := t.Underlying().(*types.Struct)
	return st, ok
}

// structFields enumerates exported struct fields, reading json/jsonschema tags.
func structFields(st *types.Struct) []argField {
	var fields []argField
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		tag := reflect.StructTag(st.Tag(i))
		jsonTag := tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		key, omitempty := parseJSONTag(jsonTag, f.Name())
		help := tag.Get("jsonschema")
		if help == "" {
			help = key
		}
		fields = append(fields, argField{
			jsonKey:  key,
			required: !omitempty,
			help:     help,
			goType:   classifyType(f.Type()),
		})
	}
	return fields
}

// parseJSONTag splits a json struct tag into its key and omitempty flag,
// falling back to the lowercased field name when the tag is absent.
func parseJSONTag(tag, fieldName string) (key string, omitempty bool) {
	if tag == "" {
		return strings.ToLower(fieldName), false
	}
	parts := strings.Split(tag, ",")
	key = parts[0]
	if key == "" {
		key = strings.ToLower(fieldName)
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
		}
	}
	return key, omitempty
}

// classifyType maps a field's Go type to a flag-kind discriminator string:
// "string" | "int" | "bool" | "[]string" | "" (opaque/non-scalar -> JSON flag).
func classifyType(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			return "string"
		case u.Info()&types.IsInteger != 0:
			return "int"
		case u.Info()&types.IsBoolean != 0:
			return "bool"
		}
	case *types.Slice:
		if eb, ok := u.Elem().Underlying().(*types.Basic); ok && eb.Info()&types.IsString != 0 {
			return "[]string"
		}
	}
	return "" // opaque/non-scalar -> raw JSON flag
}
