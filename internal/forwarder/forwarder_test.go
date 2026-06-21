package forwarder

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func TestTryConnect_NoSocket(t *testing.T) {
	conn, client, err := tryConnect(context.Background(), "/tmp/helix-test-nonexistent.sock", "", tracenoop.NewTracerProvider())
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "socket not found")
}

func TestStartDaemon_ExecutableLookup(t *testing.T) {
	// Verify we can find the current executable (basic sanity check)
	exe, err := os.Executable()
	require.NoError(t, err)
	assert.NotEmpty(t, exe)
}

func TestWaitForDaemon_Timeout(t *testing.T) {
	ctx := context.Background()
	// Use a nonexistent socket path with a very short timeout
	client, conn, err := waitForDaemon(ctx, "/tmp/helix-test-timeout-"+generateSessionID()+".sock", 200*time.Millisecond, tracenoop.NewTracerProvider())
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "daemon did not start")
}

func TestWaitForDaemon_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	client, conn, err := waitForDaemon(ctx, "/tmp/helix-test-cancel.sock", 5*time.Second, tracenoop.NewTracerProvider())
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Nil(t, conn)
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()
	assert.Len(t, id1, 32) // 16 bytes = 32 hex chars
	assert.Len(t, id2, 32)
	assert.NotEqual(t, id1, id2)
}
