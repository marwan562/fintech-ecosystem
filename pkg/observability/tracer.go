package observability

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds configuration for the tracer provider.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Endpoint       string // e.g., "localhost:4317"
	Environment    string // e.g., "production", "development"
}

// InitTracer initializes the OpenTelemetry tracer provider.
// It returns a shutdown function that should be called when the application exits.
func InitTracer(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	// Create resource describing this service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP exporter
	// If Endpoint is empty, we could fall back to stdout or no-op, but for now we'll require it or error
	var exporter sdktrace.SpanExporter
	if cfg.Endpoint != "" {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, cfg.Endpoint,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
		}

		exporter, err = otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
		if err != nil {
			return nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}
	} else {
		// Log that tracing is disabled or fallback?
		// For this ecosystem, let's assume if it's not configured we might just log
		log.Println("Tracing endpoint not set, skipping exporter setup")
		return func(context.Context) error { return nil }, nil
	}

	// Create Tracer Provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set global Tracer Provider
	otel.SetTracerProvider(tp)

	// Set global Propagator (W3C Trace Context)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
