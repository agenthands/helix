//go:build ignore

// gen_corpus.go is the LOCAL-ONLY authoring helper that emits the committed
// drift corpus JSON (testdata/drift/{go,python,rust}.json) from real vendored
// fixture code blocks by applying the deterministic per-tier perturbations in
// the fuzzyrobust package. It is excluded from the package build (//go:build
// ignore). The corpus it writes is the committed ground truth; this script only
// regenerates it. The seeded captured-outcomes are authored to match (Task 3),
// then refreshed by bench/runtime/fuzzy_robust_capture_regen.go.
//
//	go run bench/evaluators/fuzzyrobust/testdata/gen_corpus.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agenthands/helix/bench/evaluators/fuzzyrobust"
)

type driftCase struct {
	ID               string `json:"id"`
	Fixture          string `json:"fixture"`
	ExpectedStrategy string `json:"expected_strategy"`
	Source           string `json:"source"`
	SearchBlock      string `json:"search_block"`
	ExpectedText     string `json:"expected_text"`
	Ambiguous        bool   `json:"ambiguous"`
}

// base is a real vendored fixture code block (the un-drifted text the match is
// expected to land on) plus the fixture ref it came from.
type base struct {
	id      string
	fixture string
	block   string
}

// lang holds the four distinct base blocks per strategy (16 single-site cases)
// for one language, plus a duplicate-block source for the ambiguous case.
type lang struct {
	name string
	// bases has 16 entries: 4 per strategy tier, indexed [tier*4 + n].
	bases [16]base
	// dupBlock is one drift-equivalent block that appears TWICE in dupSource;
	// the ambiguous SEARCH pattern matches BOTH sites.
	dupBlock  string
	dupSource string
	dupFix    string
}

func main() {
	langs := []lang{goLang(), pyLang(), rustLang()}
	tiers := []struct {
		name      string
		transform func(string) string
	}{
		{fuzzyrobust.StrategyExact, fuzzyrobust.PerturbExact},
		{fuzzyrobust.StrategyWhitespace, fuzzyrobust.PerturbWhitespace},
		{fuzzyrobust.StrategyIndentationFlex, fuzzyrobust.PerturbIndent},
		{fuzzyrobust.StrategyEllipsis, fuzzyrobust.PerturbEllipsis},
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "getwd:", err)
		os.Exit(1)
	}
	driftDir := filepath.Join(root, "bench", "evaluators", "fuzzyrobust", "testdata", "drift")
	if err := os.MkdirAll(driftDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	for _, l := range langs {
		var cases []driftCase
		for ti, tier := range tiers {
			for n := 0; n < 4; n++ {
				b := l.bases[ti*4+n]
				if b.block == "" {
					fmt.Fprintf(os.Stderr, "%s: empty base at tier %d n %d\n", l.name, ti, n)
					os.Exit(1)
				}
				cases = append(cases, driftCase{
					ID:               b.id,
					Fixture:          b.fixture,
					ExpectedStrategy: tier.name,
					Source:           b.block,
					SearchBlock:      tier.transform(b.block),
					ExpectedText:     b.block,
				})
			}
		}
		// The duplicate-block ambiguous case: the SEARCH block is a
		// whitespace-drifted copy of dupBlock, which appears twice in dupSource
		// → fuzzy.Match MUST refuse (ErrAmbiguous → ambiguous_match).
		cases = append(cases, driftCase{
			ID:               l.name + "-ambiguous-0",
			Fixture:          l.dupFix,
			ExpectedStrategy: fuzzyrobust.OutcomeAmbiguous,
			Source:           l.dupSource,
			SearchBlock:      fuzzyrobust.PerturbWhitespace(l.dupBlock),
			ExpectedText:     l.dupBlock,
			Ambiguous:        true,
		})
		out, err := json.MarshalIndent(cases, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "marshal:", err)
			os.Exit(1)
		}
		out = append(out, '\n')
		path := filepath.Join(driftDir, l.name+".json")
		if err := os.WriteFile(path, out, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%d cases)\n", path, len(cases))
	}
}

func b(id, fixture, block string) base { return base{id: id, fixture: fixture, block: block} }

