package diag

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	gen "github.com/postfix/serena/protocol/gen"
)

func TestDiagnosticStore_StoreAndRetrieve(t *testing.T) {
	store := NewDiagnosticStore()

	uri := "file:///tmp/test.go"
	sev := gen.DiagnosticSeverityError
	diags := []gen.Diagnostic{
		{
			Range: gen.Range{
				Start: gen.Position{Line: 10, Character: 5},
				End:   gen.Position{Line: 10, Character: 15},
			},
			Severity: &sev,
			Message:  "undefined variable",
		},
	}

	store.HandlePublishDiagnostics(gen.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})

	got := store.GetDiagnostics(uri)
	if len(got) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(got))
	}
	if got[0].Message != "undefined variable" {
		t.Errorf("expected message %q, got %q", "undefined variable", got[0].Message)
	}
}

func TestDiagnosticStore_GetAll(t *testing.T) {
	store := NewDiagnosticStore()

	store.HandlePublishDiagnostics(gen.PublishDiagnosticsParams{
		URI:         "file:///a.go",
		Diagnostics: []gen.Diagnostic{{Message: "err1"}},
	})
	store.HandlePublishDiagnostics(gen.PublishDiagnosticsParams{
		URI:         "file:///b.go",
		Diagnostics: []gen.Diagnostic{{Message: "err2"}},
	})

	all := store.GetAllDiagnostics()
	if len(all) != 2 {
		t.Fatalf("expected 2 URIs, got %d", len(all))
	}
}

func TestDiagnosticStore_Clear(t *testing.T) {
	store := NewDiagnosticStore()

	uri := "file:///tmp/test.go"
	store.HandlePublishDiagnostics(gen.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: []gen.Diagnostic{{Message: "err"}},
	})

	store.Clear(uri)

	got := store.GetDiagnostics(uri)
	if len(got) != 0 {
		t.Fatalf("expected 0 diagnostics after clear, got %d", len(got))
	}
}

func TestDiagnosticStore_WaitForDiagnostics(t *testing.T) {
	store := NewDiagnosticStore()
	uri := "file:///tmp/wait.go"

	// Publish diagnostics after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		sev := gen.DiagnosticSeverityWarning
		store.HandlePublishDiagnostics(gen.PublishDiagnosticsParams{
			URI: uri,
			Diagnostics: []gen.Diagnostic{
				{Severity: &sev, Message: "warning here"},
			},
		})
	}()

	ctx := context.Background()
	diags, err := store.WaitForDiagnostics(ctx, uri, 2*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Message != "warning here" {
		t.Errorf("expected message %q, got %q", "warning here", diags[0].Message)
	}
}

func TestDiagnosticStore_WaitForDiagnostics_Timeout(t *testing.T) {
	store := NewDiagnosticStore()
	uri := "file:///tmp/timeout.go"

	ctx := context.Background()
	_, err := store.WaitForDiagnostics(ctx, uri, 50*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestDiagnosticStore_WaitForDiagnostics_AlreadyPresent(t *testing.T) {
	store := NewDiagnosticStore()
	uri := "file:///tmp/present.go"

	store.HandlePublishDiagnostics(gen.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: []gen.Diagnostic{{Message: "already here"}},
	})

	ctx := context.Background()
	diags, err := store.WaitForDiagnostics(ctx, uri, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) != 1 || diags[0].Message != "already here" {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestApplyFormatEdits_ReverseOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")

	original := "line0\nline1\nline2\nline3\nline4\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	// Two edits: replace "line1" with "LINE1" and "line3" with "LINE3".
	edits := []TextEditResult{
		{StartLine: 1, StartCol: 0, EndLine: 1, EndCol: 5, NewText: "LINE1"},
		{StartLine: 3, StartCol: 0, EndLine: 3, EndCol: 5, NewText: "LINE3"},
	}

	if err := ApplyFormatEdits(path, edits); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	expected := "line0\nLINE1\nline2\nLINE3\nline4\n"
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestCodeActionResult_Parsing(t *testing.T) {
	// Test that CodeActionResult correctly represents an action with and without edit.
	kindQF := gen.CodeActionKindQuickFix
	pref := true
	actions := []gen.CodeAction{
		{
			Title:       "Add import",
			Kind:        &kindQF,
			IsPreferred: &pref,
			Edit: &gen.WorkspaceEdit{
				Changes: map[string][]gen.TextEdit{
					"file:///test.go": {
						{Range: gen.Range{}, NewText: "import \"fmt\"\n"},
					},
				},
			},
		},
		{
			Title: "No-edit action",
		},
	}

	results := make([]CodeActionResult, 0, len(actions))
	for _, ca := range actions {
		r := CodeActionResult{
			Title:       ca.Title,
			Edit:        ca.Edit,
			Diagnostics: ca.Diagnostics,
		}
		if ca.Kind != nil {
			r.Kind = string(*ca.Kind)
		}
		if ca.IsPreferred != nil {
			r.IsPreferred = *ca.IsPreferred
		}
		results = append(results, r)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Kind != "quickfix" {
		t.Errorf("expected kind quickfix, got %s", results[0].Kind)
	}
	if !results[0].IsPreferred {
		t.Error("expected first result to be preferred")
	}
	if results[0].Edit == nil {
		t.Error("expected first result to have an edit")
	}
	if results[1].Edit != nil {
		t.Error("expected second result to have no edit")
	}
}

func TestHandleNotification_Dispatch(t *testing.T) {
	store := NewDiagnosticStore()

	// Simulate raw JSON for publishDiagnostics.
	raw := []byte(`{"uri":"file:///test.go","diagnostics":[{"message":"test error","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":5}}}]}`)
	store.HandleNotification("textDocument/publishDiagnostics", raw)

	diags := store.GetDiagnostics("file:///test.go")
	if len(diags) != 1 || diags[0].Message != "test error" {
		t.Fatalf("expected 1 diagnostic with 'test error', got %v", diags)
	}

	// Non-matching method should be ignored.
	store.HandleNotification("textDocument/didSave", raw)
}
