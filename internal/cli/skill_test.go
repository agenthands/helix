package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// --- Phase 105: anti-vacuity decision-matrix guards (SKILL-01/02/03) ---
//
// Architecture mirrors reference_contract_test.go's confirm-and-seal +
// revert-and-fail pattern: each checker is a factored PURE function run on BOTH
// the real embedded SKILL.md bytes (positive arm) AND a synthetic fabricated
// table parsed by the SAME parseMatrixRows (negative / anti-vacuity arm). A
// green-path-only guard is a defect in this milestone, so every checker carries
// a fabricated offender that MUST be reported.

// matrixRow holds the parsed cells of one `## Decision matrix` data row. A row is
// `| Capability | Question | Use this | Not this |` after the Phase 105 rewrite,
// but parseMatrixRows is shape-tolerant: it keeps the FIRST cell as capability
// (empty if only 3 columns), the second-to-last as useThis, and the last as
// notThis, so it runs identically on the pre-rewrite 3-column matrix and the
// post-rewrite 4-column matrix.
type matrixRow struct {
	capability string
	question   string
	useThis    string
	notThis    string
}

// parseMatrixRows locates `## Decision matrix`, takes the `|`-leading lines after
// the header+separator rows, splits on `|`, trims each cell, and drops the outer
// empty cells produced by the leading/trailing pipes. It returns the data rows.
// Stdlib only — no YAML/markdown dependency (zero-dep invariant). It runs
// identically on the real embeddedSkillBytes() and on synthetic fabricated
// tables (the confirm-and-seal discriminator pattern).
func parseMatrixRows(body string) []matrixRow {
	idx := strings.Index(body, "## Decision matrix")
	if idx < 0 {
		return nil
	}
	region := body[idx:]
	var rows []matrixRow
	for _, ln := range strings.Split(region, "\n") {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, "|") {
			continue
		}
		// Split on pipes and drop the leading/trailing empty cells.
		raw := strings.Split(t, "|")
		var cells []string
		for _, c := range raw {
			cells = append(cells, strings.TrimSpace(c))
		}
		// Trim the outer empties from the leading/trailing pipes.
		if len(cells) > 0 && cells[0] == "" {
			cells = cells[1:]
		}
		if len(cells) > 0 && cells[len(cells)-1] == "" {
			cells = cells[:len(cells)-1]
		}
		if len(cells) < 2 {
			continue
		}
		// Skip the separator row (all cells are runs of '-' / ':' / spaces).
		isSep := true
		for _, c := range cells {
			if strings.Trim(c, "-: ") != "" {
				isSep = false
				break
			}
		}
		if isSep {
			continue
		}
		// Skip the header row (contains the literal "Question" / "Use this" /
		// "Not this" labels rather than data).
		joined := strings.ToLower(strings.Join(cells, "|"))
		if strings.Contains(joined, "use this") && strings.Contains(joined, "not this") {
			continue
		}
		var row matrixRow
		row.notThis = cells[len(cells)-1]
		row.useThis = cells[len(cells)-2]
		switch len(cells) {
		case 2:
			// Legacy/synthetic 2-column "| useThis | notThis |".
		case 3:
			// "| Question | Use this | Not this |" (pre-rewrite shape).
			row.question = cells[0]
		default:
			// 4+ columns: "| Capability | Question | Use this | Not this |".
			row.capability = cells[0]
			row.question = cells[1]
		}
		rows = append(rows, row)
	}
	return rows
}

// rowVerbs extracts every `helix <kebab>` verb cited in a `Use this` cell and
// returns the snake-case tool names (kebab->snake), reusing helixVerbRe.
func rowVerbs(useThis string) []string {
	var out []string
	for _, m := range helixVerbRe.FindAllStringSubmatch(useThis, -1) {
		out = append(out, strings.ReplaceAll(m[1], "-", "_"))
	}
	return out
}

