package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// frontmatterBlock splits the leading `---`...`---` YAML block out of the
// embedded SKILL.md. It returns the raw frontmatter text (between the fences)
// and ok=false if the file does not open with a `---` fence followed by a
// closing `---`. It is a minimal splitter — no YAML dependency is added (the
// zero-dep invariant: `git diff go.mod` must stay empty).
func frontmatterBlock(md string) (string, bool) {
	if !strings.HasPrefix(md, "---\n") && !strings.HasPrefix(md, "---\r\n") {
		return "", false
	}
	rest := md
	// Drop the opening fence line.
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[i+1:]
	}
	// Find the closing fence (a line that is exactly "---").
	lines := strings.Split(rest, "\n")
	var fm []string
	for _, ln := range lines {
		if strings.TrimRight(ln, "\r") == "---" {
			return strings.Join(fm, "\n"), true
		}
		fm = append(fm, ln)
	}
	return "", false
}

// frontmatterValue extracts a single-line `key:` value or a YAML block scalar
// (`>-` / `|` style folded/literal) from the frontmatter text. For block
// scalars it concatenates the indented continuation lines. Returns the trimmed
// value and ok.
func frontmatterValue(fm, key string) (string, bool) {
	lines := strings.Split(fm, "\n")
	prefix := key + ":"
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		// Block scalar indicator: gather indented continuation lines.
		if val == ">-" || val == ">" || val == "|" || val == "|-" {
			var parts []string
			for _, cont := range lines[i+1:] {
				if strings.TrimSpace(cont) == "" {
					parts = append(parts, "")
					continue
				}
				// Continuation lines for a block scalar are indented; a
				// non-indented line ends the scalar.
				if cont[0] != ' ' && cont[0] != '\t' {
					break
				}
				parts = append(parts, strings.TrimSpace(cont))
			}
			return strings.TrimSpace(strings.Join(parts, " ")), true
		}
		return val, true
	}
	return "", false
}

// TestSkillEmbedNonEmpty asserts the SKILL.md asset is compiled into the binary
// (embed non-empty). (Task 1, behavior 1.)
func TestSkillEmbedNonEmpty(t *testing.T) {
	if strings.TrimSpace(embeddedSkillBytes()) == "" {
		t.Fatal("embedded SKILL.md is empty; SKILL.md was not embedded")
	}
}

// TestSkillFrontmatterValid asserts the leading frontmatter parses and carries
// name/description/allowed-tools, with name == "helix". (Task 1, behavior 2.)
func TestSkillFrontmatterValid(t *testing.T) {
	fm, ok := frontmatterBlock(embeddedSkillBytes())
	if !ok {
		t.Fatal("SKILL.md has no leading --- frontmatter block")
	}
	name, ok := frontmatterValue(fm, "name")
	if !ok {
		t.Fatal("frontmatter missing 'name' key")
	}
	if name != "helix" {
		t.Errorf("frontmatter name = %q, want %q", name, "helix")
	}
	if _, ok := frontmatterValue(fm, "description"); !ok {
		t.Error("frontmatter missing 'description' key")
	}
	if _, ok := frontmatterValue(fm, "allowed-tools"); !ok {
		t.Error("frontmatter missing 'allowed-tools' key")
	}
}

// TestSkillDescriptionCap asserts the description (+ when_to_use if present) is
// within the Claude Code 1,536-char listing cap — the SKILL-04 idle-cost upper
// bound, asserted with no API key. (Task 1, behavior 3.)
func TestSkillDescriptionCap(t *testing.T) {
	fm, ok := frontmatterBlock(embeddedSkillBytes())
	if !ok {
		t.Fatal("SKILL.md has no leading --- frontmatter block")
	}
	desc, ok := frontmatterValue(fm, "description")
	if !ok {
		t.Fatal("frontmatter missing 'description'")
	}
	total := len(desc)
	if wtu, ok := frontmatterValue(fm, "when_to_use"); ok {
		total += len(wtu)
	}
	const cap1536 = 1536
	if total > cap1536 {
		t.Errorf("description(+when_to_use) = %d bytes, exceeds Claude Code listing cap %d", total, cap1536)
	}
}

// TestSkillDescriptionAccessor asserts the production skillDescription() accessor
// returns the non-empty frontmatter description string (SKILL-04 Task 1, Test 1).
func TestSkillDescriptionAccessor(t *testing.T) {
	desc, err := skillDescription()
	if err != nil {
		t.Fatalf("skillDescription() error: %v", err)
	}
	if strings.TrimSpace(desc) == "" {
		t.Fatal("skillDescription() returned empty string")
	}
}

