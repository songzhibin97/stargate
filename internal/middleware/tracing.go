package middleware

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"github.com/songzhibin97/stargate/internal/config"
)

// TracingMiddleware provides OpenTelemetry distributed tracing
type TracingMiddleware struct {
	config     *config.TracingConfig
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// NewTracingMiddleware creates a new tracing middleware
func NewTracingMiddleware(cfg *config.TracingConfig) (*TracingMiddleware, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tracing config cannot be nil")
	}

	// Get the global tracer
	tracer := otel.Tracer("stargate-node")

	// Use the global propagator
	propagator := otel.GetTextMapPropagator()

	return &TracingMiddleware{
		config:     cfg,
		tracer:     tracer,
		propagator: propagator,
	}, nil
}

// Handler returns the HTTP middleware handler
func (m *TracingMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if tracing is disabled
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Extract trace context from incoming request headers
			ctx := m.propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start a new span for this request
			spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			ctx, span := m.tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.scheme", r.URL.Scheme),
					attribute.String("http.host", r.Host),
					attribute.String("http.target", r.URL.Path),
					attribute.String("http.user_agent", r.UserAgent()),
					attribute.String("http.remote_addr", r.RemoteAddr),
					attribute.String("http.proto", r.Proto),
				),
			)
			defer span.End()

			// Add route ID if available
			if routeID := r.Context().Value("route_id"); routeID != nil {
				if id, ok := routeID.(string); ok {
					span.SetAttributes(attribute.String("stargate.route_id", id))
				}
			}

			// Add request size if available
			if r.ContentLength > 0 {
				span.SetAttributes(attribute.Int64("http.request_content_length", r.ContentLength))
			}

			// Create a response wrapper to capture response details
			wrapper := &tracingResponseWrapper{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Update request context with trace context
			r = r.WithContext(ctx)

			// Process request
			next.ServeHTTP(wrapper, r)

			// Set response attributes
			span.SetAttributes(
				attribute.Int("http.status_code", wrapper.statusCode),
				attribute.Int64("http.response_size", wrapper.responseSize),
			)

			// Set span status based on HTTP status code
			if wrapper.statusCode >= 400 {
				span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", wrapper.statusCode))
				if wrapper.statusCode >= 500 {
					span.RecordError(fmt.Errorf("server error: HTTP %d", wrapper.statusCode))
				}
			} else {
				span.SetStatus(codes.Ok, "")
			}
		})
	}
}

// InjectTraceContext injects trace context into outbound HTTP requests
func (m *TracingMiddleware) InjectTraceContext(ctx context.Context, req *http.Request) {
	if !m.config.Enabled {
		return
	}

	// Inject trace context into request headers
	m.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// ExtractTraceContext extracts trace context from HTTP request headers
func (m *TracingMiddleware) ExtractTraceContext(r *http.Request) context.Context {
	if !m.config.Enabled {
		return r.Context()
	}

	// Extract trace context from request headers
	return m.propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
}

// StartSpan starts a new span with the given name and context
func (m *TracingMiddleware) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !m.config.Enabled {
		return ctx, trace.SpanFromContext(ctx)
	}

	return m.tracer.Start(ctx, name, opts...)
}

// tracingResponseWrapper wraps http.ResponseWriter to capture response details
type tracingResponseWrapper struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
	wroteHeader  bool
}

func (rw *tracingResponseWrapper) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *tracingResponseWrapper) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.responseSize += int64(n)
	return n, err
}

// GetTracer returns the tracer instance
func (m *TracingMiddleware) GetTracer() trace.Tracer {
	return m.tracer
}

// GetPropagator returns the propagator instance
func (m *TracingMiddleware) GetPropagator() propagation.TextMapPropagator {
	return m.propagator
}

// AddSpanEvent adds an event to the current span
func (m *TracingMiddleware) AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if !m.config.Enabled {
		return
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// SetSpanAttributes sets attributes on the current span
func (m *TracingMiddleware) SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	if !m.config.Enabled {
		return
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// RecordError records an error on the current span
func (m *TracingMiddleware) RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	if !m.config.Enabled || err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err, trace.WithAttributes(attrs...))
		span.SetStatus(codes.Error, err.Error())
	}
}

// GetSpanContext returns the span context from the given context
func (m *TracingMiddleware) GetSpanContext(ctx context.Context) trace.SpanContext {
	return trace.SpanFromContext(ctx).SpanContext()
}

// IsTraceEnabled returns whether tracing is enabled
func (m *TracingMiddleware) IsTraceEnabled() bool {
	return m.config.Enabled
}

// GetConfig returns the tracing configuration
func (m *TracingMiddleware) GetConfig() *config.TracingConfig {
	return m.config
}
