package fuzzy

import (
	"fmt"
	"strings"
)

// formatAmbiguity renders the human-readable detail for a FUZZ-07
// ambiguity error. Line numbers in `hits` are 1-indexed. The shown
// list is capped at 5; the overflow suffix is ", ...and N more"
// where N = len(hits)-5.
//
// Format (locked by CONTEXT.md S4):
//
//	ambiguous match: search block matches <N> locations (lines <list>[, ...and <M> more])
func formatAmbiguity(hits []int) string {
	shown := hits
	suffix := ""
	if len(hits) > 5 {
		shown = hits[:5]
		suffix = fmt.Sprintf(", ...and %d more", len(hits)-5)
	}
	parts := make([]string, len(shown))
	for i, ln := range shown {
		parts[i] = fmt.Sprintf("%d", ln)
	}
	return fmt.Sprintf(
		"ambiguous match: search block matches %d locations (lines %s%s)",
		len(hits), strings.Join(parts, ", "), suffix,
	)
}

// formatFailureDiff renders a minimal unified-diff-like payload
// comparing a search block against the nearest source window. This is
// NOT a real diff -- it is a labeled two-column view for the agent.
//
// Output shape (locked by RESEARCH.md S-Code Examples Example 4):
//
//	--- search
//	+++ nearest source region
//	@@ -1,<search lines> +1,<nearest lines> @@
//	-<search line 1>
//	...
//	+<nearest line 1>
//	...
func formatFailureDiff(search, nearestWindow string) string {
	searchCount := strings.Count(search, "\n") + 1
	nearestCount := strings.Count(nearestWindow, "\n") + 1
	if nearestWindow == "" {
		nearestCount = 0
	}

	var b strings.Builder
	b.WriteString("--- search\n")
	b.WriteString("+++ nearest source region\n")
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", searchCount, nearestCount)
	for _, line := range strings.Split(search, "\n") {
		b.WriteString("-")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if nearestWindow != "" {
		for _, line := range strings.Split(nearestWindow, "\n") {
			b.WriteString("+")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// findNearestWindow selects the source window of the same length as
// search with the highest count of byte-equal lines. Ties pick the
// earliest window. When source is shorter than search, the entire
// source is returned.
func findNearestWindow(source, search []string) []string {
	if len(search) == 0 {
		return nil
	}
	if len(source) < len(search) {
		out := make([]string, len(source))
		copy(out, source)
		return out
	}
	bestStart := 0
	bestScore := -1
	for i := 0; i <= len(source)-len(search); i++ {
		score := 0
		for j := 0; j < len(search); j++ {
			if source[i+j] == search[j] {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestStart = i
		}
	}
	out := make([]string, len(search))
	copy(out, source[bestStart:bestStart+len(search)])
	return out
}
