// Package testutil — golden-file helpers shared by all per-language
// provider tests (internal/semantic/extract/<lang>/provider_test.go).
package testutil

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// Update is the dev-only `-update` flag that regenerates expected.json from
// the provider's current emit. CI MUST run without it; the goldens act as
// the regression net.
var Update = flag.Bool("update", false, "regenerate testdata/<scenario>/expected.json files from current provider emit")

// NormalizedSymbol is the on-disk shape of a symbol record. SymbolFact
// fields that are not load-bearing for golden comparison (raw monotone IDs,
// containerID pointers that depend on emit order) are omitted; the
// stable-key fields are included so renames/whitespace edits produce a
// detectable diff.
type NormalizedSymbol struct {
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	QualifiedName string         `json:"qualified_name,omitempty"`
	File          string         `json:"file"`
	Range         NormalizedRange `json:"range"`
	Signature     string         `json:"signature,omitempty"`
	SignatureHash string         `json:"signature_hash,omitempty"`
	Receiver      *NormalizedRecv `json:"receiver,omitempty"`
	Visibility    string         `json:"visibility,omitempty"`
	Confidence    float32        `json:"confidence"`
	Source        string         `json:"extraction_source"`
}

// NormalizedReference mirrors ReferenceFact for golden output.
type NormalizedReference struct {
	Kind         string         `json:"kind,omitempty"`
	Name         string         `json:"name"`
	File         string         `json:"file"`
	Range        NormalizedRange `json:"range"`
	ReceiverText string         `json:"receiver_text,omitempty"`
	Confidence   float32        `json:"confidence"`
}

// NormalizedImport mirrors ImportFact for golden output.
type NormalizedImport struct {
	Source  string         `json:"source"`
	Alias   string         `json:"alias,omitempty"`
	Symbols []string       `json:"symbols,omitempty"`
	File    string         `json:"file"`
	Range   NormalizedRange `json:"range"`
}

// NormalizedType mirrors TypeFact for golden output.
type NormalizedType struct {
	Subject    string         `json:"subject"`
	Annotation string         `json:"annotation"`
	Range      NormalizedRange `json:"range"`
}

// NormalizedHeritage mirrors HeritageFact for golden output.
type NormalizedHeritage struct {
	Subject  string         `json:"subject"`
	Relation string         `json:"relation"`
	Target   string         `json:"target"`
	Range    NormalizedRange `json:"range"`
}

// NormalizedRange is the on-disk shape of an extract.Range.
type NormalizedRange struct {
	StartLine uint32 `json:"start_line"`
	StartCol  uint32 `json:"start_col"`
	EndLine   uint32 `json:"end_line"`
	EndCol    uint32 `json:"end_col"`
}

// NormalizedRecv is the on-disk shape of a *ReceiverFact.
type NormalizedRecv struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// NormalizedFile is the on-disk shape of a FileFact (status-relevant
// subset).
type NormalizedFile struct {
	Path             string `json:"path"`
	Language         string `json:"language"`
	ExtractionStatus string `json:"extraction_status"`
	Partial          bool   `json:"partial,omitempty"`
	PartialReason    string `json:"partial_reason,omitempty"`
}

// NormalizedExtractedFile is the on-disk shape of an ExtractedFile,
// optimized for human-readable diffs.
type NormalizedExtractedFile struct {
	File       NormalizedFile         `json:"file"`
	Symbols    []NormalizedSymbol     `json:"symbols,omitempty"`
	References []NormalizedReference  `json:"references,omitempty"`
	Imports    []NormalizedImport     `json:"imports,omitempty"`
	Types      []NormalizedType       `json:"types,omitempty"`
	Heritage   []NormalizedHeritage   `json:"heritage,omitempty"`
	Partial    bool                   `json:"partial,omitempty"`
	Reason     string                 `json:"partial_reason,omitempty"`
}

