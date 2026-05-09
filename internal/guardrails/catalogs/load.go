package catalogs

import (
	"path"
	"regexp"
	"strings"

	"github.com/agenthands/helix/internal/semantic"
	"gopkg.in/yaml.v3"
)

// Catalog holds the merged security-sensitive detection patterns for a single language.
type Catalog struct {
	ImportPatterns     []string
	PathGlobs          []string
	IdentifierPatterns []string
}

// rawCatalog is the YAML wire shape for embedded catalog files.
type rawCatalog struct {
	ImportPatterns     []string `yaml:"import_patterns"`
	PathGlobs          []string `yaml:"path_globs"`
	IdentifierPatterns []string `yaml:"identifier_patterns"`
}

// Load returns the merged Catalog for language (e.g., "go", "typescript",
// "javascript", "python"). It reads the embedded YAML for that language and
// then applies operator overrides from cfg:
//
//   - cfg.G005.ImportPatterns[language] REPLACES (not merges) the default
//     import_patterns when non-nil and non-empty (T-66-17 operator-wins rule).
//   - cfg.G005.PathGlobs (global) is merged (union) with the embedded defaults.
//   - cfg.G005.IdentifierPatterns (global) is merged (union) with the embedded defaults.
//
// Unknown language returns an empty Catalog and nil error.
func Load(language string, cfg semantic.GuardrailsConfig) (Catalog, error) {
	fileName := language + ".yaml"
	data, err := EmbeddedCatalogs.ReadFile(fileName)
	if err != nil {
		// Unknown language — no catalog file for this language; return empty.
		return Catalog{}, nil
	}

	var raw rawCatalog
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Catalog{}, err
	}

	cat := Catalog{
		ImportPatterns:     raw.ImportPatterns,
		PathGlobs:          raw.PathGlobs,
		IdentifierPatterns: raw.IdentifierPatterns,
	}

	// Apply operator overrides from cfg.G005.
	// Per D-22: cfg.G005.ImportPatterns[lang] OVERRIDES (replaces) the
	// embedded default for that language when non-nil and non-empty.
	if overrideImports, ok := cfg.G005.ImportPatterns[language]; ok && len(overrideImports) > 0 {
		cat.ImportPatterns = overrideImports
	}

	// cfg.G005.PathGlobs (global) is merged with embedded defaults (union, no dedup required).
	if len(cfg.G005.PathGlobs) > 0 {
		cat.PathGlobs = append(cat.PathGlobs, cfg.G005.PathGlobs...)
	}

	// cfg.G005.IdentifierPatterns (global) is merged with embedded defaults.
	if len(cfg.G005.IdentifierPatterns) > 0 {
		cat.IdentifierPatterns = append(cat.IdentifierPatterns, cfg.G005.IdentifierPatterns...)
	}

	return cat, nil
}

// MatchesImportPattern returns true if importPath matches any pattern in patterns.
//
// Five pattern syntaxes are supported (per D-22):
//
//  1. Exact match:        "bcrypt" matches "bcrypt" only.
//  2. Suffix wildcard:    "crypto/*" matches "crypto/aes", "crypto/rand", etc.
//     but NOT "crypto" itself.
//  3. Prefix wildcard:    "passport-*" matches "passport-local", "passport-jwt".
//  4. Scoped wildcard:    "@auth/*" matches "@auth/core", "@auth/sveltekit".
//  5. Dotted prefix:      "cryptography.*" matches "cryptography.fernet",
//     "cryptography.hazmat.primitives.ciphers".
//
// Patterns without wildcards are matched exactly (case-sensitive).
func MatchesImportPattern(importPath string, patterns []string) bool {
	for _, p := range patterns {
		if matchSingle(importPath, p) {
			return true
		}
	}
	return false
}

// matchSingle applies a single pattern to importPath.
func matchSingle(importPath, pattern string) bool {
	switch {
	// Dotted prefix wildcard: "cryptography.*"
	case strings.HasSuffix(pattern, ".*"):
		prefix := strings.TrimSuffix(pattern, ".*")
		// Must match "prefix" exactly OR "prefix." + something.
		return importPath == prefix || strings.HasPrefix(importPath, prefix+".")

	// Suffix wildcard: "crypto/*"
	case strings.HasSuffix(pattern, "/*"):
		// The prefix is everything before the trailing "/*".
		prefix := strings.TrimSuffix(pattern, "/*")
		if !strings.HasPrefix(importPath, prefix+"/") {
			return false
		}
		// Must have at least one character after the slash.
		remainder := importPath[len(prefix)+1:]
		return len(remainder) > 0

	// Prefix wildcard: "passport-*" or "@nextauth/*" handled by the case above,
	// but bare "prefix-*" (no /) falls here.
	case strings.HasSuffix(pattern, "-*"):
		prefix := strings.TrimSuffix(pattern, "-*")
		return strings.HasPrefix(importPath, prefix+"-")

	// Exact match.
	default:
		return importPath == pattern
	}
}

// MatchesPathGlob returns true if filePath matches any glob in globs.
//
// Supports doublestar-style "**" which matches zero or more path segments.
// Internally, "**" is expanded by splitting the path into segments and matching
// the non-"**" parts using stdlib path.Match. No external dependencies required.
func MatchesPathGlob(filePath string, globs []string) bool {
	// Normalise to forward slashes (handles Windows-style paths if ever needed).
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	for _, g := range globs {
		if matchGlob(filePath, g) {
			return true
		}
	}
	return false
}

// matchGlob implements doublestar "**" glob matching.
// The algorithm splits both the pattern and the file path on "/" and walks
// them together. When a "**" segment is encountered it consumes zero or more
// file-path segments.
func matchGlob(filePath, pattern string) bool {
	// Strip leading "./" from pattern for normalisation.
	pattern = strings.TrimPrefix(pattern, "./")
	filePath = strings.TrimPrefix(filePath, "./")

	patSegs := strings.Split(pattern, "/")
	fileSegs := strings.Split(filePath, "/")
	return matchSegments(fileSegs, patSegs)
}

// matchSegments recursively matches fileSegs against patSegs.
func matchSegments(fileSegs, patSegs []string) bool {
	for len(patSegs) > 0 {
		pat := patSegs[0]
		patSegs = patSegs[1:]

		if pat == "**" {
			// "**" matches zero or more path segments.
			// Try matching the remaining pattern against every suffix of fileSegs.
			for i := 0; i <= len(fileSegs); i++ {
				if matchSegments(fileSegs[i:], patSegs) {
					return true
				}
			}
			return false
		}

		if len(fileSegs) == 0 {
			return false
		}

		matched, err := path.Match(pat, fileSegs[0])
		if err != nil || !matched {
			return false
		}
		fileSegs = fileSegs[1:]
	}

	return len(fileSegs) == 0
}

// MatchesIdentifier returns true if name matches any pattern in patterns.
// Each pattern is treated as a fully-anchored Go regular expression.
// Invalid patterns are silently skipped (no match).
func MatchesIdentifier(name string, patterns []string) bool {
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		if re.MatchString(name) {
			return true
		}
	}
	return false
}