func goLang() lang {
	const fix = "go/exercises/practice"
	// 16 distinct real Go fixture blocks (wordy / bowling / two-bucket).
	bk := [16]base{
		b("go-wordy-answer", fix+"/wordy", "func Answer(q string) (a int, ok bool) {\n\tw := strings.Fields(q)\n\tif len(w) < 3 {\n\t\treturn\n\t}\n}"),
		b("go-wordy-trail", fix+"/wordy", "\tw = w[2:]\n\tlast := len(w) - 1\n\twl := w[last]\n\tif wl[len(wl)-1] != '?' {\n\t\treturn\n\t}"),
		b("go-bowling-newgame", fix+"/bowling", "func NewGame() *Game {\n\treturn &Game{}\n}"),
		b("go-bowling-roll", fix+"/bowling", "func (g *Game) Roll(pins int) error {\n\tif pins > pinsPerFrame {\n\t\treturn ErrPinCountExceedsPinsOnTheLane\n\t}\n\treturn nil\n}"),

		b("go-bowling-game", fix+"/bowling", "type Game struct {\n\trolls       [maxRolls]int\n\tnRolls      int\n\tnFrames     int\n\trFrameStart int\n}"),
		b("go-twobucket-bucket", fix+"/two-bucket", "type bucket int\n\nconst (\n\tbOne bucket = iota\n\tbTwo\n)"),
		b("go-twobucket-problem", fix+"/two-bucket", "type problem struct {\n\tcapacity [2]int\n\tgoal     int\n\tstart    bucket\n}"),
		b("go-bowling-consts", fix+"/bowling", "const (\n\tpinsPerFrame      = 10\n\tframesPerGame     = 10\n\tmaxRollsPerFrame  = 2\n)"),

		b("go-wordy-loop", fix+"/wordy", "\tfor i := 1; i < len(w); i++ {\n\t\top := w[i]\n\t\ti++\n\t\tswitch op {\n\t\tcase \"multiplied\", \"divided\":\n\t\t\ti++\n\t\t}\n\t}"),
		b("go-wordy-apply", fix+"/wordy", "\t\tswitch op {\n\t\tcase \"plus\":\n\t\t\ta += x\n\t\tcase \"minus\":\n\t\t\ta -= x\n\t\t}"),
		b("go-bowling-import", fix+"/bowling", "import (\n\t\"errors\"\n\t\"fmt\"\n)"),
		b("go-twobucket-step", fix+"/two-bucket", "const (\n\temptyOne step = iota\n\temptyTwo\n\tfillOne\n\tfillTwo\n)"),

		b("go-wordy-fields", fix+"/wordy", "func Answer(q string) (a int, ok bool) {\n\tw := strings.Fields(q)\n\tlast := len(w) - 1\n\t_ = last\n\treturn 0, true\n}"),
		b("go-bowling-pkg", fix+"/bowling", "package bowling\n\nimport \"errors\"\n\nvar ErrNegativeRollIsInvalid = errors.New(\"Negative roll is invalid\")"),
		b("go-twobucket-pkg", fix+"/two-bucket", "package twobucket\n\nimport (\n\t\"errors\"\n)\n\ntype bucket int"),
		b("go-bowling-score", fix+"/bowling", "func (g *Game) Score() (int, error) {\n\tif g.nFrames < framesPerGame {\n\t\treturn 0, ErrPrematureScore\n\t}\n\treturn 0, nil\n}"),
	}
	return lang{
		name:  "go",
		bases: bk,
		// dupBlock is a shared BODY FRAGMENT that appears at TWO sites in
		// dupSource (inside func a and func helper) — the whitespace-drifted
		// SEARCH pattern matches BOTH → fuzzy.Match MUST refuse (ambiguity).
		dupBlock:  "\tg.nFrames++\n\treturn nil",
		dupSource: "func a(g *Game) error {\n\tg.nFrames++\n\treturn nil\n}\nfunc helper(g *Game) error {\n\tg.nFrames++\n\treturn nil\n}\n",
		dupFix:    fix + "/bowling",
	}
}

func pyLang() lang {
	const fix = "python/exercises/practice"
	bk := [16]base{
		b("py-piglatin-split", fix+"/pig-latin", "def split_initial_consonant_sound(word):\n    return re_cons.match(word).groups()"),
		b("py-piglatin-vowel", fix+"/pig-latin", "def starts_with_vowel_sound(word):\n    return re_vowel.match(word) is not None"),
		b("py-piglatin-translate", fix+"/pig-latin", "def translate(text):\n    words = []\n    for word in text.split():\n        words.append(word)\n    return ' '.join(words)"),
		b("py-piglatin-regex", fix+"/pig-latin", "re_cons = re.compile('^([^aeiou]?qu|[^aeiouy]+|y(?=[aeiou]))([a-z]*)')\nre_vowel = re.compile('^([aeiou]|y[^aeiou]|xr)[a-z]*')"),

		b("py-piglatin-branch", fix+"/pig-latin", "        if starts_with_vowel_sound(word):\n            words.append(word + 'ay')\n        else:\n            head, tail = split_initial_consonant_sound(word)\n            words.append(tail + head + 'ay')"),
		b("py-piglatin-import", fix+"/pig-latin", "import re\n\n\nre_cons = None\nre_vowel = None"),
		b("py-wordy-answer", fix+"/wordy", "def answer(question):\n    words = question.split()\n    if not words:\n        return 0\n    return 5"),
		b("py-wordy-guard", fix+"/wordy", "def answer(question):\n    if not question.startswith('What is'):\n        raise ValueError('unknown')\n    return 0"),

		b("py-piglatin-head", fix+"/pig-latin", "        head, tail = split_initial_consonant_sound(word)\n        words.append(tail + head + 'ay')\n        continue"),
		b("py-piglatin-loop", fix+"/pig-latin", "    for word in text.split():\n        if starts_with_vowel_sound(word):\n            words.append(word + 'ay')"),
		b("py-wordy-compute", fix+"/wordy", "def compute(a, op, b):\n    if op == 'plus':\n        return a + b\n    if op == 'minus':\n        return a - b\n    return 0"),
		b("py-wordy-parse", fix+"/wordy", "def parse(tokens):\n    result = []\n    for t in tokens:\n        result.append(t)\n    return result"),

		b("py-piglatin-defcons", fix+"/pig-latin", "def split_initial_consonant_sound(word):\n    m = re_cons.match(word)\n    return m.groups()"),
		b("py-wordy-tokenize", fix+"/wordy", "def tokenize(q):\n    q = q.rstrip('?')\n    return q.split()"),
		b("py-piglatin-result", fix+"/pig-latin", "    words = []\n    for word in text.split():\n        words.append(word)\n    return ' '.join(words)"),
		b("py-wordy-validate", fix+"/wordy", "def validate(words):\n    if len(words) < 3:\n        raise ValueError('too short')\n    return True"),
	}
	return lang{
		name:  "python",
		bases: bk,
		// Shared body fragment appearing at TWO sites; whitespace-drift refuses.
		dupBlock:  "    words = []\n    words.append(word)\n    return words",
		dupSource: "def a(word):\n    words = []\n    words.append(word)\n    return words\n\n\ndef helper(word):\n    words = []\n    words.append(word)\n    return words\n",
		dupFix:    fix + "/pig-latin",
	}
}

