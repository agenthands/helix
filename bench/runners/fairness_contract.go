// Package runners defines the single compile-time fairness contract that every
// Helix benchmark runner (Phases 80+) loads its effective model configuration
// from. Same-model-same-budget is enforceable only because there is exactly one
// config source: the package-level var DefaultContract.
//
// Fairness invariants enforced here:
//   - ModelID is a DATED snapshot, never a runtime alias (FAIR-02). Aliases
//     resolve to different weights over time, which would silently break
//     reproducibility; the dated pin makes the exact model an audit fact.
//   - Validate() refuses to start if any mode override lacks a WaiverReason
//     (D-10). Every deviation from the shared budget must carry a human-readable
//     justification + maintainer signoff.
//   - DeprecationGate() fails when the pinned snapshot is within 30 days of EOL,
//     using an INJECTED clock so runs are reproducible from --run-id (D-11).
//   - SystemPromptHash is the sha256 of the committed system_prompt.txt; a
//     recompute test (TestSystemPromptHashMatches) turns prompt drift into a
//     hard CI failure (T-75-07).
//
// Security note (T-75-09, accepted): WaiverReason and ApprovedBy are free-text
// display/attestation fields only. They are surfaced in result.v2.json
// fairness.overrides[] for auditability and are NEVER exec'd, shelled, or used
// as a query — there is no injection sink.
package runners

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

// dateLayout is the calendar-date layout used for deprecation dates (ISO 8601).
const dateLayout = "2006-01-02"

// deprecationWindow is the minimum time-to-EOL before the deprecation gate
// fails. A pin within this window of its deprecation date is rejected (D-11).
const deprecationWindow = 30 * 24 * time.Hour

// RetryPolicy pins the shared retry behavior every runner uses.
type RetryPolicy struct {
	MaxRetries int
	BackoffMs  int
}

// CachePolicy pins the shared prompt-cache behavior (e.g. "ephemeral_5m").
type CachePolicy struct {
	Mode string
}

// ModeOverride records a per-mode deviation from the shared budget. Pointer
// fields distinguish "override present" (non-nil) from "inherit the contract
// default" (nil). WaiverReason and ApprovedBy are free-text attestation fields
// (D-10) — emergent reasons, no closed enum — and are display-only (T-75-09).
type ModeOverride struct {
	MaxTokens    *int     // nil = inherit FairnessContract.MaxTokens
	Temperature  *float64 // nil = inherit FairnessContract.Temperature
	WaiverReason string   // free-text justification; MUST be non-empty (D-10)
	ApprovedBy   string   // maintainer signoff (free-text)
}

// FairnessContract is the single source of effective model config for all
// benchmark runners. It is a compile-time literal (DefaultContract), not loaded
// from a file at runtime, so the exact config is a git-committed audit fact.
type FairnessContract struct {
	ModelID          string                  // DATED snapshot (FAIR-02), e.g. "claude-sonnet-4-5-20250929"
	Temperature      float64                 // shared sampling temperature
	MaxTokens        int                     // shared output token budget
	SystemPromptHash string                  // sha256 hex of system_prompt.txt
	Retry            RetryPolicy             // shared retry policy
	Cache            CachePolicy             // shared cache policy
	Overrides        map[string]ModeOverride // keyed by mode name; each MUST carry a WaiverReason
}

// DefaultContract is the pinned, compile-time fairness contract. Every runner
// loads its effective config from this value and calls Validate() at startup,
// fataling on a non-nil return.
//
// ModelID is a DATED snapshot (NOT the bare alias "claude-sonnet-4-6" used by
// internal/eval/judge — that alias is exactly what FAIR-02 forbids here).
// SystemPromptHash is sha256(system_prompt.txt); re-pin it whenever the prompt
// changes or TestSystemPromptHashMatches fails CI.
var DefaultContract = FairnessContract{
	ModelID:          "claude-sonnet-4-5-20250929",
	Temperature:      0.0,
	MaxTokens:        8192,
	SystemPromptHash: "7873294ae45a555ff47a6164d7c8eba2eb5e13cfa163c623b6556e6ad313d636",
	Retry: RetryPolicy{
		MaxRetries: 3,
		BackoffMs:  1000,
	},
	Cache: CachePolicy{
		Mode: "ephemeral_5m",
	},
	// No overrides by default — every competing runner shares the identical
	// budget. Modes that genuinely need a different budget add an entry here
	// with a non-empty WaiverReason + ApprovedBy (Validate enforces this).
	Overrides: map[string]ModeOverride{},
}

// Validate returns a non-nil error if any mode override lacks a WaiverReason
// (D-10). Real runner startup MUST treat a non-nil return as fatal — an
// unjustified deviation from the shared budget would produce an unfair
// benchmark. Returning an error (rather than calling log.Fatal directly) keeps
// the gate unit-testable.
func (c FairnessContract) Validate() error {
	for mode, ov := range c.Overrides {
		if strings.TrimSpace(ov.WaiverReason) == "" {
			return fmt.Errorf("fairness_contract: mode %q overrides the shared budget without a WaiverReason — refusing to run an unfair benchmark", mode)
		}
	}
	return nil
}

// DeprecationGate returns a non-nil error if the pinned model snapshot is within
// 30 days of the given deprecation date. The clock is INJECTED (today) so runs
// are deterministic and reproducible from --run-id (D-11/FAIR-02); never call
// time.Now() inside the gate. The CLI/runner passes time.Now().UTC(); tests pass
// a fixed date. deprecationAt is an ISO-8601 calendar date ("YYYY-MM-DD") sourced
// from the cost-table row (owned by Plan 05) and passed in as a string here so
// this file does not depend on the cost-table type.
func (c FairnessContract) DeprecationGate(today time.Time, deprecationAt string) error {
	dep, err := time.Parse(dateLayout, deprecationAt)
	if err != nil {
		return fmt.Errorf("fairness_contract: cannot parse deprecation date %q for model %s: %w", deprecationAt, c.ModelID, err)
	}
	if dep.Sub(today) < deprecationWindow {
		return fmt.Errorf("fairness gate: model %s is within 30d of deprecation (%s) — re-pin to a fresh dated snapshot before running", c.ModelID, deprecationAt)
	}
	return nil
}

// systemPromptHash recomputes the sha256 hex of the canonical system prompt
// file. Exposed for tooling that re-pins SystemPromptHash after a prompt edit.
func systemPromptHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("fairness_contract: reading system prompt %q: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
