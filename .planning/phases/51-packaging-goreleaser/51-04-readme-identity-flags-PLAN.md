---
phase: 51-packaging-goreleaser
plan: 04
type: execute
wave: 3
depends_on: [51-01, 51-02]
files_modified:
  - README.md
autonomous: true
gap_closure: true
requirements: [PKG-01]
requirements_addressed: [PKG-01]
tags: [packaging, documentation, repo-identity, gap-closure, cr-01, wr-03, wr-04]
must_haves:
  truths:
    - "CLOSES VERIFICATION gap 'README.md still publishes go install github.com/postfix/serena/cmd/serena@latest as the PRIMARY install path' -- README.md no longer advertises the broken module path"
    - "CLOSES VERIFICATION/REVIEW finding CR-01 -- the phase brief deliverable 'postfix/serena -> agenthands/helix in README.md' is satisfied for real"
    - "CLOSES VERIFICATION/REVIEW finding WR-03 -- README.md and INSTALL.md HTTP-mode flags agree (both use --mode=http --http-addr=...) AND match the actual binary's flag definitions in internal/cli/root.go"
    - "CLOSES VERIFICATION/REVIEW finding WR-04 -- two conflicting upstream attributions (oraios/serena vs lks-ai/serena) are reconciled to a single canonical attribution"
    - "A user copy-pasting the README.md Quick Start install snippet does NOT pull from github.com/postfix/serena/cmd/serena (which may not be controlled by this project)"
    - "README.md HTTP-mode example invokes the binary with flags that exist in internal/cli/root.go (verified via grep against the source)"
    - "The agenthands/helix repo is consistently identified as the canonical home; the legacy postfix/serena module path is not surfaced as a primary install path"
  artifacts:
    - path: "README.md"
      provides: "Quick Start install snippet without the broken go-install line; HTTP-mode example aligned with INSTALL.md and the actual binary's --mode=http --http-addr=:8080 flag surface; upstream attribution reconciled to a single source"
      contains: ["pre-built binaries", "INSTALL.md", "git clone https://github.com/agenthands/helix.git", "cd helix", "--mode=http --http-addr"]
  key_links:
    - from: "README.md Quick Start"
      to: "INSTALL.md (Plan 51-02 lead section)"
      via: "Markdown link to [INSTALL.md](INSTALL.md)"
      pattern: "INSTALL\\.md"
    - from: "README.md HTTP mode example"
      to: "internal/cli/root.go flag definitions"
      via: "Documented flags --mode=http and --http-addr match cobra StringVar registrations on lines 30 and 38"
      pattern: "--mode=http"
    - from: "README.md upstream attribution"
      to: "single canonical upstream URL"
      via: "Either oraios/serena or lks-ai/serena (whichever is correct), used in BOTH the line-12 lead paragraph AND the line-348 footnote"
      pattern: "(oraios|lks-ai)/serena"
---

<objective>
Close VERIFICATION Concern D: repo identity drift in README.md and the HTTP-mode flag mismatch between README.md and INSTALL.md. Plan 51-02's prior approach of leaving the postfix/serena `go install` line in place with an explanatory in-fence comment was deliberately deferred per "Open Question 1" -- but the verification report and code review (CR-01) confirm that this is documentation theater: a copy-paste user pulls from the wrong module path regardless of the comment. The phase brief explicitly listed this correction as a deliverable, so the deferral is now lifted.

