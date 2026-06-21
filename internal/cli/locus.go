package cli

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// locus.go is the pure parse + path-normalization + sort/dedup core of the
// Phase 92 terse renderer. It turns the daemon's heterogeneous pre-formatted
// tool text into structured (relpath, line, col, payload) tuples. It does NOT
// print or color — the renderer (92-02) owns layout and color.
//
// Coordinate convention (CRITICAL): the daemon already emits 1-based loci
// (internal/kernel/symbols/tools.go:189-190 adds +1 to LSP 0-based coords at
// the formatLocations boundary). CONTEXT/ROADMAP say "convert LSP 0-based at
// the render boundary" — that conversion is ALREADY done by the daemon's +1.
// This parser is the render boundary's CONSUMER: it PARSES the already-1-based
// value and must NOT re-convert. Re-converting here would double-shift and
// produce off-by-one goldens.

// locus is a parsed daemon locus line. relpath is workspace-relative
// (forward-slash normalized) unless abs parsing was requested. line/col are
// 1-based exactly as the daemon emitted them (no re-conversion). payload is the
// trailing text after the coordinates (a symbol name+kind, a code preview, or a
// search match line), empty for the bare locus form.
type locus struct {
	relpath string
	payload string
	line    int
	col     int
}

// parseLocusLine recognizes the two daemon locus grammars and returns a
// structured locus. It returns ok=false for any line that matches neither
// grammar (the "(no results)" sentinel, markdown fences, hover prose) so the
// caller passes them through verbatim — it never panics on malformed daemon
// text (T-92-01 DoS mitigation; Security V5: never drop output, never crash).
//
// The two grammars (see internal/kernel/symbols/tools.go formatLocations and
// internal/kernel/fileops/search.go):
//
//	(a) "<path>:<L>:<C>[ — <payload>]"  — file:// abs path, has a column
//	(b) "<path>:<L>: <text>"            — fileops relpath, NO column (col→1)
//
// When abs is false and workspaceRoot is non-empty, an absolute path is made
// workspace-relative via filepath.Rel then filepath.ToSlash. When abs is true
// the absolute path is kept. Already-relative inputs (grammar b) are left as-is
// but still ToSlash-normalized for golden stability across OSes.
func parseLocusLine(line, workspaceRoot string, abs bool) (locus, bool) {
	raw := line
	if raw == "" {
		return locus{}, false
	}

	// Split off an optional " — <payload>" suffix (em-dash separator emitted by
	// formatLocations when a Name/Preview is present). Only the bare-locus /
	// file:// grammar uses this; the search grammar uses ": <text>" instead.
	pathCoords := raw
	payload := ""
	if i := strings.Index(raw, " — "); i >= 0 {
		pathCoords = raw[:i]
		payload = strings.TrimSpace(raw[i+len(" — "):])
	}

	// Strip a leading file:// prefix (handle the file:/// host-empty form → /).
	hadScheme := false
	if strings.HasPrefix(pathCoords, "file://") {
		hadScheme = true
		pathCoords = strings.TrimPrefix(pathCoords, "file://")
		// file:///abs → /abs ; file://host/abs is not emitted by the daemon but
		// guard anyway: an empty host leaves a leading slash already.
	}

	// Parse from the right: the trailing ":<L>:<C>" or ":<L>:" tokens.
	// Grammar (a): path:L:C            (two trailing numeric colon tokens)
	// Grammar (b): path:L: <text>      (one trailing numeric colon, then text)
	//
	// Distinguish them: if we already split a " — payload", we are in grammar
	// (a) territory (or bare). Otherwise a ": <text>" tail signals grammar (b).

	// Detect grammar (b): a "<path>:<L>: <text>" shape (the colon after L is
	// followed by a space-delimited text, and there is no second numeric token).
	if payload == "" {
		if l, txt, ok := parseSearchForm(pathCoords); ok {
			path := normalizePath(l.path, workspaceRoot, abs, hadScheme)
			if path == "" {
				return locus{}, false
			}
			return locus{relpath: path, line: l.line, col: 1, payload: txt}, true
		}
	}

	// Grammar (a): "<path>:<L>:<C>".
	path, lnum, cnum, ok := parseColonLC(pathCoords)
	if !ok {
		return locus{}, false
	}
	np := normalizePath(path, workspaceRoot, abs, hadScheme)
	if np == "" {
		return locus{}, false
	}
	return locus{relpath: np, line: lnum, col: cnum, payload: payload}, true
}

// searchHead holds the path+line of a parsed grammar-(b) head.
type searchHead struct {
	path string
	line int
}

