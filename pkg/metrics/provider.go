package metrics

import (
	"context"
	"io"
	"net/http"
)

// Provider defines the main interface for metrics providers
// It abstracts the creation and management of metrics across different backends
type Provider interface {
	// Counter creation methods
	NewCounter(opts MetricOptions) (Counter, error)
	NewCounterVec(opts MetricOptions) (CounterVec, error)
	
	// Gauge creation methods
	NewGauge(opts MetricOptions) (Gauge, error)
	NewGaugeVec(opts MetricOptions) (GaugeVec, error)
	
	// Histogram creation methods
	NewHistogram(opts MetricOptions) (Histogram, error)
	NewHistogramVec(opts MetricOptions) (HistogramVec, error)
	
	// Summary creation methods
	NewSummary(opts MetricOptions) (Summary, error)
	NewSummaryVec(opts MetricOptions) (SummaryVec, error)
	
	// Registry operations
	Register(collector Collector) error
	Unregister(collector Collector) error
	
	// Metrics collection
	Gather() ([]*MetricFamily, error)
	GatherWithOptions(opts *GatherOptions) ([]*MetricFamily, error)
	
	// HTTP handler for metrics endpoint
	Handler() http.Handler
	HandlerFor(gatherer Gatherer, opts HandlerOpts) http.Handler
	
	// Provider lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() error
	
	// Provider information
	Name() string
	Version() string
}

// HandlerOpts specifies options for creating HTTP handlers
type HandlerOpts struct {
	// ErrorLog specifies an optional logger for errors collecting and serving metrics
	ErrorLog Logger
	
	// ErrorHandling defines how errors are handled
	ErrorHandling ErrorHandling
	
	// Registry is the registry to gather metrics from
	Registry Registry
	
	// DisableCompression disables gzip compression
	DisableCompression bool
	
	// MaxRequestsInFlight is the maximum number of concurrent HTTP requests
	MaxRequestsInFlight int
	
	// Timeout for gathering metrics
	Timeout context.Context
	
	// ProcessStartTime is the start time of the process
	ProcessStartTime int64
}

// ErrorHandling defines how errors are handled in HTTP handlers
type ErrorHandling int

const (
	// PanicOnError panics on errors
	PanicOnError ErrorHandling = iota
	// ContinueOnError continues on errors
	ContinueOnError
	// HTTPErrorOnError returns HTTP errors
	HTTPErrorOnError
)

// Logger interface for error logging
type Logger interface {
	Println(v ...interface{})
}

// Factory defines the interface for creating metrics providers
type Factory interface {
	// Create creates a new metrics provider with the given options
	Create(opts ProviderOptions) (Provider, error)
	
	// Name returns the name of the factory
	Name() string
	
	// Description returns a description of the factory
	Description() string
}

// ProviderBuilder helps build providers with fluent API
type ProviderBuilder interface {
	// WithNamespace sets the namespace for all metrics
	WithNamespace(namespace string) ProviderBuilder
	
	// WithSubsystem sets the subsystem for all metrics
	WithSubsystem(subsystem string) ProviderBuilder
	
	// WithConstLabels sets constant labels for all metrics
	WithConstLabels(labels map[string]string) ProviderBuilder
	
	// WithRegistry sets the registry to use
	WithRegistry(registry Registry) ProviderBuilder
	
	// WithGatherer sets the gatherer to use
	WithGatherer(gatherer Gatherer) ProviderBuilder
	
	// Build creates the provider
	Build() (Provider, error)
}

// NewProviderBuilder creates a new provider builder
func NewProviderBuilder(factory Factory) ProviderBuilder {
	return &providerBuilder{
		factory: factory,
		opts:    ProviderOptions{},
	}
}

// providerBuilder implements ProviderBuilder
type providerBuilder struct {
	factory Factory
	opts    ProviderOptions
}

func (b *providerBuilder) WithNamespace(namespace string) ProviderBuilder {
	b.opts.Namespace = namespace
	return b
}

func (b *providerBuilder) WithSubsystem(subsystem string) ProviderBuilder {
	b.opts.Subsystem = subsystem
	return b
}

func (b *providerBuilder) WithConstLabels(labels map[string]string) ProviderBuilder {
	b.opts.ConstLabels = labels
	return b
}

func (b *providerBuilder) WithRegistry(registry Registry) ProviderBuilder {
	b.opts.Registerer = registry
	b.opts.Gatherer = registry
	return b
}

func (b *providerBuilder) WithGatherer(gatherer Gatherer) ProviderBuilder {
	b.opts.Gatherer = gatherer
	return b
}

func (b *providerBuilder) Build() (Provider, error) {
	return b.factory.Create(b.opts)
}

// ConvenientProvider provides convenient methods for common metric operations
type ConvenientProvider interface {
	Provider
	
	// Convenient counter methods
	Counter(name, help string, labels ...string) Counter
	CounterVec(name, help string, labels []string) CounterVec
	
	// Convenient gauge methods
	Gauge(name, help string, labels ...string) Gauge
	GaugeVec(name, help string, labels []string) GaugeVec
	
	// Convenient histogram methods
	Histogram(name, help string, labels ...string) Histogram
	HistogramWithBuckets(name, help string, buckets []float64, labels ...string) Histogram
	HistogramVec(name, help string, labels []string) HistogramVec
	HistogramVecWithBuckets(name, help string, buckets []float64, labels []string) HistogramVec
	
	// Convenient summary methods
	Summary(name, help string, labels ...string) Summary
	SummaryWithObjectives(name, help string, objectives map[float64]float64, labels ...string) Summary
	SummaryVec(name, help string, labels []string) SummaryVec
	SummaryVecWithObjectives(name, help string, objectives map[float64]float64, labels []string) SummaryVec
}

// MultiProvider allows using multiple providers simultaneously
type MultiProvider interface {
	Provider
	
	// AddProvider adds a provider to the multi-provider
	AddProvider(name string, provider Provider) error
	
	// RemoveProvider removes a provider from the multi-provider
	RemoveProvider(name string) error
	
	// GetProvider gets a provider by name
	GetProvider(name string) (Provider, bool)
	
	// ListProviders lists all provider names
	ListProviders() []string
}

// WriterProvider allows writing metrics to different outputs
type WriterProvider interface {
	// WriteMetrics writes metrics to the given writer
	WriteMetrics(w io.Writer, format string) error
	
	// WriteMetricsWithOptions writes metrics with options
	WriteMetricsWithOptions(w io.Writer, opts *GatherOptions) error
}

// TimerProvider provides timer functionality
type TimerProvider interface {
	// Timer creates a timer that measures duration and records it to a histogram
	Timer(histogram Histogram) Timer
	
	// NewTimer creates a new timer with the given name and labels
	NewTimer(name, help string, labels ...string) Timer
}

// Timer represents a timer for measuring durations
type Timer interface {
	// Start starts the timer
	Start()
	
	// Stop stops the timer and records the duration
	Stop()
	
	// ObserveDuration observes a duration
	ObserveDuration() func()
	
	// Since records the time since the given start time
	Since(start int64)
}
