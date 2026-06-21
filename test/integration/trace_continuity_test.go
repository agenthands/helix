//go:build integration

package integration_test

// TestE2ETraceContinuity proves D-06 (Phase 58) end-to-end: a single TraceID
// must cover the forwarder client root span (forwarder.tools.call) and the
// daemon-side gRPC server span (serena.v1.ForwarderService/StreamMCP) when
// the W3C TraceContext propagator is wired on BOTH otelgrpc handlers
// per-handler (no global otel.SetTextMapPropagator).
//
// RED state (before Task 2 lands):
//
//   - The forwarder's gRPC client handler is constructed without
//     otelgrpc.WithPropagators(propagation.TraceContext{}), so otelgrpc
//     falls back to the OTel global propagator (NoOp by default — D-01
//     forbids us from setting one). No traceparent is injected into the
//     gRPC metadata at stream open, so the server-side handler creates
//     a new TraceID rather than continuing the client's TraceID.
//   - require.Equal(fwdTraceID, srvTraceID) fails — the assertion is the
//     contract Task 2 makes hold.

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/forwarder"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/skill"
)

// startDaemonOverSocket starts a real daemon listening on a Unix socket with
// the supplied TracerProvider. Returns the socket path and a cancel func.
func startDaemonOverSocket(t *testing.T, tp *sdktrace.TracerProvider) (string, context.CancelFunc) {
	t.Helper()

	cfg := &config.SerenaConfig{}
	tmpDir := t.TempDir()
	cfg.Daemon.SocketPath = filepath.Join(tmpDir, "s.sock")
	cfg.Daemon.HTTPAddr = ""
	cfg.Daemon.ShutdownTimeout = 2
	cfg.Profile = "full"
	cfg.WorkerPool.MaxWorkers = 2
	cfg.WorkerPool.BaseTTL = 30

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	projectDir := filepath.Join(tmpDir, ".helix")
	globalDir := filepath.Join(tmpDir, ".helix-global")
	require.NoError(t, os.MkdirAll(projectDir, 0o755))
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	require.NoError(t, skill.InitAll(skill.SkillDeps{
		ProjectDir: projectDir,
		GlobalDir:  globalDir,
		Logger:     logger,
	}))

	d, err := daemon.NewWithObsProvider(cfg, logger, obs.NewForTest(tp))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = d.Run(ctx)
	}()

	// Wait for the Unix socket to appear (up to 5s).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.Daemon.SocketPath); err == nil {
			// Verify the socket is alive by attempting a dial.
			conn, derr := net.DialTimeout("unix", cfg.Daemon.SocketPath, 500*time.Millisecond)
			if derr == nil {
				_ = conn.Close()
				return cfg.Daemon.SocketPath, cancel
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	t.Fatalf("daemon socket %s did not become reachable within 5s", cfg.Daemon.SocketPath)
	return "", cancel
}

func TestE2ETraceContinuity(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	socketPath, cancel := startDaemonOverSocket(t, tp)
	t.Cleanup(cancel)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Connect a real forwarder gRPC client with the SAME TracerProvider.
	// This mirrors what Task 2 will make production do — both forwarder and
	// daemon Provider point at the test exporter.
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dialCancel()
	client, conn, err := forwarder.ConnectOrStartDaemon(dialCtx, socketPath, "", logger, tp)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Phase 58 contract: the forwarder.tools.call span MUST be the active
	// span when the gRPC stream is opened so otelgrpc's traceparent injection
	// at stream open carries this TraceID across the wire. Once Task 2 wires
	// the W3C TraceContext propagator on both handlers, the server's
	// serena.v1.ForwarderService/StreamMCP span sees the same TraceID as
	// fwdSpan. Pre-Task-2, the global propagator (NoOp) drops the context
	// and TraceIDs differ.
	tracer := tp.Tracer("test-forwarder")
	spanCtx, fwdSpan := tracer.Start(context.Background(), "forwarder.tools.call")

	stream, err := client.StreamMCP(spanCtx)
	require.NoError(t, err)

	msg := &serenav1.MCPMessage{
		Payload:   []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_token_budget"}}`),
		SessionId: "trace-continuity-session",
	}
	require.NoError(t, stream.Send(msg))

	// Close send side — daemon will end its StreamMCP handler and emit the
	// server span. Server-side span is created by otelgrpc.NewServerHandler.
	require.NoError(t, stream.CloseSend())

	// Drain any responses (best-effort; the daemon may or may not reply
	// depending on session state — what we care about is that the stream
	// ran end-to-end and the server span was emitted).
	for {
		_, recvErr := stream.Recv()
		if recvErr != nil {
			break
		}
	}

	fwdSpan.End()

	// Drain spans.
	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exporter.GetSpans()

	// otelgrpc emits TWO spans named "serena.v1.ForwarderService/StreamMCP":
	// one client-kind (child of forwarder.tools.call, in-process) and one
	// server-kind (the real server-side span we care about, on the other
	// side of the gRPC wire). The contract under test is that the
	// SERVER-kind span shares TraceID with forwarder.tools.call — which
	// only holds when the W3C TraceContext propagator is wired on both
	// handlers. Filter by SpanKind to disambiguate.
	var fwdStub, srvStub tracetest.SpanStub
	var foundFwd, foundSrv bool
	for _, s := range spans {
		switch {
		case s.Name == "forwarder.tools.call":
			fwdStub = s
			foundFwd = true
		case s.Name == "serena.v1.ForwarderService/StreamMCP" && s.SpanKind == trace.SpanKindServer:
			srvStub = s
			foundSrv = true
		}
	}

	require.True(t, foundFwd,
		"forwarder.tools.call span missing — got: %v", spanNamesContinuity(spans))
	require.True(t, foundSrv,
		"serena.v1.ForwarderService/StreamMCP span missing — got: %v", spanNamesContinuity(spans))

	require.True(t, fwdStub.SpanContext.TraceID().IsValid(),
		"forwarder span TraceID must be valid")
	require.True(t, srvStub.SpanContext.TraceID().IsValid(),
		"server span TraceID must be valid")

	require.Equal(t, fwdStub.SpanContext.TraceID(), srvStub.SpanContext.TraceID(),
		"trace IDs differ — propagator not wired (Phase 58 D-06 contract)")
}

// spanNamesContinuity is local to this file to avoid a name collision with
// trace_propagation_test.go's spanNames helper (both files are in the same
// integration_test package and build tag).
func spanNamesContinuity(spans []tracetest.SpanStub) []string {
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = s.Name
	}
	return out
}
