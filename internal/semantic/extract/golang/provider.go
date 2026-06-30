// Package goextract is the per-language tree-sitter extraction provider
// for Go source files. It is constructed via NewProvider with the
// daemon-injected *treesitter.GrammarRegistry (BUG-04 invariant) and
// satisfies the extract.Provider interface.
//
// Per Phase 59 D-01 the embedded queries.scm is NET-NEW — it is not copied
// from internal/repomap/queries/go_tags.scm. Per Phase 59 D-02 there is no
// init() registration and no blank-import wiring.
package goextract

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/treesitter"
)

//go:embed queries.scm
var goQueries string

// Provider is the concrete Go extraction provider. It implements
// extract.Provider plus an Extract method used by the test driver and
// the Phase 59 P03 scheduler.
type Provider struct {
	grammar *tree_sitter.Language
	query   *tree_sitter.Query
}

// NewProvider compiles the embedded queries against the injected
// GrammarRegistry's "go" grammar. Panics on missing grammar (wiring bug)
// or query-compile failure (build bug). Both are start-time conditions.
func NewProvider(grammars *treesitter.GrammarRegistry) extract.Provider {
	if grammars == nil {
		panic("goextract.NewProvider: nil GrammarRegistry")
	}
	g, ok := grammars.GetLanguage("go")
	if !ok {
		panic("goextract.NewProvider: 'go' grammar missing from injected GrammarRegistry")
	}
	q, qerr := tree_sitter.NewQuery(g, goQueries)
	if qerr != nil {
		panic("goextract.NewProvider: query compile failed: " + qerr.Message)
	}
	return &Provider{grammar: g, query: q}
}

// Language returns the canonical language identifier.
func (p *Provider) Language() string { return "go" }

// Extensions returns the file extensions this provider claims.
func (p *Provider) Extensions() []string { return []string{".go"} }

// TreeSitterLanguage returns the daemon-owned grammar pointer.
func (p *Provider) TreeSitterLanguage() *tree_sitter.Language { return p.grammar }

// Queries returns the raw embedded queries.scm text.
func (p *Provider) Queries() string { return goQueries }

// SupportsLSPEnrichment indicates Phase 61 may merge LSP facts with this
// provider's emit. Go has stable LSP coverage via gopls.
func (p *Provider) SupportsLSPEnrichment() bool { return true }

