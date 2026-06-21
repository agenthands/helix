package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/fatih/color"
)

// render.go is the terse renderer: the single CLI-side seam that turns a
// daemon CallToolResult into the frozen Phase 92 output contract. It dispatches
// on the tool's render class (render_policy.go), reuses the pure parse/sort
// core (locus.go), gates color up front via fatih/color, and reads a clamped
// CLI-side snippet line for nav verbs. Layout and color live HERE; the daemon
// only ships pre-formatted text and locus.go only parses it.

// colorMode is the resolved tri-state for the --color flag.
type colorMode int

const (
	// colorAuto leaves fatih/color's default behavior: off for non-TTY and when
	// NO_COLOR is set, on for an interactive terminal.
	colorAuto colorMode = iota
	// colorAlways forces color on.
	colorAlways
	// colorNever forces color off (byte-equivalent to a piped render).
	colorNever
)

// parseColorMode maps the --color flag string to a colorMode, defaulting to
// auto for any unrecognized value.
func parseColorMode(s string) colorMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "always":
		return colorAlways
	case "never":
		return colorNever
	default:
		return colorAuto
	}
}

// renderOpts carries the resolved per-invocation render options the verb path
// builds from the inherited root flags and the CLI working directory.
type renderOpts struct {
	// workspaceRoot is the CLI working directory used to relativize loci and to
	// clamp the snippet read (security boundary).
	workspaceRoot string
	// abs emits absolute paths instead of workspace-relative (OUT-07).
	abs bool
	// jsonOut emits compact JSON lines instead of the terse TAB form (OUT-06).
	jsonOut bool
	// color is the resolved color tri-state.
	color colorMode
}

// navToolsWithSnippet is the set of locus-list tools whose daemon loci carry NO
// payload (go_to_definition / find_references emit bare "file://…:L:C" lines —
// RESEARCH Pitfall 5). For these the renderer reads ONE source line CLI-side
// and appends it as the snippet. search_in_files / search_symbols already carry
// a payload, so they are NOT in this set (we never force a snippet onto a line
// that already has one).
var navToolsWithSnippet = map[string]bool{
	"go_to_definition": true,
	"find_references":  true,
}

// renderResult is the original verb.go seam, preserved as a thin adapter so the
// happy-path caller signature is unchanged. It resolves render options from the
// inherited root flags + working directory and delegates to renderResultFor.
// (The verb runner in verb.go calls renderResultFor directly with the resolved
// spec; this adapter keeps the cobra-only signature available.)
func renderResult(cmd *cobra.Command, spec verbSpec, res *mcpsdk.CallToolResult) {
	renderResultFor(cmd.OutOrStdout(), spec, res, resolveRenderOpts(cmd))
}

// resolveRenderOpts reads the inherited root --abs/--json/--color flags and the
// CLI working directory into a renderOpts. workspaceRoot defaults to os.Getwd()
// (mirroring activate's default; RESEARCH Open Q1).
func resolveRenderOpts(cmd *cobra.Command) renderOpts {
	opts := renderOpts{color: colorAuto}
	if wd, err := os.Getwd(); err == nil {
		opts.workspaceRoot = wd
	}
	if cmd != nil {
		if root := cmd.Root(); root != nil {
			fl := root.Flags()
			if v, err := fl.GetBool("abs"); err == nil {
				opts.abs = v
			}
			if v, err := fl.GetBool("json"); err == nil {
				opts.jsonOut = v
			}
			if v, err := fl.GetString("color"); err == nil {
				opts.color = parseColorMode(v)
			}
		}
	}
	return opts
}

// renderResultFor is the richer renderer entrypoint. It applies the color gate
// up front, then dispatches on the tool's render class:
//   - classLocusList: terse relpath:line:col<TAB>payload, sorted+deduped, with a
//     clamped CLI-side snippet for bare nav loci (OUT-01/02/03);
//   - classTree / classOpaque: passthrough verbatim (OUT-03).
//
// Color is gated by setting color.NoColor BEFORE any generation — never emit
// then strip ANSI.
func renderResultFor(out io.Writer, spec verbSpec, res *mcpsdk.CallToolResult, opts renderOpts) {
	applyColorGate(opts.color)

	if res == nil {
		return
	}

	switch renderClassFor(spec.toolName) {
	case classLocusList:
		renderLocusList(out, spec, res, opts)
	default:
		// classTree / classOpaque: passthrough.
		renderPassthrough(out, res, opts)
	}
}

// applyColorGate sets fatih/color's global NoColor toggle per the resolved
// colorMode. auto leaves the package default (off for non-TTY and when NO_COLOR
// is present), so a piped render and --color=never are byte-identical.
func applyColorGate(mode colorMode) {
	switch mode {
	case colorNever:
		color.NoColor = true
	case colorAlways:
		color.NoColor = false
	default:
		// auto: do nothing — fatih/color already resolved NoColor at init from
		// the TTY check + NO_COLOR. Leaving it untouched keeps piped == never.
	}
}

