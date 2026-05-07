package semantic

import (
	stderrors "errors"
	"strings"
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/mcp"
)

// TestCheckMode_TierReadAcceptsAny asserts modeTierRead returns nil for every
// mode value (read+ tools are universally accessible).
func TestCheckMode_TierReadAcceptsAny(t *testing.T) {
	cases := []string{"read", "edit", "review", "admin", "READ", "Edit", "REVIEW", ""}
	for _, mode := range cases {
		t.Run(mode, func(t *testing.T) {
			snap := mcp.SessionSnapshot{Mode: mode}
			if err := checkMode(snap, modeTierRead); err != nil {
				t.Fatalf("modeTierRead must accept mode=%q; got error: %v", mode, err)
			}
		})
	}
}

// TestCheckMode_TierReviewAcceptsReviewAndAdmin asserts modeTierReview returns
// nil for review/admin and PermissionDenied for read/edit.
func TestCheckMode_TierReviewAcceptsReviewAndAdmin(t *testing.T) {
	allowed := []string{"review", "admin", "Review", "ADMIN"}
	denied := []string{"read", "edit", "READ", "Edit", ""}

	for _, mode := range allowed {
		t.Run("allowed/"+mode, func(t *testing.T) {
			snap := mcp.SessionSnapshot{Mode: mode}
			if err := checkMode(snap, modeTierReview); err != nil {
				t.Fatalf("modeTierReview must accept mode=%q; got error: %v", mode, err)
			}
		})
	}
	for _, mode := range denied {
		t.Run("denied/"+mode, func(t *testing.T) {
			snap := mcp.SessionSnapshot{Mode: mode}
			err := checkMode(snap, modeTierReview)
			if err == nil {
				t.Fatalf("modeTierReview must reject mode=%q; got nil", mode)
			}
			if !stderrors.Is(err, serr.ErrPermissionDenied) {
				t.Fatalf("expected PermissionDenied for mode=%q; got %v", mode, err)
			}
		})
	}
}

// TestCheckMode_TierAdminOnlyAdmin asserts modeTierAdmin returns nil only for
// admin and PermissionDenied for everything else.
func TestCheckMode_TierAdminOnlyAdmin(t *testing.T) {
	allowed := []string{"admin", "ADMIN", "Admin"}
	denied := []string{"read", "edit", "review", ""}

	for _, mode := range allowed {
		t.Run("allowed/"+mode, func(t *testing.T) {
			snap := mcp.SessionSnapshot{Mode: mode}
			if err := checkMode(snap, modeTierAdmin); err != nil {
				t.Fatalf("modeTierAdmin must accept mode=%q; got error: %v", mode, err)
			}
		})
	}
	for _, mode := range denied {
		t.Run("denied/"+mode, func(t *testing.T) {
			snap := mcp.SessionSnapshot{Mode: mode}
			err := checkMode(snap, modeTierAdmin)
			if err == nil {
				t.Fatalf("modeTierAdmin must reject mode=%q; got nil", mode)
			}
			if !stderrors.Is(err, serr.ErrPermissionDenied) {
				t.Fatalf("expected PermissionDenied for mode=%q; got %v", mode, err)
			}
		})
	}
}

// TestCheckMode_ErrorEnvelopeShape asserts the structured error envelope:
//   - Kind == PermissionDenied (matched via errors.Is(err, serr.ErrPermissionDenied))
//   - Message contains "current mode is"
//   - Detail hints at switch_mode elevation
func TestCheckMode_ErrorEnvelopeShape(t *testing.T) {
	snap := mcp.SessionSnapshot{Mode: "read"}
	err := checkMode(snap, modeTierReview)
	if err == nil {
		t.Fatal("expected error for mode=read at modeTierReview")
	}

	if !stderrors.Is(err, serr.ErrPermissionDenied) {
		t.Fatalf("expected PermissionDenied via errors.Is; got %v", err)
	}

	var typed *serr.Error
	if !stderrors.As(err, &typed) {
		t.Fatalf("expected *serr.Error; got %T", err)
	}
	if typed.Kind != serr.PermissionDenied {
		t.Fatalf("expected Kind=PermissionDenied; got %q", typed.Kind)
	}

	msg := err.Error()
	if !strings.Contains(msg, "current mode is") {
		t.Fatalf("error must contain 'current mode is'; got %q", msg)
	}

	if !strings.Contains(typed.Detail, "switch_mode") {
		t.Fatalf("error Detail must hint at switch_mode; got %q", typed.Detail)
	}

	// Required-tier text should reflect the requested tier (review).
	if !strings.Contains(msg, "review") {
		t.Fatalf("error message must mention 'review' for modeTierReview; got %q", msg)
	}

	// modeTierAdmin should suggest admin elevation, not review.
	snap2 := mcp.SessionSnapshot{Mode: "review"}
	err2 := checkMode(snap2, modeTierAdmin)
	if err2 == nil {
		t.Fatal("expected error for mode=review at modeTierAdmin")
	}
	var typed2 *serr.Error
	if !stderrors.As(err2, &typed2) {
		t.Fatalf("expected *serr.Error; got %T", err2)
	}
	if !strings.Contains(typed2.Detail, `target_mode="admin"`) {
		t.Fatalf("modeTierAdmin must suggest target_mode=admin; got Detail=%q", typed2.Detail)
	}
}