// Extract parses the source bytes and returns an ExtractedFile per the
// Phase 59 P02 fact contract. Confidence is stamped to ConfidenceTSOnly
// (0.70) per SPEC §11.2.
func (p *Provider) Extract(ctx context.Context, source []byte, file extract.SourceFile) (*extract.ExtractedFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(p.grammar); err != nil {
		return partialFile(file, extract.PartialReasonExtractorBug, fmt.Errorf("set go language: %w", err)), nil
	}
	tree := parser.Parse(source, nil)
	if tree == nil {
		return partialFile(file, extract.PartialReasonParseError, fmt.Errorf("parse returned nil")), nil
	}
	defer tree.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	out := &extract.ExtractedFile{
		File: extract.FileFact{
			Path:             file.Path,
			Language:         "go",
			ExtractionStatus: extract.ExtractionStatusReady,
			ExtractorName:    "goextract",
			ExtractorVersion: "1",
		},
	}

	captureNames := p.query.CaptureNames()
	matches := cursor.Matches(p.query, tree.RootNode(), source)
	// Deduplicate symbol emit when multiple queries match the same node
	// (e.g. a struct type_spec matches both definition.struct and the
	// generic type_spec rule). Key on (kind, name, start-line, start-col).
	// First match wins; specific captures (struct, interface) appear before
	// the catch-all `definition.type` rule in queries.scm.
	seenSymbol := make(map[string]bool)
	for m := matches.Next(); m != nil; m = matches.Next() {
		if err := ctx.Err(); err != nil {
			return partialFile(file, extract.PartialReasonTimeout, err), nil
		}

		var (
			defKind     extract.SymbolKind
			defNameNode *tree_sitter.Node
			defBodyNode *tree_sitter.Node
			recvType    string
			recvName    string
			refKind     string
			refNameNode *tree_sitter.Node
			importSrc   string
			importAlias string
			importRange extract.Range
			haveImport  bool
		)

		for _, c := range m.Captures {
			cn := captureNames[c.Index]
			node := c.Node
			switch {
			case cn == "definition.function":
				defKind = extract.KindFunction
				defNameNode = &node
			case cn == "definition.method":
				defKind = extract.KindMethod
				defNameNode = &node
			case cn == "definition.struct":
				defKind = extract.KindStruct
				defNameNode = &node
			case cn == "definition.interface":
				defKind = extract.KindInterface
				defNameNode = &node
			case cn == "definition.type":
				defKind = extract.KindType
				defNameNode = &node
			case cn == "definition.constant":
				defKind = extract.KindConstant
				defNameNode = &node
			case cn == "definition.variable":
				defKind = extract.KindVariable
				defNameNode = &node
			case cn == "definition.field":
				defKind = extract.KindField
				defNameNode = &node
			case cn == "definition.parameter":
				defKind = extract.KindParameter
				defNameNode = &node
			case cn == "receiver.type":
				recvType = node.Utf8Text(source)
			case cn == "receiver.name":
				recvName = node.Utf8Text(source)
			case cn == "reference.call":
				refKind = "call"
				refNameNode = &node
			case cn == "reference.identifier":
				refKind = "identifier"
				refNameNode = &node
			case cn == "reference.field":
				refKind = "field"
				refNameNode = &node
			case cn == "reference.type":
				refKind = "type"
				refNameNode = &node
			case cn == "import.source":
				importSrc = trimQuotes(node.Utf8Text(source))
				importRange = nodeRange(node)
				haveImport = true
			case cn == "import.alias":
				importAlias = node.Utf8Text(source)
			case cn == "heritage.embeds":
				out.Heritage = append(out.Heritage, extract.HeritageFact{
					Language: "go",
					Relation: "embeds",
					Target:   node.Utf8Text(source),
					Range:    nodeRange(node),
				})
			case strings.HasPrefix(cn, "def.") && cn != "def.method.body":
				defBodyNode = &node
			case cn == "def.method.body":
				defBodyNode = &node
			case cn == "type.return", cn == "type.annotation", cn == "type.parameter":
				out.Types = append(out.Types, extract.TypeFact{
					Language:   "go",
					Annotation: condenseWhitespace(node.Utf8Text(source)),
					Range:      nodeRange(node),
				})
			}
		}

		if defNameNode != nil && defKind != "" {
			name := defNameNode.Utf8Text(source)
			startPos := defNameNode.StartPosition()
			dedupKey := string(defKind) + "\x00" + name + "\x00" +
				itoa(int(startPos.Row)) + "\x00" + itoa(int(startPos.Column))
			if seenSymbol[dedupKey] {
				continue
			}
			// Also dedupe across kinds for the struct/type and interface/type
			// overlap on the same node — first kind wins (struct beats type).
			alt := string(extract.KindType) + "\x00" + name + "\x00" +
				itoa(int(startPos.Row)) + "\x00" + itoa(int(startPos.Column))
			if defKind != extract.KindType && seenSymbol[alt] {
				// the type-row was emitted earlier; remove it in favor of
				// the more specific kind. Walk back and drop.
				for i := len(out.Symbols) - 1; i >= 0; i-- {
					if out.Symbols[i].Kind == extract.KindType && out.Symbols[i].Name == name &&
						out.Symbols[i].SelectionRange.Start.Line == uint32(startPos.Row) &&
						out.Symbols[i].SelectionRange.Start.Column == uint32(startPos.Column) {
						out.Symbols = append(out.Symbols[:i], out.Symbols[i+1:]...)
						break
					}
				}
			} else if defKind == extract.KindType {
				// if a more specific kind was already emitted, skip this type-row.
				for _, k := range []extract.SymbolKind{extract.KindStruct, extract.KindInterface, extract.KindClass} {
					altK := string(k) + "\x00" + name + "\x00" +
						itoa(int(startPos.Row)) + "\x00" + itoa(int(startPos.Column))
					if seenSymbol[altK] {
						defNameNode = nil // signal skip
						break
					}
				}
			}
			seenSymbol[dedupKey] = true
			if defNameNode == nil {
				continue
			}
			body := defNameNode
			if defBodyNode != nil {
				body = defBodyNode
			}
			sf := extract.SymbolFact{
				Language:         "go",
				Kind:             defKind,
				Name:             name,
				QualifiedName:    qualifiedName(name, recvType, defKind),
				File:             file.Path,
				Range:            nodeRange(*body),
				SelectionRange:   nodeRange(*defNameNode),
				Visibility:       visibility(name),
				Confidence:       extract.ConfidenceTSOnly,
				ExtractionSource: "tree_sitter",
			}
			if defKind == extract.KindMethod && recvType != "" {
				sf.Receiver = &extract.ReceiverFact{Type: recvType, Name: recvName}
			}
			sf.SignatureHash = signatureHash(*body, source)
			sf.Signature = condenseWhitespace(body.Utf8Text(source))
			if len(sf.Signature) > 200 {
				sf.Signature = sf.Signature[:200]
			}
			sf.StableKey = extract.BuildProviderKey(extract.SymbolMeta{
				Language:      "go",
				PackagePath:   packagePathFromFile(file.Path),
				OwnerPath:     ownerPathForGo(recvType, defKind),
				QualifiedName: name,
				Kind:          string(defKind),
				SignatureHash: sf.SignatureHash,
				RelPath:       relPathForKey(file.Path),
				Visibility:    sf.Visibility,
			})
			sf.ID = extract.StableSymbolID(sf.StableKey)
			out.Symbols = append(out.Symbols, sf)
		}

		if refNameNode != nil && refKind != "" {
			rf := extract.ReferenceFact{
				Language:        "go",
				Kind:            extract.ReferenceKind(refKind),
				Name:            refNameNode.Utf8Text(source),
				File:            file.Path,
				Range:           nodeRange(*refNameNode),
				ValidationState: "syntactic",
				Confidence:      extract.ConfidenceTSOnly,
			}
			// Receiver capture for member-call references (pkg.Foo()): enables
			// CROSS_CALLS edges by matching the receiver to an external import.
			if refKind == "call" {
				if p := refNameNode.Parent(); p != nil && p.Kind() == "selector_expression" {
					if op := p.ChildByFieldName("operand"); op != nil {
						rf.ReceiverText = op.Utf8Text(source)
					}
				}
			}
			out.References = append(out.References, rf)
		}

		if haveImport {
			out.Imports = append(out.Imports, extract.ImportFact{
				Language: "go",
				Source:   importSrc,
				Alias:    importAlias,
				File:     file.Path,
				Range:    importRange,
			})
		}
	}

	// HTTP route detection (net/http + gin/echo/gorilla/chi call sites).
	out.Routes = detectGoRoutes(*tree.RootNode(), source, file.Path)
	// GORM entity detection (structs with gorm struct tags / gorm.Model embed).
	out.Resources = detectGoResources(*tree.RootNode(), source, file.Path)
	return out, nil
}