// renderLocusList renders a classLocusList result. It parses each TextContent
// line into a locus (passing non-loci through verbatim — never drop), sorts and
// dedups the collected loci, optionally reads a clamped snippet for bare nav
// loci, and emits either the terse TAB form or compact JSON lines.
func renderLocusList(out io.Writer, spec verbSpec, res *mcpsdk.CallToolResult, opts renderOpts) {
	wantSnippet := navToolsWithSnippet[spec.toolName]

	var loci []locus
	var passthrough []string
	for _, c := range res.Content {
		tc, ok := c.(*mcpsdk.TextContent)
		if !ok {
			// Non-text content under a locus-list class: emit compact JSON
			// (defensive — never drop).
			if b, err := json.Marshal(c); err == nil {
				passthrough = append(passthrough, string(b))
			}
			continue
		}
		for _, line := range strings.Split(tc.Text, "\n") {
			if line == "" {
				continue
			}
			if l, ok := parseLocusLine(line, opts.workspaceRoot, opts.abs); ok {
				loci = append(loci, l)
				continue
			}
			// Not a locus (e.g. "(no results)"): pass through verbatim.
			passthrough = append(passthrough, line)
		}
	}

	loci = sortDedupLoci(loci)

	if opts.jsonOut {
		for _, l := range loci {
			emitLocusJSON(out, l)
		}
		for _, p := range passthrough {
			fmt.Fprintln(out, p)
		}
		return
	}

	for _, l := range loci {
		payload := l.payload
		if wantSnippet && payload == "" {
			if snip, ok := readSnippetLine(opts.workspaceRoot, l.relpath, l.line); ok {
				payload = snip
			}
		}
		fmt.Fprintf(out, "%s:%d:%d\t%s\n", l.relpath, l.line, l.col, payload)
	}
	for _, p := range passthrough {
		fmt.Fprintln(out, p)
	}
}

// emitLocusJSON writes one compact JSON object for a locus.
func emitLocusJSON(out io.Writer, l locus) {
	obj := struct {
		Path    string `json:"path"`
		Payload string `json:"payload"`
		Line    int    `json:"line"`
		Col     int    `json:"col"`
	}{Path: l.relpath, Payload: l.payload, Line: l.line, Col: l.col}
	if b, err := json.Marshal(obj); err == nil {
		fmt.Fprintln(out, string(b))
	}
}

// renderPassthrough prints tree/opaque content verbatim: TextContent as-is,
// non-text content as compact JSON. Under --json the whole content is marshaled
// compactly. This preserves the pre-92 behavior for shape-only and opaque
// classes (OUT-03 outline = shape only).
func renderPassthrough(out io.Writer, res *mcpsdk.CallToolResult, opts renderOpts) {
	if opts.jsonOut {
		if b, err := json.Marshal(res.Content); err == nil {
			fmt.Fprintln(out, string(b))
		}
		return
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			// Verbatim: write the daemon text as-is and add a trailing newline
			// only when the text does not already end in one, so passthrough is
			// byte-faithful (OUT-03 shape only).
			fmt.Fprint(out, tc.Text)
			if !strings.HasSuffix(tc.Text, "\n") {
				fmt.Fprintln(out)
			}
			continue
		}
		if b, err := json.Marshal(c); err == nil {
			fmt.Fprintln(out, string(b))
		}
	}
}

// readSnippetLine reads the 1-based `line` from the file at workspaceRoot/relpath
// and returns it trimmed of the trailing newline. It is the security boundary
// for the CLI-side snippet read (T-92-04): it resolves the absolute path and
// verifies it stays WITHIN workspaceRoot via filepath.Rel + a `..`/absolute
// escape check BEFORE opening the file. Any failure or escape yields
// ("", false) — no snippet, no error, no read outside the root.
func readSnippetLine(workspaceRoot, relpath string, line int) (string, bool) {
	if workspaceRoot == "" || relpath == "" || line < 1 {
		return "", false
	}

	abs := relpath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(workspaceRoot, relpath)
	}
	abs = filepath.Clean(abs)

	// Containment check: the resolved path must be inside workspaceRoot.
	rootClean := filepath.Clean(workspaceRoot)
	rel, err := filepath.Rel(rootClean, abs)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}

	f, err := os.Open(abs)
	if err != nil {
		return "", false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// Allow long lines (default 64K token cap is fine for source, but bump for
	// safety on minified files).
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	cur := 0
	for sc.Scan() {
		cur++
		if cur == line {
			return strings.TrimRight(sc.Text(), "\r"), true
		}
	}
	return "", false
}
