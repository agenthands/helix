package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/forwarder"
)

// callToolFn is an overridable seam over the real one-shot helper
// (forwarder.CallTool). Production uses the real function; verb_test.go
// substitutes a fake so flag→args mapping and the "required arg errors before
// dialing" behavior can be asserted without spawning or dialing a daemon.
var callToolFn = forwarder.CallTool

// flagKind enumerates the supported flag value types for a verb argument.
type flagKind int

const (
	flagString flagKind = iota
	flagInt
	flagBool
	// flagStringSlice maps a `[]string` tool argument to a cobra StringSlice
	// flag (repeatable / comma-separated).
	flagStringSlice
	// flagJSON maps a non-scalar/opaque tool argument (e.g.
	// []guardrails.ReceiptID, maps) to a `--<name>-json` string flag whose value
	// is JSON-unmarshalled into the args map at dial time. This is the
	// generator's deterministic policy for arg types with no scalar flag mapping.
	flagJSON
)

// verbFlag describes a single flag a verb accepts and how it maps to a tool
// argument.
type verbFlag struct {
	name string
	// toolArg is the underlying MCP tool argument key this flag maps to. When
	// empty the flag name is used verbatim. This indirection lets a user-facing
	// flag (e.g. --workspace) map to the tool's actual schema key (repo_path)
	// without forcing the CLI vocabulary to leak the tool's internal names.
	toolArg  string
	kind     flagKind
	required bool
	help     string
}

// argKey returns the tool-argument key for a flag (toolArg override or the
// flag name).
func (f verbFlag) argKey() string {
	if f.toolArg != "" {
		return f.toolArg
	}
	return f.name
}

// verbSpec maps a CLI verb to an underlying MCP tool Name plus its flag set.
type verbSpec struct {
	toolName string
	short    string
	// groupID is the cobra command-group the generated verb attaches to on the
	// ROOT command (one of the 6 capability groups). Empty for the legacy spine
	// verb (which attaches under the workspace group via newVerbCommand).
	groupID string
	flags   []verbFlag
}

// VerbToolNames returns the sorted set of underlying tool names from the verb
// catalog (verbSpecs), one entry per verb. It is the read-only seam
// out-of-package consumers (e.g. the 91-03 integration tests) use to read the
// generated verb surface without touching internal state — the returned slice
// is a fresh copy, so mutating it does not affect verbSpecs.
func VerbToolNames() []string {
	names := make([]string, 0, len(verbSpecs))
	for _, spec := range verbSpecs {
		names = append(names, spec.toolName)
	}
	sort.Strings(names)
	return names
}

// registerGeneratedVerbs attaches one root subcommand per entry in verbSpecs,
// grouped by capability (flatten-onto-root per Open Q1; replaces the legacy
// `call` parent). Each verb reuses the spine's newVerbSubcommand so the pre-dial
// required-flag validation in buildVerbArgs is preserved for free.
func registerGeneratedVerbs(rootCmd *cobra.Command) {
	verbs := make([]string, 0, len(verbSpecs))
	for v := range verbSpecs {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)
	for _, v := range verbs {
		spec := verbSpecs[v]
		sub := newVerbSubcommand(v, spec)
		sub.GroupID = spec.groupID
		rootCmd.AddCommand(sub)
	}
}

// verbSpecs is the verb→tool catalog. It is GENERATED into verbs_gen.go by
// cmd/helix-cligen (one entry per live-registry tool) and must NOT be hand-
// edited. The generator owns this variable; the drift gate
// (`go run ./cmd/helix-cligen --check`) fails CI if it is stale.

// newVerbSubcommand builds the cobra subcommand for a single verb.
func newVerbSubcommand(verb string, spec verbSpec) *cobra.Command {
	sub := &cobra.Command{
		Use:           verb,
		Short:         spec.short,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runVerb(cmd, spec)
		},
	}
	for _, f := range spec.flags {
		switch f.kind {
		case flagString:
			sub.Flags().String(f.name, "", f.help)
		case flagInt:
			sub.Flags().Int(f.name, 0, f.help)
		case flagBool:
			sub.Flags().Bool(f.name, false, f.help)
		case flagStringSlice:
			sub.Flags().StringSlice(f.name, nil, f.help)
		case flagJSON:
			sub.Flags().String(f.name, "", f.help)
		}
	}
	return sub
}