// TestSkillIdleCostBound is the dependency-free SKILL-04 idle-cost proof: the
// frontmatter description (+ when_to_use if present, via skillDescription) must
// be within the Claude Code 1,536-char listing cap. Runs in the DEFAULT
// (untagged) suite with NO API key — SKILL-04's hermetic non-gated proof.
// (SKILL-04 Task 1, Test 2.)
func TestSkillIdleCostBound(t *testing.T) {
	desc, err := skillDescription()
	if err != nil {
		t.Fatalf("skillDescription() error: %v", err)
	}
	const cap1536 = 1536
	if n := len([]byte(desc)); n > cap1536 {
		t.Errorf("idle skill cost = %d bytes, exceeds Claude Code listing cap %d", n, cap1536)
	}
}

// TestSkillTokenNoteFilled asserts the SKILL-04 token-note carries a real
// digit-bearing idle-cost figure and that the <N>/<M> placeholders reserved in
// 93-01 are gone. Runs in the DEFAULT suite (no API key). (SKILL-04 Task 1, Test 3.)
func TestSkillTokenNoteFilled(t *testing.T) {
	if strings.Contains(embeddedSkillBytes(), "<N>") || strings.Contains(embeddedSkillBytes(), "<M>") {
		t.Error("SKILL.md token-note still contains <N>/<M> placeholders; fill with measured numbers")
	}
	// A real, digit-bearing idle-cost number must be present somewhere in the
	// token-note region (the SKILL-04 acceptance: no placeholder, real number).
	digitRe := regexp.MustCompile(`\d`)
	noteIdx := strings.Index(strings.ToLower(embeddedSkillBytes()), "token note")
	if noteIdx < 0 {
		t.Fatal("SKILL.md missing the 'Token note' line")
	}
	note := embeddedSkillBytes()[noteIdx:]
	if !digitRe.MatchString(note) {
		t.Error("SKILL-04 token-note carries no digit-bearing idle-cost figure")
	}
}

// helixVerbRe matches a backtick-fenced `helix <kebab-verb>` occurrence in the
// SKILL.md body and captures the kebab verb.
var helixVerbRe = regexp.MustCompile("`helix ([a-z][a-z0-9-]+)")

// TestSkillVerbMembershipDrift is the drift gate (Pitfall 5): every helix verb
// cited in the SKILL.md decision table must map (kebab->snake) to a member of
// cli.VerbToolNames(). Runs in the DEFAULT (untagged) suite. (Task 1, behavior 4.)
func TestSkillVerbMembershipDrift(t *testing.T) {
	catalog := make(map[string]bool)
	for _, n := range VerbToolNames() {
		catalog[n] = true
	}

	matches := helixVerbRe.FindAllStringSubmatch(embeddedSkillBytes(), -1)
	if len(matches) == 0 {
		t.Fatal("no `helix <verb>` citations found in SKILL.md decision table")
	}

	seen := make(map[string]bool)
	for _, m := range matches {
		kebab := m[1]
		if seen[kebab] {
			continue
		}
		seen[kebab] = true
		snake := strings.ReplaceAll(kebab, "-", "_")
		if !catalog[snake] {
			t.Errorf("SKILL.md cites `helix %s` (tool %q) which is NOT in cli.VerbToolNames()", kebab, snake)
		}
	}
}

// TestSkillNoVerbCountLiteral asserts the body carries no hardcoded total-verb
// -count literal (VERB-01 lesson: cite by capability group, never a count).
// (Task 1, behavior 5.)
func TestSkillNoVerbCountLiteral(t *testing.T) {
	countRe := regexp.MustCompile(`\b5[03]\s+(verbs|tools)\b`)
	if loc := countRe.FindString(embeddedSkillBytes()); loc != "" {
		t.Errorf("SKILL.md contains a hardcoded verb-count literal %q; cite by capability group instead", loc)
	}
}

// TestSkillTokenNotePresent asserts the SKILL-04 token-note line is present in
// the body (reserves the idle-cost vs preloaded-schema numbers).
func TestSkillTokenNotePresent(t *testing.T) {
	if !strings.Contains(strings.ToLower(embeddedSkillBytes()), "token note") &&
		!strings.Contains(strings.ToLower(embeddedSkillBytes()), "skill-04") {
		t.Error("SKILL.md is missing the SKILL-04 token-note line")
	}
}

// --- Task 2: installSkill atomic contained writer ---

// TestSkillTargetDir asserts skillTargetDir resolves <claudeDir>/skills/helix.
func TestSkillTargetDir(t *testing.T) {
	got := skillTargetDir("/home/u/.claude")
	want := filepath.Join("/home/u/.claude", "skills", "helix")
	if got != want {
		t.Errorf("skillTargetDir = %q, want %q", got, want)
	}
}

