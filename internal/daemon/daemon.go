package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	serenav1 "github.com/postfix/serena/api/proto/serena/v1"
	"github.com/postfix/serena/internal/config"
	serenaMCP "github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/workspace"
)

// Daemon is the persistent supervisor process (DMN-01).
// It manages workspace registry, MCP server, listeners, and survives client disconnects (DMN-02).
type Daemon struct {
	config         *config.SerenaConfig
	logger         *slog.Logger
	workspaces     *workspace.Registry
	mcpServer      *serenaMCP.SerenaMCPServer
	grpcServer     *grpc.Server
	socketListener net.Listener
}

// New creates a new Daemon with the given config and logger.
func New(cfg *config.SerenaConfig, logger *slog.Logger) *Daemon {
	workspaces := workspace.NewRegistry()
	return &Daemon{
		config:     cfg,
		logger:     logger,
		workspaces: workspaces,
		mcpServer:  serenaMCP.NewSerenaMCPServer(workspaces, logger),
	}
}

// Run starts the daemon and blocks until shutdown (DMN-01, DMN-12).
// Signal handlers are registered FIRST per Pitfall 3.
func (d *Daemon) Run(ctx context.Context) error {
	// Register signal handlers FIRST (Pitfall 3: before any goroutine starts)
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Check for stale socket and clean up (DMN-13)
	if err := ensureSocket(d.config.Daemon.SocketPath); err != nil {
		return fmt.Errorf("socket check: %w", err)
	}
	if err := createSocketDir(d.config.Daemon.SocketPath); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Unix socket listener for forwarder connections via gRPC (DMN-03)
	g.Go(func() error {
		return d.listenSocket(gctx)
	})

	// HTTP listener for Streamable HTTP MCP (DMN-04, MCP-02)
	if d.config.Daemon.HTTPAddr != "" {
		g.Go(func() error {
			return d.listenHTTP(gctx)
		})
	}

	d.logger.Info("daemon started",
		"socket", d.config.Daemon.SocketPath,
		"http_addr", d.config.Daemon.HTTPAddr,
		"workspaces", d.workspaces.WorkspaceCount(),
	)

	err := g.Wait()
	d.shutdown()
	return err
}

// listenSocket starts the Unix domain socket listener with a gRPC server.
func (d *Daemon) listenSocket(ctx context.Context) error {
	ln, err := net.Listen("unix", d.config.Daemon.SocketPath)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", d.config.Daemon.SocketPath, err)
	}
	d.socketListener = ln
	d.logger.Info("unix socket listener started", "path", d.config.Daemon.SocketPath)

	// Create and register gRPC server
	d.grpcServer = grpc.NewServer()
	serenav1.RegisterForwarderServiceServer(d.grpcServer, &forwarderServiceHandler{
		mcpServer: d.mcpServer,
		logger:    d.logger,
	})

	// Serve in a goroutine so we can wait for context cancellation
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- d.grpcServer.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		d.grpcServer.GracefulStop()
		return ctx.Err()
	case err := <-serveDone:
		return err
	}
}

// listenHTTP starts the Streamable HTTP listener for MCP (DMN-04, MCP-02).
func (d *Daemon) listenHTTP(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/mcp", d.mcpServer.HTTPHandler())

	server := &http.Server{
		Addr:    d.config.Daemon.HTTPAddr,
		Handler: mux,
	}

	d.logger.Info("HTTP listener started", "addr", d.config.Daemon.HTTPAddr)

	// Serve in a goroutine
	serveDone := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveDone <- err
		}
		close(serveDone)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serveDone:
		return err
	}
}

// Workspaces returns the workspace registry (for use by MCP tools).
func (d *Daemon) Workspaces() *workspace.Registry {
	return d.workspaces
}

// forwarderServiceHandler implements the gRPC ForwarderService.
type forwarderServiceHandler struct {
	serenav1.UnimplementedForwarderServiceServer
	mcpServer *serenaMCP.SerenaMCPServer
	logger    *slog.Logger
}

// StreamMCP handles a bidirectional MCP stream from a forwarder.
func (h *forwarderServiceHandler) StreamMCP(stream serenav1.ForwarderService_StreamMCPServer) error {
	// Read the first message to get the session ID
	firstMsg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving first message: %w", err)
	}

	sessionID := firstMsg.SessionId
	h.logger.Info("new forwarder stream", "session_id", sessionID)

	// Create a GRPCTransport that bridges this stream to the MCP SDK.
	// Pass firstMsg so it gets replayed into the transport pipe.
	transport := serenaMCP.NewGRPCTransport(stream, sessionID, firstMsg)

	// Connect the MCP server to this transport
	session, err := h.mcpServer.SDK().Connect(stream.Context(), transport, nil)
	if err != nil {
		return fmt.Errorf("connecting MCP session: %w", err)
	}

	// Wait for the session to end
	err = session.Wait()
	h.logger.Info("forwarder stream ended", "session_id", sessionID)
	return err
}
