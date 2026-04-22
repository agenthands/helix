package mcp

import "strings"

// levenshteinDistance returns the minimum edit distance between two strings
// using standard dynamic programming with two-row optimization.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// extractBadParams parses SDK schema validation error format:
//
//	validating "arguments": unexpected additional properties ["param1" "param2"]
//
// Returns the list of bad parameter names, or nil if the format doesn't match.
func extractBadParams(errMsg string) []string {
	const prefix = `unexpected additional properties [`
	idx := strings.Index(errMsg, prefix)
	if idx < 0 {
		return nil
	}
	rest := errMsg[idx+len(prefix):]
	end := strings.Index(rest, "]")
	if end < 0 {
		return nil
	}
	raw := rest[:end]
	var params []string
	for _, part := range strings.Split(raw, " ") {
		p := strings.Trim(part, `"`)
		if p != "" {
			params = append(params, p)
		}
	}
	return params
}

// bestParamSuggestion returns the best matching parameter name and its distance.
// It checks exact substring matches (either direction) and Levenshtein distance.
// If no match meets the threshold, returns ("", maxDistance+1).
func bestParamSuggestion(unknown string, validParams []string, maxDistance int) (string, int) {
	best := ""
	bestDist := maxDistance + 1

	for _, valid := range validParams {
		d := levenshteinDistance(unknown, valid)
		// Exact substring match (D-01): bypass maxDistance threshold.
		// Substring matches are high-confidence even when edit distance is large
		// (e.g., "path" -> "relative_path").
		if strings.Contains(valid, unknown) || strings.Contains(unknown, valid) {
			if best == "" || d < bestDist {
				best, bestDist = valid, d
			}
			continue
		}
		if d <= maxDistance && d < bestDist {
			best, bestDist = valid, d
		}
	}
	return best, bestDist
}

// bestValueSuggestion returns the best matching enum value and its distance.
// Same algorithm as bestParamSuggestion but for enum values (D-02, D-08).
func bestValueSuggestion(value string, validValues []string, maxDistance int) (string, int) {
	best := ""
	bestDist := maxDistance + 1

	for _, valid := range validValues {
		d := levenshteinDistance(value, valid)
		// Exact substring match (D-01): bypass maxDistance threshold.
		if strings.Contains(valid, value) || strings.Contains(value, valid) {
			if best == "" || d < bestDist {
				best, bestDist = valid, d
			}
			continue
		}
		if d <= maxDistance && d < bestDist {
			best, bestDist = valid, d
		}
	}
	return best, bestDist
}

// ForTest exports for the _test package (follows ClassifyOutcomeForTest pattern).

// LevenshteinDistanceForTest exposes levenshteinDistance to the _test package.
func LevenshteinDistanceForTest(a, b string) int {
	return levenshteinDistance(a, b)
}

// ExtractBadParamsForTest exposes extractBadParams to the _test package.
func ExtractBadParamsForTest(errMsg string) []string {
	return extractBadParams(errMsg)
}

// BestParamSuggestionForTest exposes bestParamSuggestion to the _test package.
func BestParamSuggestionForTest(unknown string, validParams []string, maxDistance int) (string, int) {
	return bestParamSuggestion(unknown, validParams, maxDistance)
}

// BestValueSuggestionForTest exposes bestValueSuggestion to the _test package.
func BestValueSuggestionForTest(value string, validValues []string, maxDistance int) (string, int) {
	return bestValueSuggestion(value, validValues, maxDistance)
}
