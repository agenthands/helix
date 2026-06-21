package main

import (
	"fmt"
	"os"
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

// newVerifyLicensesCmd returns the 'verify-licenses' subcommand. Like
// newVerifyTOSCmd it is NOT added to the root command tree (the --help
// acceptance fixes the subcommand count); it is invoked directly by the
// `make verify-licenses` Makefile target. It returns the error via RunE so the
// process exits non-zero on failure (no continue-on-error).
func newVerifyLicensesCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "verify-licenses [path]",
		SilenceUsage: true,
		Short:        "Strict HARD-FAIL per-track license gate for the Aider-Polyglot LICENSE-AUDIT.md",
		Long: `verify-licenses strict-decodes every per-track license frontmatter block in
LICENSE-AUDIT.md (default bench/datasets/aider-polyglot/LICENSE-AUDIT.md) and
exits NON-ZERO on a malformed or unknown-key block, an empty license / sha256
field, or a file with zero track blocks. It checks structural validity and the
presence of a recorded license + sha256 ONLY — not legal accuracy.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "bench/datasets/aider-polyglot/LICENSE-AUDIT.md"
			if len(args) == 1 {
				path = args[0]
			}
			if err := verifyLicenses(path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "verify-licenses: %s OK\n", path)
			return nil
		},
	}
}
