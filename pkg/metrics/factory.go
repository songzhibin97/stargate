package metrics

import (
	"fmt"
	"sync"
)

// Global registry for metric factories
var (
	factoryRegistry = make(map[string]Factory)
	factoryMutex    sync.RWMutex
)

// RegisterFactory registers a metrics factory
func RegisterFactory(name string, factory Factory) error {
	factoryMutex.Lock()
	defer factoryMutex.Unlock()
	
	if _, exists := factoryRegistry[name]; exists {
		return fmt.Errorf("factory %s already registered", name)
	}
	
	factoryRegistry[name] = factory
	return nil
}

// GetFactory retrieves a registered factory by name
func GetFactory(name string) (Factory, error) {
	factoryMutex.RLock()
	defer factoryMutex.RUnlock()
	
	factory, exists := factoryRegistry[name]
	if !exists {
		return nil, fmt.Errorf("factory %s not found", name)
	}
	
	return factory, nil
}

// ListFactories returns all registered factory names
func ListFactories() []string {
	factoryMutex.RLock()
	defer factoryMutex.RUnlock()
	
	names := make([]string, 0, len(factoryRegistry))
	for name := range factoryRegistry {
		names = append(names, name)
	}
	return names
}

// NewProvider creates a new provider using the specified factory
func NewProvider(factoryName string, opts ProviderOptions) (Provider, error) {
	factory, err := GetFactory(factoryName)
	if err != nil {
		return nil, err
	}
	
	return factory.Create(opts)
}

// MustNewProvider creates a new provider and panics on error
func MustNewProvider(factoryName string, opts ProviderOptions) Provider {
	provider, err := NewProvider(factoryName, opts)
	if err != nil {
		panic(fmt.Sprintf("failed to create provider: %v", err))
	}
	return provider
}

// ConvenientFactory provides convenient methods for creating common metrics
type ConvenientFactory struct {
	provider Provider
}

// NewConvenientFactory creates a new convenient factory
func NewConvenientFactory(provider Provider) *ConvenientFactory {
	return &ConvenientFactory{provider: provider}
}

// Counter creates a counter with the given name and help
func (f *ConvenientFactory) Counter(name, help string, labels ...string) (Counter, error) {
	opts := MetricOptions{
		Name:   name,
		Help:   help,
		Labels: labels,
	}
	return f.provider.NewCounter(opts)
}

// MustCounter creates a counter and panics on error
func (f *ConvenientFactory) MustCounter(name, help string, labels ...string) Counter {
	counter, err := f.Counter(name, help, labels...)
	if err != nil {
		panic(fmt.Sprintf("failed to create counter %s: %v", name, err))
	}
	return counter
}

// CounterVec creates a counter vector with the given name, help, and labels
func (f *ConvenientFactory) CounterVec(name, help string, labels []string) (CounterVec, error) {
	opts := MetricOptions{
		Name:   name,
		Help:   help,
		Labels: labels,
	}
	return f.provider.NewCounterVec(opts)
}

// MustCounterVec creates a counter vector and panics on error
func (f *ConvenientFactory) MustCounterVec(name, help string, labels []string) CounterVec {
	counterVec, err := f.CounterVec(name, help, labels)
	if err != nil {
		panic(fmt.Sprintf("failed to create counter vector %s: %v", name, err))
	}
	return counterVec
}

// Gauge creates a gauge with the given name and help
func (f *ConvenientFactory) Gauge(name, help string, labels ...string) (Gauge, error) {
	opts := MetricOptions{
		Name:   name,
		Help:   help,
		Labels: labels,
	}
	return f.provider.NewGauge(opts)
}

// MustGauge creates a gauge and panics on error
func (f *ConvenientFactory) MustGauge(name, help string, labels ...string) Gauge {
	gauge, err := f.Gauge(name, help, labels...)
	if err != nil {
		panic(fmt.Sprintf("failed to create gauge %s: %v", name, err))
	}
	return gauge
}

