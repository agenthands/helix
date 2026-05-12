package score

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Rules holds the parsed content of an expected_tools.yaml file.
// All fields use the canonical snake_case DSL names from the RESEARCH §"Heuristic Rule DSL".
type Rules struct {
	TaskKind       string          `yaml:"task_kind"`
	ExpectSequence []SequenceRule  `yaml:"expect_sequence"`
	ForbidSequence []SequenceRule  `yaml:"forbid_sequence"`
	ExpectSet      []SetRule       `yaml:"expect_set"`
	ForbidSet      []ForbidSetRule `yaml:"forbid_set"`
	Receipts       []ReceiptRule   `yaml:"receipts"`
}

// SequenceRule describes an ordered subsequence pattern with optional arg matching.
type SequenceRule struct {
	ID      string        `yaml:"id"`
	Score   int           `yaml:"score"`
	Pattern []PatternStep `yaml:"pattern"`
}

// patternStepRaw is the unmarshalling target before regex compilation.
type patternStepRaw struct {
	Tool      string            `yaml:"tool"`
	ArgsMatch map[string]string `yaml:"args_match"`
}

// PatternStep is a single step in a sequence rule after parsing.
// ArgsMatch holds plain substring matchers; ArgsMatchRegex holds compiled
// regexp matchers for keys that end in _regex.
type PatternStep struct {
	Tool           string
	ArgsMatch      map[string]string
	ArgsMatchRegex map[string]*regexp.Regexp
}

// SetRule requires that all listed tools appear at least once (any order).
type SetRule struct {
	ID    string   `yaml:"id"`
	Score int      `yaml:"score"`
	Tools []string `yaml:"tools"`
}

// ForbidSetRule scores negatively when tool_used fires without a required prior tool.
type ForbidSetRule struct {
	ID           string          `yaml:"id"`
	Score        int             `yaml:"score"`
	When         ForbidWhen      `yaml:"when"`
	RequirePrior ForbidRequire   `yaml:"require_prior"`
}

// ForbidWhen specifies which tool triggers the evaluation of this rule.
type ForbidWhen struct {
	ToolUsed string `yaml:"tool_used"`
}

// ForbidRequire specifies which tools must have appeared prior to the trigger.
type ForbidRequire struct {
	AnyOf []string `yaml:"any_of"`
}

// ReceiptRule scores positively when a tool fires with non-empty receipts.
type ReceiptRule struct {
	ID      string         `yaml:"id"`
	Score   int            `yaml:"score"`
	When    ReceiptWhen    `yaml:"when"`
	Require ReceiptRequire `yaml:"require"`
}

// ReceiptWhen specifies the tool that triggers receipt checking.
type ReceiptWhen struct {
	ToolUsed string `yaml:"tool_used"`
}

// ReceiptRequire describes the receipt presence requirement.
type ReceiptRequire struct {
	ReceiptsNonEmpty bool `yaml:"receipts_non_empty"`
}

// rawRules is the intermediate struct for strict YAML decoding before post-processing.
// We separate the raw PatternStep slices so we can compile regexes after decoding.
type rawRules struct {
	TaskKind       string             `yaml:"task_kind"`
	ExpectSequence []rawSequenceRule  `yaml:"expect_sequence"`
	ForbidSequence []rawSequenceRule  `yaml:"forbid_sequence"`
	ExpectSet      []SetRule          `yaml:"expect_set"`
	ForbidSet      []ForbidSetRule    `yaml:"forbid_set"`
	Receipts       []ReceiptRule      `yaml:"receipts"`
}

type rawSequenceRule struct {
	ID      string           `yaml:"id"`
	Score   int              `yaml:"score"`
	Pattern []patternStepRaw `yaml:"pattern"`
}

