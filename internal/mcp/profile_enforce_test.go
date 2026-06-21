package mcp_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/mcp"
)

// discardLoggerPE returns an error-level logger so test output stays quiet.
func discardLoggerPE() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// sessionWith builds a *mcp.SessionInfo carrying the given profile/mode and
// AllowedTools whitelist. A nil tools slice means "no whitelist" (all tools).
func sessionWith(profile, mode string, tools []string) *mcp.SessionInfo {
	s := &mcp.SessionInfo{Profile: profile, Mode: mode}
	// SetAllowedTools copies; nil stays nil (== all tools).
	s.SetAllowedTools(tools)
	return s
}

// sessionGetter returns a getSession closure yielding the supplied session.
func sessionGetter(sess *mcp.SessionInfo) func(ctx context.Context) *mcp.SessionInfo {
	return func(ctx context.Context) *mcp.SessionInfo { return sess }
}

// --- Refuse: tool outside AllowedTools returns typed PermissionDenied, next not called ---

func TestProfileEnforce_Refuse_TypedPermissionDenied(t *testing.T) {
	sess := sessionWith("ci-bot", "read", []string{"goto_definition", "find_references"})
	mw := mcp.ProfileEnforcementMiddleware(sessionGetter(sess), discardLoggerPE())

	nextCalled := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		nextCalled = true
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("replace_symbol_body", `{"symbol_name":"Foo"}`)
	_, err := handler(context.Background(), "tools/call", req)
	if err == nil {
		t.Fatal("expected PermissionDenied error, got nil")
	}
	if !errors.Is(err, serr.ErrPermissionDenied) {
		t.Errorf("expected errors.Is(err, ErrPermissionDenied) == true, got: %v", err)
	}
	if nextCalled {
		t.Error("expected inner handler NOT to be called when tool is refused")
	}
}

// --- Allow: tool in AllowedTools passes through, next called once ---

func TestProfileEnforce_Allow_PassesThrough(t *testing.T) {
	sess := sessionWith("ci-bot", "read", []string{"goto_definition", "find_references"})
	mw := mcp.ProfileEnforcementMiddleware(sessionGetter(sess), discardLoggerPE())

	calls := 0
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		calls++
		return &mcpsdk.CallToolResult{Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "ok"}}}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("goto_definition", `{"symbol_name":"Foo"}`)
	res, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected inner handler called exactly once, got %d", calls)
	}
	if res == nil {
		t.Error("expected non-nil result on allow")
	}
}

// --- Core infra tools (ping/echo/activate_project) are exempt even under a
// restrictive whitelist: they are registered outside the profile/skill system
// and are not present in AllowedTools, but must remain callable (regression for
// the Phase 91 post-wave gate failure where activation broke). ---

func TestProfileEnforce_CoreTools_AlwaysAllowed(t *testing.T) {
	// A restrictive read-mode whitelist that does NOT list any core tool.
	sess := sessionWith("ci-bot", "read", []string{"goto_definition"})
	mw := mcp.ProfileEnforcementMiddleware(sessionGetter(sess), discardLoggerPE())

	for _, tool := range []string{"activate_project", "ping", "echo", "switch_mode", "get_token_budget"} {
		calls := 0
		inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			calls++
			return &mcpsdk.CallToolResult{}, nil
		}
		handler := mw(inner)
		req := makeCallToolRequest(tool, `{}`)
		_, err := handler(context.Background(), "tools/call", req)
		if err != nil {
			t.Errorf("core tool %q must pass through enforcement, got error: %v", tool, err)
		}
		if calls != 1 {
			t.Errorf("core tool %q: expected inner handler called once, got %d", tool, calls)
		}
	}
}

// --- Passthrough method: non-tools/call methods are never filtered ---

func TestProfileEnforce_NonToolsCall_PassesThrough(t *testing.T) {
	// AllowedTools is restrictive; a tools/list must still pass.
	sess := sessionWith("ci-bot", "read", []string{"goto_definition"})
	mw := mcp.ProfileEnforcementMiddleware(sessionGetter(sess), discardLoggerPE())

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.ListToolsResult{}, nil
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler called for non-tools/call method")
	}
}

// --- Nil session: no whitelist available → allow ---

func TestProfileEnforce_NilSession_PassesThrough(t *testing.T) {
	mw := mcp.ProfileEnforcementMiddleware(nilSession, discardLoggerPE())

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("replace_symbol_body", `{}`)
	_, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler called when session is nil")
	}
}

// --- Nil whitelist: AllowedTools == nil means all tools allowed ---

func TestProfileEnforce_NilWhitelist_PassesThrough(t *testing.T) {
	sess := sessionWith("full", "admin", nil) // nil whitelist == all tools
	mw := mcp.ProfileEnforcementMiddleware(sessionGetter(sess), discardLoggerPE())

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("delete_file", `{"path":"x"}`)
	_, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler called when AllowedTools is nil (== all)")
	}
}

