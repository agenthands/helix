package fuzzyrobust

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DriftCase is one committed drift-corpus entry. It is authored ONCE from a real
// vendored fixture code block by applying a deterministic per-tier perturbation;
// the ExpectedStrategy IS the perturbation tier (structural derivation, never
// read from observed tool behavior — D-05).
type DriftCase struct {
	// ID is a stable, unique case identifier within its language bucket (e.g.
	// "wordy-ws-0"). It is the join key between the drift case and its captured
	// outcome.
	ID string `json:"id"`
	// Fixture is the source fixture ref the base block was drawn from (e.g.
	// "go/exercises/practice/wordy"), for provenance/auditing.
	Fixture string `json:"fixture"`
	// ExpectedStrategy is the structurally-derived expected outcome label: one
	// of the four cascade tiers (exact / whitespace_normalized /
	// indentation_flexible / ellipsis) for a single-site case, or the refusal
	// outcome (ambiguous_match) for a duplicate-block case. NEVER read from
	// observed behavior (D-05).
	ExpectedStrategy string `json:"expected_strategy"`
	// Source is the haystack the capture harness runs fuzzy.Match against. For a
	// single-site case it is the un-drifted base block (so the perturbed
	// SearchBlock re-matches it through the expected tier). For the duplicate-
	// block ambiguous case it is a source containing TWO drift-equivalent sites
	// (so the SearchBlock matches BOTH and Match refuses).
	Source string `json:"source"`
	// SearchBlock is the drifted SEARCH block fed to fuzzy.Match by the non-leaf
	// capture harness (perturbation of the base fixture block).
	SearchBlock string `json:"search_block"`
	// ExpectedText is the text the match is expected to land on (the un-drifted
	// base block, or the matched region) — the editsim.ES similarity gold.
	ExpectedText string `json:"expected_text"`
	// Ambiguous marks the duplicate-block must-refuse case (D-06): a fixture with
	// two drift-equivalent blocks plus a pattern matching BOTH sites, which
	// fuzzy.Match MUST refuse (ErrAmbiguous → ambiguous_match), NOT no_match and
	// NOT a successful match.
	Ambiguous bool `json:"ambiguous"`
}

// DriftCorpus is the committed drift corpus for one language.
type DriftCorpus struct {
	// Language is the per-track language the corpus was loaded for.
	Language string
	// Cases are the drift cases in committed (source) order. The metrics/tests
	// index by ID; iteration order is for readability only.
	Cases []DriftCase
}

// CapturedOutcome is the committed observed outcome for one drift case, recorded
// by the //go:build ignore non-leaf harness (bench/runtime/
// fuzzy_robust_capture_regen.go) which calls internal/fuzzy.Match. It maps a
// successful match to its Result strategy label (exact / whitespace_normalized /
// indentation_flexible), ErrAmbiguous → ambiguous_match (Refused=true),
// ErrNoMatch → no_match. "failed" is NEVER a strategy label (Pitfall 6).
type CapturedOutcome struct {
	// ID joins to the DriftCase.ID.
	ID string `json:"id"`
	// Strategy is the observed outcome label, one of the five-value vocabulary:
	// {exact, whitespace_normalized, indentation_flexible, ambiguous_match,
	// no_match}.
	Strategy string `json:"strategy"`
	// Refused is true iff the outcome was the ambiguity refusal (ErrAmbiguous).
	Refused bool `json:"refused"`
	// ErrorKind records the sentinel kind for the two error outcomes
	// ("ambiguous" / "no_match"), empty for a successful match. Deterministic;
	// no latency / timestamp / absolute path is ever recorded.
	ErrorKind string `json:"error_kind,omitempty"`
	// MatchedText is the region fuzzy.Match landed on for a successful match
	// (the editsim.ES similarity input), empty for a refusal/no-match.
	MatchedText string `json:"matched_text,omitempty"`
}

// CapturedOutcomes is the committed captured outcomes for one language.
type CapturedOutcomes struct {
	// Language is the per-track language the outcomes were loaded for.
	Language string
	// Outcomes maps DriftCase.ID -> the observed outcome.
	Outcomes map[string]CapturedOutcome
}

// validatePathSegment rejects a name that could escape a join root once it
// becomes a path segment. It is a clone of bench/datasets/aider-polyglot/
// loader.go:81 (validatePathSegment) kept LOCAL to this stdlib-only leaf so the
// leaf carries NO bench/datasets import (the boundary TestLeafImports enforces).
// It rejects "", anything filepath.Clean rewrites ("..", "a//b"), any embedded
// separator, and a leading dot. MUST run BEFORE any filepath.Join (T-102-05, V5).
func validatePathSegment(name, kind string) error {
	if name == "" {
		return fmt.Errorf("fuzzyrobust: %s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("fuzzyrobust: %s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

// LoadDrift reads dir/drift/<language>.json into a DriftCorpus. The language is
// validated as a path segment BEFORE any filepath.Join (T-102-05) so a traversal
// segment is rejected up front. No network, no kernel import: stdlib only.
func LoadDrift(dir, language string) (DriftCorpus, error) {
	if err := validatePathSegment(language, "language"); err != nil {
		return DriftCorpus{}, err
	}
	path := filepath.Join(dir, "drift", language+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return DriftCorpus{}, fmt.Errorf("fuzzyrobust: read drift %s: %w", path, err)
	}
	var cases []DriftCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		return DriftCorpus{}, fmt.Errorf("fuzzyrobust: parse drift %s: %w", path, err)
	}
	if len(cases) == 0 {
		return DriftCorpus{}, fmt.Errorf("fuzzyrobust: drift %s is empty (fail-closed)", path)
	}
	return DriftCorpus{Language: language, Cases: cases}, nil
}

// LoadCaptured reads dir/captured/<language>.json into CapturedOutcomes. The
// language is validated as a path segment BEFORE any filepath.Join (T-102-05).
// No network, no kernel import: stdlib only.
func LoadCaptured(dir, language string) (CapturedOutcomes, error) {
	if err := validatePathSegment(language, "language"); err != nil {
		return CapturedOutcomes{}, err
	}
	path := filepath.Join(dir, "captured", language+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return CapturedOutcomes{}, fmt.Errorf("fuzzyrobust: read captured %s: %w", path, err)
	}
	var list []CapturedOutcome
	if err := json.Unmarshal(raw, &list); err != nil {
		return CapturedOutcomes{}, fmt.Errorf("fuzzyrobust: parse captured %s: %w", path, err)
	}
	if len(list) == 0 {
		return CapturedOutcomes{}, fmt.Errorf("fuzzyrobust: captured %s is empty (fail-closed)", path)
	}
	out := make(map[string]CapturedOutcome, len(list))
	for _, o := range list {
		out[o.ID] = o
	}
	return CapturedOutcomes{Language: language, Outcomes: out}, nil
}
