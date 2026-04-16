package fuzzy

import "strings"

// sweepExact returns every line index in whole where part matches
// byte-for-byte. The outer loop uses `i <= len(whole)-partLen` (not `<`)
// so a 1-line search at end-of-file is covered (see 25-RESEARCH.md Pitfall 1).
//
// Empty part and part-longer-than-whole both return nil -- ambiguity check
// in match.go treats nil as "no hit, fall through to next strategy".
func sweepExact(whole, part []string) []int {
	partLen := len(part)
	if partLen == 0 || partLen > len(whole) {
		return nil
	}
	var hits []int
	for i := 0; i <= len(whole)-partLen; i++ {
		match := true
		for j := 0; j < partLen; j++ {
			if whole[i+j] != part[j] {
				match = false
				break
			}
		}
		if match {
			hits = append(hits, i)
		}
	}
	return hits
}

// sweepWhitespace matches using strings.TrimSpace per line -- leading AND
// trailing whitespace are ignored, but internal whitespace inside a line
// remains significant (CONTEXT.md: do NOT collapse internal spaces).
func sweepWhitespace(whole, part []string) []int {
	partLen := len(part)
	if partLen == 0 || partLen > len(whole) {
		return nil
	}
	var hits []int
	for i := 0; i <= len(whole)-partLen; i++ {
		match := true
		for j := 0; j < partLen; j++ {
			if strings.TrimSpace(whole[i+j]) != strings.TrimSpace(part[j]) {
				match = false
				break
			}
		}
		if match {
			hits = append(hits, i)
		}
	}
	return hits
}

// sweepIndentFlex matches after stripping leading spaces and tabs per line --
// tabs and spaces are interchangeable at the start of a line (a 1-tab indent
// matches a 4-space indent). Trailing whitespace remains significant.
func sweepIndentFlex(whole, part []string) []int {
	partLen := len(part)
	if partLen == 0 || partLen > len(whole) {
		return nil
	}
	var hits []int
	for i := 0; i <= len(whole)-partLen; i++ {
		match := true
		for j := 0; j < partLen; j++ {
			if strings.TrimLeft(whole[i+j], " \t") != strings.TrimLeft(part[j], " \t") {
				match = false
				break
			}
		}
		if match {
			hits = append(hits, i)
		}
	}
	return hits
}
