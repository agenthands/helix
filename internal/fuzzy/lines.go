package fuzzy

import "strings"

// splitLines splits s on '\n' with no further trimming. A trailing '\n'
// in s produces a trailing empty element in the returned slice -- callers
// must account for this when reconstructing. Convention chosen to match
// strings.Split exactly (see 25-RESEARCH.md Pitfall 4).
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

// lineByteOffsets returns a slice where element i is the byte offset
// of the start of line i in the reconstructed string
// strings.Join(lines, "\n"). Used by Match to translate a matched
// line index to the Result.StartByte field (byte offset into source).
//
// Invariants:
//   - len(lineByteOffsets(lines)) == len(lines)
//   - lineByteOffsets(lines)[0] == 0
//   - for i > 0: offsets[i] == offsets[i-1] + len(lines[i-1]) + 1  (the +1 is the '\n')
func lineByteOffsets(lines []string) []int {
	offsets := make([]int, len(lines))
	cursor := 0
	for i, line := range lines {
		offsets[i] = cursor
		cursor += len(line) + 1 // +1 for the '\n' delimiter
	}
	return offsets
}
