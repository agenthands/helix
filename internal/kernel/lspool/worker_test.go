package lspool

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/kernel/jsonrpc"
)

// TestBuildDispatcher_KnownMethod asserts that a registered method is invoked
// exactly once and that an unknown method does not invoke any handler.
func TestBuildDispatcher_KnownMethod(t *testing.T) {
	var calls atomic.Int32
	handlers := map[string]func(json.RawMessage){
		"foo/bar": func(json.RawMessage) { calls.Add(1) },
	}
	d := buildDispatcher(handlers, slog.New(slog.NewTextHandler(io.Discard, nil)), "w-known")
	require.NotNil(t, d, "dispatcher must be non-nil for non-empty handlers")

	d("foo/bar", json.RawMessage(`{}`))
	require.Equal(t, int32(1), calls.Load(), "registered method must dispatch")

	d("baz/qux", json.RawMessage(`{}`))
	require.Equal(t, int32(1), calls.Load(), "unknown method must not invoke any handler")
}

// TestBuildDispatcher_UnknownMethodLogsAndDrops asserts D-04: when an unknown
// method arrives, the dispatcher debug-logs the method name and drops the
// notification without invoking any handler.
func TestBuildDispatcher_UnknownMethodLogsAndDrops(t *testing.T) {
	var calls atomic.Int32
	handlers := map[string]func(json.RawMessage){
		"foo/bar": func(json.RawMessage) { calls.Add(1) },
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	d := buildDispatcher(handlers, logger, "w-unknown")

	require.NotPanics(t, func() { d("totally/unknown", json.RawMessage(`{}`)) })
	require.Equal(t, int32(0), calls.Load(), "unknown method must not invoke any handler")
	require.Contains(t, buf.String(), "unhandled LS notification")
	require.Contains(t, buf.String(), "totally/unknown")
}

// TestBuildDispatcher_PanicRecovers asserts D-05 (LSDISP-03): a panicking
// handler must NOT propagate, must be logged at Error level, and subsequent
// dispatches must continue working.
func TestBuildDispatcher_PanicRecovers(t *testing.T) {
	handlers := map[string]func(json.RawMessage){
		"boom": func(json.RawMessage) { panic("kaboom") },
		"ok":   func(json.RawMessage) { /* no-op */ },
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	d := buildDispatcher(handlers, logger, "w-1")
	require.NotPanics(t, func() { d("boom", json.RawMessage(`{}`)) })
	require.NotPanics(t, func() { d("ok", json.RawMessage(`{}`)) })
	require.Contains(t, buf.String(), "panicked")
	require.Contains(t, buf.String(), "kaboom")
	require.Contains(t, buf.String(), "w-1")
}

// TestWorker_DispatcherWired (LSDISP-01) asserts the dispatcher wiring contract
// at the unit level using Shape B from the plan: assert buildDispatcher routes
// the QuirkAdapter's declared method to the registered handler. When handlers
// are non-empty, the closure returned by buildDispatcher is a non-nil
// NotificationFunc that dispatches to the right handler. (Shape A — driving
// Worker.Start end-to-end with a cat-backed ProcessHandle — is also acceptable;
// this fixture takes the pure-unit route for speed and to avoid the full LSP
// initialize handshake which is hard to fake under unit tests.)
func TestWorker_DispatcherWired(t *testing.T) {
	var got int32
	handlers := map[string]func(json.RawMessage){
		"lsdisp-01/probe": func(json.RawMessage) { atomic.AddInt32(&got, 1) },
	}
	d := buildDispatcher(handlers, slog.New(slog.NewTextHandler(io.Discard, nil)), "w-lsdisp-01")
	require.NotNil(t, d, "dispatcher must be non-nil for non-empty handlers")
	d("lsdisp-01/probe", json.RawMessage(`{}`))
	require.Equal(t, int32(1), atomic.LoadInt32(&got), "registered method must dispatch")
}

// TestWorker_DispatcherWiringRegression (D-11) asserts the regression check
// extracted as assertDispatcherWired: it returns a non-nil error and logs at
// Error level if quirks declared handlers but OnNotification ended up nil.
func TestWorker_DispatcherWiringRegression(t *testing.T) {
	t.Run("non-empty handlers + wired dispatcher returns nil", func(t *testing.T) {
		handlers := map[string]func(json.RawMessage){
			"x/y": func(json.RawMessage) {},
		}
		conn := jsonrpc.NewConn(&nopRWC{}, "test")
		conn.OnNotification = func(string, json.RawMessage) {}
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		require.NoError(t, assertDispatcherWired(handlers, conn, logger, "w-ok"))
	})

	t.Run("non-empty handlers + nil OnNotification returns error and logs", func(t *testing.T) {
		handlers := map[string]func(json.RawMessage){
			"x/y": func(json.RawMessage) {},
		}
		conn := jsonrpc.NewConn(&nopRWC{}, "test")
		// Intentionally do NOT set OnNotification.
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		err := assertDispatcherWired(handlers, conn, logger, "w-broken")
		require.Error(t, err)
		require.Contains(t, err.Error(), "dispatcher wiring failed")
		require.Contains(t, buf.String(), "w-broken")
		// Error level log emitted.
		require.Contains(t, buf.String(), `"level":"ERROR"`)
	})

	t.Run("empty handlers + nil OnNotification returns nil", func(t *testing.T) {
		conn := jsonrpc.NewConn(&nopRWC{}, "test")
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		require.NoError(t, assertDispatcherWired(nil, conn, logger, "w-noop"))
		require.NoError(t, assertDispatcherWired(map[string]func(json.RawMessage){}, conn, logger, "w-noop"))
	})
}

// nopRWC is a minimal io.ReadWriteCloser that returns EOF on read so
// jsonrpc.NewConn can be constructed for assertion-only tests.
type nopRWC struct{}

func (n *nopRWC) Read(p []byte) (int, error)  { return 0, io.EOF }
func (n *nopRWC) Write(p []byte) (int, error) { return len(p), nil }
func (n *nopRWC) Close() error                { return nil }