// LoadRules parses an expected_tools.yaml file using strict KnownFields(true)
// decoding. Unknown YAML keys cause an error. After decoding, it validates
// required fields and compiles regex matchers for args_match keys ending in _regex.
func LoadRules(path string) (Rules, error) {
	f, err := os.Open(path)
	if err != nil {
		return Rules{}, fmt.Errorf("score.LoadRules open %q: %w", path, err)
	}
	defer f.Close()

	var raw rawRules
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return Rules{}, fmt.Errorf("score.LoadRules decode %q: %w", path, err)
	}

	r := Rules{
		TaskKind:  raw.TaskKind,
		ExpectSet: raw.ExpectSet,
		ForbidSet: raw.ForbidSet,
		Receipts:  raw.Receipts,
	}

	// Validate and compile ExpectSequence rules.
	for i, rsr := range raw.ExpectSequence {
		sr, err := compileSequenceRule(rsr, fmt.Sprintf("expect_sequence[%d]", i))
		if err != nil {
			return Rules{}, fmt.Errorf("score.LoadRules %q: %w", path, err)
		}
		r.ExpectSequence = append(r.ExpectSequence, sr)
	}

	// Validate and compile ForbidSequence rules.
	for i, rsr := range raw.ForbidSequence {
		sr, err := compileSequenceRule(rsr, fmt.Sprintf("forbid_sequence[%d]", i))
		if err != nil {
			return Rules{}, fmt.Errorf("score.LoadRules %q: %w", path, err)
		}
		r.ForbidSequence = append(r.ForbidSequence, sr)
	}

	// Validate ExpectSet rules.
	for i, es := range r.ExpectSet {
		if es.ID == "" {
			return Rules{}, fmt.Errorf("score.LoadRules %q: expect_set[%d].id is required", path, i)
		}
	}

	// Validate ForbidSet rules.
	for i, fs := range r.ForbidSet {
		if fs.ID == "" {
			return Rules{}, fmt.Errorf("score.LoadRules %q: forbid_set[%d].id is required", path, i)
		}
		if fs.When.ToolUsed == "" {
			return Rules{}, fmt.Errorf("score.LoadRules %q: forbid_set[%d].when.tool_used is required", path, i)
		}
	}

	// Validate Receipt rules.
	for i, rec := range r.Receipts {
		if rec.ID == "" {
			return Rules{}, fmt.Errorf("score.LoadRules %q: receipts[%d].id is required", path, i)
		}
		if rec.When.ToolUsed == "" {
			return Rules{}, fmt.Errorf("score.LoadRules %q: receipts[%d].when.tool_used is required", path, i)
		}
	}

	return r, nil
}

// compileSequenceRule converts a rawSequenceRule to SequenceRule, validating
// required fields and compiling _regex args_match entries.
func compileSequenceRule(rsr rawSequenceRule, path string) (SequenceRule, error) {
	if rsr.ID == "" {
		return SequenceRule{}, fmt.Errorf("%s.id is required", path)
	}
	sr := SequenceRule{
		ID:    rsr.ID,
		Score: rsr.Score,
	}
	for j, step := range rsr.Pattern {
		if step.Tool == "" {
			return SequenceRule{}, fmt.Errorf("%s.pattern[%d].tool is required", path, j)
		}
		ps := PatternStep{
			Tool:           step.Tool,
			ArgsMatch:      make(map[string]string),
			ArgsMatchRegex: make(map[string]*regexp.Regexp),
		}
		for k, v := range step.ArgsMatch {
			if strings.HasSuffix(k, "_regex") {
				re, err := regexp.Compile(v)
				if err != nil {
					return SequenceRule{}, fmt.Errorf("%s.pattern[%d].args_match.%s: invalid regex %q: %w", path, j, k, v, err)
				}
				ps.ArgsMatchRegex[k] = re
			} else {
				ps.ArgsMatch[k] = v
			}
		}
		sr.Pattern = append(sr.Pattern, ps)
	}
	return sr, nil
}

// ValidateCorpus walks corpusDir looking for expected_tools.yaml files and
// returns a slice of errors for each file that fails to parse. An empty slice
// means all files are valid.
func ValidateCorpus(corpusDir string) []error {
	pattern := filepath.Join(corpusDir, "*", "expected_tools.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return []error{fmt.Errorf("ValidateCorpus glob %q: %w", pattern, err)}
	}

	var errs []error
	for _, path := range matches {
		if _, err := LoadRules(path); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// ExpectedToolNames returns the deduplicated set of tool names referenced by
// ExpectSequence patterns and ExpectSet rules. Used by F-10 context-recall
// computation as the ground-truth tool set.
//
// Receipt rules' when.tool_used is NOT included — receipts are a downstream
// effect, not a tool-invocation expectation in the precision/recall sense.
func (r Rules) ExpectedToolNames() []string {
	seen := make(map[string]struct{})
	for _, seq := range r.ExpectSequence {
		for _, step := range seq.Pattern {
			if step.Tool != "" {
				seen[step.Tool] = struct{}{}
			}
		}
	}
	for _, set := range r.ExpectSet {
		for _, tool := range set.Tools {
			if tool != "" {
				seen[tool] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	return out
}

// IsRelevant reports whether tool is classified as on-task by the rule set.
// A tool is on-task iff it appears in ExpectedToolNames(). Used by F-10
// context-precision computation.
//
// Tools not mentioned in the rules are classified as not-relevant. The
// ForbidSet / ForbidSequence rules describe penalties applied by score.Apply
// but are not consulted here — Precision is the fraction of calls that match
// any expected tool, irrespective of separate forbidden-tool penalties.
func IsRelevant(tool string, rules Rules) bool {
	if tool == "" {
		return false
	}
	for _, seq := range rules.ExpectSequence {
		for _, step := range seq.Pattern {
			if step.Tool == tool {
				return true
			}
		}
	}
	for _, set := range rules.ExpectSet {
		for _, t := range set.Tools {
			if t == tool {
				return true
			}
		}
	}
	return false
}
