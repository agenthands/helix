// Package pyextract is the per-language tree-sitter extraction provider
// for Python. Mirrors goextract / tsextract: constructor-injected
// GrammarRegistry, embedded queries.scm, no init() registration.
package pyextract

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
var pyQueries string

// Provider is the concrete Python extraction provider.
type Provider struct {
	grammar *tree_sitter.Language
	query   *tree_sitter.Query
}

// NewProvider compiles the embedded queries against the injected
// GrammarRegistry's "python" grammar. Panics on missing grammar or query
// compile failure.
func NewProvider(grammars *treesitter.GrammarRegistry) extract.Provider {
	if grammars == nil {
		panic("pyextract.NewProvider: nil GrammarRegistry")
	}
	g, ok := grammars.GetLanguage("python")
	if !ok {
		panic("pyextract.NewProvider: 'python' grammar missing from injected GrammarRegistry")
	}
	q, qerr := tree_sitter.NewQuery(g, pyQueries)
	if qerr != nil {
		panic("pyextract.NewProvider: query compile failed: " + qerr.Message)
	}
	return &Provider{grammar: g, query: q}
}

func (p *Provider) Language() string                          { return "python" }
func (p *Provider) Extensions() []string                      { return []string{".py"} }
func (p *Provider) TreeSitterLanguage() *tree_sitter.Language { return p.grammar }
func (p *Provider) Queries() string                           { return pyQueries }
func (p *Provider) SupportsLSPEnrichment() bool               { return true }

// Extract parses the source bytes and returns an ExtractedFile per the
// Phase 59 P02 fact contract. Decorator presence is captured as a
// reference, NOT folded into the symbol's stable-ID input — Python
// decorator add/remove preserves ID per CONTEXT.md D-03.
func (p *Provider) Extract(ctx context.Context, source []byte, file extract.SourceFile) (*extract.ExtractedFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(p.grammar); err != nil {
		return partialFile(file, extract.PartialReasonExtractorBug, fmt.Errorf("set py language: %w", err)), nil
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
			Language:         "python",
			ExtractionStatus: extract.ExtractionStatusReady,
			ExtractorName:    "pyextract",
			ExtractorVersion: "1",
		},
	}

	captureNames := p.query.CaptureNames()
	matches := cursor.Matches(p.query, tree.RootNode(), source)
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
			heritageT   string
			haveHerit   bool
			heritageR   extract.Range
			decorName   string
			decorRange  extract.Range
			haveDecor   bool
		)
		for _, c := range m.Captures {
			cn := captureNames[c.Index]
			node := c.Node
			switch cn {
			case "definition.function":
				defKind = extract.KindFunction
				defNameNode = &node
			case "definition.class":
				defKind = extract.KindClass
				defNameNode = &node
			case "definition.parameter":
				defKind = extract.KindParameter
				defNameNode = &node
			case "definition.variable":
				defKind = extract.KindVariable
				defNameNode = &node
			case "decorator.name":
				haveDecor = true
				decorName = node.Utf8Text(source)
				decorRange = nodeRange(node)
			case "heritage.extends":
				haveHerit = true
				heritageT = node.Utf8Text(source)
				heritageR = nodeRange(node)
			case "reference.call":
				refKind = "call"
				refNameNode = &node
			case "reference.field":
				refKind = "field"
				refNameNode = &node
			case "reference.type":
				refKind = "type"
				refNameNode = &node
			case "import.source":
				importSrc = node.Utf8Text(source)
				importRange = nodeRange(node)
				haveImport = true
			case "import.alias":
				importAlias = node.Utf8Text(source)
			case "import.symbol":
				importSym = node.Utf8Text(source)
			case "type.annotation", "type.return":
				out.Types = append(out.Types, extract.TypeFact{
					Language:   "python",
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
				visib := pyVisibility(name)
				// Determine method-vs-function by walking up to find
				// enclosing class_definition.
				kind := defKind
				ownerPath := ""
				if defKind == extract.KindFunction {
					if cls := enclosingClassName(*defNameNode, source); cls != "" {
						kind = extract.KindMethod
						ownerPath = cls
					}
				}
				sf := extract.SymbolFact{
					Language:         "python",
					Kind:             kind,
					Name:             name,
					QualifiedName:    qualifiedName(name, ownerPath),
					File:             file.Path,
					Range:            nodeRange(*defNameNode),
					SelectionRange:   nodeRange(*defNameNode),
					Visibility:       visib,
					Confidence:       extract.ConfidenceTSOnly,
					ExtractionSource: "tree_sitter",
					Signature:        condenseWhitespace(name),
					SignatureHash:    name,
				}
				if kind == extract.KindMethod {
					sf.Receiver = &extract.ReceiverFact{Type: ownerPath}
				}
				sf.StableKey = extract.BuildProviderKey(extract.SymbolMeta{
					Language:      "python",
					PackagePath:   packagePathFromFile(file.Path),
					OwnerPath:     ownerPath,
					QualifiedName: name,
					Kind:          string(kind),
					SignatureHash: name,
					RelPath:       relPathForKey(file.Path),
					Visibility:    visib,
				})
				sf.ID = extract.StableSymbolID(sf.StableKey)
				if extract.IsFingerprintableKind(kind) {
					if decl := defNameNode.Parent(); decl != nil {
						extract.FingerprintBody(&sf, decl, source)
					}
				}
				out.Symbols = append(out.Symbols, sf)
			}
		}

		if refNameNode != nil && refKind != "" {
			rf := extract.ReferenceFact{
				Language:        "python",
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
				if p := refNameNode.Parent(); p != nil && p.Kind() == "attribute" {
					if obj := p.ChildByFieldName("object"); obj != nil {
						rf.ReceiverText = obj.Utf8Text(source)
					}
				}
			}
			out.References = append(out.References, rf)
		}

		if haveImport {
			impr := extract.ImportFact{
				Language: "python",
				Source:   importSrc,
				Alias:    importAlias,
				File:     file.Path,
				Range:    importRange,
			}
			if importSym != "" {
				impr.Symbols = []string{importSym}
			}
			out.Imports = append(out.Imports, impr)
		}

		if haveHerit {
			out.Heritage = append(out.Heritage, extract.HeritageFact{
				Language: "python",
				Relation: "extends",
				Target:   heritageT,
				Range:    heritageR,
			})
		}

		if haveDecor {
			out.References = append(out.References, extract.ReferenceFact{
				Language:        "python",
				Kind:            extract.ReferenceKind("decorator"),
				Name:            decorName,
				File:            file.Path,
				Range:           decorRange,
				Confidence:      extract.ConfidenceTSOnly,
				ValidationState: "syntactic",
			})
		}
	}

	// HTTP route detection (Flask @app.route / FastAPI @app.get / ...).
	out.Routes = detectPyRoutes(*tree.RootNode(), source, file.Path)
	// SQLAlchemy ORM-entity detection (class with __tablename__).
	out.Resources = detectPyResources(*tree.RootNode(), source, file.Path)
	return out, nil
}

