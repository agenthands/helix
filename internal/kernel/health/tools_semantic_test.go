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
// a non-nil Probe error renders as state=unhealthy. WR-NEW-01: the Reason
// field is a closed enum, never the raw err.Error() text. A generic
// connection error maps to the "db_error" bucket.
func TestSemanticStoreStatus_Unhealthy(t *testing.T) {
	st := health.ComputeSemanticStoreStatus(context.Background(), fakeProbeUnhealthy{})
	if st.State != "unhealthy" {
		t.Fatalf("State=%q, want unhealthy", st.State)
	}
	if st.Reason != health.SemanticReasonDBError {
		t.Fatalf("Reason=%q, want %q (closed enum)", st.Reason, health.SemanticReasonDBError)
	}
	// Belt-and-braces: assert the raw err.Error() text is NOT leaked.
	if st.Reason == "connection refused" {
		t.Fatalf("Reason leaked raw err text; closed enum violated")
	}
}

// TestSemanticStoreStatus_Unhealthy_NilHandle proves the daemon-side
// nil-handle sentinels ("DB handle nil", "store unavailable") map to the
// "nil_handle" bucket. WR-NEW-01.
func TestSemanticStoreStatus_Unhealthy_NilHandle(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"db_handle_nil", errors.New("semantic store DB handle nil")},
		{"store_unavailable", errors.New("semantic store unavailable")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := fakeProbeWithErr{err: tc.err}
			st := health.ComputeSemanticStoreStatus(context.Background(), p)
			if st.State != "unhealthy" {
				t.Fatalf("State=%q, want unhealthy", st.State)
			}
			if st.Reason != health.SemanticReasonNilHandle {
				t.Fatalf("Reason=%q, want %q", st.Reason, health.SemanticReasonNilHandle)
			}
		})
	}
}

// TestSemanticStoreStatus_Unhealthy_ProbeTimeout proves a context-deadline
// error from the probe maps to the "probe_timeout" bucket. WR-NEW-01.
func TestSemanticStoreStatus_Unhealthy_ProbeTimeout(t *testing.T) {
	p := fakeProbeWithErr{err: context.DeadlineExceeded}
	st := health.ComputeSemanticStoreStatus(context.Background(), p)
	if st.State != "unhealthy" {
		t.Fatalf("State=%q, want unhealthy", st.State)
	}
	if st.Reason != health.SemanticReasonProbeTimeout {
		t.Fatalf("Reason=%q, want %q", st.Reason, health.SemanticReasonProbeTimeout)
	}
}

// fakeProbeWithErr returns a configurable error from Probe. Used by the
// closed-enum classifier tests above.
type fakeProbeWithErr struct{ err error }

func (fakeProbeWithErr) Available() bool                  { return true }
func (p fakeProbeWithErr) Probe(_ context.Context) error  { return p.err }

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
