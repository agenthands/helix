package main

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/agenthands/helix/bench/ragindex"
)

// version is the Implementation.Version reported to MCP clients. Kept as a plain
// var so a future ldflag can override it; the bench arm does not need a real
// version handshake, only a stable, non-empty string.
var version = "dev"

// querier is the narrow read surface tools.go needs from the embedding index.
// It is an interface (not the concrete *ragindex.Index) so the server can be
// unit-tested with a deterministic, no-network stub.
type querier interface {
	Query(ctx context.Context, queryText string, k int) ([]ragindex.Result, error)
}

// benchServer wraps the SDK server plus the set of registered tool names. The
// SDK exposes no public "list registered tools" accessor, so we track the names
// ourselves at AddTool time; TestToolListIsExactlyFour asserts on ToolNames().
type benchServer struct {
	sdk   *mcpsdk.Server
	names []string
}

// ToolNames returns the names of every tool registered on the server, in
// registration order. The exactly-four contract (Pitfall 5) is asserted against
// this set.
func (s *benchServer) ToolNames() []string {
	out := make([]string, len(s.names))
	copy(out, s.names)
	return out
}

// Run serves the MCP server over stdio until the context is cancelled or the
// client disconnects.
func (s *benchServer) Run(ctx context.Context) error {
	return s.sdk.Run(ctx, &mcpsdk.StdioTransport{})
}

// RagSearchArgs is the input schema for rag_search.
type RagSearchArgs struct {
	Query string `json:"query" jsonschema:"Natural-language or code query to embed and search"`
	K     int    `json:"k" jsonschema:"Number of nearest chunks to return (default 5)"`
}

// RagReadChunkArgs is the input schema for rag_read_chunk.
type RagReadChunkArgs struct {
	ChunkID string `json:"chunk_id" jsonschema:"Chunk id of the form <relPath>#<ordinal>"`
}

// GrepArgs is the input schema for grep.
type GrepArgs struct {
	Pattern string `json:"pattern" jsonschema:"Regular expression to search for"`
	Path    string `json:"path" jsonschema:"Corpus-root-relative file or directory to search"`
}

// ReadFileArgs is the input schema for read_file.
type ReadFileArgs struct {
	Path string `json:"path" jsonschema:"Corpus-root-relative file to read"`
}

// buildServer constructs the standalone MCP server over the given index and
// corpus root, registering EXACTLY the four baseline_rag tools. It imports ONLY
// the MCP SDK + bench/ragindex + stdlib — NEVER internal/mcp (Pitfall 2).
func buildServer(idx querier, root string) (*benchServer, error) {
	if idx == nil {
		return nil, fmt.Errorf("helix-bench-rag: nil index")
	}
	if root == "" {
		return nil, fmt.Errorf("helix-bench-rag: empty corpus root")
	}

	sdk := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "helix-bench-rag", Version: version},
		nil,
	)
	h := &handlers{idx: idx, root: root}
	s := &benchServer{sdk: sdk}

	addTool := func(tool *mcpsdk.Tool, register func()) {
		register()
		s.names = append(s.names, tool.Name)
	}

	// 1. rag_search — k-NN over the embedding index.
	ragSearch := &mcpsdk.Tool{Name: "rag_search", Description: "Semantic k-NN search over the corpus embedding index"}
	addTool(ragSearch, func() {
		mcpsdk.AddTool(sdk, ragSearch, func(ctx context.Context, _ *mcpsdk.CallToolRequest, args RagSearchArgs) (*mcpsdk.CallToolResult, any, error) {
			out, err := h.ragSearch(ctx, args.Query, args.K)
			return textResult(out, err), nil, nil
		})
	})

	// 2. rag_read_chunk — return a chunk's content by id.
	ragReadChunk := &mcpsdk.Tool{Name: "rag_read_chunk", Description: "Return the full content of a single corpus chunk by its id"}
	addTool(ragReadChunk, func() {
		mcpsdk.AddTool(sdk, ragReadChunk, func(ctx context.Context, _ *mcpsdk.CallToolRequest, args RagReadChunkArgs) (*mcpsdk.CallToolResult, any, error) {
			out, err := h.readChunk(ctx, args.ChunkID)
			return textResult(out, err), nil, nil
		})
	})

	// 3. grep — regex search confined to the corpus root.
	grep := &mcpsdk.Tool{Name: "grep", Description: "Regular-expression search of a corpus-root-relative file or directory"}
	addTool(grep, func() {
		mcpsdk.AddTool(sdk, grep, func(_ context.Context, _ *mcpsdk.CallToolRequest, args GrepArgs) (*mcpsdk.CallToolResult, any, error) {
			out, err := h.grep(args.Pattern, args.Path)
			return textResult(out, err), nil, nil
		})
	})

	// 4. read_file — read a corpus-root-relative file.
	readFile := &mcpsdk.Tool{Name: "read_file", Description: "Read a corpus-root-relative file"}
	addTool(readFile, func() {
		mcpsdk.AddTool(sdk, readFile, func(_ context.Context, _ *mcpsdk.CallToolRequest, args ReadFileArgs) (*mcpsdk.CallToolResult, any, error) {
			out, err := h.readFile(args.Path)
			return textResult(out, err), nil, nil
		})
	})

	return s, nil
}

// textResult adapts a (string, error) handler result into a CallToolResult,
// marking errors as IsError so the MCP client sees the failure without the
// transport tearing down.
func textResult(out string, err error) *mcpsdk.CallToolResult {
	if err != nil {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
			IsError: true,
		}
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: out}},
	}
}

// newRootCmd constructs the cobra root. Exported (package-private but
// test-reachable) for in-process --help testing. RunE opens the corpus index
// out-of-band and serves the MCP server over stdio.
func newRootCmd() *cobra.Command {
	var corpus string

	root := &cobra.Command{
		Use:          "helix-bench-rag",
		Short:        "Standalone baseline_rag MCP server (4 tools over an embedding index)",
		SilenceUsage: true,
		Long: `helix-bench-rag is the provably-isolated baseline_rag control arm for the
Helix benchmark. It serves EXACTLY four MCP tools over stdio:

  rag_search      semantic k-NN search over the corpus embedding index
  rag_read_chunk  return one chunk's content by id
  grep            regex search confined to the corpus root
  read_file       read a corpus-root-relative file

It shares NO code with the Helix daemon: it never links internal/kernel,
internal/semantic, or internal/mcp (enforced by make vet + a transitive
import-boundary test).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if corpus == "" {
				return fmt.Errorf("helix-bench-rag: --corpus is required")
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			// The index is built/loaded OUT-OF-BAND (the cell excludes this from
			// the timed agent budget); here we only open it for serving.
			idx, err := ragindex.Open(ctx, corpus)
			if err != nil {
				return fmt.Errorf("helix-bench-rag: open index: %w", err)
			}
			srv, err := buildServer(idx, corpus)
			if err != nil {
				return err
			}
			return srv.Run(ctx)
		},
	}

	root.Flags().StringVar(&corpus, "corpus", "", "path to the corpus working-copy root (required for serving)")

	return root
}
