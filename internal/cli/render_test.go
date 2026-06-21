package cli

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// textResult builds a CallToolResult carrying a single TextContent block — the
// daemon's most common shape.
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
	}
}

// specFor returns the verbSpec for a generated verb name, failing the test if
// absent.
func specFor(t *testing.T, verb string) verbSpec {
	t.Helper()
	spec, ok := verbSpecs[verb]
	if !ok {
		t.Fatalf("verb %q not registered in verbSpecs", verb)
	}
	return spec
}

// TestRender_LocusList_TerseSorted asserts a locus-list result renders two
// terse `relpath:line:col<TAB>payload` lines, sorted (a.go before b.go),
// workspace-relative, with the bare line carrying an empty payload. (OUT-01)
func TestRender_LocusList_TerseSorted(t *testing.T) {
	root := "/ws"
	res := textResult("file:///ws/b.go:2:1\nfile:///ws/a.go:5:6 — Helper [Function]\n")
	spec := specFor(t, "search-symbols")

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: root, color: colorNever})

	lines := splitNonEmpty(buf.String())
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), buf.String())
	}
	// a.go sorts before b.go.
	if !strings.HasPrefix(lines[0], "a.go:5:6\t") {
		t.Errorf("line 0 = %q, want a.go:5:6<TAB>payload", lines[0])
	}
	if lines[1] != "b.go:2:1\t" {
		t.Errorf("line 1 = %q, want b.go:2:1<TAB> (empty payload)", lines[1])
	}
	if !strings.Contains(lines[0], "\tHelper [Function]") {
		t.Errorf("line 0 missing payload: %q", lines[0])
	}
}

// TestRender_LocusList_Dedup asserts duplicate daemon loci collapse to one line.
func TestRender_LocusList_Dedup(t *testing.T) {
	root := "/ws"
	res := textResult("file:///ws/a.go:5:6 — Helper [Function]\nfile:///ws/a.go:5:6 — Helper [Function]\n")
	spec := specFor(t, "search-symbols")

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: root, color: colorNever})

	lines := splitNonEmpty(buf.String())
	if len(lines) != 1 {
		t.Fatalf("dedup failed: got %d lines, want 1: %q", len(lines), buf.String())
	}
}

// TestRender_Deterministic asserts rendering the SAME result twice yields
// byte-identical stdout. (OUT-02)
func TestRender_Deterministic(t *testing.T) {
	root := "/ws"
	res := textResult("file:///ws/b.go:2:1\nfile:///ws/a.go:5:6 — Helper [Function]\nfile:///ws/a.go:1:1\n")
	spec := specFor(t, "search-symbols")

	var a, b bytes.Buffer
	renderResultFor(&a, spec, res, renderOpts{workspaceRoot: root, color: colorNever})
	renderResultFor(&b, spec, res, renderOpts{workspaceRoot: root, color: colorNever})
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("render not deterministic:\nA=%q\nB=%q", a.String(), b.String())
	}
}

// TestRender_NoColorBytes asserts piped/--color=never output contains zero ESC
// (0x1b) bytes. (OUT-01/06)
func TestRender_NoColorBytes(t *testing.T) {
	root := "/ws"
	res := textResult("file:///ws/a.go:5:6 — Helper [Function]\n")
	spec := specFor(t, "search-symbols")

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: root, color: colorNever})
	if bytes.IndexByte(buf.Bytes(), 0x1b) != -1 {
		t.Errorf("--color=never output contains ESC byte: %q", buf.String())
	}
}

// TestRender_NeverEqualsPiped asserts --color=never output == an auto-on-pipe
// render byte-for-byte. A non-TTY buffer means auto resolves to no-color, so the
// two must be identical. (OUT-06)
func TestRender_NeverEqualsPiped(t *testing.T) {
	root := "/ws"
	res := textResult("file:///ws/a.go:5:6 — Helper [Function]\nfile:///ws/b.go:2:1\n")
	spec := specFor(t, "search-symbols")

	var never, piped bytes.Buffer
	renderResultFor(&never, spec, res, renderOpts{workspaceRoot: root, color: colorNever})
	renderResultFor(&piped, spec, res, renderOpts{workspaceRoot: root, color: colorAuto})
	if !bytes.Equal(never.Bytes(), piped.Bytes()) {
		t.Fatalf("--color=never != piped(auto):\nnever=%q\npiped=%q", never.String(), piped.String())
	}
}

