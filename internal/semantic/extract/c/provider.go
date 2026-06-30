// Package cextract is the per-language tree-sitter extraction provider
// for C source files. Constructed via NewProvider with the
// daemon-injected *treesitter.GrammarRegistry and satisfies
// the extract.Provider interface.
package cextract

import (
	"context"
	"fmt"
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/treesitter"
)

//go:embed queries.scm
var cQueries string

type Provider struct {
	grammar *tree_sitter.Language
	query   *tree_sitter.Query
}

func NewProvider(grammars *treesitter.GrammarRegistry) extract.Provider {
	if grammars == nil {
		panic("cextract.NewProvider: nil GrammarRegistry")
	}
	g, ok := grammars.GetLanguage("c")
	if !ok {
		panic("cextract.NewProvider: 'c' grammar missing from injected GrammarRegistry")
	}
	q, qerr := tree_sitter.NewQuery(g, cQueries)
	if qerr != nil {
		panic("cextract.NewProvider: query compile failed: " + qerr.Message)
	}
	return &Provider{grammar: g, query: q}
}

func (p *Provider) Language() string                     { return "c" }
func (p *Provider) Extensions() []string                  { return []string{".c", ".h"} }
func (p *Provider) TreeSitterLanguage() *tree_sitter.Language { return p.grammar }
func (p *Provider) Queries() string                       { return cQueries }
func (p *Provider) SupportsLSPEnrichment() bool           { return true }

func (p *Provider) Extract(ctx context.Context, source []byte, file extract.SourceFile) (*extract.ExtractedFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(p.grammar); err != nil {
		return partialFile(file, extract.PartialReasonExtractorBug, fmt.Errorf("set c language: %w", err)), nil
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
			Language:         "c",
			ExtractionStatus: extract.ExtractionStatusReady,
			ExtractorName:    "cextract",
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
			defBodyNode *tree_sitter.Node
			refKind     string
			refNameNode *tree_sitter.Node
			importSrc   string
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
			case cn == "definition.struct":
				defKind = extract.KindStruct
				defNameNode = &node
			case cn == "definition.enum":
				defKind = extract.KindEnum
				defNameNode = &node
			case cn == "definition.type":
				defKind = extract.KindType
				defNameNode = &node
			case cn == "definition.field":
				defKind = extract.KindField
				defNameNode = &node
			case cn == "definition.variable":
				defKind = extract.KindVariable
				defNameNode = &node
			case cn == "definition.parameter":
				defKind = extract.KindParameter
				defNameNode = &node
			case cn == "reference.call":
				refKind = "call"
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
			case cn == "type.return", cn == "type.annotation", cn == "type.parameter":
				out.Types = append(out.Types, extract.TypeFact{
					Language:   "c",
					Annotation: condenseWhitespace(node.Utf8Text(source)),
					Range:      nodeRange(node),
				})
			case strings.HasPrefix(cn, "def."):
				defBodyNode = &node
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
			seenSymbol[dedupKey] = true
			body := defNameNode
			if defBodyNode != nil {
				body = defBodyNode
			}
			sf := extract.SymbolFact{
				Language:         "c",
				Kind:             defKind,
				Name:             name,
				QualifiedName:    name,
				File:             file.Path,
				Range:            nodeRange(*body),
				SelectionRange:   nodeRange(*defNameNode),
				Visibility:       "global",
				Confidence:       extract.ConfidenceTSOnly,
				ExtractionSource: "tree_sitter",
			}
			sf.SignatureHash = signatureHash(*body, source)
			sf.Signature = condenseWhitespace(body.Utf8Text(source))
			if len(sf.Signature) > 200 {
				sf.Signature = sf.Signature[:200]
			}
			sf.StableKey = extract.BuildProviderKey(extract.SymbolMeta{
				Language:      "c",
				PackagePath:   packagePathFromFile(file.Path),
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
			out.References = append(out.References, extract.ReferenceFact{
				Language:        "c",
				Kind:            extract.ReferenceKind(refKind),
				Name:            refNameNode.Utf8Text(source),
				File:            file.Path,
				Range:           nodeRange(*refNameNode),
				ValidationState: "syntactic",
				Confidence:      extract.ConfidenceTSOnly,
			})
		}
		if haveImport {
			out.Imports = append(out.Imports, extract.ImportFact{
				Language: "c",
				Source:   importSrc,
				File:     file.Path,
				Range:    importRange,
			})
		}
	}
	return out, nil
}

func (p *Provider) ExtractFile(ctx context.Context, _repoID, path string) (*extract.ExtractedFile, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return partialFile(extract.SourceFile{Path: path, Language: "c"}, extract.PartialReasonPermissionDenied, err), nil
	}
	return p.Extract(ctx, source, extract.SourceFile{Path: path, Language: "c"})
}

func partialFile(file extract.SourceFile, reason extract.PartialReason, err error) *extract.ExtractedFile {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &extract.ExtractedFile{
		File: extract.FileFact{
			Path:              file.Path,
			Language:          "c",
			ExtractionStatus:  extract.ExtractionStatusPartial,
			ExtractionPartial: true,
			PartialReason:     reason,
			ExtractorName:     "cextract",
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
	if (len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"') ||
		(len(s) >= 2 && s[0] == '<' && s[len(s)-1] == '>') {
		return s[1 : len(s)-1]
	}
	return s
}

func signatureHash(body tree_sitter.Node, source []byte) string {
	text := body.Utf8Text(source)
	if idx := strings.Index(text, "{"); idx >= 0 {
		text = text[:idx]
	}
	return condenseWhitespace(text)
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
