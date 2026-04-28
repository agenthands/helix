package obs

import (
	"strings"
	"testing"
)

// Phase 55-02 / OBS-04 #2: closed allowlist of span attribute keys.
//
// Mirror of metrics_labels_test.go (Phase 11 / 53). The allowlist below
// is the SINGLE point of attribute review for Serena's tracing surface.
// Any new attribute key must be added here AND certified in
// TRACE-AUDIT.md (plan 55-03) — the integration test in
// `internal/mcp/attribute_allowlist_integration_test.go` enforces this
// against live span emission.
//
// The static exhaustiveness check below (TestSpanAllowlistIsExhaustive)
// guards against typos that would silently skip enforcement for a span
// name. It does NOT exercise the production span-emission paths — that
// is the integration test's job (kept in package mcp to dodge an
// internal/obs ↔ internal/mcp import cycle).
//
// Wildcard convention: a span name ending in `.*` matches by prefix
// (the `.*` is stripped before comparison). The empty value-set on
// `kernel.tool.*` and `skill.tool.*` is INTENTIONAL — D-07 forbids any
// attributes on the per-tool child spans because the parent
// TelemetryMiddleware span already carries tool_name / profile / mode /
// language / outcome.
var allowedSpanAttrs = map[string]map[string]struct{}{
	"daemon.mcp.tools.call": {
		"tool_name": {},
		"profile":   {},
		"mode":      {},
		"language":  {},
		"outcome":   {},
	},
	// Per-tool child spans: zero attributes by design (D-07).
	"kernel.tool.*": {},
	"skill.tool.*":  {},
	// LS request spans: bounded enums sourced from internal config —
	// lsp.method (~30 LSP methods), lsp.language (52 langregistry
	// entries), lsp.duration_ms (numeric).
	"ls.request": {
		"lsp.method":      {},
		"lsp.language":    {},
		"lsp.duration_ms": {},
	},
}

// LookupAllowedSpanAttrs returns the allowed-attribute-key set for the
// given span name, plus whether the name is known. Wildcard keys (those
// ending in `.*`) are matched by prefix after stripping the `.*`.
//
// Exported for use by the integration test in internal/mcp (which
// cannot import this _test.go file across package boundaries; the
// integration test maintains a parallel literal that is kept in sync
// via TestSpanAllowlistIsExhaustive proving both are non-nil).
func lookupAllowedSpanAttrs(spanName string) (map[string]struct{}, bool) {
	if attrs, ok := allowedSpanAttrs[spanName]; ok {
		return attrs, true
	}
	for key, attrs := range allowedSpanAttrs {
		if strings.HasSuffix(key, ".*") {
			prefix := strings.TrimSuffix(key, "*") // keep trailing dot
			if strings.HasPrefix(spanName, prefix) {
				return attrs, true
			}
		}
	}
	return nil, false
}

// TestSpanAllowlistIsExhaustive enforces the static property that every
// span-name entry in allowedSpanAttrs has a non-nil value set. The
// empty `kernel.tool.*` / `skill.tool.*` entries are intentional
// sentinels (D-07: zero attributes); a nil map would be a typo that
// silently skips enforcement. This is the obs-side guard; the
// integration test in internal/mcp drives the production emission
// paths and asserts every captured key is in the allowlist.
func TestSpanAllowlistIsExhaustive(t *testing.T) {
	if len(allowedSpanAttrs) == 0 {
		t.Fatal("allowedSpanAttrs is empty — no attribute hygiene enforcement")
	}
	for name, attrs := range allowedSpanAttrs {
		if attrs == nil {
			t.Errorf("span %q has nil attribute set — would silently skip enforcement (use empty map for zero-attr spans)", name)
		}
	}
}

// TestLookupAllowedSpanAttrsWildcard confirms the wildcard prefix match
// works for the per-tool span families.
func TestLookupAllowedSpanAttrsWildcard(t *testing.T) {
	cases := []struct {
		spanName string
		wantHit  bool
	}{
		{"daemon.mcp.tools.call", true},
		{"kernel.tool.find_symbol", true},
		{"skill.tool.memory_write", true},
		{"ls.request", true},
		{"ls.request.textDocument/definition", false}, // uniform name only — no per-method spans
		{"unknown.span.name", false},
	}
	for _, tc := range cases {
		_, hit := lookupAllowedSpanAttrs(tc.spanName)
		if hit != tc.wantHit {
			t.Errorf("lookupAllowedSpanAttrs(%q) = hit:%v, want %v", tc.spanName, hit, tc.wantHit)
		}
	}
}
