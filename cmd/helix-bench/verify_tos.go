package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ProviderAttestation is one provider's TOS attestation frontmatter block (D-14).
// attested_on is a YYYY-MM-DD date parsed with dateLayout.
type ProviderAttestation struct {
	Provider              string `yaml:"provider"`
	TOSURL                string `yaml:"tos_url"`
	AttestedBy            string `yaml:"attested_by"`
	AttestedOn            string `yaml:"attested_on"`
	BenchmarkingPermitted bool   `yaml:"benchmarking_permitted"`
	PublishPermitted      bool   `yaml:"publish_permitted"`
}

// verifyTOS reads PROVIDERS.md at path, extracts every `---`-delimited YAML
// frontmatter block that is an attestation block (contains a `provider:` key),
// strict-decodes each into a ProviderAttestation, and HARD-FAILS (returns a
// non-nil error) on: a malformed/unknown-key block, a missing required field,
// an unparseable attested_on, or an attested_on more than stalenessWindowDays
// old relative to the INJECTED today (never time.Now() inside, D-11).
// It returns the first error encountered.
func verifyTOS(today time.Time, path string) error {
	_, err := verifyTOSCount(today, path)
	return err
}

// verifyTOSCount is the implementation behind verifyTOS. It additionally
// returns the number of provider attestation blocks that were validated, which
// the regression tests assert against (CR-01: prove the COUNT is checked, not
// just that the file passes). On error the count is the number validated before
// the failure.
func verifyTOSCount(today time.Time, path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("verify-tos: cannot read %s: %w", path, err)
	}

	today = today.UTC().Truncate(24 * time.Hour)
	staleCutoff := today.AddDate(0, 0, -stalenessWindowDays)

	// Split the file on lines that are exactly `---` (horizontal rules AND
	// frontmatter fences alike). Each resulting segment is a candidate; a
	// segment is an attestation IFF it strict-decodes AND carries a non-empty
	// top-level `provider:` key. This is robust to stray/leading Markdown
	// horizontal rules: a `---` rule simply produces an extra non-attestation
	// segment (prose) that is skipped, rather than mis-pairing the fences and
	// swallowing the next real attestation block (CR-01).
	segments := splitOnHorizontalRules(string(data))

	attestations := 0
	for _, seg := range segments {
		// Cheap pre-filter: only segments that mention a top-level provider key
		// are even candidates for strict-decode. This avoids attempting a strict
		// YAML decode of every prose paragraph (which would spuriously error on
		// Markdown content). The authoritative attestation test is the
		// post-decode `att.Provider != ""` check below (WR-05).
		if !hasTopLevelProviderKey(seg) {
			continue
		}

		dec := yaml.NewDecoder(strings.NewReader(seg))
		dec.KnownFields(true) // unknown key → error → exit non-zero (D-16)
		var att ProviderAttestation
		if err := dec.Decode(&att); err != nil {
			// A segment that looks like an attestation (has a top-level
			// provider: key) but fails strict decode FAILS CLOSED (WR-04).
			return attestations, fmt.Errorf("verify-tos: strict decode of an attestation block in %s failed: %w", path, err)
		}

		// Authoritative attestation signal: a real, post-decode top-level
		// `provider` key (WR-05), not a loose substring match.
		if att.Provider == "" {
			continue
		}
		attestations++

		if att.TOSURL == "" || att.AttestedBy == "" || att.AttestedOn == "" {
			return attestations, fmt.Errorf("verify-tos: attestation block for %q in %s is missing a required field", att.Provider, path)
		}

		attestedOn, err := time.Parse(dateLayout, att.AttestedOn)
		if err != nil {
			return attestations, fmt.Errorf("verify-tos: provider %q: cannot parse attested_on %q: %w", att.Provider, att.AttestedOn, err)
		}
		if attestedOn.Before(staleCutoff) {
			return attestations, fmt.Errorf("verify-tos: provider %q: attested_on %s is more than %dd stale (cutoff %s) — re-attest TOS freshness", att.Provider, att.AttestedOn, stalenessWindowDays, staleCutoff.Format(dateLayout))
		}
	}

	if attestations == 0 {
		return 0, fmt.Errorf("verify-tos: %s contains no provider attestation blocks", path)
	}
	return attestations, nil
}

// splitOnHorizontalRules splits s into segments delimited by lines that are
// exactly `---` (after trimming surrounding whitespace). Both Markdown
// horizontal rules and YAML frontmatter fences are such lines; treating them
// uniformly as separators — rather than as alternating open/close toggles —
// is what makes attestation extraction immune to stray horizontal rules
// (CR-01 / WR-04). The delimiter lines themselves are dropped.
func splitOnHorizontalRules(s string) []string {
	var segments []string
	lines := strings.Split(s, "\n")
	var cur []string
	flush := func() {
		segments = append(segments, strings.Join(cur, "\n"))
		cur = nil
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return segments
}

// hasTopLevelProviderKey reports whether seg contains a line whose first
// non-whitespace content is a top-level (non-indented) `provider:` key. This
// is a pre-filter only; the authoritative attestation signal is the
// post-decode ProviderAttestation.Provider field (WR-05).
func hasTopLevelProviderKey(seg string) bool {
	for _, line := range strings.Split(seg, "\n") {
		// A top-level YAML key has no leading indentation.
		if line == strings.TrimLeft(line, " \t") &&
			strings.HasPrefix(strings.TrimSpace(line), "provider:") {
			return true
		}
	}
	return false
}

// newVerifyTOSCmd returns the 'verify-tos' subcommand. It is NOT added to the
// root command tree (BENCH-02 fixes the subcommand count at five); it is
// invoked directly by the `make verify-tos` Makefile target via a dedicated
// entry. It passes time.Now().UTC() and returns the error via RunE so the
// process exits non-zero on failure (D-16, no continue-on-error).
func newVerifyTOSCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "verify-tos [path]",
		SilenceUsage: true,
		Short:        "Strict HARD-FAIL freshness gate for bench/PROVIDERS.md TOS attestations",
		Long: `verify-tos strict-decodes every provider attestation frontmatter block in
PROVIDERS.md (default bench/PROVIDERS.md) and exits NON-ZERO on a malformed or
unknown-key block, a missing required field, or an attested_on more than 90
days old. It checks freshness and parse validity ONLY — not legal accuracy.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "bench/PROVIDERS.md"
			if len(args) == 1 {
				path = args[0]
			}
			if err := verifyTOS(time.Now().UTC(), path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "verify-tos: %s OK\n", path)
			return nil
		},
	}
}
