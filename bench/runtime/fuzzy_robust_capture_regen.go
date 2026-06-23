//go:build ignore

// fuzzy_robust_capture_regen.go is the LOCAL-ONLY regenerator for the committed
// fuzzy-robustness captured outcomes (FUZZBENCH-01/02). It is invoked by
// `make bench-fuzzy-robust` (NOT compiled into any binary — the //go:build ignore
// tag excludes it from the package build). For each case in the committed drift
// corpus (bench/evaluators/fuzzyrobust/testdata/drift/{go,python,rust}.json) it
// calls internal/fuzzy.Match against the case's Source haystack and records the
// observed {strategy, refused, error_kind, matched_text}, writing the result into
// bench/evaluators/fuzzyrobust/testdata/captured/{go,python,rust}.json.
//
// NON-LEAF: this harness MAY import internal/fuzzy — the leaf NEVER can, since
// internal/fuzzy/match.go:8 imports internal/errors (so internal/fuzzy is not
// stdlib-only — RESEARCH Pitfall 5). The leaf consumes the captured JSON as data
// (committed-vs-committed scoring); the hermetic golden
// (go test ./bench/evaluators/fuzzyrobust/, NO binary) is the authoritative proof.
//
// Outcome mapping (Pitfall 6): a successful match records its Result.Strategy
// label (exact / whitespace_normalized / indentation_flexible);
// errors.Is(err, fuzzy.ErrAmbiguous) → "ambiguous_match" (refused=true);
// errors.Is(err, fuzzy.ErrNoMatch) → "no_match". StrategyFailed ("failed") is
// NEVER emitted as a strategy label.
//
// Usage (from repo root):
//
//	go run bench/runtime/fuzzy_robust_capture_regen.go
//
// FAIL-NOT-SKIP (the Phase 100 determinism contract): this regenerator
// os.Exit(2)s on an empty corpus, an empty language bucket, or an outcome that
// maps to none of the five vocabulary values — a missing capture is a HARD
// failure, never a silent skip. The harness exercises a pure in-process function,
// so HELIX_BIN is not required; the fail-not-skip-on-empty contract still holds.
// Only deterministic bytes are written (no latency / timestamp / absolute path).
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agenthands/helix/internal/fuzzy"
)

// corpusLanguages mirrors the committed testdata/{drift,captured} files (D-03).
var corpusLanguages = []string{"go", "python", "rust"}

// the five-value outcome vocabulary (Pitfall 6). "failed" is never one of these.
const (
	outExact        = "exact"
	outWhitespace   = "whitespace_normalized"
	outIndentFlex   = "indentation_flexible"
	outAmbiguous    = "ambiguous_match"
	outNoMatch      = "no_match"
	expectedEllipse = "ellipsis"
)

// driftCase mirrors the committed drift JSON shape (fuzzyrobust.DriftCase).
type driftCase struct {
	ID               string `json:"id"`
	Fixture          string `json:"fixture"`
	ExpectedStrategy string `json:"expected_strategy"`
	Source           string `json:"source"`
	SearchBlock      string `json:"search_block"`
	ExpectedText     string `json:"expected_text"`
	Ambiguous        bool   `json:"ambiguous"`
}

// capturedOutcome mirrors the committed captured JSON shape
// (fuzzyrobust.CapturedOutcome). Deterministic-only: no latency / timestamp /
// absolute path is ever recorded.
type capturedOutcome struct {
	ID          string `json:"id"`
	Strategy    string `json:"strategy"`
	Refused     bool   `json:"refused"`
	ErrorKind   string `json:"error_kind,omitempty"`
	MatchedText string `json:"matched_text,omitempty"`
}

// isVocabulary reports whether s is one of the five permitted outcome labels.
func isVocabulary(s string) bool {
	switch s {
	case outExact, outWhitespace, outIndentFlex, outAmbiguous, outNoMatch:
		return true
	}
	return false
}

func main() {
	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "getwd:", err)
		os.Exit(1)
	}
	testdata := filepath.Join(repoRoot, "bench", "evaluators", "fuzzyrobust", "testdata")
	capturedDir := filepath.Join(testdata, "captured")
	if err := os.MkdirAll(capturedDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir captured:", err)
		os.Exit(1)
	}

	totalCases := 0
	for _, lang := range corpusLanguages {
		driftPath := filepath.Join(testdata, "drift", lang+".json")
		raw, err := os.ReadFile(driftPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read drift %s: %v\n", driftPath, err)
			os.Exit(2)
		}
		var cases []driftCase
		if err := json.Unmarshal(raw, &cases); err != nil {
			fmt.Fprintf(os.Stderr, "parse drift %s: %v\n", driftPath, err)
			os.Exit(2)
		}
		// FAIL-NOT-SKIP: an empty language bucket is a hard failure.
		if len(cases) == 0 {
			fmt.Fprintf(os.Stderr, "drift %s is empty (fail-not-skip)\n", driftPath)
			os.Exit(2)
		}

		outcomes := make([]capturedOutcome, 0, len(cases))
		for _, c := range cases {
			opts := fuzzy.Options{}
			// The ellipsis tier needs AllowEllipsis + a replacement with a
			// matching "..." segment count; mirror the search block so the
			// segment counts agree (internal/fuzzy validates this).
			if c.ExpectedStrategy == expectedEllipse {
				opts.AllowEllipsis = true
				opts.Replacement = c.SearchBlock
			}

			res, matchErr := fuzzy.Match(c.Source, c.SearchBlock, opts)
			o := capturedOutcome{ID: c.ID}
			switch {
			case matchErr == nil:
				// A successful match records its Result.Strategy label. The four
				// cascade labels are the only success values; "failed" is never
				// emitted (Pitfall 6).
				o.Strategy = string(res.Strategy)
				o.MatchedText = res.MatchedText
			case errors.Is(matchErr, fuzzy.ErrAmbiguous):
				o.Strategy = outAmbiguous
				o.Refused = true
				o.ErrorKind = "ambiguous"
			case errors.Is(matchErr, fuzzy.ErrNoMatch):
				o.Strategy = outNoMatch
				o.ErrorKind = "no_match"
			default:
				// Any other error is an unmappable outcome — a "did it RUN"
				// sentinel violation; fail-not-skip.
				fmt.Fprintf(os.Stderr, "%s case %q: unmappable Match error: %v\n", lang, c.ID, matchErr)
				os.Exit(2)
			}
			// FAIL-NOT-SKIP: the recorded strategy MUST be one of the five
			// vocabulary values (Pitfall 6 — "failed" / "" are not allowed).
			if !isVocabulary(o.Strategy) {
				fmt.Fprintf(os.Stderr, "%s case %q: outcome %q is not in the five-value vocabulary (fail-not-skip)\n",
					lang, c.ID, o.Strategy)
				os.Exit(2)
			}
			outcomes = append(outcomes, o)
			totalCases++
		}

		out, err := json.MarshalIndent(outcomes, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "marshal:", err)
			os.Exit(1)
		}
		out = append(out, '\n')
		capturedPath := filepath.Join(capturedDir, lang+".json")
		if err := os.WriteFile(capturedPath, out, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write captured:", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%d outcomes)\n", capturedPath, len(outcomes))
	}

	// FAIL-NOT-SKIP: an empty corpus overall is a hard failure.
	if totalCases == 0 {
		fmt.Fprintln(os.Stderr, "drift corpus produced zero cases (fail-not-skip)")
		os.Exit(2)
	}
	fmt.Printf("captured %d total drift cases across %d languages\n", totalCases, len(corpusLanguages))
}