// querySet / actionSet encode the §D classification of all 50 frozen verbs
// (SKILL-ISSUE.md Appendix, snake-case keys). QUERY verbs read state; ACTION
// verbs mutate state. The maps are keyed to VerbToolNames() and asserted
// complete by TestSkillMatrixClassificationComplete (every frozen verb appears
// in exactly one set), so the classification cannot silently fall behind a
// future verb add.
var querySet = map[string]bool{
	"go_to_definition":          true,
	"find_references":           true,
	"find_implementations":      true,
	"get_type_hierarchy":        true,
	"get_hover_info":            true,
	"get_symbol_overview":       true,
	"analyze_blast_radius":      true,
	"search_symbols":            true,
	"get_call_hierarchy":        true,
	"find_files":                true,
	"list_directory":            true,
	"read_file":                 true,
	"search_in_files":           true,
	"get_diagnostics":           true,
	"trace_data_flow":          true,
	"get_code_actions":          true,
	"get_repo_map":              true,
	"get_context":               true,
	"get_semantic_context":      true,
	"get_cluster_map":           true,
	"explain_cluster":           true,
	"explain_symbol_deep":       true,
	"find_related_symbols":      true,
	"get_change_impact_graph":   true,
	"get_semantic_graph_status": true,
	"validate_graph_edge":       true,
	"read_memory":               true,
	"list_memories":             true,
	"search_memories":           true,
	"onboard_project":           true,
	"get_token_budget":          true,
	"get_health":                true,
	"get_tool_help":             true,
}

var actionSet = map[string]bool{
	"create_file":                  true,
	"replace_in_file":              true,
	"rename_symbol":                true,
	"replace_symbol_body":          true,
	"fuzzy_edit":                   true,
	"insert_before_symbol":         true,
	"insert_after_symbol":          true,
	"safe_delete_symbol":           true,
	"verify_edit":                  true,
	"format_code":                  true,
	"index_semantic_graph":         true,
	"refresh_semantic_graph":       true,
	"write_memory":                 true,
	"edit_memory":                  true,
	"rename_memory":                true,
	"delete_memory":                true,
	"prepare_for_new_conversation": true,
	"switch_mode":                  true,
}

// graphReaderVerbs are the 8 indexed-graph READER verbs that require a built
// semantic graph (`helix index-semantic-graph`) first. The two BUILDERS
// (index_semantic_graph / refresh_semantic_graph) are NOT in this set — they
// create the prerequisite, they do not consume it.
var graphReaderVerbs = map[string]bool{
	"get_semantic_graph_status": true,
	"explain_cluster":           true,
	"explain_symbol_deep":       true,
	"get_change_impact_graph":   true,
	"validate_graph_edge":       true,
	"find_related_symbols":      true,
	"get_semantic_context":      true,
	"get_cluster_map":           true,
	"trace_data_flow":      true,
}

// matrixRowMixesQueryAction (SKILL-01) returns true iff the row's `Use this`
// cell cites BOTH a QUERY verb and an ACTION verb — the semantic mixing the
// rewrite must eliminate (an agent reading a mixed row cannot tell whether the
// row routes to a reader or a mutator).
func matrixRowMixesQueryAction(row matrixRow) bool {
	var hasQuery, hasAction bool
	for _, v := range rowVerbs(row.useThis) {
		if querySet[v] {
			hasQuery = true
		}
		if actionSet[v] {
			hasAction = true
		}
	}
	return hasQuery && hasAction
}

// matrixRowEmptyNotThis (SKILL-02) returns true iff the row's `Not this` cell is
// an em-dash placeholder or empty after trim — i.e. the row names no concrete
// shell/manual fallback to displace.
func matrixRowEmptyNotThis(row matrixRow) bool {
	// emDash is constructed from its rune so the literal placeholder character
	// never appears as a source token the SKILL-02 negative-grep could scan.
	emDash := string(rune(0x2014))
	c := strings.TrimSpace(row.notThis)
	return c == "" || c == emDash
}