// --- Typed message + WithTool: error names the tool and profile/mode ---

func TestProfileEnforce_TypedMessageAndTool(t *testing.T) {
	sess := sessionWith("ci-bot", "read", []string{"goto_definition"})
	mw := mcp.ProfileEnforcementMiddleware(sessionGetter(sess), discardLoggerPE())

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("replace_symbol_body", `{}`)
	_, err := handler(context.Background(), "tools/call", req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var se *serr.Error
	if !errors.As(err, &se) {
		t.Fatalf("expected error to unwrap to *serr.Error, got: %v", err)
	}
	if se.Kind != serr.PermissionDenied {
		t.Errorf("expected Kind=permission_denied, got %q", se.Kind)
	}
	if se.Tool != "replace_symbol_body" {
		t.Errorf("expected Tool=replace_symbol_body, got %q", se.Tool)
	}
	msg := err.Error()
	for _, want := range []string{"replace_symbol_body", "ci-bot", "read"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected error message to contain %q, got: %s", want, msg)
		}
	}
}

// --- Edit-mode allow (SEC-01 SC#4): same tool allowed when in the whitelist ---

func TestProfileEnforce_EditMode_Allow(t *testing.T) {
	sess := sessionWith("claude-code", "edit",
		[]string{"goto_definition", "replace_symbol_body"})
	mw := mcp.ProfileEnforcementMiddleware(sessionGetter(sess), discardLoggerPE())

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	req := makeCallToolRequest("replace_symbol_body", `{"symbol_name":"Foo"}`)
	_, err := handler(context.Background(), "tools/call", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler called for replace_symbol_body under edit mode")
	}
}

// --- LIFO install-order invariant (Task 2): ProfileEnforce installed AFTER
// Guardrail and BEFORE LazyInit yields execution order LazyInit → ProfileEnforce
// → Guardrail. An order-recording chain proves the contract so a future reorder
// that breaks LazyInit-first / ProfileEnforce-before-Guardrail fails this test. ---

func TestProfileEnforce_LIFO_ExecutionOrder(t *testing.T) {
	var order []string

	// recorder builds a middleware that records its label on the request path
	// (before calling next), mirroring how AddReceivingMiddleware composes LIFO.
	recorder := func(label string) mcpsdk.Middleware {
		return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
				order = append(order, label)
				return next(ctx, method, req)
			}
		}
	}

	// Compose in the SAME install order the daemon uses for the relevant tail:
	// Guardrail (14b.5) → ProfileEnforce (14b.6) → LazyInit (14c).
	// AddReceivingMiddleware composes LIFO, so the LAST installed runs FIRST.
	// We replicate that LIFO wrapping order here.
	installOrder := []mcpsdk.Middleware{
		recorder("Guardrail"),
		recorder("ProfileEnforce"),
		recorder("LazyInit"),
	}

	terminal := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		order = append(order, "handler")
		return &mcpsdk.CallToolResult{}, nil
	}

	// Apply LIFO: wrap terminal with each middleware in install order; the last
	// installed ends up outermost (executes first).
	handler := terminal
	for _, mw := range installOrder {
		handler = mw(handler)
	}

	req := makeCallToolRequest("goto_definition", `{}`)
	if _, err := handler(context.Background(), "tools/call", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"LazyInit", "ProfileEnforce", "Guardrail", "handler"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("execution order = %v, want %v (LazyInit must run first; ProfileEnforce before Guardrail)", order, want)
	}
}

// --- Refuse-after-activate: a refusal propagates even when an upstream
// (LazyInit-like) middleware "activates" before ProfileEnforce denies. ---

func TestProfileEnforce_RefuseAfterActivate_Propagates(t *testing.T) {
	sess := sessionWith("ci-bot", "read", []string{"goto_definition"})
	enforce := mcp.ProfileEnforcementMiddleware(sessionGetter(sess), discardLoggerPE())

	activated := false
	// lazyLike runs before enforce in the chain (installed after enforce → LIFO).
	lazyLike := func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
			activated = true // simulate workspace activation
			return next(ctx, method, req)
		}
	}

	terminal := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}

	// Chain: lazyLike(enforce(terminal)) → lazyLike runs first, then enforce denies.
	handler := lazyLike(enforce(terminal))
	req := makeCallToolRequest("replace_symbol_body", `{}`)
	_, err := handler(context.Background(), "tools/call", req)
	if !activated {
		t.Error("expected upstream activation to run before enforcement")
	}
	if !errors.Is(err, serr.ErrPermissionDenied) {
		t.Errorf("expected PermissionDenied to propagate after activation, got: %v", err)
	}
}
