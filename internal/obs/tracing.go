// Phase 12: TracerProvider construction, OTLP/gRPC exporter lifecycle,
// and the WithTracing constructor that returns a degraded-optional Provider.
//
// Design rules (frozen for downstream Phase 12 plans):
//
//   - newTracerProvider is an internal factory; only WithTracing calls it.
//   - WithTracing starts from Noop(inner) so every field is always non-nil.
//     If exporter construction fails, WithTracing logs a warning and returns
//     the noop Provider unchanged — degraded-optional (D-09).
//   - The default path (TracingEndpoint == "") MUST use tracenoop, NOT the
//     SDK tracer with a 0% sampler. SDK tracer allocates per span even when
//     dropped (D-17 hot-path budget).
//   - No call to otel.SetTracerProvider anywhere in this package (D-01).
package obs

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// TracingConfig is the package-local copy of tracing configuration fields.
// The daemon maps config.ObservabilityConfig into this struct.
type TracingConfig struct {
	// Endpoint is the OTLP/gRPC collector endpoint. Empty disables tracing.
	Endpoint string
	// ServiceName is the OTel resource service.name attribute.
	ServiceName string
	// SampleRatio is the TraceIDRatioBased fraction (0.0 = off, 1.0 = all).
	SampleRatio float64
}

// newTracerProvider constructs an SDK TracerProvider with a batched OTLP/gRPC
// exporter, ParentBased(TraceIDRatioBased(cfg.SampleRatio)) sampler, and
// a resource carrying service.name. Returns errors for exporter or resource
// construction failures.
func newTracerProvider(ctx context.Context, cfg TracingConfig) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlptrace exporter: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
		sdktrace.WithResource(res),
	)
	return tp, nil
}

// WithTracing constructs a Provider with a real SDK TracerProvider backed by
// an OTLP/gRPC exporter. If cfg.Endpoint is empty, the caller should use
// Noop() instead — this function is intended for the non-empty endpoint path.
//
// On any construction error, WithTracing logs a warning via logger and returns
// a noop-initialized Provider (degraded-optional, D-09). The returned Provider
// is always non-nil and safe to use.
func WithTracing(inner slog.Handler, cfg TracingConfig, logger *slog.Logger) *Provider {
	p := Noop(inner)

	if cfg.Endpoint == "" {
		return p
	}

	tp, err := newTracerProvider(context.Background(), cfg)
	if err != nil {
		logger.Warn("tracing exporter construction failed; falling back to noop",
			"error", err,
		)
		return p
	}

	p.tracerProvider = tp
	return p
}
