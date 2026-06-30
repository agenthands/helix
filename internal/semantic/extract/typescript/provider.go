// Package tsextract is the per-language tree-sitter extraction provider
// for TypeScript and JavaScript. A single provider serves both languages
// per CONTEXT.md Claude's Discretion — the syntax overlap is large and
// TS-only constructs are captured via query alternation rather than two
// providers.
//
// Provider claims .ts, .tsx, .js, .jsx, .mjs, .cjs. Internally the provider
// holds two compiled queries (typescript and tsx grammars) and dispatches
// based on file extension. Pure JS files run through the typescript
// grammar — TypeScript is a strict syntactic superset of JavaScript so
// this is safe (TS-only constructs simply don't appear in .js source).
package tsextract

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
var tsQueries string

// Provider is the concrete TS+JS extraction provider.
type Provider struct {
	tsGrammar  *tree_sitter.Language
	tsxGrammar *tree_sitter.Language
	tsQuery    *tree_sitter.Query
	tsxQuery   *tree_sitter.Query
}

// NewProvider compiles the embedded queries against both the typescript
// and tsx grammars. Panics on missing grammar or query-compile failure.
func NewProvider(grammars *treesitter.GrammarRegistry) extract.Provider {
	if grammars == nil {
		panic("tsextract.NewProvider: nil GrammarRegistry")
	}
	ts, ok := grammars.GetLanguage("typescript")
	if !ok {
		panic("tsextract.NewProvider: 'typescript' grammar missing")
	}
	tsx, ok := grammars.GetLanguage("tsx")
	if !ok {
		panic("tsextract.NewProvider: 'tsx' grammar missing")
	}
	tsq, qerr := tree_sitter.NewQuery(ts, tsQueries)
	if qerr != nil {
		panic("tsextract.NewProvider: ts query compile failed: " + qerr.Message)
	}
	tsxq, qerr := tree_sitter.NewQuery(tsx, tsQueries)
	if qerr != nil {
		panic("tsextract.NewProvider: tsx query compile failed: " + qerr.Message)
	}
	return &Provider{tsGrammar: ts, tsxGrammar: tsx, tsQuery: tsq, tsxQuery: tsxq}
}

func (p *Provider) Language() string { return "typescript" }
func (p *Provider) Extensions() []string {
	return []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
}
func (p *Provider) TreeSitterLanguage() *tree_sitter.Language { return p.tsGrammar }
func (p *Provider) Queries() string                           { return tsQueries }
func (p *Provider) SupportsLSPEnrichment() bool               { return true }

