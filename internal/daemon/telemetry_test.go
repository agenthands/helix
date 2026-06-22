// Unit and integration tests for the admin listener (Phase 10 OBS-03/04/05).
package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/config"
)

func TestValidateAdminAddr(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		wantErr bool
		errHas  string
	}{
		{name: "empty", addr: "", wantErr: true}, // empty has no port, SplitHostPort fails
		{name: "loopback_v4", addr: "127.0.0.1:9090", wantErr: false},
		{name: "localhost", addr: "localhost:0", wantErr: false},
		{name: "loopback_v6", addr: "[::1]:9090", wantErr: false},
		{name: "auto_port", addr: "127.0.0.1:0", wantErr: false},
		{name: "wildcard_empty_host", addr: ":9090", wantErr: true},   // empty host binds all interfaces
		{name: "wildcard_empty_host_zero", addr: ":0", wantErr: true}, // empty host, auto-port
		{name: "wildcard_v6", addr: "[::]:9090", wantErr: true},       // unspecified IPv6
		{name: "zero_bind", addr: "0.0.0.0:9090", wantErr: true, errHas: "v1.3"},
		{name: "lan_bind", addr: "192.168.1.5:9090", wantErr: true, errHas: "v1.3"},
		{name: "dns_host", addr: "example.com:9090", wantErr: true, errHas: "v1.3"},
		{name: "malformed", addr: "not-a-host", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAdminAddr(tc.addr)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.addr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.addr, err)
			}
			if tc.errHas != "" && err != nil && !strings.Contains(err.Error(), tc.errHas) {
				t.Fatalf("error for %q missing %q: %v", tc.addr, tc.errHas, err)
			}
		})
	}
}

// minimalDaemon constructs a Daemon with only the fields the admin listener needs.
func minimalDaemon(obsCfg config.ObservabilityConfig) *Daemon {
	return &Daemon{
		config: &config.SerenaConfig{Observability: obsCfg},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestHandleHealthz(t *testing.T) {
	d := minimalDaemon(config.ObservabilityConfig{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	d.handleHealthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q, want ok", body["status"])
	}
}

// resetReady returns a cleanup func that restores the ready atomic to zero.
func resetReady(t *testing.T) {
	t.Helper()
	orig := ready.Load()
	t.Cleanup(func() { ready.Store(orig) })
	ready.Store(0)
}

func TestHandleReadyz_NotReady(t *testing.T) {
	resetReady(t)
	d := minimalDaemon(config.ObservabilityConfig{})
	rec := httptest.NewRecorder()
	d.handleReadyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "starting" {
		t.Fatalf("status = %q, want starting", body["status"])
	}
}

func TestHandleReadyz_Ready(t *testing.T) {
	resetReady(t)
	ready.Store(1)
	d := minimalDaemon(config.ObservabilityConfig{})
	rec := httptest.NewRecorder()
	d.handleReadyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "ready" {
		t.Fatalf("status = %q, want ready", body["status"])
	}
}

func TestListenAdmin_Disabled(t *testing.T) {
	d := minimalDaemon(config.ObservabilityConfig{AdminAddr: ""})
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := d.listenAdmin(ctx); err != nil {
		t.Fatalf("listenAdmin disabled should return nil, got %v", err)
	}
}

func TestListenAdmin_NonLoopbackRejected(t *testing.T) {
	d := minimalDaemon(config.ObservabilityConfig{AdminAddr: "0.0.0.0:0"})
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := d.listenAdmin(ctx)
	if err == nil {
		t.Fatal("expected error for non-loopback addr, got nil")
	}
	if !strings.Contains(err.Error(), "v1.3") {
		t.Fatalf("error missing v1.3: %v", err)
	}
}

// waitForAdminAddr polls the adminListenerAddr test hook until it has a value or timeout.
func waitForAdminAddr(t *testing.T, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p := adminListenerAddr.Load(); p != nil {
			return *p
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("admin listener did not publish address within %s", timeout)
	return ""
}

func TestListenAdmin_LoopbackLifecycle(t *testing.T) {
	resetReady(t)
	d := minimalDaemon(config.ObservabilityConfig{AdminAddr: "127.0.0.1:0"})
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- d.listenAdmin(ctx) }()

	addr := waitForAdminAddr(t, 2*time.Second)

	// Poll /healthz until it returns 200.
	deadline := time.Now().Add(2 * time.Second)
	var got int
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			got = resp.StatusCode
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if got == http.StatusOK {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got != http.StatusOK {
		t.Fatalf("/healthz never returned 200 (last=%d)", got)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			// server.Shutdown returns nil on clean shutdown.
			t.Logf("listenAdmin returned: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listenAdmin did not exit within 5s of ctx cancel")
	}
	// Ensure hook cleared.
	if adminListenerAddr.Load() != nil {
		t.Fatal("adminListenerAddr should be nil after exit")
	}
}

func TestListenAdmin_PprofGated(t *testing.T) {
	resetReady(t)
	// Disabled case.
	runOne := func(enable bool) int {
		d := minimalDaemon(config.ObservabilityConfig{AdminAddr: "127.0.0.1:0", EnablePprof: enable})
		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() { errCh <- d.listenAdmin(ctx) }()
		defer func() {
			cancel()
			select {
			case <-errCh:
			case <-time.After(5 * time.Second):
				t.Fatal("listenAdmin hang")
			}
		}()
		addr := waitForAdminAddr(t, 2*time.Second)
		// Give it a beat for Serve loop.
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := http.Get("http://" + addr + "/debug/pprof/")
			if err == nil {
				code := resp.StatusCode
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				return code
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("pprof probe never succeeded")
		return 0
	}

	if code := runOne(false); code != http.StatusNotFound {
		t.Fatalf("pprof disabled: got %d, want 404", code)
	}
	// Because adminListenerAddr is a package-level hook, ensure it is fully cleared
	// before the next iteration.
	for adminListenerAddr.Load() != nil {
		time.Sleep(5 * time.Millisecond)
	}
	if code := runOne(true); code != http.StatusOK {
		t.Fatalf("pprof enabled: got %d, want 200", code)
	}
}