// GaugeVec creates a gauge vector with the given name, help, and labels
func (f *ConvenientFactory) GaugeVec(name, help string, labels []string) (GaugeVec, error) {
	opts := MetricOptions{
		Name:   name,
		Help:   help,
		Labels: labels,
	}
	return f.provider.NewGaugeVec(opts)
}

// MustGaugeVec creates a gauge vector and panics on error
func (f *ConvenientFactory) MustGaugeVec(name, help string, labels []string) GaugeVec {
	gaugeVec, err := f.GaugeVec(name, help, labels)
	if err != nil {
		panic(fmt.Sprintf("failed to create gauge vector %s: %v", name, err))
	}
	return gaugeVec
}

// Histogram creates a histogram with the given name and help
func (f *ConvenientFactory) Histogram(name, help string, labels ...string) (Histogram, error) {
	opts := MetricOptions{
		Name:    name,
		Help:    help,
		Labels:  labels,
		Buckets: DefaultBuckets,
	}
	return f.provider.NewHistogram(opts)
}

// HistogramWithBuckets creates a histogram with custom buckets
func (f *ConvenientFactory) HistogramWithBuckets(name, help string, buckets []float64, labels ...string) (Histogram, error) {
	opts := MetricOptions{
		Name:    name,
		Help:    help,
		Labels:  labels,
		Buckets: buckets,
	}
	return f.provider.NewHistogram(opts)
}

// MustHistogram creates a histogram and panics on error
func (f *ConvenientFactory) MustHistogram(name, help string, labels ...string) Histogram {
	histogram, err := f.Histogram(name, help, labels...)
	if err != nil {
		panic(fmt.Sprintf("failed to create histogram %s: %v", name, err))
	}
	return histogram
}

// HistogramVec creates a histogram vector with the given name, help, and labels
func (f *ConvenientFactory) HistogramVec(name, help string, labels []string) (HistogramVec, error) {
	opts := MetricOptions{
		Name:    name,
		Help:    help,
		Labels:  labels,
		Buckets: DefaultBuckets,
	}
	return f.provider.NewHistogramVec(opts)
}

// HistogramVecWithBuckets creates a histogram vector with custom buckets
func (f *ConvenientFactory) HistogramVecWithBuckets(name, help string, buckets []float64, labels []string) (HistogramVec, error) {
	opts := MetricOptions{
		Name:    name,
		Help:    help,
		Labels:  labels,
		Buckets: buckets,
	}
	return f.provider.NewHistogramVec(opts)
}

// MustHistogramVec creates a histogram vector and panics on error
func (f *ConvenientFactory) MustHistogramVec(name, help string, labels []string) HistogramVec {
	histogramVec, err := f.HistogramVec(name, help, labels)
	if err != nil {
		panic(fmt.Sprintf("failed to create histogram vector %s: %v", name, err))
	}
	return histogramVec
}

// MustHistogramVecWithBuckets creates a histogram vector with custom buckets and panics on error
func (f *ConvenientFactory) MustHistogramVecWithBuckets(name, help string, buckets []float64, labels []string) HistogramVec {
	histogramVec, err := f.HistogramVecWithBuckets(name, help, buckets, labels)
	if err != nil {
		panic(fmt.Sprintf("failed to create histogram vector %s: %v", name, err))
	}
	return histogramVec
}

// Summary creates a summary with the given name and help
func (f *ConvenientFactory) Summary(name, help string, labels ...string) (Summary, error) {
	opts := MetricOptions{
		Name:       name,
		Help:       help,
		Labels:     labels,
		Objectives: DefaultObjectives,
	}
	return f.provider.NewSummary(opts)
}

// SummaryWithObjectives creates a summary with custom objectives
func (f *ConvenientFactory) SummaryWithObjectives(name, help string, objectives map[float64]float64, labels ...string) (Summary, error) {
	opts := MetricOptions{
		Name:       name,
		Help:       help,
		Labels:     labels,
		Objectives: objectives,
	}
	return f.provider.NewSummary(opts)
}