// Extract parses source bytes using the appropriate grammar (tsx for .tsx
// / .jsx, typescript otherwise) and returns an ExtractedFile.
func (p *Provider) Extract(ctx context.Context, source []byte, file extract.SourceFile) (*extract.ExtractedFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	grammar, query := p.dispatch(file.Path)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(grammar); err != nil {
		return partialFile(file, extract.PartialReasonExtractorBug, fmt.Errorf("set ts language: %w", err)), nil
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
			Language:         "typescript",
			ExtractionStatus: extract.ExtractionStatusReady,
			ExtractorName:    "tsextract",
			ExtractorVersion: "1",
		},
	}

	captureNames := query.CaptureNames()
	matches := cursor.Matches(query, tree.RootNode(), source)
	seenSymbol := make(map[string]bool)

	for m := matches.Next(); m != nil; m = matches.Next() {
		if err := ctx.Err(); err != nil {
			return partialFile(file, extract.PartialReasonTimeout, err), nil
		}
		var (
			defKind     extract.SymbolKind
			defNameNode *tree_sitter.Node
			refKind     string
			refNameNode *tree_sitter.Node
			importSrc   string
			importAlias string
			importSym   string
			importRange extract.Range
			haveImport  bool
			haveDecor   bool
			decorName   string
			heritageRel string
			heritageTgt string
			haveHerit   bool
			heritageRng extract.Range
		)

		for _, c := range m.Captures {
			cn := captureNames[c.Index]
			node := c.Node
			switch cn {
			case "definition.function":
				defKind = extract.KindFunction
				defNameNode = &node
			case "definition.method":
				defKind = extract.KindMethod
				defNameNode = &node
			case "definition.class":
				defKind = extract.KindClass
				defNameNode = &node
			case "definition.interface":
				defKind = extract.KindInterface
				defNameNode = &node
			case "definition.type":
				defKind = extract.KindType
				defNameNode = &node
			case "definition.enum":
				defKind = extract.KindEnum
				defNameNode = &node
			case "definition.field":
				defKind = extract.KindField
				defNameNode = &node
			case "definition.parameter":
				defKind = extract.KindParameter
				defNameNode = &node
			case "decorator.name":
				haveDecor = true
				decorName = node.Utf8Text(source)
			case "heritage.extends":
				heritageRel = "extends"
				heritageTgt = node.Utf8Text(source)
				heritageRng = nodeRange(node)
				haveHerit = true
			case "heritage.implements":
				heritageRel = "implements"
				heritageTgt = node.Utf8Text(source)
				heritageRng = nodeRange(node)
				haveHerit = true
			case "reference.call":
				refKind = "call"
				refNameNode = &node
			case "reference.type":
				refKind = "type"
				refNameNode = &node
			case "reference.field":
				refKind = "field"
				refNameNode = &node
			case "import.source":
				importSrc = trimQuotes(node.Utf8Text(source))
				importRange = nodeRange(node)
				haveImport = true
			case "import.alias":
				importAlias = node.Utf8Text(source)
			case "import.symbol":
				importSym = node.Utf8Text(source)
			case "type.annotation", "type.return", "type.parameter":
				out.Types = append(out.Types, extract.TypeFact{
					Language:   "typescript",
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
			if !seenSymbol[dedupKey] {
				seenSymbol[dedupKey] = true
				visib := tsVisibility(name)
				sf := extract.SymbolFact{
					Language:         "typescript",
					Kind:             defKind,
					Name:             name,
					QualifiedName:    name,
					File:             file.Path,
					Range:            nodeRange(*defNameNode),
					SelectionRange:   nodeRange(*defNameNode),
					Visibility:       visib,
					Confidence:       extract.ConfidenceTSOnly,
					ExtractionSource: "tree_sitter",
					Signature:        condenseWhitespace(name),
					SignatureHash:    name,
				}
				sf.StableKey = extract.BuildProviderKey(extract.SymbolMeta{
					Language:      "typescript",
					PackagePath:   packagePathFromFile(file.Path),
					OwnerPath:     "",
					QualifiedName: name,
					Kind:          string(defKind),
					SignatureHash: name,
					RelPath:       relPathForKey(file.Path),
					Visibility:    visib,
				})
				sf.ID = extract.StableSymbolID(sf.StableKey)
				if extract.IsFingerprintableKind(defKind) {
					if decl := defNameNode.Parent(); decl != nil {
						extract.FingerprintBody(&sf, decl, source)
					}
				}
				out.Symbols = append(out.Symbols, sf)
			}
		}

		if refNameNode != nil && refKind != "" {
			rf := extract.ReferenceFact{
				Language:        "typescript",
				Kind:            extract.ReferenceKind(refKind),
				Name:            refNameNode.Utf8Text(source),
				File:            file.Path,
				Range:           nodeRange(*refNameNode),
				ValidationState: "syntactic",
				Confidence:      extract.ConfidenceTSOnly,
			}
			// Receiver capture for member-call references (lib.foo()): enables
			// CROSS_CALLS edges by matching the receiver to an external import.
			if refKind == "call" {
				if p := refNameNode.Parent(); p != nil && p.Kind() == "member_expression" {
					if obj := p.ChildByFieldName("object"); obj != nil {
						rf.ReceiverText = obj.Utf8Text(source)
					}
				}
			}
			out.References = append(out.References, rf)
		}

		if haveImport {
			impr := extract.ImportFact{
				Language: "typescript",
				Source:   importSrc,
				Alias:    importAlias,
				File:     file.Path,
				Range:    importRange,
			}
			if importSym != "" {
				impr.Symbols = []string{importSym}
			}
			out.Imports = append(out.Imports, impr)
		} else if importSym != "" {
			// import_specifier match without an import_statement context
			out.Imports = append(out.Imports, extract.ImportFact{
				Language: "typescript",
				Symbols:  []string{importSym},
				File:     file.Path,
			})
		}

		if haveDecor {
			out.References = append(out.References, extract.ReferenceFact{
				Language:        "typescript",
				Kind:            extract.ReferenceKind("decorator"),
				Name:            decorName,
				File:            file.Path,
				Confidence:      extract.ConfidenceTSOnly,
				ValidationState: "syntactic",
			})
		}

		if haveHerit {
			out.Heritage = append(out.Heritage, extract.HeritageFact{
				Language: "typescript",
				Relation: heritageRel,
				Target:   heritageTgt,
				Range:    heritageRng,
			})
		}
	}

	// HTTP route detection (Express app.get / NestJS @Get / ...).
	out.Routes = detectTSRoutes(*tree.RootNode(), source, file.Path)
	// ORM-entity detection (TypeORM @Entity on a class_declaration).
	out.Resources = detectTSResources(*tree.RootNode(), source, file.Path)
	return out, nil
}

// ExtractFile is the Phase 68 D-03 per-file extractor shim: reads `path`
// from disk and delegates to Extract. The repoID argument is part of the
// stable Phase 68 contract for the live handler call site but is not
// consumed by the TS provider — module context is derived from the file
// path. I/O failures surface as partial-status ExtractedFile (NOT error)
// per the partialFile convention.
func (p *Provider) ExtractFile(ctx context.Context, _repoID, path string) (*extract.ExtractedFile, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return partialFile(
			extract.SourceFile{Path: path, Language: "typescript"},
			extract.PartialReasonPermissionDenied, err), nil
	}
	return p.Extract(ctx, source, extract.SourceFile{Path: path, Language: "typescript"})
}

func (p *Provider) dispatch(path string) (*tree_sitter.Language, *tree_sitter.Query) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".tsx", ".jsx":
		return p.tsxGrammar, p.tsxQuery
	default:
		return p.tsGrammar, p.tsQuery
	}
}

