package semantic

import (
	"fmt"
	"strings"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/mcp"
)

// modeTier classifies a tool's required mode tier. Phase 64 establishes this
// per-handler enforcement pattern; Phase 66 GuardrailMiddleware will piggy-back
// on the error envelope shape introduced here.
//
// Tier semantics (matches SPEC §30.2 mode-gating language):
//   - modeTierRead: every session passes (read+ tools).
//   - modeTierReview: session must be in "review" or "admin" (review+ tools).
//   - modeTierAdmin: session must be in "admin" (admin-only tools).
type modeTier int

const (
	modeTierRead   modeTier = iota
	modeTierReview          // review+ — session must be in review or admin
	modeTierAdmin           // admin only
)

// checkMode returns nil when the session's current mode satisfies the required
// tier; otherwise returns a structured PermissionDenied error whose Detail
// field hints at switch_mode elevation.
//
// NEW pattern: Phase 64 establishes per-tool handler-side mode-tier
// enforcement. The session.Mode read uses the SessionSnapshot RLock pattern
// (internal/mcp/middleware.go:339-346) for race-free reads.
func checkMode(snap mcp.SessionSnapshot, required modeTier) error {
	cur := strings.ToLower(snap.Mode)
	switch required {
	case modeTierRead:
		// Every session passes; no mode check needed.
		return nil
	case modeTierReview:
		if cur == "review" || cur == "admin" {
			return nil
		}
	case modeTierAdmin:
		if cur == "admin" {
			return nil
		}
	}

	// Build the human-readable required-tier label for the error envelope.
	var requiredLabel string
	switch required {
	case modeTierReview:
		requiredLabel = "review or admin"
	case modeTierAdmin:
		requiredLabel = "admin"
	default:
		// Unreachable in practice (modeTierRead returns nil above), but keep
		// a safe default to avoid emitting an empty label on a future tier
		// added without updating this switch.
		requiredLabel = "read"
	}

	// Suggest the lowest-privilege tier that satisfies the requirement so
	// agents can elevate via switch_mode without overshooting to admin.
	suggestedTarget := "review"
	if required == modeTierAdmin {
		suggestedTarget = "admin"
	}

	return serr.New(serr.PermissionDenied,
		fmt.Sprintf("tool requires mode %s; current mode is %q", requiredLabel, cur)).
		WithDetail(fmt.Sprintf("call switch_mode(target_mode=%q) to elevate", suggestedTarget))
}
