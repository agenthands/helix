package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"

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

// FlagDoc is the read-only, doc-facing view of a single verb flag. It mirrors the
// package-private verbFlag fields with the flagKind rendered as a stable lowercase
// string token ("string"/"int"/"bool"/"string-slice"/"json") so an out-of-package
// generator (cmd/helix-refgen) can render the reference deterministically without
// reaching into verbSpecs.
type FlagDoc struct {
	Name     string
	ToolArg  string
	Kind     string
	Required bool
	Help     string
}

// VerbDoc is the read-only, doc-facing view of a single verb (a verbSpec keyed by
// its kebab verb name) plus its flags. It is the out-of-package seam refgen uses to
// render reference.md.
type VerbDoc struct {
	Verb     string
	ToolName string
	GroupID  string
	Short    string
	Flags    []FlagDoc
}

// flagKindToken maps the package-private flagKind to a stable lowercase string
// token used in the generated reference. The mapping is exhaustive over the
// declared flagKind constants; an unknown kind renders as "string" (the scalar
// default) so the generator never panics on a future kind.
func flagKindToken(k flagKind) string {
	switch k {
	case flagString:
		return "string"
	case flagInt:
		return "int"
	case flagBool:
		return "bool"
	case flagStringSlice:
		return "string-slice"
	case flagJSON:
		return "json"
	default:
		return "string"
	}
}

// VerbSpecsForDocs returns the verb catalog (verbSpecs) as a sorted, doc-facing
// view: one VerbDoc per verb, sorted by the kebab verb key, each carrying a fresh
// deep copy of its flag slice. It is the read-only seam out-of-package consumers
// (e.g. cmd/helix-refgen) use to render per-verb documentation without touching
// internal state or depending on the daemon's InputSchema — the returned slices
// are fresh copies, so mutating them (or their flag slices) does not affect
// verbSpecs. Mirrors VerbToolNames's fresh-copy discipline.
func VerbSpecsForDocs() []VerbDoc {
	verbs := make([]string, 0, len(verbSpecs))
	for v := range verbSpecs {
		verbs = append(verbs, v)
	}
	sort.Strings(verbs)

	docs := make([]VerbDoc, 0, len(verbs))
	for _, v := range verbs {
		spec := verbSpecs[v]
		flags := make([]FlagDoc, 0, len(spec.flags))
		for _, f := range spec.flags {
			flags = append(flags, FlagDoc{
				Name:     f.name,
				ToolArg:  f.toolArg,
				Kind:     flagKindToken(f.kind),
				Required: f.required,
				Help:     f.help,
			})
		}
		docs = append(docs, VerbDoc{
			Verb:     v,
			ToolName: spec.toolName,
			GroupID:  spec.groupID,
			Short:    spec.short,
			Flags:    flags,
		})
	}
	return docs
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
// and renders the result via the Phase 92 terse renderer (render.go dispatches
// on the tool's render class and resolves --abs/--json/--color CLI-side).
func runVerb(cmd *cobra.Command, spec verbSpec) error {
	args, err := buildVerbArgs(cmd, spec)
	if err != nil {
		return err
	}

	socketPath := resolveVerbSocket(cmd)
	tcpAddr := resolveVerbGRPCAddr(cmd)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	res, err := callToolFn(cmd.Context(), socketPath, tcpAddr, logger, CurrentVersion(), spec.toolName, args)
	if err != nil {
		return fmt.Errorf("calling %s: %w", spec.toolName, err)
	}

	// OUT-05: a tool error must surface the typed `<kind>: msg` so main.go's
	// parseKind picks the per-kind exit code. The daemon ships the typed message
	// in the result's TextContent; return it VERBATIM (no generic
	// "tool X reported an error" wrap that would drop the kind — replaces the old
	// kind-dropping branch). The error text is routed to stderr by main.go; we do
	// NOT also render it to stdout.
	if res.IsError {
		return errors.New(resultErrorText(res, spec.toolName))
	}

	renderResult(cmd, spec, res)
	return nil
}

// resultErrorText extracts the typed error message from an IsError result's
// TextContent (joined, trimmed). It falls back to a generic message keyed by the
// tool name only when the daemon shipped no text — that fallback yields no
// parseKind match, so main.go exits with the generic code 1.
func resultErrorText(res *mcpsdk.CallToolResult, toolName string) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok && tc.Text != "" {
			parts = append(parts, tc.Text)
		}
	}
	if msg := strings.TrimSpace(strings.Join(parts, "\n")); msg != "" {
		return msg
	}
	return fmt.Sprintf("tool %s failed", toolName)
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

// resolveVerbGRPCAddr resolves the OPTIONAL loopback gRPC TCP endpoint the
// one-shot call dials (Phase 94 RETIRE-04), with the precedence: inherited root
// --grpc-addr flag > HELIX_GRPC_ADDR env > "" (empty = unix-socket dial, the
// default). It is the dial-side mirror of the daemon's daemon.grpc_addr /
// --grpc-addr config, and HELIX_GRPC_ADDR mirrors how HELIX_SOCKET is read for
// the unix path. When this returns "" the dial path is byte-identical to today.
func resolveVerbGRPCAddr(cmd *cobra.Command) string {
	if cmd != nil {
		if root := cmd.Root(); root != nil {
			if v, _ := root.Flags().GetString("grpc-addr"); v != "" {
				return v
			}
		}
	}
	if env := os.Getenv("HELIX_GRPC_ADDR"); env != "" {
		return env
	}
	return ""
}

// renderResult moved to render.go (Phase 92 terse renderer). verb.go's runVerb
// calls renderResult(cmd, spec, res) which dispatches on the tool's render class.