// matrixRowMissingGraphPrereq (SKILL-03) returns true iff a row that cites any
// of the 8 graph-reader verbs carries NO per-row prerequisite marker: neither
// the `†` dagger nor an inline `index-semantic-graph` mention. A legend line
// alone is insufficient — the marker must be ON the row so the agent sees the
// prerequisite at the point of routing.
func matrixRowMissingGraphPrereq(row matrixRow) bool {
	citesGraphReader := false
	for _, v := range rowVerbs(row.useThis) {
		if graphReaderVerbs[v] {
			citesGraphReader = true
			break
		}
	}
	if !citesGraphReader {
		return false
	}
	rowText := row.capability + " " + row.question + " " + row.useThis + " " + row.notThis
	hasMarker := strings.Contains(rowText, "†") ||
		strings.Contains(rowText, "index-semantic-graph")
	return !hasMarker
}

// TestSkillMatrixClassificationComplete is the drift guard on the classification
// ITSELF: every frozen verb in VerbToolNames() must appear in EXACTLY one of
// querySet/actionSet (none unclassified, none double-classified). This keys the
// QUERY/ACTION maps to VerbToolNames() (never a hardcoded list) so the maps
// cannot silently fall behind a future verb add. (SKILL-01/02/03 substrate.)
func TestSkillMatrixClassificationComplete(t *testing.T) {
	names := VerbToolNames()
	require.NotEmpty(t, names, "VerbToolNames() must be non-empty")
	for _, n := range names {
		q := querySet[n]
		a := actionSet[n]
		assert.Falsef(t, q && a, "verb %q is double-classified (in BOTH querySet and actionSet)", n)
		assert.Truef(t, q || a, "verb %q is unclassified (in NEITHER querySet nor actionSet)", n)
	}
	// And no stray classification keys that are not real frozen verbs.
	catalog := make(map[string]bool, len(names))
	for _, n := range names {
		catalog[n] = true
	}
	for k := range querySet {
		assert.Truef(t, catalog[k], "querySet has a non-frozen key %q (not in VerbToolNames())", k)
	}
	for k := range actionSet {
		assert.Truef(t, catalog[k], "actionSet has a non-frozen key %q (not in VerbToolNames())", k)
	}
	// Total classified count equals the frozen verb count (every verb once).
	assert.Equal(t, len(names), len(querySet)+len(actionSet),
		"classified count (querySet+actionSet) must equal len(VerbToolNames())")
}

// TestSkillMatrixNoQueryActionMix is SKILL-01: no real matrix row's `Use this`
// cell mixes a QUERY and an ACTION verb (positive arm), AND a fabricated mixed
// row IS reported by the same checker (anti-vacuity negative arm).
func TestSkillMatrixNoQueryActionMix(t *testing.T) {
	rows := parseMatrixRows(embeddedSkillBytes())
	require.NotEmpty(t, rows, "parseMatrixRows found no data rows in the embedded matrix")

	// Positive arm: every real row is single-purpose.
	for _, row := range rows {
		assert.Falsef(t, matrixRowMixesQueryAction(row),
			"matrix row mixes a QUERY and an ACTION verb in its `Use this` cell: %q", row.useThis)
	}

	// Negative (anti-vacuity) arm: a fabricated row whose `Use this` cites a
	// known QUERY+ACTION mix (read-memory QUERY + write-memory ACTION) MUST be
	// reported by the SAME checker, parsed by the SAME parseMatrixRows.
	synthetic := "## Decision matrix\n\n| Capability | Question | Use this | Not this |\n|---|---|---|---|\n" +
		"| Memory | read or write memory | `helix read-memory` / `helix write-memory` | scratch files |\n"
	srows := parseMatrixRows(synthetic)
	require.Len(t, srows, 1, "synthetic table must parse to exactly one data row")
	assert.Truef(t, matrixRowMixesQueryAction(srows[0]),
		"SKILL-01 checker is vacuous: it did NOT reject a fabricated QUERY+ACTION mixed row")
}