// ExtractFile is the Phase 68 D-03 per-file extractor shim: reads `path`
// from disk and delegates to Extract. The repoID argument is part of the
// stable Phase 68 contract for the live handler call site but is not
// consumed by the Python provider. I/O failures surface as partial-status
// ExtractedFile (NOT error) per the partialFile convention.
func (p *Provider) ExtractFile(ctx context.Context, _repoID, path string) (*extract.ExtractedFile, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return partialFile(
			extract.SourceFile{Path: path, Language: "python"},
			extract.PartialReasonPermissionDenied, err), nil
	}
	return p.Extract(ctx, source, extract.SourceFile{Path: path, Language: "python"})
}

func partialFile(file extract.SourceFile, reason extract.PartialReason, err error) *extract.ExtractedFile {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &extract.ExtractedFile{
		File: extract.FileFact{
			Path:              file.Path,
			Language:          "python",
			ExtractionStatus:  extract.ExtractionStatusPartial,
			ExtractionPartial: true,
			PartialReason:     reason,
			ExtractorName:     "pyextract",
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

func qualifiedName(name, owner string) string {
	if owner != "" {
		return owner + "." + name
	}
	return name
}

// pyVisibility — Python convention: leading underscore is private.
func pyVisibility(name string) string {
	if name == "" {
		return "private"
	}
	if strings.HasPrefix(name, "_") {
		return "private"
	}
	return "exported"
}

func packagePathFromFile(p string) string {
	return filepath.ToSlash(filepath.Dir(p))
}

func relPathForKey(p string) string {
	return filepath.Base(p)
}

// enclosingClassName walks up from the given identifier node to find the
// closest enclosing class_definition. Returns the class name if found,
// "" otherwise. Used to promote function definitions inside classes to
// SymbolKind.method per RESEARCH.md line 484.
func enclosingClassName(name tree_sitter.Node, source []byte) string {
	for n := name.Parent(); n != nil; {
		if n.Kind() == "class_definition" {
			if nameNode := n.ChildByFieldName("name"); nameNode != nil {
				return nameNode.Utf8Text(source)
			}
			return ""
		}
		// don't cross out of a deeply-nested function; we just want the
		// nearest class. Continue walking up unconditionally.
		next := n.Parent()
		if next == nil {
			return ""
		}
		n = next
	}
	return ""
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