// MustSummary creates a summary and panics on error
func (f *ConvenientFactory) MustSummary(name, help string, labels ...string) Summary {
	summary, err := f.Summary(name, help, labels...)
	if err != nil {
		panic(fmt.Sprintf("failed to create summary %s: %v", name, err))
	}
	return summary
}

// SummaryVec creates a summary vector with the given name, help, and labels
func (f *ConvenientFactory) SummaryVec(name, help string, labels []string) (SummaryVec, error) {
	opts := MetricOptions{
		Name:       name,
		Help:       help,
		Labels:     labels,
		Objectives: DefaultObjectives,
	}
	return f.provider.NewSummaryVec(opts)
}

// SummaryVecWithObjectives creates a summary vector with custom objectives
func (f *ConvenientFactory) SummaryVecWithObjectives(name, help string, objectives map[float64]float64, labels []string) (SummaryVec, error) {
	opts := MetricOptions{
		Name:       name,
		Help:       help,
		Labels:     labels,
		Objectives: objectives,
	}
	return f.provider.NewSummaryVec(opts)
}

// MustSummaryVec creates a summary vector and panics on error
func (f *ConvenientFactory) MustSummaryVec(name, help string, labels []string) SummaryVec {
	summaryVec, err := f.SummaryVec(name, help, labels)
	if err != nil {
		panic(fmt.Sprintf("failed to create summary vector %s: %v", name, err))
	}
	return summaryVec
}

// CommonMetrics provides commonly used metrics
type CommonMetrics struct {
	// HTTP metrics
	HTTPRequestsTotal    CounterVec
	HTTPRequestDuration  HistogramVec
	HTTPRequestSize      HistogramVec
	HTTPResponseSize     HistogramVec
	HTTPActiveConnections Gauge
	
	// System metrics
	ProcessStartTime     Gauge
	ProcessCPUSeconds    Counter
	ProcessMemoryBytes   Gauge
	ProcessOpenFDs       Gauge
	
	// Application metrics
	BuildInfo            Gauge
	UpTime              Counter
}

// NewCommonMetrics creates a set of common metrics
func NewCommonMetrics(factory *ConvenientFactory) *CommonMetrics {
	return &CommonMetrics{
		HTTPRequestsTotal: factory.MustCounterVec(
			"http_requests_total",
			"Total number of HTTP requests",
			[]string{"method", "route", "status_code"},
		),
		HTTPRequestDuration: factory.MustHistogramVecWithBuckets(
			"http_request_duration_seconds",
			"HTTP request duration in seconds",
			GetDefaultBuckets("duration"),
			[]string{"method", "route", "status_code"},
		),
		HTTPRequestSize: factory.MustHistogramVecWithBuckets(
			"http_request_size_bytes",
			"HTTP request size in bytes",
			GetDefaultBuckets("size"),
			[]string{"method", "route"},
		),
		HTTPResponseSize: factory.MustHistogramVecWithBuckets(
			"http_response_size_bytes",
			"HTTP response size in bytes",
			GetDefaultBuckets("size"),
			[]string{"method", "route", "status_code"},
		),
		HTTPActiveConnections: factory.MustGauge(
			"http_active_connections",
			"Number of active HTTP connections",
		),
		ProcessStartTime: factory.MustGauge(
			"process_start_time_seconds",
			"Start time of the process since unix epoch in seconds",
		),
		ProcessCPUSeconds: factory.MustCounter(
			"process_cpu_seconds_total",
			"Total user and system CPU time spent in seconds",
		),
		ProcessMemoryBytes: factory.MustGauge(
			"process_resident_memory_bytes",
			"Resident memory size in bytes",
		),
		ProcessOpenFDs: factory.MustGauge(
			"process_open_fds",
			"Number of open file descriptors",
		),
		BuildInfo: factory.MustGauge(
			"build_info",
			"Build information",
		),
		UpTime: factory.MustCounter(
			"uptime_seconds_total",
			"Total uptime in seconds",
		),
	}
}