// parseSearchForm matches the fileops "<path>:<L>: <text>" grammar (no column).
// It requires a trailing ": <text>" where the token immediately before that
// colon is numeric (the line) and there is NO further numeric colon token
// (which would make it grammar (a)). Returns ok=false otherwise.
func parseSearchForm(s string) (searchHead, string, bool) {
	// Find "<line>: " — the line number followed by a colon and a space.
	// Walk from the left looking for the FIRST "<digits>: " boundary that
	// leaves a non-coordinate remainder. The path itself may contain colons
	// only on Windows drive letters, which the daemon does not emit here, so a
	// simple "split on the first ':<digits>: '" is sufficient.
	for i := 0; i < len(s); i++ {
		if s[i] != ':' {
			continue
		}
		// Consume digits after this colon.
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i+1 {
			continue // no digits — not a line token
		}
		// Must be followed by ":" for the line-colon boundary.
		if j < len(s) && s[j] == ':' {
			text := s[j+1:]
			// Reject grammar (a): if the remainder is a pure numeric column
			// token (optionally with a " — payload" already stripped), this is
			// "<path>:<L>:<C>", not the search form. Defer to parseColonLC.
			if isAllDigits(strings.TrimSpace(text)) {
				return searchHead{}, "", false
			}
			path := s[:i]
			lnum, err := strconv.Atoi(s[i+1 : j])
			if err != nil || path == "" {
				return searchHead{}, "", false
			}
			text = strings.TrimPrefix(text, " ")
			return searchHead{path: path, line: lnum}, text, true
		}
	}
	return searchHead{}, "", false
}

// isAllDigits reports whether s is non-empty and consists solely of ASCII
// digits — used to tell a trailing numeric column (grammar a) apart from a
// search-match text (grammar b).
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseColonLC matches the "<path>:<L>:<C>" grammar, parsing the two rightmost
// colon-delimited numeric tokens as line and col. Returns ok=false if either
// trailing token is non-numeric or the path is empty.
func parseColonLC(s string) (path string, line, col int, ok bool) {
	last := strings.LastIndex(s, ":")
	if last < 0 {
		return "", 0, 0, false
	}
	colTok := s[last+1:]
	rest := s[:last]
	prev := strings.LastIndex(rest, ":")
	if prev < 0 {
		return "", 0, 0, false
	}
	lineTok := rest[prev+1:]
	path = rest[:prev]
	if path == "" {
		return "", 0, 0, false
	}
	c, err1 := strconv.Atoi(colTok)
	l, err2 := strconv.Atoi(lineTok)
	if err1 != nil || err2 != nil {
		return "", 0, 0, false
	}
	return path, l, c, true
}

// normalizePath converts a parsed path to its final relpath form. When abs is
// true the path is kept as-is (ToSlash for stability). Otherwise, an absolute
// path under workspaceRoot is made relative via filepath.Rel; already-relative
// paths are kept. All outputs are filepath.ToSlash-normalized for golden
// stability across OSes.
func normalizePath(path, workspaceRoot string, abs, hadScheme bool) string {
	if path == "" {
		return ""
	}
	if abs {
		return toSlash(path)
	}
	// A file:// path or an OS-absolute path: try to relativize against root.
	if workspaceRoot != "" && (hadScheme || filepath.IsAbs(path)) {
		if rel, err := filepath.Rel(workspaceRoot, path); err == nil {
			return toSlash(rel)
		}
	}
	return toSlash(path)
}

// toSlash forward-slash-normalizes a path for golden stability. It applies
// filepath.ToSlash (a no-op on POSIX) AND an unconditional backslash→slash
// replacement so a daemon line carrying Windows-style separators yields a
// stable forward-slash relpath regardless of the host OS the CLI runs on.
func toSlash(p string) string {
	return strings.ReplaceAll(filepath.ToSlash(p), `\`, "/")
}

// sortDedupLoci sorts by (relpath, line, col) and drops exact-duplicate tuples
// so repeated runs over the same multiset produce byte-identical output
// (OUT-02 determinism). Duplicate detection compares the full locus including
// payload. The input slice is not mutated.
func sortDedupLoci(in []locus) []locus {
	if len(in) == 0 {
		return nil
	}
	out := make([]locus, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool {
		if out[i].relpath != out[j].relpath {
			return out[i].relpath < out[j].relpath
		}
		if out[i].line != out[j].line {
			return out[i].line < out[j].line
		}
		if out[i].col != out[j].col {
			return out[i].col < out[j].col
		}
		return out[i].payload < out[j].payload
	})
	deduped := out[:0]
	var prev locus
	for i, l := range out {
		if i > 0 && l == prev {
			continue
		}
		deduped = append(deduped, l)
		prev = l
	}
	return deduped
}