// NormalizeForGolden returns a deterministic JSON representation of an
// ExtractedFile. Sorts records by (file, range, kind, name); strips
// absolute paths to relative; uses two-space indent.
func NormalizeForGolden(ef *extract.ExtractedFile) ([]byte, error) {
	if ef == nil {
		return nil, fmt.Errorf("NormalizeForGolden: nil ExtractedFile")
	}
	out := NormalizedExtractedFile{
		File: NormalizedFile{
			Path:             relPath(ef.File.Path),
			Language:         ef.File.Language,
			ExtractionStatus: string(ef.File.ExtractionStatus),
			Partial:          ef.File.ExtractionPartial,
			PartialReason:    string(ef.File.PartialReason),
		},
		Partial: ef.Partial,
		Reason:  ef.PartialReason,
	}

	for _, s := range ef.Symbols {
		ns := NormalizedSymbol{
			Kind:          string(s.Kind),
			Name:          s.Name,
			QualifiedName: s.QualifiedName,
			File:          relPath(s.File),
			Range:         normRange(s.Range),
			Signature:     s.Signature,
			SignatureHash: s.SignatureHash,
			Visibility:    s.Visibility,
			Confidence:    s.Confidence,
			Source:        s.ExtractionSource,
		}
		if s.Receiver != nil {
			ns.Receiver = &NormalizedRecv{Type: s.Receiver.Type, Name: s.Receiver.Name}
		}
		out.Symbols = append(out.Symbols, ns)
	}
	sort.SliceStable(out.Symbols, func(i, j int) bool {
		return cmpSymbol(out.Symbols[i], out.Symbols[j])
	})

	for _, r := range ef.References {
		out.References = append(out.References, NormalizedReference{
			Kind:         string(r.Kind),
			Name:         r.Name,
			File:         relPath(r.File),
			Range:        normRange(r.Range),
			ReceiverText: r.ReceiverText,
			Confidence:   r.Confidence,
		})
	}
	sort.SliceStable(out.References, func(i, j int) bool {
		return cmpReference(out.References[i], out.References[j])
	})

	for _, im := range ef.Imports {
		ni := NormalizedImport{
			Source:  im.Source,
			Alias:   im.Alias,
			Symbols: append([]string(nil), im.Symbols...),
			File:    relPath(im.File),
			Range:   normRange(im.Range),
		}
		sort.Strings(ni.Symbols)
		out.Imports = append(out.Imports, ni)
	}
	sort.SliceStable(out.Imports, func(i, j int) bool {
		return cmpImport(out.Imports[i], out.Imports[j])
	})

	for _, t := range ef.Types {
		out.Types = append(out.Types, NormalizedType{
			Subject:    fmt.Sprintf("%d", t.SubjectID),
			Annotation: t.Annotation,
			Range:      normRange(t.Range),
		})
	}
	sort.SliceStable(out.Types, func(i, j int) bool {
		return cmpType(out.Types[i], out.Types[j])
	})

	for _, h := range ef.Heritage {
		out.Heritage = append(out.Heritage, NormalizedHeritage{
			Subject:  fmt.Sprintf("%d", h.SubjectID),
			Relation: h.Relation,
			Target:   h.Target,
			Range:    normRange(h.Range),
		})
	}
	sort.SliceStable(out.Heritage, func(i, j int) bool {
		return cmpHeritage(out.Heritage[i], out.Heritage[j])
	})

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GoldenCompare reads testdata/<scenario>/expected.json and compares it
// against the normalized form of the supplied ExtractedFile. With -update,
// rewrites expected.json instead.
func GoldenCompare(t *testing.T, testdataRoot, scenario string, actual *extract.ExtractedFile) {
	t.Helper()
	path := filepath.Join(testdataRoot, scenario, "expected.json")
	got, err := NormalizeForGolden(actual)
	if err != nil {
		t.Fatalf("NormalizeForGolden(%s): %v", scenario, err)
	}
	if *Update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (re-run with -update to generate)", path, err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Errorf("golden mismatch for %s\n--- want ---\n%s\n--- got ---\n%s",
			scenario, want, got)
	}
}