func partialFile(file extract.SourceFile, reason extract.PartialReason, err error) *extract.ExtractedFile {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &extract.ExtractedFile{
		File: extract.FileFact{
			Path:              file.Path,
			Language:          "typescript",
			ExtractionStatus:  extract.ExtractionStatusPartial,
			ExtractionPartial: true,
			PartialReason:     reason,
			ExtractorName:     "tsextract",
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
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'' || s[0] == '`') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// tsVisibility — TS doesn't have Go-style capitalization-as-visibility;
// in TS module-level declarations are private to the module unless
// `export`-ed. For Phase 59 we conservatively classify by leading char
// (capital = exported in our stable-ID model). Phase 61 may refine.
func tsVisibility(name string) string {
	if name == "" {
		return "private"
	}
	c := name[0]
	if c >= 'A' && c <= 'Z' {
		return "exported"
	}
	return "private"
}

func condenseWhitespace(s string) string {
	var b strings.Builder
	prev := false
	for _, r := range strings.TrimSpace(s) {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prev {
				b.WriteByte(' ')
				prev = true
			}
			continue
		}
		b.WriteRune(r)
		prev = false
	}
	return b.String()
}

func packagePathFromFile(p string) string {
	return filepath.ToSlash(filepath.Dir(p))
}

func relPathForKey(p string) string {
	return filepath.Base(p)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b strings.Builder
	for n > 0 {
		b.WriteByte('0' + byte(n%10))
		n /= 10
	}
	r := []byte(b.String())
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
