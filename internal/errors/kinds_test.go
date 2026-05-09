package errors_test

import (
	"errors"
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
)

func TestGuardrailViolationKind(t *testing.T) {
	t.Parallel()

	t.Run("Kind string value", func(t *testing.T) {
		t.Parallel()
		if string(serr.GuardrailViolation) != "guardrail_violation" {
			t.Errorf("GuardrailViolation Kind = %q; want %q", serr.GuardrailViolation, "guardrail_violation")
		}
	})

	t.Run("ErrGuardrailViolation sentinel kind matches", func(t *testing.T) {
		t.Parallel()
		if serr.ErrGuardrailViolation.Kind != serr.GuardrailViolation {
			t.Errorf("ErrGuardrailViolation.Kind = %q; want %q", serr.ErrGuardrailViolation.Kind, serr.GuardrailViolation)
		}
	})
}

func TestNewGuardrailViolation_ErrorsIs(t *testing.T) {
	t.Parallel()

	err := serr.NewGuardrailViolation(
		"G-001",
		"rename by grep detected",
		[]serr.ReceiptClassRef{"references_checked"},
		[]string{"find_references"},
		[]serr.SeeAlsoRef{{Tool: "get_tool_help", Args: map[string]string{"topic": "workflow:rename"}}},
	)

	if err == nil {
		t.Fatal("NewGuardrailViolation returned nil")
	}

	if !errors.Is(err, serr.ErrGuardrailViolation) {
		t.Errorf("errors.Is(err, ErrGuardrailViolation) = false; want true")
	}

	// Must NOT match a different sentinel.
	if errors.Is(err, serr.ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = true; want false")
	}
}

func TestNewGuardrailViolation_ErrorsAs(t *testing.T) {
	t.Parallel()

	rule := "G-002"
	message := "delete without refs"
	required := []serr.ReceiptClassRef{"references_checked", "impact_checked"}
	suggested := []string{"find_references", "analyze_blast_radius"}
	seeAlso := []serr.SeeAlsoRef{
		{Tool: "get_tool_help", Args: map[string]string{"topic": "workflow:delete"}},
	}

	err := serr.NewGuardrailViolation(rule, message, required, suggested, seeAlso)

	// Use AsGuardrailViolation to recover the typed payload.
	detail := serr.AsGuardrailViolation(err)
	if detail == nil {
		t.Fatal("AsGuardrailViolation returned nil")
	}
	if detail.Rule != rule {
		t.Errorf("detail.Rule = %q; want %q", detail.Rule, rule)
	}
	if detail.Message != message {
		t.Errorf("detail.Message = %q; want %q", detail.Message, message)
	}
	if len(detail.RequiredReceipts) != len(required) {
		t.Errorf("detail.RequiredReceipts len = %d; want %d", len(detail.RequiredReceipts), len(required))
	}
	if len(detail.SuggestedTools) != len(suggested) {
		t.Errorf("detail.SuggestedTools len = %d; want %d", len(detail.SuggestedTools), len(suggested))
	}
	if len(detail.SeeAlso) != len(seeAlso) {
		t.Errorf("detail.SeeAlso len = %d; want %d", len(detail.SeeAlso), len(seeAlso))
	}
}

func TestNewGuardrailViolation_ErrorString(t *testing.T) {
	t.Parallel()

	err := serr.NewGuardrailViolation("G-003", "public API edit", nil, nil, nil)
	got := err.Error()
	if got == "" {
		t.Errorf("err.Error() is empty")
	}
}