// buildVerbArgs maps the command's parsed flags to a tool Arguments map with the
// declared value types, validating required-ness. It performs NO network I/O so
// it (and the required-arg validation) run BEFORE any dial.
func buildVerbArgs(cmd *cobra.Command, spec verbSpec) (map[string]any, error) {
	args := make(map[string]any, len(spec.flags))
	for _, f := range spec.flags {
		changed := cmd.Flags().Changed(f.name)
		if f.required && !changed {
			return nil, fmt.Errorf("required flag --%s not set", f.name)
		}
		// Only forward flags the caller actually set, except required flags
		// (which are guaranteed set by the check above).
		if !changed && !f.required {
			continue
		}
		key := f.argKey()
		switch f.kind {
		case flagString:
			v, err := cmd.Flags().GetString(f.name)
			if err != nil {
				return nil, err
			}
			args[key] = v
		case flagInt:
			v, err := cmd.Flags().GetInt(f.name)
			if err != nil {
				return nil, err
			}
			args[key] = v
		case flagBool:
			v, err := cmd.Flags().GetBool(f.name)
			if err != nil {
				return nil, err
			}
			args[key] = v
		case flagStringSlice:
			v, err := cmd.Flags().GetStringSlice(f.name)
			if err != nil {
				return nil, err
			}
			args[key] = v
		case flagJSON:
			raw, err := cmd.Flags().GetString(f.name)
			if err != nil {
				return nil, err
			}
			// Unmarshal the raw JSON into a generic value so the daemon receives
			// the structured argument (e.g. a []ReceiptID array) rather than a
			// string. An empty value for a non-required flag is skipped above.
			var decoded any
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				return nil, fmt.Errorf("flag --%s: invalid JSON: %w", f.name, err)
			}
			args[key] = decoded
		}
	}
	return args, nil
}

// runVerb builds the args (pre-dial validation), issues the one-shot tools/call,
// and renders the result. Terse relpath:line:col rendering is Phase 92 — here
// raw text / compact JSON is acceptable.
func runVerb(cmd *cobra.Command, spec verbSpec) error {
	args, err := buildVerbArgs(cmd, spec)
	if err != nil {
		return err
	}

	socketPath := resolveVerbSocket(cmd)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	res, err := callToolFn(cmd.Context(), socketPath, logger, CurrentVersion(), spec.toolName, args)
	if err != nil {
		return fmt.Errorf("calling %s: %w", spec.toolName, err)
	}

	renderResult(cmd, res)
	if res.IsError {
		return fmt.Errorf("tool %s reported an error", spec.toolName)
	}
	return nil
}

// resolveVerbSocket resolves the daemon socket the one-shot call dials, with the
// precedence: inherited root --socket flag > HELIX_SOCKET env > the per-uid
// default (config.DefaultSocketPath). The flag lets a user point a verb at a
// non-default daemon; the HELIX_SOCKET env is the hook the HELIX_BIN-gated E2E
// oracle (90-04) uses to point a real `helix call` subprocess at an isolated
// sandbox daemon socket without depending on os.TempDir layout.
func resolveVerbSocket(cmd *cobra.Command) string {
	// IN-05: --socket lives on the ROOT command only; newVerbSubcommand defines no
	// per-verb --socket, so a local-flag lookup on the verb is dead code. Read the
	// inherited root flag directly.
	if cmd != nil {
		if root := cmd.Root(); root != nil {
			if v, _ := root.Flags().GetString("socket"); v != "" {
				return v
			}
		}
	}
	if env := os.Getenv("HELIX_SOCKET"); env != "" {
		return env
	}
	return config.DefaultSocketPath()
}

// renderResult prints the tool result to stdout: text content verbatim,
// non-text content as compact JSON. (Terse rendering is Phase 92.)
func renderResult(cmd *cobra.Command, res *mcpsdk.CallToolResult) {
	out := cmd.OutOrStdout()
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			fmt.Fprintln(out, tc.Text)
			continue
		}
		if b, err := json.Marshal(c); err == nil {
			fmt.Fprintln(out, string(b))
		}
	}
}
