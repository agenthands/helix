package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/kernel/health"
)

type fakeProbeReady struct{}

func (fakeProbeReady) Available() bool                 { return true }
func (fakeProbeReady) Probe(ctx context.Context) error { return nil }

type fakeProbeDisabled struct{}

func (fakeProbeDisabled) Available() bool                 { return false }
func (fakeProbeDisabled) Probe(ctx context.Context) error { return errors.New("disabled") }

type fakeProbeUnhealthy struct{}

func (fakeProbeUnhealthy) Available() bool                 { return true }
func (fakeProbeUnhealthy) Probe(ctx context.Context) error { return errors.New("connection refused") }

// TestSemanticStoreStatus_Ready proves a probe that reports Available + nil
// Probe error renders as state=ready. SC-1.
func TestSemanticStoreStatus_Ready(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), fakeProbeReady{})
	if st.State != "ready" {
		t.Fatalf("State=%q, want ready", st.State)
	}
}

// TestSemanticStoreStatus_Disabled proves an Available()==false probe
// renders as state=disabled (the v1.10 CGO=0 / disabled-flag path).
func TestSemanticStoreStatus_Disabled(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), fakeProbeDisabled{})
	if st.State != "disabled" {
		t.Fatalf("State=%q, want disabled", st.State)
	}
}

// TestSemanticStoreStatus_NilProbe proves the nil-probe (no daemon wiring)
// path renders as state=disabled rather than panicking.
func TestSemanticStoreStatus_NilProbe(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), nil)
	if st.State != "disabled" {
		t.Fatalf("State=%q, want disabled (nil probe)", st.State)
	}
}

// TestSemanticStoreStatus_Unhealthy proves an Available()==true probe with
// a non-nil Probe error renders as state=unhealthy with the error reason.
func TestSemanticStoreStatus_Unhealthy(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), fakeProbeUnhealthy{})
	if st.State != "unhealthy" {
		t.Fatalf("State=%q, want unhealthy", st.State)
	}
	if st.Reason == "" {
		t.Fatalf("Reason must be non-empty for unhealthy state")
	}
}

// TestSemanticStoreStatus_JSONShape proves the marshalled block uses
// snake_case field names per the documented contract.
func TestSemanticStoreStatus_JSONShape(t *testing.T) {
	st := health.SemanticStoreStatus{State: "ready"}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(b), `"state":"ready"`) {
		t.Fatalf("json shape mismatch: %s", b)
	}
}