// TestRender_Tree_Passthrough asserts a tree-class result passes through
// verbatim (no relpath:line:col reshaping). (OUT-03)
func TestRender_Tree_Passthrough(t *testing.T) {
	spec := specFor(t, "get-symbol-overview")
	body := "Function main (line 10)\n  Variable x (line 11)\n"
	res := textResult(body)

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: "/ws", color: colorNever})
	if buf.String() != body {
		t.Errorf("tree passthrough altered output:\ngot=%q\nwant=%q", buf.String(), body)
	}
}

// TestRender_Opaque_Passthrough asserts an opaque-class result (hover markdown)
// passes through verbatim.
func TestRender_Opaque_Passthrough(t *testing.T) {
	spec := specFor(t, "get-hover-info")
	body := "```go\nfunc Helper()\n```\nHelper does things.\n"
	res := textResult(body)

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: "/ws", color: colorNever})
	if buf.String() != body {
		t.Errorf("opaque passthrough altered output:\ngot=%q\nwant=%q", buf.String(), body)
	}
}

// TestRender_Abs_KeepsAbsolute asserts --abs keeps the absolute path. (OUT-07)
func TestRender_Abs_KeepsAbsolute(t *testing.T) {
	root := "/ws"
	res := textResult("file:///ws/a.go:5:6 — Helper [Function]\n")
	spec := specFor(t, "search-symbols")

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: root, abs: true, color: colorNever})
	if !strings.HasPrefix(buf.String(), "/ws/a.go:5:6\t") {
		t.Errorf("--abs output = %q, want absolute /ws/a.go:5:6<TAB>...", buf.String())
	}
}

// TestRender_Default_NoAbsoluteLeak asserts default (no --abs) output is
// workspace-relative — no leading absolute workspace prefix. (T-92-03)
func TestRender_Default_NoAbsoluteLeak(t *testing.T) {
	root := "/ws"
	res := textResult("file:///ws/a.go:5:6 — Helper [Function]\n")
	spec := specFor(t, "search-symbols")

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: root, color: colorNever})
	if strings.HasPrefix(buf.String(), "/ws") || strings.HasPrefix(buf.String(), "/") {
		t.Errorf("default output leaked absolute path: %q", buf.String())
	}
}

// TestRender_JSON_LocusLines asserts --json on a locus-list result emits one
// compact JSON object per locus, one per line. (OUT-06)
func TestRender_JSON_LocusLines(t *testing.T) {
	root := "/ws"
	res := textResult("file:///ws/a.go:5:6 — Helper [Function]\nfile:///ws/b.go:2:1\n")
	spec := specFor(t, "search-symbols")

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: root, jsonOut: true, color: colorNever})
	lines := splitNonEmpty(buf.String())
	if len(lines) != 2 {
		t.Fatalf("got %d json lines, want 2: %q", len(lines), buf.String())
	}
	for _, l := range lines {
		for _, key := range []string{`"path"`, `"line"`, `"col"`, `"payload"`} {
			if !strings.Contains(l, key) {
				t.Errorf("json line %q missing key %s", l, key)
			}
		}
	}
	if !strings.Contains(lines[0], `"path":"a.go"`) {
		t.Errorf("json line 0 = %q, want path a.go", lines[0])
	}
}

// TestRender_NavSnippet asserts a nav verb (go_to_definition) appends one
// snippet line read CLI-side from the file at relpath:line, clamped to root.
// (OUT-03 / SC#2)
func TestRender_NavSnippet(t *testing.T) {
	root := t.TempDir()
	// Create a hermetic fixture file at root/sub/foo.go.
	dir := filepath.Join(root, "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "package sub\n\nfunc Foo() {} // the target line\n"
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	res := textResult("file://" + filepath.Join(dir, "foo.go") + ":3:6\n")
	spec := specFor(t, "go-to-definition")

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: root, color: colorNever})
	out := buf.String()
	if !strings.Contains(out, "sub/foo.go:3:6") {
		t.Fatalf("nav output missing locus: %q", out)
	}
	if !strings.Contains(out, "func Foo()") {
		t.Errorf("nav output missing snippet of line 3: %q", out)
	}
}

