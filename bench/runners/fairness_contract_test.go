package runners

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestEmptyWaiverReasonFatal asserts the D-10 loader gate: any mode override with
// an empty (or whitespace-only) WaiverReason makes Validate return a non-nil error;
// a populated WaiverReason validates clean.
func TestEmptyWaiverReasonFatal(t *testing.T) {
	tokens := 4096
	cases := []struct {
		name      string
		overrides map[string]ModeOverride
		wantErr   bool
	}{
		{
			name:      "no overrides validates",
			overrides: nil,
			wantErr:   false,
		},
		{
			name: "populated WaiverReason validates",
			overrides: map[string]ModeOverride{
				"no_semantic": {
					MaxTokens:    &tokens,
					WaiverReason: "ablation mode needs a smaller budget to bound retries",
					ApprovedBy:   "maintainer",
				},
			},
			wantErr: false,
		},
		{
			name: "empty WaiverReason fatals",
			overrides: map[string]ModeOverride{
				"no_semantic": {
					MaxTokens:  &tokens,
					ApprovedBy: "maintainer",
				},
			},
			wantErr: true,
		},
		{
			name: "whitespace-only WaiverReason fatals",
			overrides: map[string]ModeOverride{
				"no_semantic": {
					MaxTokens:    &tokens,
					WaiverReason: "   \t ",
					ApprovedBy:   "maintainer",
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultContract
			c.Overrides = tc.overrides
			err := c.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate() = nil, want non-nil error for %q", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil for %q", err, tc.name)
			}
		})
	}
}

// TestDeprecationGate asserts the D-11/FAIR-02 injected-clock 30-day gate: a snapshot
// 60 days from EOL passes; one 20 days out fails with an error naming the model+date.
// The clock is injected so the test is deterministic.
func TestDeprecationGate(t *testing.T) {
	today := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name          string
		deprecationAt string
		wantErr       bool
	}{
		{name: "60 days out passes", deprecationAt: "2026-08-14", wantErr: false},
		{name: "exactly 30 days out passes", deprecationAt: "2026-07-15", wantErr: false},
		{name: "20 days out fails", deprecationAt: "2026-07-05", wantErr: true},
		{name: "already past fails", deprecationAt: "2026-05-01", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := DefaultContract.DeprecationGate(today, tc.deprecationAt)
			if tc.wantErr && err == nil {
				t.Fatalf("DeprecationGate(%s) = nil, want non-nil error", tc.deprecationAt)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("DeprecationGate(%s) = %v, want nil", tc.deprecationAt, err)
			}
			if tc.wantErr {
				if !strings.Contains(err.Error(), DefaultContract.ModelID) {
					t.Errorf("error %q does not name the model %q", err, DefaultContract.ModelID)
				}
				if !strings.Contains(err.Error(), tc.deprecationAt) {
					t.Errorf("error %q does not name the date %q", err, tc.deprecationAt)
				}
			}
		})
	}
}

// TestDeprecationGateRejectsBadDate ensures a malformed date string is an error,
// not a silent pass.
func TestDeprecationGateRejectsBadDate(t *testing.T) {
	today := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	if err := DefaultContract.DeprecationGate(today, "not-a-date"); err == nil {
		t.Fatal("DeprecationGate with malformed date = nil, want parse error")
	}
}

// TestModelIDIsDatedSnapshot asserts FAIR-02: DefaultContract.ModelID is a DATED
// snapshot (8-digit date suffix), never a bare runtime alias.
func TestModelIDIsDatedSnapshot(t *testing.T) {
	datedSuffix := regexp.MustCompile(`-\d{8}$`)
	if !datedSuffix.MatchString(DefaultContract.ModelID) {
		t.Fatalf("ModelID %q is not a dated snapshot (want trailing -YYYYMMDD)", DefaultContract.ModelID)
	}
	if DefaultContract.ModelID == "claude-sonnet-4-6" {
		t.Fatalf("ModelID is the forbidden bare alias %q", DefaultContract.ModelID)
	}
}

// TestSystemPromptHashMatches asserts the T-75-07 drift guard: the pinned
// SystemPromptHash equals sha256(system_prompt.txt).
func TestSystemPromptHashMatches(t *testing.T) {
	data, err := os.ReadFile("system_prompt.txt")
	if err != nil {
		t.Fatalf("reading system_prompt.txt: %v", err)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != DefaultContract.SystemPromptHash {
		t.Fatalf("system_prompt.txt sha256 = %s, but DefaultContract.SystemPromptHash = %s (prompt drift — re-pin the hash)", got, DefaultContract.SystemPromptHash)
	}
}

// TestDefaultContractValidates is a sanity check: the committed DefaultContract
// must itself pass Validate (so real runner startup does not fatal).
func TestDefaultContractValidates(t *testing.T) {
	if err := DefaultContract.Validate(); err != nil {
		t.Fatalf("committed DefaultContract fails Validate(): %v", err)
	}
}
