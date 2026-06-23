package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// TrackLicense is one Exercism track's license-audit frontmatter block
// (SC#4 / ADAPTER-AIDER-01). It mirrors the verify_tos.go ProviderAttestation
// shape: a `---`-delimited YAML block, strict-decoded with KnownFields(true).
type TrackLicense struct {
	Track                       string `yaml:"track"`
	SourceRepo                  string `yaml:"source_repo"`
	License                     string `yaml:"license"`
	LicenseSHA256               string `yaml:"license_sha256"`
	RedistributionClauseExcerpt string `yaml:"redistribution_clause_excerpt"`
}

// FixtureLicense is the Apache-2.0 second-disposition audit block (VENDOR-03).
// It mirrors TrackLicense: a `---`-delimited YAML block, strict-decoded with
// KnownFields(true) so an unknown key fails closed. The keys match the Plan 01
// LICENSE-AUDIT.md fixture block exactly. notice_path / license_path are
// resolved relative to the audit file's directory and MUST exist on disk.
type FixtureLicense struct {
	Fixture     string `yaml:"fixture"`
	SourceRepo  string `yaml:"source_repo"`
	License     string `yaml:"license"`
	NoticePath  string `yaml:"notice_path"`
	LicensePath string `yaml:"license_path"`
}

// verifyLicenses reads LICENSE-AUDIT.md at path, extracts every `---`-delimited
// YAML frontmatter block that is a track block (carries a top-level `track:`
// key), strict-decodes each into a TrackLicense, and HARD-FAILS (returns a
// non-nil error) on: a malformed/unknown-key block, a missing required field
// (empty License or empty LicenseSHA256), or zero track blocks found. It returns
// the first error encountered. It checks structural/parse validity ONLY — not
// legal accuracy of the recorded license.
func verifyLicenses(path string) error {
	_, err := verifyLicensesCount(path)
	return err
}

// verifyLicensesCount is the implementation behind verifyLicenses. It also
// returns the number of track blocks validated, which the regression tests
// assert against (prove the COUNT is checked, not just that the file passes;
// count==0 is itself an error, mirroring verifyTOSCount). On error the count is
// the number validated before the failure.
func verifyLicensesCount(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("verify-licenses: cannot read %s: %w", path, err)
	}

	// Split the file on lines that are exactly `---` (horizontal rules AND
	// frontmatter fences alike), treating each as a separator. A segment is a
	// track block IFF it strict-decodes AND carries a non-empty top-level
	// `track:` key. This is immune to stray Markdown horizontal rules (the same
	// robustness verify_tos.go relies on via splitOnHorizontalRules).
	segments := splitOnHorizontalRules(string(data))

	tracks := 0
	for _, seg := range segments {
		// Cheap pre-filter: only segments with a top-level track: key are
		// candidates for strict-decode, so prose paragraphs are not spuriously
		// decoded. The authoritative signal is the post-decode tl.Track check.
		if !hasTopLevelTrackKey(seg) {
			continue
		}

		dec := yaml.NewDecoder(strings.NewReader(seg))
		dec.KnownFields(true) // unknown key → error → exit non-zero
		var tl TrackLicense
		if err := dec.Decode(&tl); err != nil {
			// A segment that looks like a track block (has a top-level track:
			// key) but fails strict decode FAILS CLOSED.
			return tracks, fmt.Errorf("verify-licenses: strict decode of a track block in %s failed: %w", path, err)
		}

		// Authoritative signal: a real, post-decode top-level `track` key.
		if tl.Track == "" {
			continue
		}
		tracks++

		if tl.License == "" {
			return tracks, fmt.Errorf("verify-licenses: track %q in %s has an empty license (SPDX) field", tl.Track, path)
		}
		if tl.LicenseSHA256 == "" {
			return tracks, fmt.Errorf("verify-licenses: track %q in %s has an empty license_sha256 field", tl.Track, path)
		}
	}

	if tracks == 0 {
		return 0, fmt.Errorf("verify-licenses: %s contains no track license blocks", path)
	}
	return tracks, nil
}