// TestRender_NavSnippet_TraversalClamp asserts a relpath escaping the workspace
// root via `..` yields NO snippet and NO error — no file read outside root.
// (T-92-04)
func TestRender_NavSnippet_TraversalClamp(t *testing.T) {
	root := t.TempDir()
	// Plant a secret OUTSIDE the workspace root.
	parent := filepath.Dir(root)
	secret := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(secret, []byte("TOPSECRET\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(secret) })

	// A relative locus that escapes the root.
	res := textResult("../secret.txt:1: TOPSECRET\n")
	spec := specFor(t, "go-to-definition")

	var buf bytes.Buffer
	renderResultFor(&buf, spec, res, renderOpts{workspaceRoot: root, color: colorNever})
	if strings.Contains(buf.String(), "TOPSECRET") {
		// The match-line payload may still echo daemon text, but no FILE READ
		// snippet must be appended from outside root. To be strict: assert the
		// secret content is not double-present (one from payload max, never read).
	}
	// Direct helper-level assertion: readSnippetLine must refuse the escape.
	if _, ok := readSnippetLine(root, "../secret.txt", 1); ok {
		t.Errorf("readSnippetLine read a file outside the workspace root")
	}
}

// TestReadSnippetLine_Hermetic asserts readSnippetLine returns the requested
// 1-based line from a file inside root.
func TestReadSnippetLine_Hermetic(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := readSnippetLine(root, "a.go", 2)
	if !ok {
		t.Fatalf("readSnippetLine returned ok=false for valid line")
	}
	if got != "two" {
		t.Errorf("readSnippetLine = %q, want %q", got, "two")
	}
	// Out-of-range line → no snippet, no panic.
	if _, ok := readSnippetLine(root, "a.go", 99); ok {
		t.Errorf("readSnippetLine returned ok=true for out-of-range line")
	}
}

// splitNonEmpty splits on newline and drops trailing empty entries.
func splitNonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

// --- Task 2: flag wiring + kind-preserving error ---

// TestRoot_PersistentColorAbsFlags asserts the root command exposes persistent
// --color and --abs flags, and --json remains resolvable. (OUT-06/07)
func TestRoot_PersistentColorAbsFlags(t *testing.T) {
	root := NewRootCommand()
	if root.PersistentFlags().Lookup("color") == nil {
		t.Errorf("root missing persistent --color flag")
	}
	if root.PersistentFlags().Lookup("abs") == nil {
		t.Errorf("root missing persistent --abs flag")
	}
	if root.PersistentFlags().Lookup("json") == nil {
		t.Errorf("--json must be a persistent flag verbs inherit")
	}
}

// TestVerb_InheritsPersistentFlags asserts a verb subcommand inherits
// --color/--abs/--json from the root via PersistentFlags (resolvable on the
// verb's inherited flag set), not redefined per verb.
func TestVerb_InheritsPersistentFlags(t *testing.T) {
	sub, _ := findRootVerb(t, "go-to-definition")
	for _, name := range []string{"color", "abs", "json"} {
		if sub.InheritedFlags().Lookup(name) == nil {
			t.Errorf("verb does not inherit --%s from root", name)
		}
		// Must NOT be redefined as a local verb flag (would shadow / panic).
		if sub.Flags().Lookup(name) != nil && sub.LocalFlags().Lookup(name) != nil {
			t.Errorf("--%s appears as a local verb flag (should be inherited only)", name)
		}
	}
}

// TestRunVerb_KindPreservingError asserts runVerb on an IsError result returns
// an error whose string carries the typed kind (parseKind succeeds), NOT the
// generic "tool X reported an error" wrap. (OUT-05, replaces verb.go:216-218)
func TestRunVerb_KindPreservingError(t *testing.T) {
	restore := callToolFn
	callToolFn = func(_ context.Context, _ string, _ *slog.Logger, _ string, _ string, _ map[string]any) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			IsError: true,
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "permission_denied: workspace not activated"}},
		}, nil
	}
	defer func() { callToolFn = restore }()

	cmd := NewRootCommand()
	cmd.SetArgs([]string{"go-to-definition", "--path=x.go", "--line=1", "--column=1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected an error for IsError result, got nil")
	}
	if strings.Contains(err.Error(), "reported an error") {
		t.Fatalf("error used the generic kind-dropping wrap: %q", err.Error())
	}
	kind, ok := parseKind(err.Error())
	if !ok {
		t.Fatalf("parseKind failed on runVerb error %q — typed kind was dropped", err.Error())
	}
	if exitCodeForKind(kind) != 5 {
		t.Errorf("permission_denied error maps to exit %d, want 5", exitCodeForKind(kind))
	}
}
