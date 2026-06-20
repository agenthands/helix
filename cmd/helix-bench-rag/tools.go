package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenthands/helix/bench/ragindex"
)

// defaultK is the rag_search k used when the caller passes k <= 0.
const defaultK = 5

// maxGrepMatches caps grep output so a pathological pattern cannot flood the
// MCP transport.
const maxGrepMatches = 200

// handlers backs the four MCP tools over the corpus rooted at root. All
// filesystem access is confined to root by validatePath (T-83-02-01).
type handlers struct {
	idx  querier
	root string
}

// validatePath resolves a corpus-root-relative path to an absolute path INSIDE
// the corpus root, rejecting absolute inputs, `..` escapes, and any path that
// resolves outside root. It mirrors the validatePathSegment / validateModeName
// discipline used elsewhere in bench/ (V5 path-traversal mitigation).
func (h *handlers) validatePath(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed: %q", rel)
	}
	// Reject any `..` segment up front; defense-in-depth alongside the prefix
	// containment check below.
	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) || strings.Contains(cleaned, string(os.PathSeparator)+".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes corpus root: %q", rel)
	}

	rootAbs, err := filepath.Abs(h.root)
	if err != nil {
		return "", fmt.Errorf("resolve corpus root: %w", err)
	}
	joined := filepath.Join(rootAbs, cleaned)

	// Final containment check: the resolved absolute path must equal the root or
	// live strictly under it (exact-OR-separator boundary, never bare prefix).
	if joined != rootAbs && !strings.HasPrefix(joined, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes corpus root: %q", rel)
	}
	return joined, nil
}

// ragSearch runs a k-NN query against the embedding index and renders the hits.
func (h *handlers) ragSearch(ctx context.Context, query string, k int) (string, error) {
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("rag_search: query is empty")
	}
	if k <= 0 {
		k = defaultK
	}
	hits, err := h.idx.Query(ctx, query, k)
	if err != nil {
		return "", fmt.Errorf("rag_search: %w", err)
	}
	if len(hits) == 0 {
		return "no results", nil
	}
	var b strings.Builder
	for i, hit := range hits {
		fmt.Fprintf(&b, "[%d] %s (chunk_id=%s, similarity=%.4f)\n", i+1, hit.RelPath, hit.ChunkID, hit.Similarity)
		b.WriteString(hit.Content)
		if !strings.HasSuffix(hit.Content, "\n") {
			b.WriteByte('\n')
		}
		if i < len(hits)-1 {
			b.WriteString("---\n")
		}
	}
	return b.String(), nil
}

// readChunk returns the content of a single chunk identified by "<relPath>#<ordinal>".
// The relPath component is validated like any other FS path so an escaping id is
// rejected; the chunk is reconstructed deterministically from the on-disk file
// via the same ragindex.Chunk windower the index was built with.
func (h *handlers) readChunk(_ context.Context, chunkID string) (string, error) {
	hash := strings.LastIndex(chunkID, "#")
	if hash < 0 {
		return "", fmt.Errorf("rag_read_chunk: malformed chunk_id %q (want <relPath>#<ordinal>)", chunkID)
	}
	relPath := chunkID[:hash]
	ordStr := chunkID[hash+1:]
	ordinal, err := strconv.Atoi(ordStr)
	if err != nil || ordinal < 0 {
		return "", fmt.Errorf("rag_read_chunk: bad ordinal in chunk_id %q", chunkID)
	}

	abs, err := h.validatePath(relPath)
	if err != nil {
		return "", fmt.Errorf("rag_read_chunk: %w", err)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("rag_read_chunk: read %q: %w", relPath, err)
	}
	// Reconstruct chunks using the SAME windower (slash-normalized relPath, as
	// the index records it) so ordinals line up with rag_search chunk_ids.
	pieces := ragindex.Chunk(string(b), filepath.ToSlash(relPath))
	if ordinal >= len(pieces) {
		return "", fmt.Errorf("rag_read_chunk: ordinal %d out of range (%d chunks) for %q", ordinal, len(pieces), relPath)
	}
	return pieces[ordinal].Content, nil
}

// grep runs a regex search over a corpus-root-relative file or directory and
// returns matching lines as "<relPath>:<lineno>: <line>".
func (h *handlers) grep(pattern, path string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("grep: pattern is empty")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("grep: bad pattern: %w", err)
	}
	abs, err := h.validatePath(path)
	if err != nil {
		return "", fmt.Errorf("grep: %w", err)
	}
	rootAbs, _ := filepath.Abs(h.root)

	var matches []string
	search := func(p string) error {
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		rel, _ := filepath.Rel(rootAbs, p)
		rel = filepath.ToSlash(rel)
		for i, line := range strings.Split(string(b), "\n") {
			if re.MatchString(line) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", rel, i+1, line))
				if len(matches) >= maxGrepMatches {
					return fs.SkipAll
				}
			}
		}
		return nil
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("grep: stat %q: %w", path, err)
	}
	if info.IsDir() {
		walkErr := filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			return search(p)
		})
		if walkErr != nil && walkErr != fs.SkipAll {
			return "", fmt.Errorf("grep: walk %q: %w", path, walkErr)
		}
	} else {
		if err := search(abs); err != nil && err != fs.SkipAll {
			return "", fmt.Errorf("grep: search %q: %w", path, err)
		}
	}

	if len(matches) == 0 {
		return "no matches", nil
	}
	return strings.Join(matches, "\n"), nil
}

// readFile reads a corpus-root-relative file and returns its content.
func (h *handlers) readFile(path string) (string, error) {
	abs, err := h.validatePath(path)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	return string(b), nil
}
