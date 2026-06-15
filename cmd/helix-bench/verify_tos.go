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
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("verify-tos: cannot read %s: %w", path, err)
	}

	blocks := extractFrontmatterBlocks(string(data))
	today = today.UTC().Truncate(24 * time.Hour)
	staleCutoff := today.AddDate(0, 0, -stalenessWindowDays)

	attestations := 0
	for _, block := range blocks {
		// Only treat blocks that declare a provider as attestation blocks; the
		// top-of-file field-set table and prose use `---` as horizontal rules.
		if !strings.Contains(block, "provider:") {
			continue
		}
		attestations++

		dec := yaml.NewDecoder(strings.NewReader(block))
		dec.KnownFields(true) // unknown key → error → exit non-zero (D-16)
		var att ProviderAttestation
		if err := dec.Decode(&att); err != nil {
			return fmt.Errorf("verify-tos: strict decode of an attestation block in %s failed: %w", path, err)
		}

		if att.Provider == "" || att.TOSURL == "" || att.AttestedBy == "" || att.AttestedOn == "" {
			return fmt.Errorf("verify-tos: attestation block for %q in %s is missing a required field", att.Provider, path)
		}

		attestedOn, err := time.Parse(dateLayout, att.AttestedOn)
		if err != nil {
			return fmt.Errorf("verify-tos: provider %q: cannot parse attested_on %q: %w", att.Provider, att.AttestedOn, err)
		}
		if attestedOn.Before(staleCutoff) {
			return fmt.Errorf("verify-tos: provider %q: attested_on %s is more than %dd stale (cutoff %s) — re-attest TOS freshness", att.Provider, att.AttestedOn, stalenessWindowDays, staleCutoff.Format(dateLayout))
		}
	}

	if attestations == 0 {
		return fmt.Errorf("verify-tos: %s contains no provider attestation blocks", path)
	}
	return nil
}

// extractFrontmatterBlocks returns the contents of every `---`-delimited block
// in s. A block opens on a line that is exactly `---` and closes on the next
// line that is exactly `---`; the lines between (exclusive) are one block.
func extractFrontmatterBlocks(s string) []string {
	var blocks []string
	lines := strings.Split(s, "\n")
	inBlock := false
	var cur []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if inBlock {
				blocks = append(blocks, strings.Join(cur, "\n"))
				cur = nil
				inBlock = false
			} else {
				inBlock = true
			}
			continue
		}
		if inBlock {
			cur = append(cur, line)
		}
	}
	return blocks
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