// ExtractFile is the Phase 68 D-03 per-file extractor shim: reads `path`
// from disk and delegates to Extract. The repoID argument is part of the
// stable Phase 68 contract for the live handler call site but is not
// consumed by the Go provider — owner/package context is derived from
// the file path itself. I/O failures surface as partial-status
// ExtractedFile (NOT error) per the partialFile convention.
func (p *Provider) ExtractFile(ctx context.Context, _repoID, path string) (*extract.ExtractedFile, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return partialFile(
			extract.SourceFile{Path: path, Language: "go"},
			extract.PartialReasonPermissionDenied, err), nil
	}
	return p.Extract(ctx, source, extract.SourceFile{Path: path, Language: "go"})
}

func partialFile(file extract.SourceFile, reason extract.PartialReason, err error) *extract.ExtractedFile {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	status := extract.ExtractionStatusPartial
	return &extract.ExtractedFile{
		File: extract.FileFact{
			Path:              file.Path,
			Language:          "go",
			ExtractionStatus:  status,
			ExtractionPartial: true,
			PartialReason:     reason,
			ExtractorName:     "goextract",
			ExtractorVersion:  "1",
			ErrorMessage:      msg,
		},
		Partial:       true,
		PartialReason: string(reason),
	}
}

func nodeRange(n tree_sitter.Node) extract.Range {
	s := n.StartPosition()
	e := n.EndPosition()
	return extract.Range{
		Start: extract.Position{Line: uint32(s.Row), Column: uint32(s.Column)},
		End:   extract.Position{Line: uint32(e.Row), Column: uint32(e.Column)},
	}
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '`') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func qualifiedName(name, recvType string, kind extract.SymbolKind) string {
	if kind == extract.KindMethod && recvType != "" {
		return strings.TrimPrefix(recvType, "*") + "." + name
	}
	return name
}

func visibility(name string) string {
	if name == "" {
		return "private"
	}
	c := name[0]
	if c >= 'A' && c <= 'Z' {
		return "exported"
	}
	return "private"
}

// ownerPathForGo returns the OwnerPath for stable-ID canonicalization per
// 59-RESEARCH.md lines 466-505. For methods, the receiver type (with
// pointer marker preserved) is the owner.
func ownerPathForGo(recvType string, kind extract.SymbolKind) string {
	if kind == extract.KindMethod {
		return recvType
	}
	return ""
}

