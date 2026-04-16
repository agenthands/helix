package fuzzy

import "strings"

// commonLeadingPrefix returns the longest common leading whitespace
// (" " and "\t" only) shared by every non-empty line. Whitespace-only
// lines are ignored in the calculation (they would otherwise force
// prefix = line, which is wrong for blank interior lines).
//
// Deviation from Aider documented: Aider's algorithm checks that every
// non-empty matched source line shares a single leading-whitespace
// offset; we use only the first matched line's leading bytes in
// reapplyPrefix, which is strictly more permissive (see reflow below).
func commonLeadingPrefix(lines []string) string {
	var prefix string
	initialized := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lead := leadingWhitespace(line)
		if !initialized {
			prefix = lead
			initialized = true
			continue
		}
		prefix = longestCommonPrefix(prefix, lead)
		if prefix == "" {
			return ""
		}
	}
	return prefix
}

// leadingWhitespace returns the prefix of s consisting of " " and "\t".
func leadingWhitespace(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return s[:i]
		}
	}
	return s
}

// longestCommonPrefix returns the longest byte prefix shared by a and b.
func longestCommonPrefix(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:n]
}

// dedent strips prefix from the start of every non-empty line that has
// it. Lines that don't start with prefix are returned unchanged. Empty
// or whitespace-only lines pass through as "".
func dedent(lines []string, prefix string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			out[i] = ""
			continue
		}
		if strings.HasPrefix(line, prefix) {
			out[i] = line[len(prefix):]
		} else {
			out[i] = line
		}
	}
	return out
}

// reapplyPrefix prepends sourcePrefix to every non-empty line in lines.
// Empty or whitespace-only lines are returned as "" to avoid trailing
// whitespace (see 25-RESEARCH.md Pitfall 6).
func reapplyPrefix(lines []string, sourcePrefix string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			out[i] = ""
			continue
		}
		out[i] = sourcePrefix + line
	}
	return out
}

// reflow applies the full Aider-style common-prefix dedent + reapply
// transform to replacement, producing the text that will become
// Result.ReplacementText.
//
//  1. searchPrefix = longest common leading ws across non-empty search lines
//  2. dedent replacement by searchPrefix
//  3. sourcePrefix = leading bytes of the first non-empty matched source line
//  4. reapply sourcePrefix to every non-empty dedented replacement line
//
// For exact-strategy matches, searchPrefix == sourcePrefix and reflow
// is effectively a no-op; for whitespace/indent-flex strategies it
// repoints the replacement at the source's actual indentation.
func reflow(searchLines, matchedRegion []string, replacement string) string {
	searchPrefix := commonLeadingPrefix(searchLines)

	var sourcePrefix string
	for _, line := range matchedRegion {
		if strings.TrimSpace(line) != "" {
			sourcePrefix = leadingWhitespace(line)
			break
		}
	}

	replacementLines := splitLines(replacement)
	dedented := dedent(replacementLines, searchPrefix)
	reapplied := reapplyPrefix(dedented, sourcePrefix)
	return strings.Join(reapplied, "\n")
}
