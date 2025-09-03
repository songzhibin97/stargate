package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"github.com/songzhibin97/stargate/internal/config"
)

// TracerProvider manages the OpenTelemetry tracer provider
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	config   *config.TracingConfig
}

// NewTracerProvider creates and configures a new tracer provider
func NewTracerProvider(cfg *config.TracingConfig) (*TracerProvider, error) {
	if cfg == nil || !cfg.Enabled {
		return &TracerProvider{config: cfg}, nil
	}

	// Create resource with service information
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.Jaeger.ServiceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	if cfg.Jaeger.Endpoint != "" {
		exporter, err = createExporter(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create exporter: %w", err)
		}
	}

	// Create tracer provider options
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}

	// Add exporter if available
	if exporter != nil {
		// Configure sampling based on sample rate
		sampler := sdktrace.AlwaysSample()
		if cfg.Jaeger.SampleRate < 1.0 {
			sampler = sdktrace.TraceIDRatioBased(cfg.Jaeger.SampleRate)
		}

		opts = append(opts,
			sdktrace.WithBatcher(exporter,
				sdktrace.WithBatchTimeout(5*time.Second),
				sdktrace.WithMaxExportBatchSize(512),
			),
			sdktrace.WithSampler(sampler),
		)
	}

	// Create tracer provider
	provider := sdktrace.NewTracerProvider(opts...)

	// Set global tracer provider
	otel.SetTracerProvider(provider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracerProvider{
		provider: provider,
		config:   cfg,
	}, nil
}

// createExporter creates the appropriate exporter based on configuration
func createExporter(cfg *config.TracingConfig) (sdktrace.SpanExporter, error) {
	endpoint := cfg.Jaeger.Endpoint

	// For local development without Jaeger, we'll use the Jaeger exporter directly
	// since OTLP requires a running OTLP collector

	// Use Jaeger exporter directly (works without external collector for testing)
	jaegerExporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(endpoint)))
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	return jaegerExporter, nil
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider == nil {
		return nil
	}

	return tp.provider.Shutdown(ctx)
}

// ForceFlush forces all spans to be exported
func (tp *TracerProvider) ForceFlush(ctx context.Context) error {
	if tp.provider == nil {
		return nil
	}

	return tp.provider.ForceFlush(ctx)
}

// IsEnabled returns whether tracing is enabled
func (tp *TracerProvider) IsEnabled() bool {
	return tp.config != nil && tp.config.Enabled
}

// GetProvider returns the underlying tracer provider
func (tp *TracerProvider) GetProvider() *sdktrace.TracerProvider {
	return tp.provider
}

// GetConfig returns the tracing configuration
func (tp *TracerProvider) GetConfig() *config.TracingConfig {
	return tp.config
}

// InitializeGlobalTracer initializes the global OpenTelemetry tracer
func InitializeGlobalTracer(cfg *config.TracingConfig) (*TracerProvider, error) {
	return NewTracerProvider(cfg)
}

// ShutdownGlobalTracer shuts down the global tracer provider
func ShutdownGlobalTracer(ctx context.Context, tp *TracerProvider) error {
	if tp == nil {
		return nil
	}

	// Force flush before shutdown
	if err := tp.ForceFlush(ctx); err != nil {
		return fmt.Errorf("failed to flush tracer: %w", err)
	}

	// Shutdown tracer provider
	if err := tp.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown tracer: %w", err)
	}

	return nil
}

// CreateSpanFromContext creates a new span from the given context
func CreateSpanFromContext(ctx context.Context, operationName string) (context.Context, func()) {
	tracer := otel.Tracer("stargate-node")
	ctx, span := tracer.Start(ctx, operationName)
	
	return ctx, func() {
		span.End()
	}
}

// GetTraceID returns the trace ID from the given context
func GetTraceID(ctx context.Context) string {
	spanCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier{})
	if spanCtx != nil {
		return spanCtx.Value("traceparent").(string)
	}
	return ""
}

// InjectTraceHeaders injects trace context into HTTP headers
func InjectTraceHeaders(ctx context.Context, headers map[string]string) {
	carrier := propagation.MapCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// ExtractTraceHeaders extracts trace context from HTTP headers
func ExtractTraceHeaders(ctx context.Context, headers map[string]string) context.Context {
	carrier := propagation.MapCarrier(headers)
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
