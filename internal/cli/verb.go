package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

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
	flags    []verbFlag
}

// representativeVerb is the single spine verb this phase ships. Phase 91
// generates the full verb set from the tool registry; here one representative
// verb proves the load-bearing one-shot round-trip end to end.
const representativeVerb = "search"

// verbSpecs is the verb→tool registry. Phase 91 will populate this from the
// generated tool catalog; this plan carries exactly one representative entry.
var verbSpecs = map[string]verbSpec{
	representativeVerb: {
		// search_in_files is the real registered workspace-search tool. (The
		// 90-03 spine pointed at a non-existent "search_for_pattern"; the daemon
		// rejects it with `unknown tool`. Fixed here so the verb round-trips a
		// successful tools/call — see 90-04 SUMMARY deviations.)
		toolName: "search_in_files",
		short:    "Search the active workspace for a regex pattern (representative one-shot verb)",
		flags: []verbFlag{
			// --query maps to the tool's `pattern` (regex) argument. search_in_files
			// operates on the daemon's ACTIVE workspace; the SDK rejects unknown
			// args (e.g. repo_path) for this tool, so workspace activation is a
			// separate concern (activate_project / lazy-init), not a search arg.
			{name: "query", toolArg: "pattern", kind: flagString, required: true, help: "Regex pattern to search for"},
			{name: "max-results", toolArg: "max_results", kind: flagInt, required: false, help: "Maximum results to return"},
			{name: "context-lines", toolArg: "context_lines", kind: flagInt, required: false, help: "Context lines before/after each match"},
		},
	},
}

// newVerbCommand builds the verb-dispatch spine: a parent command with one
// subcommand per registered verb. Each subcommand maps its flags to a tool
// arguments map (validated pre-dial) and issues a single MCP tools/call via the
// one-shot helper over the existing StreamMCP wire.
func newVerbCommand() *cobra.Command {
	parent := &cobra.Command{
		Use:           "call",
		Short:         "Run a single code-intelligence tool call against the warm daemon",
		Long:          "Dispatches a one-shot MCP tools/call to the Helix daemon (auto-starting it if needed) and prints the result. Phase 91 generates the full always-visible verb set; this spine ships one representative verb.",
		GroupID:       groupWorkspace,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	for verb, spec := range verbSpecs {
		parent.AddCommand(newVerbSubcommand(verb, spec))
	}
	return parent
}

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