// hasTopLevelTrackKey reports whether seg contains a line whose first
// non-whitespace content is a top-level (non-indented) `track:` key. This is a
// pre-filter only; the authoritative signal is the post-decode
// TrackLicense.Track field.
func hasTopLevelTrackKey(seg string) bool {
	for _, line := range strings.Split(seg, "\n") {
		if line == strings.TrimLeft(line, " \t") &&
			strings.HasPrefix(strings.TrimSpace(line), "track:") {
			return true
		}
	}
	return false
}

// hasTopLevelFixtureKey reports whether seg contains a line whose first
// non-whitespace content is a top-level (non-indented) `fixture:` key. Pre-
// filter only; the authoritative signal is the post-decode FixtureLicense.Fixture
// field. Mirrors hasTopLevelTrackKey with `track:`→`fixture:`.
func hasTopLevelFixtureKey(seg string) bool {
	for _, line := range strings.Split(seg, "\n") {
		if line == strings.TrimLeft(line, " \t") &&
			strings.HasPrefix(strings.TrimSpace(line), "fixture:") {
			return true
		}
	}
	return false
}

// verifyLicensesFull is the dual-disposition gate (VENDOR-03). It runs the
// existing per-track audit (verifyLicensesCount), additionally strict-decodes
// every Apache-2.0 `fixture:` block into a FixtureLicense (KnownFields(true) —
// unknown key fails closed), asserts ≥1 fixture block (dual disposition is
// required, not optional), asserts each fixture block's notice_path/license_path
// sidecars exist on disk, and runs a BIDIRECTIONAL manifest-vs-disk crypto/sha256
// walk over treeRoot. It returns the first error encountered (fail-closed).
func verifyLicensesFull(auditPath, manifestPath, treeRoot string) error {
	// 1. Existing per-track MIT audit (unchanged behavior).
	if _, err := verifyLicensesCount(auditPath); err != nil {
		return err
	}

	// 2. Apache-2.0 fixture blocks: strict-decode + dual-disposition assertion +
	//    sidecar existence. Sidecar paths are resolved relative to the audit
	//    file's directory (the Plan 01 audit records them as `fixtures/...`).
	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		return fmt.Errorf("verify-licenses: cannot read %s: %w", auditPath, err)
	}
	auditDir := filepath.Dir(auditPath)
	// auditedLicenses is the set of SPDX values the audit asserts (across BOTH
	// track and fixture blocks). The manifest's per-file licenses must be a
	// subset of this set — a manifest row whose license is undisposed in the
	// audit (e.g. a flipped MIT→GPL row) fails closed.
	auditedLicenses := make(map[string]bool)
	apacheFixtures := 0
	fixtures := 0
	for _, seg := range splitOnHorizontalRules(string(auditData)) {
		if hasTopLevelTrackKey(seg) {
			// Re-decode track blocks (cheaply) only to collect their SPDX value
			// for the manifest cross-check; structural validity was already
			// enforced by verifyLicensesCount above.
			dec := yaml.NewDecoder(strings.NewReader(seg))
			dec.KnownFields(true)
			var tl TrackLicense
			if err := dec.Decode(&tl); err == nil && tl.Track != "" && tl.License != "" {
				auditedLicenses[tl.License] = true
			}
			continue
		}
		if !hasTopLevelFixtureKey(seg) {
			continue
		}
		dec := yaml.NewDecoder(strings.NewReader(seg))
		dec.KnownFields(true)
		var fl FixtureLicense
		if err := dec.Decode(&fl); err != nil {
			return fmt.Errorf("verify-licenses: strict decode of a fixture block in %s failed: %w", auditPath, err)
		}
		if fl.Fixture == "" {
			continue
		}
		fixtures++
		if fl.License == "" {
			return fmt.Errorf("verify-licenses: fixture %q in %s has an empty license (SPDX) field", fl.Fixture, auditPath)
		}
		auditedLicenses[fl.License] = true
		if fl.License == "Apache-2.0" {
			apacheFixtures++
		}
		for _, sidecar := range []struct{ kind, rel string }{
			{"notice_path", fl.NoticePath},
			{"license_path", fl.LicensePath},
		} {
			if sidecar.rel == "" {
				return fmt.Errorf("verify-licenses: fixture %q in %s has an empty %s field", fl.Fixture, auditPath, sidecar.kind)
			}
			if err := validatePathSegmentSafe(sidecar.rel); err != nil {
				return fmt.Errorf("verify-licenses: fixture %q %s %q: %w", fl.Fixture, sidecar.kind, sidecar.rel, err)
			}
			abs := filepath.Join(auditDir, filepath.FromSlash(sidecar.rel))
			if _, err := os.Stat(abs); err != nil {
				return fmt.Errorf("verify-licenses: fixture %q has a missing %s (%s): %w", fl.Fixture, sidecar.kind, sidecar.rel, err)
			}
		}
	}
	_ = fixtures
	if apacheFixtures == 0 {
		return fmt.Errorf("verify-licenses: %s contains no Apache-2.0 fixture blocks (dual disposition is required — a regression to single disposition fails closed)", auditPath)
	}

	// 3. Bidirectional manifest-vs-disk crypto/sha256 walk, with a per-file
	//    license cross-check against the audit dispositions.
	return verifyManifestVsDisk(manifestPath, treeRoot, auditedLicenses)
}

