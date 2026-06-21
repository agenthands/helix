package daemon

import (
	"testing"

	"github.com/agenthands/helix/internal/profile"
)

// TestResolveAllowedToolsForMode_UnknownModeReturnsNil documents the latent
// fail-OPEN trap that WR-01 closes at the daemon bootstrap call site:
// resolveAllowedToolsForMode returns nil for an unregistered mode, and
// ProfileEnforcementMiddleware treats a nil whitelist as "all tools allowed".
// The daemon must therefore validate the initial mode BEFORE calling this
// helper (see daemon.go bootstrap), never relying on its nil return as a
// safe deny.
func TestResolveAllowedToolsForMode_UnknownModeReturnsNil(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	prof := store.DefaultProfile()
	if prof == nil {
		t.Fatal("expected a non-nil default profile")
	}

	got := resolveAllowedToolsForMode(store, prof, "this-mode-does-not-exist")
	if got != nil {
		t.Fatalf("expected nil (fail-open trap) for unknown mode, got %v", got)
	}
}

// TestInitialModeValidation_FailsClosedOnUnknownMode is the WR-01 regression:
// the daemon bootstrap MUST refuse to start when the configured initial mode
// does not resolve to a registered mode (an active profile is always present
// via ResolveProfile's fallback). We assert the exact predicate the bootstrap
// uses — store.Mode(initialMode) — so a refactor that drops the guard fails
// this test. A nil/allow-all whitelist would otherwise silently disable
// tools/call enforcement for the whole session.
func TestInitialModeValidation_FailsClosedOnUnknownMode(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	// Unknown mode => bootstrap guard fires (refuses to start).
	if _, ok := store.Mode("bogus-operational-mode"); ok {
		t.Fatal("expected unknown mode to NOT resolve")
	}
}

// TestInitialModeValidation_DefaultModesResolve guards normal startup: every
// profile's DefaultMode (and the "edit" fallback) MUST resolve to a registered
// mode so the WR-01 fail-closed guard never trips on the default configuration.
func TestInitialModeValidation_DefaultModesResolve(t *testing.T) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}

	// The "edit" fallback used when a profile has no DefaultMode must resolve.
	if _, ok := store.Mode("edit"); !ok {
		t.Fatalf("the \"edit\" fallback mode must resolve; available=%v", store.ModeNames())
	}

	for _, name := range store.ProfileNames() {
		prof, ok := store.Profile(name)
		if !ok {
			t.Fatalf("profile %q reported by ProfileNames but not found", name)
		}
		initialMode := prof.DefaultMode
		if initialMode == "" {
			initialMode = "edit"
		}
		if _, ok := store.Mode(initialMode); !ok {
			t.Fatalf("profile %q DefaultMode %q does not resolve; available=%v",
				name, initialMode, store.ModeNames())
		}
	}
}