// TestSkillMatrixNoEmptyNotThis is SKILL-02: no real matrix row's `Not this`
// cell is an em-dash/empty placeholder (positive arm), AND a fabricated
// placeholder row IS reported by the same checker (anti-vacuity negative arm).
func TestSkillMatrixNoEmptyNotThis(t *testing.T) {
	rows := parseMatrixRows(embeddedSkillBytes())
	require.NotEmpty(t, rows, "parseMatrixRows found no data rows in the embedded matrix")

	// Positive arm: every real row names a concrete fallback.
	for _, row := range rows {
		assert.Falsef(t, matrixRowEmptyNotThis(row),
			"matrix row has an empty/placeholder `Not this` cell (no displacement target): useThis=%q", row.useThis)
	}

	// Negative (anti-vacuity) arm: a fabricated row whose `Not this` is the
	// em-dash placeholder MUST be reported. The placeholder char lives ONLY in
	// this synthetic string, built from its rune so it is not a scannable token.
	emDash := string(rune(0x2014))
	synthetic := "## Decision matrix\n\n| Capability | Question | Use this | Not this |\n|---|---|---|---|\n" +
		"| Memory | search memory | `helix search-memories` | " + emDash + " |\n"
	srows := parseMatrixRows(synthetic)
	require.Len(t, srows, 1, "synthetic table must parse to exactly one data row")
	assert.Truef(t, matrixRowEmptyNotThis(srows[0]),
		"SKILL-02 checker is vacuous: it did NOT reject a fabricated placeholder `Not this` row")
}

// TestSkillMatrixGraphPrereq is SKILL-03: every real graph-reader row carries the
// per-row prerequisite marker AND the `## Decision matrix` legend cites
// `helix index-semantic-graph` (positive arm); a fabricated graph-reader row
// missing the marker IS reported by the same checker (anti-vacuity negative arm).
func TestSkillMatrixGraphPrereq(t *testing.T) {
	body := embeddedSkillBytes()
	rows := parseMatrixRows(body)
	require.NotEmpty(t, rows, "parseMatrixRows found no data rows in the embedded matrix")

	// Positive arm: every real graph-reader row carries the marker.
	for _, row := range rows {
		assert.Falsef(t, matrixRowMissingGraphPrereq(row),
			"graph-reader matrix row missing its index-semantic-graph prerequisite marker: useThis=%q", row.useThis)
	}

	// Positive arm (legend): the matrix carries a legend line citing the builder
	// verb backtick-fenced, so the drift gate sees a real verb and the agent
	// learns the prerequisite by exact kebab name.
	idx := strings.Index(body, "## Decision matrix")
	require.GreaterOrEqual(t, idx, 0, "## Decision matrix heading must be present")
	// The legend line lives between the heading and the table; the table starts
	// at the first `|`-leading line.
	region := body[idx:]
	if tbl := strings.Index(region, "\n|"); tbl > 0 {
		legendRegion := region[:tbl]
		assert.Containsf(t, legendRegion, "`helix index-semantic-graph`",
			"SKILL-03 legend must cite `helix index-semantic-graph` between the heading and the table")
	} else {
		t.Fatal("could not locate the matrix table after the heading")
	}

	// Negative (anti-vacuity) arm: a fabricated graph-reader row with NO marker
	// MUST be reported by the SAME checker.
	synthetic := "## Decision matrix\n\n| Capability | Question | Use this | Not this |\n|---|---|---|---|\n" +
		"| Semantic graph | related symbols | `helix find-related-symbols` | recursive grep |\n"
	srows := parseMatrixRows(synthetic)
	require.Len(t, srows, 1, "synthetic table must parse to exactly one data row")
	assert.Truef(t, matrixRowMissingGraphPrereq(srows[0]),
		"SKILL-03 checker is vacuous: it did NOT reject a fabricated graph-reader row missing its prerequisite marker")
}
