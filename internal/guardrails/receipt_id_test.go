package guardrails

import (
	"strings"
	"testing"
)

func TestParseReceiptID(t *testing.T) {
	t.Parallel()

	// Known-good ID for acceptance tests.
	// 26-char base32 body using standard alphabet.
	validID := "rcpt_aaaaaaaaaaaaaaaaaaaaaaaaa2"

	// Build a known-good 26-char body from NewReceiptID for reference test.
	goodID, err := NewReceiptID()
	if err != nil {
		t.Fatalf("NewReceiptID failed: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr error
		wantOK  bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: ErrInvalidReceiptIDPrefix,
		},
		{
			name:    "prefix only no underscore",
			input:   "rcpt",
			wantErr: ErrInvalidReceiptIDPrefix,
		},
		{
			name:    "prefix with underscore but no body",
			input:   "rcpt_",
			wantErr: ErrInvalidReceiptIDLength,
		},
		{
			name:    "body too short (3 chars)",
			input:   "rcpt_xxx",
			wantErr: ErrInvalidReceiptIDLength,
		},
		{
			name:    "body 25 chars (one short)",
			input:   "rcpt_" + strings.Repeat("a", 25),
			wantErr: ErrInvalidReceiptIDLength,
		},
		{
			name:    "body 27 chars (one long)",
			input:   "rcpt_" + strings.Repeat("a", 27),
			wantErr: ErrInvalidReceiptIDLength,
		},
		{
			name:    "wrong prefix",
			input:   "rct_" + strings.Repeat("a", 26),
			wantErr: ErrInvalidReceiptIDPrefix,
		},
		{
			name:    "invalid base32 chars (uppercase O which is not in alphabet)",
			input:   "rcpt_" + strings.Repeat("a", 25) + "0", // '0' not in base32
			wantErr: ErrInvalidReceiptIDAlphabet,
		},
		{
			name:    "invalid char underscore in body",
			input:   "rcpt__" + strings.Repeat("a", 25),
			wantErr: ErrInvalidReceiptIDAlphabet,
		},
		{
			name:   "known good 31-char id",
			input:  validID,
			wantOK: true,
		},
		{
			name:   "NewReceiptID output is valid",
			input:  string(goodID),
			wantOK: true,
		},
		{
			name:    "total len 30 (body 25)",
			input:   "rcpt_" + strings.Repeat("2", 25),
			wantErr: ErrInvalidReceiptIDLength,
		},
		{
			name:    "total len 32 (body 27)",
			input:   "rcpt_" + strings.Repeat("2", 27),
			wantErr: ErrInvalidReceiptIDLength,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseReceiptID(tc.input)
			if tc.wantOK {
				if err != nil {
					t.Errorf("ParseReceiptID(%q) returned error %v; want nil", tc.input, err)
				}
				if string(got) != tc.input {
					t.Errorf("ParseReceiptID(%q) returned %q; want same", tc.input, got)
				}
				return
			}
			if err == nil {
				t.Errorf("ParseReceiptID(%q) returned nil error; want %v", tc.input, tc.wantErr)
				return
			}
			if tc.wantErr != nil && err != tc.wantErr {
				t.Errorf("ParseReceiptID(%q) returned error %v; want %v", tc.input, err, tc.wantErr)
			}
		})
	}
}

func TestNewReceiptID(t *testing.T) {
	t.Parallel()

	t.Run("generates valid ID", func(t *testing.T) {
		t.Parallel()
		id, err := NewReceiptID()
		if err != nil {
			t.Fatalf("NewReceiptID() error = %v", err)
		}
		if _, err := ParseReceiptID(string(id)); err != nil {
			t.Errorf("NewReceiptID() returned ID that fails ParseReceiptID: %v (id=%q)", err, id)
		}
	})

	t.Run("has correct prefix", func(t *testing.T) {
		t.Parallel()
		id, err := NewReceiptID()
		if err != nil {
			t.Fatalf("NewReceiptID() error = %v", err)
		}
		if !strings.HasPrefix(string(id), receiptIDPrefix) {
			t.Errorf("NewReceiptID() = %q; missing prefix %q", id, receiptIDPrefix)
		}
	})

	t.Run("has correct total length", func(t *testing.T) {
		t.Parallel()
		id, err := NewReceiptID()
		if err != nil {
			t.Fatalf("NewReceiptID() error = %v", err)
		}
		if len(string(id)) != receiptIDTotalLen {
			t.Errorf("NewReceiptID() len = %d; want %d (id=%q)", len(string(id)), receiptIDTotalLen, id)
		}
	})

	t.Run("consecutive IDs are monotonically non-decreasing", func(t *testing.T) {
		t.Parallel()
		id1, err := NewReceiptID()
		if err != nil {
			t.Fatalf("NewReceiptID() 1 error = %v", err)
		}
		id2, err := NewReceiptID()
		if err != nil {
			t.Fatalf("NewReceiptID() 2 error = %v", err)
		}
		// UUIDv7 timestamp prefix makes body[0:13] (time bits) monotone
		// when sorted lexicographically.
		if string(id1) > string(id2) {
			t.Errorf("NewReceiptID() ordering violated: id1=%q > id2=%q", id1, id2)
		}
	})
}

func TestReceiptClassExhaustive(t *testing.T) {
	t.Parallel()

	// Verify the enum slice contains exactly the 5 declared constants.
	expectedClasses := []ReceiptClass{
		ClassReferencesChecked,
		ClassImpactChecked,
		ClassContextGathered,
		ClassStructuralOverview,
		ClassDiagnosticsClean,
	}

	if len(receiptClassEnum) != len(expectedClasses) {
		t.Errorf("receiptClassEnum has %d entries; want %d", len(receiptClassEnum), len(expectedClasses))
	}

	for i, want := range expectedClasses {
		if i >= len(receiptClassEnum) {
			t.Errorf("receiptClassEnum missing entry at index %d: want %q", i, want)
			continue
		}
		if receiptClassEnum[i] != want {
			t.Errorf("receiptClassEnum[%d] = %q; want %q", i, receiptClassEnum[i], want)
		}
	}

	// Verify each ReceiptClass has a corresponding concrete scope type.
	// This acts as an exhaustiveness check: if a new class is added without
	// a scope type, this test must be updated.
	classToScope := map[ReceiptClass]ReceiptScope{
		ClassReferencesChecked:  ReferencesCheckedScope{},
		ClassImpactChecked:      ImpactCheckedScope{},
		ClassContextGathered:    ContextGatheredScope{},
		ClassStructuralOverview: StructuralOverviewScope{},
		ClassDiagnosticsClean:   DiagnosticsCleanScope{},
	}

	for _, class := range receiptClassEnum {
		if _, ok := classToScope[class]; !ok {
			t.Errorf("receiptClassEnum contains %q but no corresponding scope type in exhaustiveness map", class)
		}
	}

	// Verify all scope types implement ReceiptScope via compile-time assertions
	// (these are in receipt.go; this test just documents the expectation).
	_ = classToScope
}

func TestReceiptScope_MarkerInterface(t *testing.T) {
	t.Parallel()

	// Each scope type must satisfy ReceiptScope at the value level.
	scopes := []ReceiptScope{
		ReferencesCheckedScope{},
		ImpactCheckedScope{},
		ContextGatheredScope{},
		StructuralOverviewScope{},
		DiagnosticsCleanScope{},
	}

	if len(scopes) != len(receiptClassEnum) {
		t.Errorf("scope count %d != receiptClassEnum count %d; exhaustiveness gap", len(scopes), len(receiptClassEnum))
	}
}