// TestInstallSkillWritesContent asserts installSkill writes <dir>/SKILL.md whose
// bytes equal the embedded SKILL.md (modulo a single trailing newline). (Task 2, b1.)
func TestInstallSkillWritesContent(t *testing.T) {
	dir := skillTargetDir(t.TempDir())
	if err := installSkill(dir); err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("reading written SKILL.md: %v", err)
	}
	want := embeddedSkillBytes()
	if !strings.HasSuffix(want, "\n") {
		want += "\n"
	}
	if string(got) != want {
		t.Errorf("written SKILL.md content does not match embedded SKILL.md (modulo trailing newline)")
	}
}

// TestInstallSkillCreatesNestedDir asserts installSkill creates a non-existent
// nested dir. (Task 2, b2.)
func TestInstallSkillCreatesNestedDir(t *testing.T) {
	dir := filepath.Join(skillTargetDir(t.TempDir()), "deep", "nested")
	if err := installSkill(dir); err != nil {
		t.Fatalf("installSkill on nested dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md not created in nested dir: %v", err)
	}
}

// TestInstallSkillIdempotent asserts two calls leave byte-identical content.
// (Task 2, b3.)
func TestInstallSkillIdempotent(t *testing.T) {
	dir := skillTargetDir(t.TempDir())
	if err := installSkill(dir); err != nil {
		t.Fatalf("installSkill (1): %v", err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read after 1st: %v", err)
	}
	if err := installSkill(dir); err != nil {
		t.Fatalf("installSkill (2): %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read after 2nd: %v", err)
	}
	if string(first) != string(second) {
		t.Error("installSkill is not byte-stable across re-runs")
	}
}

// TestInstallSkillNoTempLeftover asserts no .tmp sibling remains after a
// successful atomic write. (Task 2, b4.)
func TestInstallSkillNoTempLeftover(t *testing.T) {
	dir := skillTargetDir(t.TempDir())
	if err := installSkill(dir); err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md.tmp")); err == nil {
		t.Error("leftover SKILL.md.tmp after successful write")
	}
}

// TestInstallSkillContainment asserts a target resolving outside its claudeDir
// root is refused — no escaping write (T-93-01, mirrors readSnippetLine's
// ..-prefix guard). (Task 2, b5.)
func TestInstallSkillContainment(t *testing.T) {
	root := t.TempDir()
	// A crafted "skills/helix" subpath that climbs out of the root via "..".
	escaping := filepath.Join(root, "skills", "helix", "..", "..", "..", "escaped")
	if err := installSkill(escaping); err == nil {
		t.Fatal("installSkill accepted an escaping target; expected refusal")
	}
	// And nothing was written outside the root.
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(root), "escaped", "SKILL.md")); statErr == nil {
		t.Error("installSkill wrote SKILL.md outside the root")
	}
}

// TestInstallSkillWritesBundle asserts installSkill ships BOTH SKILL.md AND
// reference.md into targetDir — each present, non-empty, and ending in a single
// trailing newline (REF-02). This is the multi-file bundle contract.
func TestInstallSkillWritesBundle(t *testing.T) {
	dir := skillTargetDir(t.TempDir())
	if err := installSkill(dir); err != nil {
		t.Fatalf("installSkill: %v", err)
	}
	for _, name := range []string{"SKILL.md", "reference.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("bundle file %s not written: %v", name, err)
		}
		if len(data) == 0 {
			t.Errorf("bundle file %s is empty", name)
		}
		if data[len(data)-1] != '\n' {
			t.Errorf("bundle file %s does not end in a single trailing newline", name)
		}
		if len(data) >= 2 && data[len(data)-2] == '\n' {
			t.Errorf("bundle file %s has a doubled trailing newline", name)
		}
	}
}

// TestEmbeddedSkillBody asserts EmbeddedSkillBody() returns SKILL.md content ONLY
// (no reference.md bleed). The reference carries a generator banner line that the
// terse SKILL.md never does, so its presence in the body would prove a leak.
func TestEmbeddedSkillBody(t *testing.T) {
	body := EmbeddedSkillBody()
	if strings.TrimSpace(body) == "" {
		t.Fatal("EmbeddedSkillBody() is empty")
	}
	// The SKILL.md frontmatter name must be present (proves it IS the skill body).
	if !strings.Contains(body, "name: helix") {
		t.Error("EmbeddedSkillBody() does not look like SKILL.md (missing frontmatter name)")
	}
	// The reference.md generator banner must NOT be present (proves no bundle bleed).
	if strings.Contains(body, "Code generated by helix-refgen") {
		t.Error("EmbeddedSkillBody() leaked reference.md content into the SKILL.md body")
	}
}
