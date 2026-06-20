package ragindex

import (
	"strconv"
	"strings"
)

// Chunking strategy (documented in EMBED-CHOICE.md):
//   - Fixed line-window chunker: chunkWindowLines lines per chunk with
//     chunkOverlapLines lines of overlap between adjacent windows.
//   - Deterministic: same (content, relPath) always yields the same chunks and
//     the same chunk IDs.
//   - Chunk ID format: "<relPath>#<ordinal>" with ordinal starting at 0.
//   - Empty or whitespace-only files yield ZERO chunks (nothing to embed).
//
// The window/overlap sizes are intentionally small so that a single function or
// declaration tends to land wholly within at least one window; the overlap
// reduces the chance of a relevant symbol being split across a window boundary.
const (
	chunkWindowLines  = 40
	chunkOverlapLines = 8
)

// Piece is a single embedding unit: a contiguous line window of a source file,
// addressable by a stable ID.
type Piece struct {
	// ID is "<relPath>#<ordinal>", stable across calls.
	ID string
	// RelPath is the corpus-relative path the chunk was cut from.
	RelPath string
	// Content is the chunk's source text (the windowed lines, newline-joined).
	Content string
	// StartLine is the 0-based index of the first line of the window.
	StartLine int
}

// Chunk splits a source file's content into deterministic line-window pieces
// with stable IDs. An empty or whitespace-only file yields no pieces.
func Chunk(content, relPath string) []Piece {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	// Drop a single trailing empty element produced by a terminating newline so
	// the last window isn't padded with a blank line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}

	step := chunkWindowLines - chunkOverlapLines
	if step < 1 {
		step = 1
	}

	var chunks []Piece
	ordinal := 0
	for start := 0; start < len(lines); start += step {
		end := start + chunkWindowLines
		if end > len(lines) {
			end = len(lines)
		}
		window := strings.Join(lines[start:end], "\n")
		chunks = append(chunks, Piece{
			ID:        relPath + "#" + strconv.Itoa(ordinal),
			RelPath:   relPath,
			Content:   window,
			StartLine: start,
		})
		ordinal++
		// If this window already reached EOF, stop (avoid emitting a duplicate
		// tail window when len(lines) <= window size).
		if end == len(lines) {
			break
		}
	}
	return chunks
}