This plan also fixes WR-03 (the README.md `--serve --http-addr=:9091` line conflicts with INSTALL.md's `--mode=http --http-addr=127.0.0.1:8080`) and WR-04 (README.md attributes the upstream to two different URLs in the same file). All three are surgical edits in a single file.

Repo identity authoritative facts (verified before drafting this plan):
- The working tree path is github.com/agenthands/helix (`git remote -v` confirms agenthands/helix.git).
- The Go module path in `go.mod` is still `github.com/postfix/serena` (a separate rename decision is out of scope for this phase per Open Question 1).
- The actual binary's flag surface (verified via grep on internal/cli/root.go):
  - Line 30: `rootCmd.Flags().String("mode", "auto", "Transport mode: stdio, http, auto")`
  - Line 32: `rootCmd.Flags().Bool("serve", false, "Run as daemon directly (skip forwarder)")`
  - Line 38: `rootCmd.Flags().String("http-addr", ":8080", "HTTP listen address for Streamable HTTP transport")`
  - Line 70: `if serve || mode == "http" { return runDaemon(cmd) }` -- BOTH flags work; they're equivalent entry points to the daemon. The default --http-addr is `:8080`, NOT `:9091`.

Decision for WR-03: align README.md to use `--mode=http --http-addr=:8080` (matching INSTALL.md and the binary default), demoting `--serve` to a brief "(equivalent: --serve)" mention. INSTALL.md uses `127.0.0.1:8080`; README.md will use the same address verbatim so users who reference both docs see ONE canonical example.

Decision for WR-04: this plan does NOT have authority to verify which upstream URL (oraios/serena vs lks-ai/serena) is historically correct. The executor researches via:
1. `git log --all --diff-filter=A -- README.md | head -5` to find the original commit that introduced the attribution
2. Web search for "github.com/oraios/serena" and "github.com/lks-ai/serena" to confirm which repo currently exists
3. If both exist, check their "Inspired by" / fork-of relationships to determine the canonical lineage
The decision rule: pick the upstream URL that resolves to a real GitHub repo with code resembling Serena's MCP-tools approach. Update BOTH lines 12 and 348 to that URL. If neither resolves or both are stale, drop the attribution to "Originally inspired by Python Serena" with no link, and document the choice in the SUMMARY.

Output: A README.md where the Quick Start advertises pre-built binaries (linking to INSTALL.md) as the primary path, the build-from-source block uses agenthands/helix correctly, the HTTP-mode example uses the documented flags + default port matching INSTALL.md, and upstream attribution is internally consistent.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/51-packaging-goreleaser/51-CONTEXT.md
@.planning/phases/51-packaging-goreleaser/51-RESEARCH.md
@.planning/phases/51-packaging-goreleaser/51-VERIFICATION.md
@.planning/phases/51-packaging-goreleaser/51-REVIEW.md
@README.md
@INSTALL.md
@internal/cli/root.go

<interfaces>
The actual binary's flag surface (source of truth: internal/cli/root.go):

  rootCmd.Flags().String("mode", "auto", "Transport mode: stdio, http, auto")     // line 30
  rootCmd.Flags().Bool("serve", false, "Run as daemon directly (skip forwarder)")  // line 32
  rootCmd.Flags().String("http-addr", ":8080", "HTTP listen address...")           // line 38

  if serve || mode == "http" { return runDaemon(cmd) }                             // line 70

The two flags --mode=http and --serve are equivalent entry points; the default
--http-addr is :8080. INSTALL.md uses --mode=http --http-addr=127.0.0.1:8080.
README.md should match INSTALL.md verbatim.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Replace README.md primary install path with pre-built binary lead + agenthands/helix build-from-source</name>
  <files>README.md</files>
  <read_first>
    - README.md lines 1-90 (the Quick Start section starts at line 60; the existing snippet is at lines 62-75)
    - INSTALL.md lines 1-55 (committed in 51-02 -- the "## Install (pre-built binary)" lead section that this README change links to)
    - .planning/phases/51-packaging-goreleaser/51-REVIEW.md CR-01 (the exact text the reviewer suggested as the fix; this plan implements that fix verbatim)
    - .planning/phases/51-packaging-goreleaser/51-VERIFICATION.md (the gap text "README.md still publishes go install github.com/postfix/serena/cmd/serena@latest as the PRIMARY install path")
  </read_first>
  <action>
Replace the README.md Quick Start "### Install" subsection (currently lines 62-75) with a pre-built-binary lead that links to INSTALL.md, demoting the source-build path to a secondary mention with the correct repo URL.

Current README.md lines 62-75 (verbatim, from the just-read snapshot):

  ### Install
  
  ```bash
  go install github.com/postfix/serena/cmd/serena@latest
  # Note: module path is github.com/postfix/serena pending a separate rename decision; the public repo lives at agenthands/helix.
  ```
  
  Or build from source:
  
  ```bash
  git clone https://github.com/agenthands/helix.git
  cd helix
  go build ./cmd/serena
  ```

Replace with this exact content (preserving the surrounding ### Install heading and the blank-line spacing the file uses elsewhere):

  ### Install
  
  Pre-built binaries for darwin/linux/windows on amd64/arm64 are published on the [Releases page](https://github.com/agenthands/helix/releases) with SHA-256 checksums and minisign signatures. See [INSTALL.md](INSTALL.md) for the full verification recipe.
  
  Or build from source:
  
  ```bash
  git clone https://github.com/agenthands/helix.git
  cd helix
  go build ./cmd/serena
  ```

Style non-negotiables:
1. The first paragraph after `### Install` is a single line of prose, not a fenced block. The ROADMAP success-criterion-3 user UX (verified-binary install) is documented in INSTALL.md, not duplicated here.
2. The paragraph references "Releases page" with a link to https://github.com/agenthands/helix/releases AND a separate link to [INSTALL.md](INSTALL.md). Two links in one sentence is intentional: power users want the Releases page directly; first-timers want the INSTALL.md walkthrough.
3. The fenced bash block uses `git clone https://github.com/agenthands/helix.git` and `cd helix` -- matching the agenthands/helix-as-canonical-home decision.
4. The legacy `go install github.com/postfix/serena/cmd/serena@latest` line is REMOVED entirely. Per CR-01: "the footnote does not save the copy-paste user." Users who want a one-liner can find it in INSTALL.md "Build from source" (which also went through the same correction in 51-02; verify the INSTALL.md restructure was committed before relying on this).
5. The existing comment line `# Note: module path is github.com/postfix/serena pending a separate rename decision; the public repo lives at agenthands/helix.` is also removed. The module-path-vs-repo-name distinction is implementation detail; the README's Quick Start should not surface it.
6. Do NOT modify the `### Configure Your Client` heading or anything below it -- that section is out of scope.
7. After the edit, the surrounding `### Install` heading (line ~62) stays unchanged, and `### Configure Your Client` (currently around line 77) stays unchanged. The diff is approximately -10 lines / +6 lines.

Why "go install" is removed entirely (vs kept-with-comment):
- `go install github.com/postfix/serena/cmd/serena@latest` works TODAY because the module path in go.mod still says postfix/serena. But the goreleaser pipeline publishes binaries to agenthands/helix, NOT a Go module proxy. A user who runs go install gets a binary built from the postfix/serena module path -- which may be a stale fork, may not exist, or may be controlled by someone else. It is unsafe to advertise.
- The previous in-fence comment was meant to mitigate this, but it is documentation theater (CR-01 verbatim). Removing the line is the only correct fix.
- A future plan (separate decision) will rename the go.mod module to github.com/agenthands/helix; at that point a new go-install line can be added back. Today, the safe move is to lead with INSTALL.md.
  </action>
  <verify>
    <automated>! grep -q "go install github.com/postfix/serena/cmd/serena@latest" README.md && ! grep -q "module path is github.com/postfix/serena pending" README.md && grep -q "git clone https://github.com/agenthands/helix.git" README.md && grep -q "^cd helix$" README.md && grep -q "Pre-built binaries for darwin/linux/windows" README.md && grep -q "\[Releases page\](https://github.com/agenthands/helix/releases)" README.md && grep -q "\[INSTALL.md\](INSTALL.md)" README.md && grep -q "^### Install$" README.md && grep -q "^### Configure Your Client$" README.md</automated>
  </verify>
  <acceptance_criteria>
    - README.md does NOT contain the literal string `go install github.com/postfix/serena/cmd/serena@latest` (the broken go-install line is gone)
    - README.md does NOT contain the literal string `module path is github.com/postfix/serena pending` (the in-fence comment is gone)
    - README.md contains the literal string `Pre-built binaries for darwin/linux/windows`
    - README.md contains a markdown link `[Releases page](https://github.com/agenthands/helix/releases)`
    - README.md contains a markdown link `[INSTALL.md](INSTALL.md)`
    - README.md contains `git clone https://github.com/agenthands/helix.git`
    - README.md contains a `cd helix` line on its own (the cloned-directory rename)
    - The `### Install` heading is preserved (still on its own line)
    - The `### Configure Your Client` heading directly follows (separated by the build-from-source block + blank lines)
    - No incidental edits to other sections (`git diff README.md` shows changes ONLY between `### Install` and `### Configure Your Client`)
  </acceptance_criteria>
  <done>README.md Quick Start advertises pre-built binaries (verified via INSTALL.md) as the primary install path. The legacy postfix/serena go-install line is gone -- copy-paste users no longer pull from a foreign module path. The phase brief deliverable for README.md repo identity correction is satisfied for real (not via a footnote).</done>
</task>

<task type="auto" tdd="false">
  <name>Task 2: Reconcile README.md HTTP-mode example with INSTALL.md and the actual binary flags (WR-03)</name>
  <files>README.md</files>
  <read_first>
    - README.md lines 130-140 (the HTTP mode block; current `serena --serve --http-addr=:9091` is on line 134)
    - INSTALL.md lines 244-252 (the canonical HTTP mode example; uses `serena --mode=http --http-addr=127.0.0.1:8080`)
    - internal/cli/root.go lines 28-40 (flag definitions: `--mode`, `--serve`, `--http-addr` with default `:8080`)
    - internal/cli/root.go lines 65-78 (runRoot function: confirms `serve || mode == "http"` triggers the daemon path)
  </read_first>
  <action>
Update the README.md "HTTP mode" example (currently lines 132-136 inside the `<details>` block of "Manual configuration") so it matches INSTALL.md's canonical command and the binary's documented default port.

Current README.md lines 132-136 (verbatim, inside the `<details>` block):

  **HTTP mode** (for IDEs, web clients, multi-client):
  ```bash
  serena --serve --http-addr=:9091
  # Connect your client to http://localhost:9091/mcp
  ```

The fenced block contains TWO problems:
1. `--serve --http-addr=:9091` uses --serve (the deprecated-style daemon entry) AND a non-default port :9091. INSTALL.md uses --mode=http --http-addr=127.0.0.1:8080. Two docs, two different invocations, neither matches the binary's default --http-addr (`:8080`).
2. The `# Connect your client to http://localhost:9091/mcp` comment hardcodes the port that doesn't match the binary default OR INSTALL.md.

Replace with this exact content:

  **HTTP mode** (for IDEs, web clients, multi-client):
  ```bash
  serena --mode=http --http-addr=127.0.0.1:8080
  # Connect your client to http://127.0.0.1:8080/mcp
  # Equivalent: serena --serve --http-addr=127.0.0.1:8080 (--serve and --mode=http both enter the daemon)
  ```

Style non-negotiables:
1. The primary command is `serena --mode=http --http-addr=127.0.0.1:8080` -- byte-identical to INSTALL.md line 249. Two docs, ONE canonical invocation.
2. The first comment line uses `127.0.0.1:8080` (loopback IP, default port) -- matching the command above. Do NOT use `localhost` here; INSTALL.md uses 127.0.0.1, and consistency between the two docs is the whole point of this fix.
3. The second comment line documents that `--serve` is equivalent. This preserves the prior invocation's discoverability without making it the primary form. The equivalent invocation also uses `127.0.0.1:8080`, NOT `:9091`.
4. Do NOT change the heading (`**HTTP mode** ...`) or the surrounding `<details>` block structure.
5. Do NOT change the line just below the fenced block ("For Cursor, Antigravity, ..." on line 138). It is unrelated.
6. After the edit, `git diff README.md` for this section should show approximately -3 lines / +3 lines.

Why --mode=http is the primary form:
- `--mode=auto|stdio|http` is the cobra flag with explicit valid values (the help output reads "Transport mode: stdio, http, auto"); --serve is a Bool that flips the same path. The String form is more discoverable.
- INSTALL.md already uses --mode=http; aligning README.md to match removes the doc-vs-doc conflict.
- Both flags work, so users with prior muscle memory for --serve see the equivalent in the comment.

Why 127.0.0.1:8080 (vs :9091, vs localhost):
- 127.0.0.1 is loopback-only (cannot be connected from another host); :9091 binds to all interfaces (potential security regression if a user copy-pastes without thinking).
- The binary default --http-addr is :8080 (cobra StringVar default); the canonical user-facing invocation uses 127.0.0.1:8080 to be explicit.
- INSTALL.md uses 127.0.0.1:8080; consistency wins.
  </action>
  <verify>
    <automated>! grep -q "serena --serve --http-addr=:9091" README.md && grep -q "serena --mode=http --http-addr=127.0.0.1:8080" README.md && grep -q "Connect your client to http://127.0.0.1:8080/mcp" README.md && grep -q "Equivalent: serena --serve --http-addr=127.0.0.1:8080" README.md && grep -c "http://localhost:9091" README.md | grep -qE '^[[:space:]]*0$'</automated>
  </verify>
  <acceptance_criteria>
    - README.md does NOT contain `serena --serve --http-addr=:9091` (the old non-default port invocation is gone)
    - README.md does NOT contain `http://localhost:9091/mcp` (the matching old comment is gone)
    - README.md contains the literal string `serena --mode=http --http-addr=127.0.0.1:8080` (matching INSTALL.md byte-for-byte)
    - README.md contains the comment `# Connect your client to http://127.0.0.1:8080/mcp`
    - README.md contains the equivalence comment `# Equivalent: serena --serve --http-addr=127.0.0.1:8080`
    - The `**HTTP mode**` heading and surrounding `<details>` structure are unchanged
    - The line "For Cursor, Antigravity, VS Code, JetBrains, Claude Desktop, Gemini CLI, and OpenCode --" (the line just below the fenced block) is byte-identical to before
    - INSTALL.md is NOT modified by this task (Plan 51-04 owns README.md only)
    - The two docs now agree: `grep -c "serena --mode=http --http-addr=127.0.0.1:8080" README.md INSTALL.md | awk -F: '{sum+=$2} END {print sum}'` returns at least 2 (one match in each file)
  </acceptance_criteria>
  <done>README.md HTTP mode example uses the same `--mode=http --http-addr=127.0.0.1:8080` invocation as INSTALL.md; both docs match the actual binary's flag surface and default port. WR-03 closed for both docs.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: Reconcile README.md upstream attribution (WR-04 -- pick canonical URL or drop both)</name>
  <files>README.md</files>
  <read_first>
    - README.md lines 10-14 (the lead paragraph; line 12 currently says "Helix started as a rewrite of [Serena MCP](https://github.com/oraios/serena)")
    - README.md lines 345-352 (the footnote at the bottom; line 348 currently says "Originally inspired by [Python Serena](https://github.com/lks-ai/serena)")
    - .planning/phases/51-packaging-goreleaser/51-REVIEW.md WR-04 (the finding)
  </read_first>
  <action>
Reconcile the two conflicting upstream attributions in README.md (lines 12 and 348). The two URLs (oraios/serena and lks-ai/serena) cannot both be the canonical upstream. This task resolves the conflict.

Step A: Determine the correct upstream

Run all three of these checks before deciding:

1. Git log: `git log --all --diff-filter=A -- README.md | head -3` to find the commit that first introduced the file. Then read that commit's README content if available.

2. Project planning history: `grep -rn "oraios\|lks-ai" .planning/ 2>/dev/null | head -20`. If a CONTEXT.md or RESEARCH.md from an earlier phase mentions one URL but not the other, that is strong evidence.

3. Web check: try `curl -sI -o /dev/null -w "%{http_code}\n" https://github.com/oraios/serena 2>/dev/null` and `curl -sI -o /dev/null -w "%{http_code}\n" https://github.com/lks-ai/serena 2>/dev/null`. A 200 response means the repo exists; 404 means it does not.

Decision rules (in priority order):
- If exactly ONE URL resolves to a real repo with code that resembles Serena's MCP-tools approach -> use that URL in BOTH lines 12 and 348.
- If BOTH URLs resolve to real but different repos, prefer oraios/serena (this matches the line-12 lead paragraph claim "Helix started as a rewrite of"; the line-348 footnote is the suspect line based on PROJECT.md's lineage).
- If NEITHER URL resolves, drop the link to a plain "[Python Serena upstream]" mention with no URL on both lines, and document the choice in the SUMMARY.
- If the project planning history pins a specific URL as canonical, use it regardless of HTTP status.

Step B: Apply the chosen URL to BOTH lines

Whichever URL is chosen, update BOTH line 12 and line 348 to use it. The two lines say slightly different things ("Helix started as a rewrite of ..." vs "Originally inspired by ...") -- preserve those existing prose differences. ONLY the URL portion of each markdown link changes.

Example: if oraios/serena is the canonical upstream, line 348's footnote changes from:
  Originally inspired by [Python Serena](https://github.com/lks-ai/serena)
to:
  Originally inspired by [Python Serena](https://github.com/oraios/serena)

Line 12 stays as-is in this scenario (it already uses oraios/serena).

Style non-negotiables:
1. Use the EXACT same URL in both lines after this task; the URLs being identical is the entire point of the fix.
2. Do NOT change the surrounding prose ("Helix started as a rewrite of" / "Originally inspired by") -- those are author voice and out of scope.
3. Do NOT add a third "we acknowledge upstream X and Y" note; pick one or none.
4. If you choose "drop both URLs" because neither resolves, the markdown link `[Python Serena](URL)` becomes plain text `Python Serena` (no link). Both lines get the same treatment.
5. After the edit, `grep -c "oraios/serena" README.md` plus `grep -c "lks-ai/serena" README.md` MUST equal either 0 (neither URL used) or 2 (one URL used, twice). It MUST NOT equal 1 (one URL used once and the other zero times -- that would be a half-fix), and it MUST NOT have non-zero counts on BOTH `oraios` and `lks-ai` (that would be the original broken state).

Document the decision (which URL was chosen and why) in the eventual 51-04-SUMMARY.md.
  </action>
  <verify>
    <automated>OC=$(grep -c "oraios/serena" README.md); LC=$(grep -c "lks-ai/serena" README.md); { [ "$OC" -eq 0 ] && [ "$LC" -eq 0 ]; } || { [ "$OC" -eq 2 ] && [ "$LC" -eq 0 ]; } || { [ "$OC" -eq 0 ] && [ "$LC" -eq 2 ]; }</automated>
  </verify>
  <acceptance_criteria>
    - Either: README.md contains exactly 0 occurrences of `oraios/serena` AND exactly 0 occurrences of `lks-ai/serena` (both URLs dropped, plain text "Python Serena" with no link in both places); OR
    - README.md contains exactly 2 occurrences of `oraios/serena` AND exactly 0 of `lks-ai/serena` (oraios chosen as canonical); OR
    - README.md contains exactly 2 occurrences of `lks-ai/serena` AND exactly 0 of `oraios/serena` (lks-ai chosen as canonical)
    - The lead paragraph prose ("Helix started as a rewrite of") is preserved
    - The footnote prose ("Originally inspired by") is preserved
    - No incidental edits elsewhere in README.md (`git diff` shows changes only in lines 12 and 348 / their immediate neighborhoods)
    - The chosen URL (or the "drop both" decision) is documented in the 51-04-SUMMARY.md output (cross-checked at plan-completion time, not at task verification)
  </acceptance_criteria>
  <done>README.md attributes the upstream consistently in both the lead paragraph and the footnote. The two-URLs-in-one-file conflict (WR-04) is resolved.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| README.md copy-paste user -> Go module proxy | A user running `go install <path>@latest` from the README pulls the module from Go's proxy. If the path is wrong (postfix/serena instead of agenthands/helix), the user gets a binary the project does not control. Mitigated by removing the broken go-install line entirely. |
| README.md HTTP-mode example -> binary flag surface | A user running the documented `serena --serve --http-addr=:9091` command relies on those flags actually existing. If the flags are wrong, the user sees `unknown flag` and either gives up or files a confusing bug report. Mitigated by Task 2's grep against internal/cli/root.go to confirm the flag names. |
| README.md upstream attribution -> external URL | A user clicking the "Python Serena" link goes to whichever URL the README points at. If the URL is stale or wrong, the user lands somewhere unrelated to the project's actual lineage. Mitigated by Task 3's resolution check + decision rules. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-51-21 (T-broken-install-path) | Tampering | README.md `go install` line pointing at postfix/serena | mitigate | Task 1 removes the line entirely. The agenthands/helix repo (the actual canonical home) does not have an authoritative module-proxy path until go.mod is renamed in a separate decision. Until then, INSTALL.md is the canonical install path. Reference: REVIEW CR-01. |
| T-51-22 (T-doc-flag-drift) | Confusion | README.md HTTP-mode example using non-default port | mitigate | Task 2 aligns the example to `--mode=http --http-addr=127.0.0.1:8080`, matching INSTALL.md and the binary default. The flag names are verified against `internal/cli/root.go` lines 28-40 in the read_first. Reference: REVIEW WR-03. |
| T-51-23 (T-non-loopback-default) | Confusion / Information Disclosure | README.md HTTP-mode example using `:9091` (all-interfaces bind) | mitigate | The new example uses `127.0.0.1:8080` (loopback only). A user who copy-pastes without changing the bind address cannot accidentally expose the daemon to other hosts on the same network. Reference: implicit in WR-03 fix. |
| T-51-24 (T-internal-attribution-conflict) | Confusion | Two conflicting upstream URLs in same README.md | mitigate | Task 3 picks ONE canonical URL (or drops both) and applies it to BOTH attribution lines. The two-URL state itself is the bug; reconciling to one (or zero) closes it. Reference: REVIEW WR-04. |

**Severity assessment:** No HIGH-severity threats. T-51-21 was the highest-impact (a broken install path is the kind of bug that loses users on first contact); the fix is the simplest possible (delete the line).
</threat_model>

<verification>
Plan-level verification (after all 3 tasks complete):

1. README.md does NOT contain `go install github.com/postfix/serena/cmd/serena@latest` (Task 1).
2. README.md contains a link to [INSTALL.md](INSTALL.md) and a link to [Releases page](https://github.com/agenthands/helix/releases) in the Install subsection (Task 1).
3. README.md `git clone` URL is `github.com/agenthands/helix.git` (Task 1).
4. README.md HTTP-mode example uses `serena --mode=http --http-addr=127.0.0.1:8080`, matching INSTALL.md byte-for-byte (Task 2).
5. README.md does NOT use port `:9091` anywhere (Task 2).
6. README.md upstream attribution count: zero or two of each candidate URL (oraios, lks-ai); never one of one and zero of the other (Task 3).
7. INSTALL.md is NOT modified by this plan (`git diff INSTALL.md` empty for Plan 51-04's commits).
8. `git diff README.md` shows changes confined to: the Install subsection (Task 1), the HTTP mode block in the `<details>` collapsible (Task 2), and the line-12 / line-348 attribution areas (Task 3). No incidental edits.
9. `go vet ./...` and `go test ./...` continue to pass (no Go source files modified).
</verification>

<success_criteria>
Plan 51-04 succeeds when ALL of the following are true:

1. README.md primary install path is "pre-built binaries via [INSTALL.md](INSTALL.md)" with a build-from-source secondary path using `git clone github.com/agenthands/helix.git`. The legacy `go install github.com/postfix/serena/cmd/serena@latest` line is gone.
2. README.md HTTP-mode example uses `serena --mode=http --http-addr=127.0.0.1:8080`, matching INSTALL.md byte-for-byte and the actual binary's flag surface (verified against internal/cli/root.go).
3. README.md upstream attribution is internally consistent: either both lines (12, 348) use the same URL, or both lines drop the URL to plain text. The bug-state of two different URLs is gone.
4. The chosen upstream URL (or the "drop both" decision) is documented in the SUMMARY with the rationale (which check resolved which way).
5. Every threat in the STRIDE register has a mitigation tied to a specific text change in this plan.
6. `git status` shows exactly one modified file: README.md. No incidental edits to other files.
7. `go vet ./...` and `go test ./...` continue to pass (no Go source files were modified).

VERIFICATION gaps closed: README.md repo identity (CR-01), HTTP-mode flag mismatch (WR-03), upstream attribution conflict (WR-04). The agenthands/helix canonical-home story is now consistent across README.md AND INSTALL.md.
</success_criteria>

<output>
After completion, create `.planning/phases/51-packaging-goreleaser/51-04-SUMMARY.md` capturing:
- The README.md edits with one-line description per Task (Install subsection rewrite, HTTP mode reconciliation, attribution reconciliation).
- The upstream URL decision from Task 3: which URL was chosen (or "drop both") and the evidence trail (git log result, web-resolution result, planning-history grep result).
- Confirmation that INSTALL.md was NOT modified by this plan.
- Confirmation that `go vet ./...` and `go test ./...` baselines are preserved (no Go source modified).
- Cross-reference to VERIFICATION gaps closed: CR-01 (README.md repo identity), WR-03 (HTTP-mode flag mismatch), WR-04 (upstream attribution).
- Note for downstream: a future phase should rename the go.mod module from github.com/postfix/serena to github.com/agenthands/helix (separate decision per Plan 51-02 Open Question 1). Once that lands, the README.md Install subsection can re-add a `go install` one-liner pointing at the new module path.
</output>