// GoldenCompareAfter reads testdata/<scenario>/expected_after.json (used
// when an `after.<ext>` fixture exists alongside `before.<ext>`).
func GoldenCompareAfter(t *testing.T, testdataRoot, scenario string, actual *extract.ExtractedFile) {
	t.Helper()
	path := filepath.Join(testdataRoot, scenario, "expected_after.json")
	got, err := NormalizeForGolden(actual)
	if err != nil {
		t.Fatalf("NormalizeForGolden(%s): %v", scenario, err)
	}
	if *Update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (re-run with -update to generate)", path, err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Errorf("golden mismatch (after) for %s\n--- want ---\n%s\n--- got ---\n%s",
			scenario, want, got)
	}
}

// ListScenarios returns the directory names under testdataRoot that contain
// a before.* file. Returns sorted to keep test order stable.
func ListScenarios(t *testing.T, testdataRoot string) []string {
	t.Helper()
	entries, err := os.ReadDir(testdataRoot)
	if err != nil {
		t.Fatalf("read %s: %v", testdataRoot, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// skip hidden / placeholder dirs
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		// must contain a before.* file
		matches, _ := filepath.Glob(filepath.Join(testdataRoot, e.Name(), "before.*"))
		if len(matches) > 0 {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// FindBefore returns the absolute path to the before.<ext> file for a
// scenario, plus the file extension (".go", ".ts", etc.).
func FindBefore(t *testing.T, testdataRoot, scenario string) (string, string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(testdataRoot, scenario, "before.*"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no before.* file in %s/%s", testdataRoot, scenario)
	}
	// stable order: pick first
	sort.Strings(matches)
	p := matches[0]
	return p, filepath.Ext(p)
}

// FindAfter returns the absolute path to the after.<ext> file for a
// scenario, or "" if none exists.
func FindAfter(testdataRoot, scenario string) string {
	matches, err := filepath.Glob(filepath.Join(testdataRoot, scenario, "after.*"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[0]
}

// relPath strips any leading testdata path so goldens are portable across
// machines. Keeps just "<scenario>/<filename>" or "<filename>".
func relPath(p string) string {
	if p == "" {
		return ""
	}
	// keep the last 1-2 segments
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) <= 2 {
		return strings.Join(parts, "/")
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func normRange(r extract.Range) NormalizedRange {
	return NormalizedRange{
		StartLine: r.Start.Line,
		StartCol:  r.Start.Column,
		EndLine:   r.End.Line,
		EndCol:    r.End.Column,
	}
}

func cmpSymbol(a, b NormalizedSymbol) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Range.StartLine != b.Range.StartLine {
		return a.Range.StartLine < b.Range.StartLine
	}
	if a.Range.StartCol != b.Range.StartCol {
		return a.Range.StartCol < b.Range.StartCol
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	return a.Name < b.Name
}

func cmpReference(a, b NormalizedReference) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Range.StartLine != b.Range.StartLine {
		return a.Range.StartLine < b.Range.StartLine
	}
	if a.Range.StartCol != b.Range.StartCol {
		return a.Range.StartCol < b.Range.StartCol
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	return a.Name < b.Name
}

func cmpImport(a, b NormalizedImport) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Range.StartLine != b.Range.StartLine {
		return a.Range.StartLine < b.Range.StartLine
	}
	return a.Source < b.Source
}

func cmpType(a, b NormalizedType) bool {
	if a.Subject != b.Subject {
		return a.Subject < b.Subject
	}
	if a.Range.StartLine != b.Range.StartLine {
		return a.Range.StartLine < b.Range.StartLine
	}
	return a.Annotation < b.Annotation
}

func cmpHeritage(a, b NormalizedHeritage) bool {
	if a.Subject != b.Subject {
		return a.Subject < b.Subject
	}
	if a.Relation != b.Relation {
		return a.Relation < b.Relation
	}
	return a.Target < b.Target
}