func rustLang() lang {
	const fix = "rust/exercises/practice"
	bk := [16]base{
		b("rust-leap-isleap", fix+"/leap", "pub fn is_leap_year(year: u64) -> bool {\n    year % 4 == 0 && (year % 100 != 0 || year % 400 == 0)\n}"),
		b("rust-piglatin-translateword", fix+"/pig-latin", "pub fn translate_word(word: &str) -> String {\n    if VOWEL.is_match(word) {\n        String::from(word) + \"ay\"\n    } else {\n        String::new()\n    }\n}"),
		b("rust-piglatin-vowel", fix+"/pig-latin", "    static VOWEL: LazyLock<Regex> =\n        LazyLock::new(|| Regex::new(r\"^([aeiou]|y[^aeiou]|xr)[a-z]*\").unwrap());"),
		b("rust-piglatin-cons", fix+"/pig-latin", "    static CONSONANTS: LazyLock<Regex> =\n        LazyLock::new(|| Regex::new(r\"^([^aeiou]?qu|[^aeiou][^aeiouy]*)([a-z]*)\").unwrap());"),

		b("rust-piglatin-else", fix+"/pig-latin", "    } else {\n        let caps = CONSONANTS.captures(word).unwrap();\n        String::from(&caps[2]) + &caps[1] + \"ay\"\n    }"),
		b("rust-piglatin-use", fix+"/pig-latin", "use std::sync::LazyLock;\n\nuse regex::Regex;"),
		b("rust-leap-test", fix+"/leap", "fn check(year: u64) -> bool {\n    is_leap_year(year)\n}"),
		b("rust-piglatin-translate", fix+"/pig-latin", "pub fn translate(text: &str) -> String {\n    text.split_whitespace()\n        .map(translate_word)\n        .collect::<Vec<_>>()\n        .join(\" \")\n}"),

		b("rust-leap-guard", fix+"/leap", "pub fn classify(year: u64) -> &'static str {\n    if is_leap_year(year) {\n        \"leap\"\n    } else {\n        \"common\"\n    }\n}"),
		b("rust-piglatin-match", fix+"/pig-latin", "    if VOWEL.is_match(word) {\n        String::from(word) + \"ay\"\n    } else {\n        String::new()\n    }"),
		b("rust-leap-mod", fix+"/leap", "mod leap {\n    pub fn is_leap_year(year: u64) -> bool {\n        year % 4 == 0\n    }\n}"),
		b("rust-piglatin-caps", fix+"/pig-latin", "        let caps = CONSONANTS.captures(word).unwrap();\n        String::from(&caps[2]) + &caps[1] + \"ay\""),

		b("rust-leap-div", fix+"/leap", "fn divisible(n: u64, d: u64) -> bool {\n    n % d == 0\n}"),
		b("rust-piglatin-collect", fix+"/pig-latin", "    text.split_whitespace()\n        .map(translate_word)\n        .collect::<Vec<_>>()\n        .join(\" \")"),
		b("rust-leap-const", fix+"/leap", "const CYCLE: u64 = 400;\nconst CENTURY: u64 = 100;\nconst LEAP: u64 = 4;"),
		b("rust-piglatin-new", fix+"/pig-latin", "fn empty() -> String {\n    String::new()\n}"),
	}
	return lang{
		name:  "rust",
		bases: bk,
		// Shared body fragment appearing at TWO sites; whitespace-drift refuses.
		dupBlock:  "    let r = is_leap_year(year);\n    r",
		dupSource: "fn a(year: u64) -> bool {\n    let r = is_leap_year(year);\n    r\n}\nfn helper(year: u64) -> bool {\n    let r = is_leap_year(year);\n    r\n}\n",
		dupFix:    fix + "/leap",
	}
}