// signatureHash returns a whitespace-condensed snippet of the symbol's
// definition node — enough to make rename/signature edits churn the ID
// while preserving body-only edits. Per CONTEXT.md the SignatureHash is
// canonicalization input; we use a compact normalized form rather than a
// raw hash so test-time inspection is straightforward.
//
// Generic type parameter names are alpha-renamed to positional placeholders
// (T0, T1, ...) so `F[T any](T) T` and `F[U any](U) U` produce the same
// SignatureHash. This delivers EXTRACT-03's
// generic_type_param_rename_preserves_id transition.
func signatureHash(body tree_sitter.Node, source []byte) string {
	text := body.Utf8Text(source)
	// strip the function/method body delimited by the first '{'
	if idx := strings.Index(text, "{"); idx >= 0 {
		text = text[:idx]
	}
	text = condenseWhitespace(text)
	return canonicalizeGenericParams(text)
}

// canonicalizeGenericParams alpha-renames generic type parameter identifiers
// in a Go signature snippet so renaming a type parameter does not churn the
// stable ID. Operates on already-condensed-whitespace text.
//
// Algorithm:
//  1. If the snippet contains `[...]` immediately after a name and before
//     `(`, that's a generic type-parameter list. Parse the bracketed group
//     to extract the parameter identifiers (split on `,`, take the first
//     ident of each segment).
//  2. Substitute each extracted identifier with a positional placeholder
//     `T0`, `T1`, ... in BOTH the bracket group and the rest of the snippet.
//
// The substitution uses word-boundary string replacement (we look for the
// identifier surrounded by non-identifier characters) — sufficient for Go
// signatures where generic params are short capitalized identifiers.
func canonicalizeGenericParams(sig string) string {
	open := strings.Index(sig, "[")
	if open < 0 {
		return sig
	}
	// Match opening bracket position with the closing bracket's `]`.
	depth := 0
	close := -1
	for i := open; i < len(sig); i++ {
		switch sig[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				close = i
			}
		}
		if close >= 0 {
			break
		}
	}
	if close < 0 {
		return sig
	}
	inner := sig[open+1 : close]
	// Each segment looks like `T any` or `U comparable` or `T, U any`.
	// Split on comma and take the first identifier of each segment.
	var params []string
	for _, seg := range strings.Split(inner, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		// first whitespace-delimited token; if multiple idents share a constraint
		// (e.g. `T, U any`) treat each prior comma-segment separately.
		parts := strings.Fields(seg)
		if len(parts) == 0 {
			continue
		}
		ident := parts[0]
		if isIdentifier(ident) {
			params = append(params, ident)
		}
	}
	if len(params) == 0 {
		return sig
	}
	// Substitute each param with T0, T1, ... — using a non-conflicting prefix
	// so we don't accidentally rewrite our own placeholders.
	const prefix = "\x01TP"
	out := sig
	for i, name := range params {
		placeholder := prefix + itoa(i)
		out = replaceWordBoundary(out, name, placeholder)
	}
	// final pass: rewrite the prefix to a clean form
	for i := range params {
		out = strings.ReplaceAll(out, prefix+itoa(i), "T"+itoa(i))
	}
	return out
}

// isIdentifier reports whether s is a Go-style identifier (letters, digits,
// underscores; first char letter or underscore).
func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return false
			}
			continue
		}
		if !(r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b strings.Builder
	for n > 0 {
		d := byte(n % 10)
		b.WriteByte('0' + d)
		n /= 10
	}
	// reverse
	r := []byte(b.String())
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// replaceWordBoundary substitutes occurrences of `name` in `s` only when
// surrounded by non-identifier characters (or at string boundaries).
func replaceWordBoundary(s, name, repl string) string {
	if !strings.Contains(s, name) {
		return s
	}
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+len(name) <= len(s) && s[i:i+len(name)] == name {
			leftOK := i == 0 || !isIdentChar(s[i-1])
			rightOK := i+len(name) == len(s) || !isIdentChar(s[i+len(name)])
			if leftOK && rightOK {
				b.WriteString(repl)
				i += len(name)
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isIdentChar(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func condenseWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.TrimSpace(s) {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

// packagePathFromFile derives a stable identifier for the Go package the
// file belongs to. In Phase 59 we lack module-resolution context (that is
// Phase 61 territory); we use the directory path as a stand-in. Stable
// across whitespace edits and rename within the same dir.
func packagePathFromFile(p string) string {
	d := filepath.Dir(p)
	return filepath.ToSlash(d)
}

// relPathForKey returns a fixed-shape relative path for use as
// FilePathFallback. Using just the basename keeps tests portable.
func relPathForKey(p string) string {
	return filepath.Base(p)
}
