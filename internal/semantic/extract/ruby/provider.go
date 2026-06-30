// Package rubyextract is the per-language tree-sitter extraction provider
// for Ruby source files.
package rubyextract

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/treesitter"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

//go:embed queries.scm
var rubyQueries string

type Provider struct {
	grammar *tree_sitter.Language
	query   *tree_sitter.Query
}

func NewProvider(grammars *treesitter.GrammarRegistry) extract.Provider {
	if grammars == nil {
		panic("rubyextract.NewProvider: nil GrammarRegistry")
	}
	g, ok := grammars.GetLanguage("ruby")
	if !ok {
		panic("rubyextract.NewProvider: 'ruby' grammar missing from injected GrammarRegistry")
	}
	q, qerr := tree_sitter.NewQuery(g, rubyQueries)
	if qerr != nil {
		panic("rubyextract.NewProvider: query compile failed: " + qerr.Message)
	}
	return &Provider{grammar: g, query: q}
}

func (p *Provider) Language() string                          { return "ruby" }
func (p *Provider) Extensions() []string                      { return []string{".rb"} }
func (p *Provider) TreeSitterLanguage() *tree_sitter.Language { return p.grammar }
func (p *Provider) Queries() string                           { return rubyQueries }
func (p *Provider) SupportsLSPEnrichment() bool               { return true }

func (p *Provider) Extract(ctx context.Context, source []byte, file extract.SourceFile) (*extract.ExtractedFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(p.grammar); err != nil {
		return partialFile(file, extract.PartialReasonExtractorBug, fmt.Errorf("set ruby language: %w", err)), nil
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
			Language:         "ruby",
			ExtractionStatus: extract.ExtractionStatusReady,
			ExtractorName:    "rubyextract",
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
		)
		for _, c := range m.Captures {
			cn := captureNames[c.Index]
			node := c.Node
			switch {
			case cn == "definition.method":
				defKind = extract.KindMethod
				defNameNode = &node
			case cn == "definition.class":
				defKind = extract.KindClass
				defNameNode = &node
			case cn == "definition.module":
				defKind = extract.KindClass // map module → class
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
			case cn == "reference.type":
				refKind = "type"
				refNameNode = &node
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
				Language:         "ruby",
				Kind:             defKind,
				Name:             name,
				QualifiedName:    name,
				File:             file.Path,
				Range:            nodeRange(*body),
				SelectionRange:   nodeRange(*defNameNode),
				Visibility:       "public",
				Confidence:       extract.ConfidenceTSOnly,
				ExtractionSource: "tree_sitter",
			}
			sf.SignatureHash = signatureHash(*body, source)
			sf.Signature = condenseWhitespace(body.Utf8Text(source))
			if len(sf.Signature) > 200 {
				sf.Signature = sf.Signature[:200]
			}
			sf.StableKey = extract.BuildProviderKey(extract.SymbolMeta{
				Language:      "ruby",
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
				Language:        "ruby",
				Kind:            extract.ReferenceKind(refKind),
				Name:            refNameNode.Utf8Text(source),
				File:            file.Path,
				Range:           nodeRange(*refNameNode),
				ValidationState: "syntactic",
				Confidence:      extract.ConfidenceTSOnly,
			})
		}
	}
	out.Routes = detectRubyRoutes(*tree.RootNode(), source, file.Path)
	out.Resources = detectRubyResources(*tree.RootNode(), source, file.Path)
	return out, nil
}

func (p *Provider) ExtractFile(ctx context.Context, _repoID, path string) (*extract.ExtractedFile, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return partialFile(extract.SourceFile{Path: path, Language: "ruby"}, extract.PartialReasonPermissionDenied, err), nil
	}
	return p.Extract(ctx, source, extract.SourceFile{Path: path, Language: "ruby"})
}

func partialFile(file extract.SourceFile, reason extract.PartialReason, err error) *extract.ExtractedFile {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &extract.ExtractedFile{
		File: extract.FileFact{
			Path:              file.Path,
			Language:          "ruby",
			ExtractionStatus:  extract.ExtractionStatusPartial,
			ExtractionPartial: true,
			PartialReason:     reason,
			ExtractorName:     "rubyextract",
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

func signatureHash(body tree_sitter.Node, source []byte) string {
	text := body.Utf8Text(source)
	if idx := strings.Index(text, "\n"); idx >= 0 {
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
