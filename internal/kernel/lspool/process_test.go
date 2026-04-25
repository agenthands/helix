package lspool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestProcessHandle_StartListenSeparate asserts the Phase 56 D-02 lifecycle
// contract: ProcessHandle.Start MUST NOT begin the JSON-RPC dispatch loop.
// Callers (Worker.Start in Plan 56-02) need a window between Start and
// StartListen to wire Conn().OnNotification without risk of dropped
// notifications. See Plan 56-01 + 56-CONTEXT for the silent-drop bug.
//
// The test uses `cat` as a benign long-lived subprocess that echoes stdin to
// stdout. We send LSP-framed notifications through the subprocess and observe
// whether OnNotification fires before vs. after StartListen.
func TestProcessHandle_StartListenSeparate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("D-02 lifecycle test requires POSIX `cat`; CI is Linux/macOS")
	}
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skipf("cat not available: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ph := NewProcessHandle("cat", nil, "", nil, testLogger())
	require.NoError(t, ph.Start(ctx, "test-proc"))
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = ph.Stop(stopCtx)
	}()

	var dispatched atomic.Int32
	ph.Conn().OnNotification = func(method string, params json.RawMessage) {
		dispatched.Add(1)
	}

	// Pre-StartListen: send a frame; nothing should dispatch because Listen
	// is not running yet. After GREEN this assertion holds because Listen
	// hasn't been started; pre-fix it FAILS because Start eagerly launches
	// Listen and the frame is dispatched.
	sendNotification(t, ph, "before/start_listen")
	time.Sleep(75 * time.Millisecond)
	require.Equal(t, int32(0), dispatched.Load(), "Listen MUST NOT run before StartListen")

	// Now start the dispatch loop and send another frame.
	ph.StartListen(ctx)
	sendNotification(t, ph, "after/start_listen")

	require.Eventually(t, func() bool { return dispatched.Load() >= 1 }, 1*time.Second, 10*time.Millisecond,
		"after StartListen, notification frames must dispatch")
}

// sendNotification writes an LSP Content-Length-framed JSON-RPC notification
// into the subprocess's stdin. With `cat` as the subprocess, the framed bytes
// are echoed back on stdout where ProcessHandle reads them.
func sendNotification(t *testing.T, ph *ProcessHandle, method string) {
	t.Helper()
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":%q,"params":{}}`, method)
	frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	_, err := io.WriteString(ph.stdin, frame)
	require.NoError(t, err)
}