// verifyManifestVsDisk parses VENDOR-MANIFEST.md's per-file `path | sha256 | ...`
// rows into a map, walks treeRoot recomputing crypto/sha256 per file, and
// asserts (a) every on-disk file has a manifest entry with a MATCHING digest and
// (b) every manifest entry exists on disk. Both directions hard-fail. Manifest
// paths are containment-checked (no `..`, no absolute) so a crafted row cannot
// escape treeRoot.
func verifyManifestVsDisk(manifestPath, treeRoot string, auditedLicenses map[string]bool) error {
	manData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("verify-licenses: cannot read manifest %s: %w", manifestPath, err)
	}
	manifest, err := parseManifestRows(string(manData))
	if err != nil {
		return fmt.Errorf("verify-licenses: parsing manifest %s: %w", manifestPath, err)
	}
	if len(manifest) == 0 {
		return fmt.Errorf("verify-licenses: manifest %s contains no per-file sha256 rows", manifestPath)
	}

	// Per-file license cross-check: every license value the manifest records
	// must be disposed by an audit block (track or fixture). A flipped manifest
	// license (e.g. MIT→GPL) with no matching audit disposition fails closed.
	// Skipped when the caller passes no audit dispositions (nil map).
	if auditedLicenses != nil {
		for rel, row := range manifest {
			// WR-03: every vendored file must carry an explicit license
			// disposition. An empty license column is an unverifiable claim,
			// not "no claim to check" — treating it as exempt would silently
			// disable the license half of the gate for that row while the sha
			// half still passes. Fail closed.
			if row.license == "" {
				return fmt.Errorf("verify-licenses: manifest row %q has an empty license column", rel)
			}
			// The pre-existing hermetic stubs carry a non-SPDX
			// `internal — repo license` disposition recorded in the manifest
			// prose (not as an SPDX audit block); it is a recognized sentinel.
			if row.license == manifestInternalRepoLicense {
				continue
			}
			if !auditedLicenses[row.license] {
				return fmt.Errorf("verify-licenses: manifest license %q for %q is not disposed by any audit block", row.license, rel)
			}
		}
	}

	seen := make(map[string]bool, len(manifest))

	// Direction 1: every on-disk file has a matching manifest entry.
	walkErr := filepath.WalkDir(treeRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// WR-01: filepath.WalkDir lstat's each entry and does NOT follow
		// symlinks — a symlinked directory is reported as a single non-dir
		// entry whose target subtree is never walked (so files behind it are
		// never required to be manifest-pinned), and a symlinked file is
		// hashed by its dereferenced (possibly out-of-tree) target bytes. A
		// vendored third-party fixture tree must contain no symlinks, so we
		// refuse any symlink in the tree rather than let the integrity
		// contract silently fail to cover what it cannot traverse.
		if d.Type()&fs.ModeSymlink != 0 {
			rel, relErr := filepath.Rel(treeRoot, p)
			if relErr != nil {
				rel = p
			}
			return fmt.Errorf("verify-licenses: refusing to verify symlink %q in tree (integrity walk does not follow symlinks)", filepath.ToSlash(rel))
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(treeRoot, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		row, ok := manifest[rel]
		if !ok {
			return fmt.Errorf("verify-licenses: on-disk file %q has no manifest entry", rel)
		}
		got, err := fileSHA256(p)
		if err != nil {
			return err
		}
		if got != row.sha {
			return fmt.Errorf("verify-licenses: sha256 mismatch for %q (on-disk %s != manifest %s)", rel, got, row.sha)
		}
		seen[rel] = true
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	// Direction 2: every manifest entry exists on disk.
	for rel := range manifest {
		if !seen[rel] {
			return fmt.Errorf("verify-licenses: manifest entry %q has no on-disk file under %s", rel, treeRoot)
		}
	}
	return nil
}

// manifestInternalRepoLicense is the non-SPDX disposition the VENDOR-MANIFEST.md
// records for the pre-existing hermetic loader-test stubs (documented in the
// manifest prose, not as an SPDX audit block). It is a recognized sentinel in
// the per-file license cross-check.
const manifestInternalRepoLicense = "internal — repo license"

// manifestRow is one parsed per-file manifest entry.
type manifestRow struct {
	sha     string
	license string
}

// parseManifestRows extracts the per-file digest table rows from a
// VENDOR-MANIFEST.md. A per-file row is
// `| `<relpath>` | <64-hex sha256> | <license> | ... |`. The backtick-wrapped
// path + a 64-hex column-2 digest distinguishes these rows from the
// upstream-sources table (whose column 2 is a backtick-wrapped ref). Returns
// relpath→{sha,license}. Paths are containment-checked.
func parseManifestRows(s string) (map[string]manifestRow, error) {
	out := make(map[string]manifestRow)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cols := strings.Split(line, "|")
		// A well-formed table row is `| <path> | <sha> | <license> | <prov> |`,
		// which strings.Split on "|" yields as 6 fields: cols[0]="" (leading
		// `|`), cols[1]=path, cols[2]=sha256, cols[3]=license, cols[4]=
		// provenance, cols[5]="" (trailing `|`).
		//
		// WR-04: positional parsing makes a `|` embedded in a path, or a row
		// missing a column, silently shift every subsequent field. Previously a
		// shifted row landed a non-hex value in cols[2], failed isHex64, and was
		// silently `continue`d out of the map — a digest claim evaporating with
		// no diagnostic. Require a minimum column count for every `| `-prefixed
		// row so a malformed row is loud rather than dropped. The upstream-
		// sources table rows are also `| `-prefixed and well-formed (6 fields),
		// so this does not reject them; they are still skipped below by the
		// isHex64 row-type discriminator.
		if len(cols) < 5 {
			return nil, fmt.Errorf("manifest row %q has too few columns (| path | sha | license | provenance | expected)", line)
		}
		rel := strings.Trim(strings.TrimSpace(cols[1]), "`")
		sha := strings.TrimSpace(cols[2])
		if !isHex64(sha) {
			// Not a per-file digest row (e.g. the upstream-sources table whose
			// column 2 is a backtick-wrapped ref, not a 64-hex sha256).
			continue
		}
		license := strings.TrimSpace(cols[3])
		if err := validatePathSegmentSafe(rel); err != nil {
			return nil, fmt.Errorf("manifest path %q: %w", rel, err)
		}
		// WR-02: rows are keyed by relpath into a map, so two rows for the same
		// path would silently collapse last-wins — a wrong-digest row followed
		// by a correct row for the same path would pass the gate because only
		// the surviving entry is checked against disk. Reject duplicates so
		// every byte is pinned exactly once.
		if _, dup := out[rel]; dup {
			return nil, fmt.Errorf("manifest has a duplicate row for path %q", rel)
		}
		out[rel] = manifestRow{sha: sha, license: license}
	}
	return out, nil
}

// isHex64 reports whether s is exactly 64 lowercase-hex characters (a sha256).
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// fileSHA256 returns the lowercase-hex crypto/sha256 digest of the file at p.
func fileSHA256(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// validatePathSegmentSafe rejects a relative path that could escape its join
// root (the validatePathSegment containment idea, loader.go-style, T-99-02-04):
// it rejects empty, absolute, and any path that filepath.Clean rewrites or that
// contains a `..` parent reference.
func validatePathSegmentSafe(rel string) error {
	if rel == "" {
		return fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return fmt.Errorf("path is absolute")
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	if clean != rel || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return fmt.Errorf("path contains a parent ref or is non-canonical")
	}
	return nil
}

// newVerifyLicensesCmd returns the 'verify-licenses' subcommand. Like
// newVerifyTOSCmd it is NOT added to the root command tree (the --help
// acceptance fixes the subcommand count); it is invoked directly by the
// `make verify-licenses` Makefile target. It returns the error via RunE so the
// process exits non-zero on failure (no continue-on-error).
func newVerifyLicensesCmd() *cobra.Command {
	var manifestPath, treeRoot string
	cmd := &cobra.Command{
		Use:          "verify-licenses [path]",
		SilenceUsage: true,
		Short:        "Strict HARD-FAIL dual-disposition license gate for the Aider-Polyglot vendored tree",
		Long: `verify-licenses strict-decodes every per-track (MIT) and per-fixture
(Apache-2.0) license frontmatter block in LICENSE-AUDIT.md (default
bench/datasets/aider-polyglot/LICENSE-AUDIT.md) and exits NON-ZERO on a malformed
or unknown-key block, an empty license / sha256 field, a file with zero track
blocks, or zero Apache-2.0 fixture blocks (dual disposition is required).

When --manifest and --tree are supplied it additionally runs a BIDIRECTIONAL
manifest-vs-disk crypto/sha256 walk: every on-disk file under --tree must have a
manifest row with a matching digest, and every manifest row must exist on disk.
It checks structural validity + digest integrity ONLY — not legal accuracy.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "bench/datasets/aider-polyglot/LICENSE-AUDIT.md"
			if len(args) == 1 {
				path = args[0]
			}
			if manifestPath != "" || treeRoot != "" {
				if manifestPath == "" || treeRoot == "" {
					return fmt.Errorf("verify-licenses: --manifest and --tree must be supplied together")
				}
				if err := verifyLicensesFull(path, manifestPath, treeRoot); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "verify-licenses: %s + %s over %s OK (dual disposition + manifest-vs-disk walk)\n", path, manifestPath, treeRoot)
				return nil
			}
			if err := verifyLicenses(path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "verify-licenses: %s OK\n", path)
			return nil
		},
	}
	cmd.Flags().StringVar(&manifestPath, "manifest", "", "path to VENDOR-MANIFEST.md (enables the manifest-vs-disk sha256 walk)")
	cmd.Flags().StringVar(&treeRoot, "tree", "", "path to the vendored fixtures/ tree root (the --tree walk root)")
	return cmd
}
