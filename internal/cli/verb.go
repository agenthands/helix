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
	name     string
	kind     flagKind
	required bool
	help     string
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
		toolName: "search_for_pattern",
		short:    "Search the workspace for a pattern (representative one-shot verb)",
		flags: []verbFlag{
			{name: "workspace", kind: flagString, required: true, help: "Workspace directory"},
			{name: "query", kind: flagString, required: true, help: "Pattern to search for"},
			{name: "max-results", kind: flagInt, required: false, help: "Maximum results to return"},
			{name: "verbose", kind: flagBool, required: false, help: "Verbose output"},
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
		switch f.kind {
		case flagString:
			v, err := cmd.Flags().GetString(f.name)
			if err != nil {
				return nil, err
			}
			args[f.name] = v
		case flagInt:
			v, err := cmd.Flags().GetInt(f.name)
			if err != nil {
				return nil, err
			}
			args[f.name] = v
		case flagBool:
			v, err := cmd.Flags().GetBool(f.name)
			if err != nil {
				return nil, err
			}
			args[f.name] = v
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

	socketPath := config.DefaultSocketPath()
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
