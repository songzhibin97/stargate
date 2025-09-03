package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/songzhibin97/stargate/pkg/metrics"
)

// PrometheusProvider implements the metrics.Provider interface using Prometheus
type PrometheusProvider struct {
	registry   *prometheus.Registry
	gatherer   prometheus.Gatherer
	namespace  string
	subsystem  string
	constLabels prometheus.Labels
	
	// Metrics storage
	counters    map[string]*prometheusCounter
	counterVecs map[string]*prometheusCounterVec
	gauges      map[string]*prometheusGauge
	gaugeVecs   map[string]*prometheusGaugeVec
	histograms  map[string]*prometheusHistogram
	histogramVecs map[string]*prometheusHistogramVec
	summaries   map[string]*prometheusSummary
	summaryVecs map[string]*prometheusSummaryVec
	
	// Synchronization
	mu sync.RWMutex
	
	// Lifecycle
	started bool
	closed  bool
}

// Options for creating a PrometheusProvider
type Options struct {
	Registry    *prometheus.Registry
	Gatherer    prometheus.Gatherer
	Namespace   string
	Subsystem   string
	ConstLabels map[string]string
}

// NewProvider creates a new PrometheusProvider
func NewProvider(opts Options) (*PrometheusProvider, error) {
	registry := opts.Registry
	if registry == nil {
		registry = prometheus.NewRegistry()
	}
	
	gatherer := opts.Gatherer
	if gatherer == nil {
		gatherer = registry
	}
	
	constLabels := make(prometheus.Labels)
	for k, v := range opts.ConstLabels {
		constLabels[k] = v
	}
	
	return &PrometheusProvider{
		registry:      registry,
		gatherer:      gatherer,
		namespace:     opts.Namespace,
		subsystem:     opts.Subsystem,
		constLabels:   constLabels,
		counters:      make(map[string]*prometheusCounter),
		counterVecs:   make(map[string]*prometheusCounterVec),
		gauges:        make(map[string]*prometheusGauge),
		gaugeVecs:     make(map[string]*prometheusGaugeVec),
		histograms:    make(map[string]*prometheusHistogram),
		histogramVecs: make(map[string]*prometheusHistogramVec),
		summaries:     make(map[string]*prometheusSummary),
		summaryVecs:   make(map[string]*prometheusSummaryVec),
	}, nil
}

// NewCounter creates a new counter metric
func (p *PrometheusProvider) NewCounter(opts metrics.MetricOptions) (metrics.Counter, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return nil, metrics.ErrProviderClosed
	}
	
	if err := metrics.ValidateMetricName(opts.Name); err != nil {
		return nil, err
	}
	
	if err := metrics.ValidateLabelNames(opts.Labels); err != nil {
		return nil, err
	}
	
	fqName := metrics.BuildFQName(p.namespace, p.subsystem, opts.Name)
	
	// Check if already exists
	if existing, exists := p.counters[fqName]; exists {
		return existing, nil
	}
	
	// Merge constant labels
	constLabels := metrics.MergeLabelMaps(p.constLabels, opts.ConstLabels)
	
	promCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   p.namespace,
		Subsystem:   p.subsystem,
		Name:        opts.Name,
		Help:        opts.Help,
		ConstLabels: constLabels,
	})
	
	if err := p.registry.Register(promCounter); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Use existing metric
			if existing, ok := are.ExistingCollector.(prometheus.Counter); ok {
				counter := &prometheusCounter{counter: existing}
				p.counters[fqName] = counter
				return counter, nil
			}
		}
		return nil, fmt.Errorf("failed to register counter %s: %w", fqName, err)
	}
	
	counter := &prometheusCounter{counter: promCounter}
	p.counters[fqName] = counter
	return counter, nil
}

// NewCounterVec creates a new counter vector metric
func (p *PrometheusProvider) NewCounterVec(opts metrics.MetricOptions) (metrics.CounterVec, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return nil, metrics.ErrProviderClosed
	}
	
	if err := metrics.ValidateMetricName(opts.Name); err != nil {
		return nil, err
	}
	
	if err := metrics.ValidateLabelNames(opts.Labels); err != nil {
		return nil, err
	}
	
	fqName := metrics.BuildFQName(p.namespace, p.subsystem, opts.Name)
	
	// Check if already exists
	if existing, exists := p.counterVecs[fqName]; exists {
		return existing, nil
	}
	
	// Merge constant labels
	constLabels := metrics.MergeLabelMaps(p.constLabels, opts.ConstLabels)
	
	promCounterVec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   p.namespace,
		Subsystem:   p.subsystem,
		Name:        opts.Name,
		Help:        opts.Help,
		ConstLabels: constLabels,
	}, opts.Labels)
	
	if err := p.registry.Register(promCounterVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Use existing metric
			if existing, ok := are.ExistingCollector.(*prometheus.CounterVec); ok {
				counterVec := &prometheusCounterVec{counterVec: existing}
				p.counterVecs[fqName] = counterVec
				return counterVec, nil
			}
		}
		return nil, fmt.Errorf("failed to register counter vector %s: %w", fqName, err)
	}
	
	counterVec := &prometheusCounterVec{counterVec: promCounterVec}
	p.counterVecs[fqName] = counterVec
	return counterVec, nil
}

// NewGauge creates a new gauge metric
func (p *PrometheusProvider) NewGauge(opts metrics.MetricOptions) (metrics.Gauge, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return nil, metrics.ErrProviderClosed
	}
	
	if err := metrics.ValidateMetricName(opts.Name); err != nil {
		return nil, err
	}
	
	if err := metrics.ValidateLabelNames(opts.Labels); err != nil {
		return nil, err
	}
	
	fqName := metrics.BuildFQName(p.namespace, p.subsystem, opts.Name)
	
	// Check if already exists
	if existing, exists := p.gauges[fqName]; exists {
		return existing, nil
	}
	
	// Merge constant labels
	constLabels := metrics.MergeLabelMaps(p.constLabels, opts.ConstLabels)
	
	promGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   p.namespace,
		Subsystem:   p.subsystem,
		Name:        opts.Name,
		Help:        opts.Help,
		ConstLabels: constLabels,
	})
	
	if err := p.registry.Register(promGauge); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Use existing metric
			if existing, ok := are.ExistingCollector.(prometheus.Gauge); ok {
				gauge := &prometheusGauge{gauge: existing}
				p.gauges[fqName] = gauge
				return gauge, nil
			}
		}
		return nil, fmt.Errorf("failed to register gauge %s: %w", fqName, err)
	}
	
	gauge := &prometheusGauge{gauge: promGauge}
	p.gauges[fqName] = gauge
	return gauge, nil
}

// NewGaugeVec creates a new gauge vector metric
func (p *PrometheusProvider) NewGaugeVec(opts metrics.MetricOptions) (metrics.GaugeVec, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return nil, metrics.ErrProviderClosed
	}
	
	if err := metrics.ValidateMetricName(opts.Name); err != nil {
		return nil, err
	}
	
	if err := metrics.ValidateLabelNames(opts.Labels); err != nil {
		return nil, err
	}
	
	fqName := metrics.BuildFQName(p.namespace, p.subsystem, opts.Name)
	
	// Check if already exists
	if existing, exists := p.gaugeVecs[fqName]; exists {
		return existing, nil
	}
	
	// Merge constant labels
	constLabels := metrics.MergeLabelMaps(p.constLabels, opts.ConstLabels)
	
	promGaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   p.namespace,
		Subsystem:   p.subsystem,
		Name:        opts.Name,
		Help:        opts.Help,
		ConstLabels: constLabels,
	}, opts.Labels)
	
	if err := p.registry.Register(promGaugeVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Use existing metric
			if existing, ok := are.ExistingCollector.(*prometheus.GaugeVec); ok {
				gaugeVec := &prometheusGaugeVec{gaugeVec: existing}
				p.gaugeVecs[fqName] = gaugeVec
				return gaugeVec, nil
			}
		}
		return nil, fmt.Errorf("failed to register gauge vector %s: %w", fqName, err)
	}
	
	gaugeVec := &prometheusGaugeVec{gaugeVec: promGaugeVec}
	p.gaugeVecs[fqName] = gaugeVec
	return gaugeVec, nil
}

// NewHistogram creates a new histogram metric
func (p *PrometheusProvider) NewHistogram(opts metrics.MetricOptions) (metrics.Histogram, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, metrics.ErrProviderClosed
	}

	if err := metrics.ValidateMetricName(opts.Name); err != nil {
		return nil, err
	}

	if err := metrics.ValidateLabelNames(opts.Labels); err != nil {
		return nil, err
	}

	buckets := opts.Buckets
	if len(buckets) == 0 {
		buckets = metrics.DefaultBuckets
	}

	if err := metrics.ValidateHistogramBuckets(buckets); err != nil {
		return nil, err
	}

	fqName := metrics.BuildFQName(p.namespace, p.subsystem, opts.Name)

	// Check if already exists
	if existing, exists := p.histograms[fqName]; exists {
		return existing, nil
	}

	// Merge constant labels
	constLabels := metrics.MergeLabelMaps(p.constLabels, opts.ConstLabels)

	promHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace:   p.namespace,
		Subsystem:   p.subsystem,
		Name:        opts.Name,
		Help:        opts.Help,
		ConstLabels: constLabels,
		Buckets:     buckets,
	})

	if err := p.registry.Register(promHistogram); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Use existing metric
			if existing, ok := are.ExistingCollector.(prometheus.Histogram); ok {
				histogram := &prometheusHistogram{histogram: existing}
				p.histograms[fqName] = histogram
				return histogram, nil
			}
		}
		return nil, fmt.Errorf("failed to register histogram %s: %w", fqName, err)
	}

	histogram := &prometheusHistogram{histogram: promHistogram}
	p.histograms[fqName] = histogram
	return histogram, nil
}

// NewHistogramVec creates a new histogram vector metric
func (p *PrometheusProvider) NewHistogramVec(opts metrics.MetricOptions) (metrics.HistogramVec, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, metrics.ErrProviderClosed
	}

	if err := metrics.ValidateMetricName(opts.Name); err != nil {
		return nil, err
	}

	if err := metrics.ValidateLabelNames(opts.Labels); err != nil {
		return nil, err
	}

	buckets := opts.Buckets
	if len(buckets) == 0 {
		buckets = metrics.DefaultBuckets
	}

	if err := metrics.ValidateHistogramBuckets(buckets); err != nil {
		return nil, err
	}

	fqName := metrics.BuildFQName(p.namespace, p.subsystem, opts.Name)

	// Check if already exists
	if existing, exists := p.histogramVecs[fqName]; exists {
		return existing, nil
	}

	// Merge constant labels
	constLabels := metrics.MergeLabelMaps(p.constLabels, opts.ConstLabels)

	promHistogramVec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   p.namespace,
		Subsystem:   p.subsystem,
		Name:        opts.Name,
		Help:        opts.Help,
		ConstLabels: constLabels,
		Buckets:     buckets,
	}, opts.Labels)

	if err := p.registry.Register(promHistogramVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Use existing metric
			if existing, ok := are.ExistingCollector.(*prometheus.HistogramVec); ok {
				histogramVec := &prometheusHistogramVec{histogramVec: existing}
				p.histogramVecs[fqName] = histogramVec
				return histogramVec, nil
			}
		}
		return nil, fmt.Errorf("failed to register histogram vector %s: %w", fqName, err)
	}

	histogramVec := &prometheusHistogramVec{histogramVec: promHistogramVec}
	p.histogramVecs[fqName] = histogramVec
	return histogramVec, nil
}

// NewSummary creates a new summary metric
func (p *PrometheusProvider) NewSummary(opts metrics.MetricOptions) (metrics.Summary, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, metrics.ErrProviderClosed
	}

	if err := metrics.ValidateMetricName(opts.Name); err != nil {
		return nil, err
	}

	if err := metrics.ValidateLabelNames(opts.Labels); err != nil {
		return nil, err
	}

	objectives := opts.Objectives
	if len(objectives) == 0 {
		objectives = metrics.DefaultObjectives
	}

	if err := metrics.ValidateSummaryObjectives(objectives); err != nil {
		return nil, err
	}

	fqName := metrics.BuildFQName(p.namespace, p.subsystem, opts.Name)

	// Check if already exists
	if existing, exists := p.summaries[fqName]; exists {
		return existing, nil
	}

	// Merge constant labels
	constLabels := metrics.MergeLabelMaps(p.constLabels, opts.ConstLabels)

	summaryOpts := prometheus.SummaryOpts{
		Namespace:   p.namespace,
		Subsystem:   p.subsystem,
		Name:        opts.Name,
		Help:        opts.Help,
		ConstLabels: constLabels,
		Objectives:  objectives,
	}

	if opts.MaxAge > 0 {
		summaryOpts.MaxAge = opts.MaxAge
	}
	if opts.AgeBuckets > 0 {
		summaryOpts.AgeBuckets = opts.AgeBuckets
	}
	if opts.BufCap > 0 {
		summaryOpts.BufCap = opts.BufCap
	}

	promSummary := prometheus.NewSummary(summaryOpts)

	if err := p.registry.Register(promSummary); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Use existing metric
			if existing, ok := are.ExistingCollector.(prometheus.Summary); ok {
				summary := &prometheusSummary{summary: existing}
				p.summaries[fqName] = summary
				return summary, nil
			}
		}
		return nil, fmt.Errorf("failed to register summary %s: %w", fqName, err)
	}

	summary := &prometheusSummary{summary: promSummary}
	p.summaries[fqName] = summary
	return summary, nil
}

// NewSummaryVec creates a new summary vector metric
func (p *PrometheusProvider) NewSummaryVec(opts metrics.MetricOptions) (metrics.SummaryVec, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, metrics.ErrProviderClosed
	}

	if err := metrics.ValidateMetricName(opts.Name); err != nil {
		return nil, err
	}

	if err := metrics.ValidateLabelNames(opts.Labels); err != nil {
		return nil, err
	}

	objectives := opts.Objectives
	if len(objectives) == 0 {
		objectives = metrics.DefaultObjectives
	}

	if err := metrics.ValidateSummaryObjectives(objectives); err != nil {
		return nil, err
	}

	fqName := metrics.BuildFQName(p.namespace, p.subsystem, opts.Name)

	// Check if already exists
	if existing, exists := p.summaryVecs[fqName]; exists {
		return existing, nil
	}

	// Merge constant labels
	constLabels := metrics.MergeLabelMaps(p.constLabels, opts.ConstLabels)

	summaryOpts := prometheus.SummaryOpts{
		Namespace:   p.namespace,
		Subsystem:   p.subsystem,
		Name:        opts.Name,
		Help:        opts.Help,
		ConstLabels: constLabels,
		Objectives:  objectives,
	}

	if opts.MaxAge > 0 {
		summaryOpts.MaxAge = opts.MaxAge
	}
	if opts.AgeBuckets > 0 {
		summaryOpts.AgeBuckets = opts.AgeBuckets
	}
	if opts.BufCap > 0 {
		summaryOpts.BufCap = opts.BufCap
	}

	promSummaryVec := prometheus.NewSummaryVec(summaryOpts, opts.Labels)

	if err := p.registry.Register(promSummaryVec); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Use existing metric
			if existing, ok := are.ExistingCollector.(*prometheus.SummaryVec); ok {
				summaryVec := &prometheusSummaryVec{summaryVec: existing}
				p.summaryVecs[fqName] = summaryVec
				return summaryVec, nil
			}
		}
		return nil, fmt.Errorf("failed to register summary vector %s: %w", fqName, err)
	}

	summaryVec := &prometheusSummaryVec{summaryVec: promSummaryVec}
	p.summaryVecs[fqName] = summaryVec
	return summaryVec, nil
}

// Register registers a collector with the registry
func (p *PrometheusProvider) Register(collector metrics.Collector) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return metrics.ErrProviderClosed
	}

	// Convert metrics.Collector to prometheus.Collector
	promCollector := &collectorAdapter{collector: collector}

	if err := p.registry.Register(promCollector); err != nil {
		return fmt.Errorf("failed to register collector: %w", err)
	}

	return nil
}

// Unregister unregisters a collector from the registry
func (p *PrometheusProvider) Unregister(collector metrics.Collector) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return metrics.ErrProviderClosed
	}

	// Convert metrics.Collector to prometheus.Collector
	promCollector := &collectorAdapter{collector: collector}

	if !p.registry.Unregister(promCollector) {
		return metrics.ErrCollectorNotFound
	}

	return nil
}

// Gather collects all metrics from the registry
func (p *PrometheusProvider) Gather() ([]*metrics.MetricFamily, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return nil, metrics.ErrProviderClosed
	}

	promFamilies, err := p.gatherer.Gather()
	if err != nil {
		return nil, fmt.Errorf("failed to gather metrics: %w", err)
	}

	families := make([]*metrics.MetricFamily, len(promFamilies))
	for i, promFamily := range promFamilies {
		families[i] = convertMetricFamily(promFamily)
	}

	return families, nil
}

// GatherWithOptions collects metrics with options
func (p *PrometheusProvider) GatherWithOptions(opts *metrics.GatherOptions) ([]*metrics.MetricFamily, error) {
	// For now, ignore options and use standard gather
	// TODO: Implement options like timeout, format, etc.
	return p.Gather()
}

// Handler returns an HTTP handler for the metrics endpoint
func (p *PrometheusProvider) Handler() http.Handler {
	return promhttp.HandlerFor(p.gatherer, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})
}

// HandlerFor returns an HTTP handler with custom options
func (p *PrometheusProvider) HandlerFor(gatherer metrics.Gatherer, opts metrics.HandlerOpts) http.Handler {
	// Convert metrics.Gatherer to prometheus.Gatherer
	var promGatherer prometheus.Gatherer
	if gatherer != nil {
		promGatherer = &gathererAdapter{gatherer: gatherer}
	} else {
		promGatherer = p.gatherer
	}

	promOpts := promhttp.HandlerOpts{
		DisableCompression: opts.DisableCompression,
		MaxRequestsInFlight: opts.MaxRequestsInFlight,
	}

	if opts.ErrorHandling == metrics.PanicOnError {
		promOpts.ErrorHandling = promhttp.PanicOnError
	} else if opts.ErrorHandling == metrics.HTTPErrorOnError {
		promOpts.ErrorHandling = promhttp.HTTPErrorOnError
	} else {
		promOpts.ErrorHandling = promhttp.ContinueOnError
	}

	if opts.ErrorLog != nil {
		promOpts.ErrorLog = opts.ErrorLog
	}

	if opts.Registry != nil {
		// Try to use the registry as gatherer if it implements prometheus.Gatherer
		// For now, we'll use the default gatherer
		promGatherer = p.gatherer
	}

	return promhttp.HandlerFor(promGatherer, promOpts)
}

// Start starts the provider
func (p *PrometheusProvider) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return metrics.ErrProviderClosed
	}

	if p.started {
		return nil
	}

	p.started = true
	return nil
}

// Stop stops the provider
func (p *PrometheusProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.started = false

	// Clear all metrics
	p.counters = make(map[string]*prometheusCounter)
	p.counterVecs = make(map[string]*prometheusCounterVec)
	p.gauges = make(map[string]*prometheusGauge)
	p.gaugeVecs = make(map[string]*prometheusGaugeVec)
	p.histograms = make(map[string]*prometheusHistogram)
	p.histogramVecs = make(map[string]*prometheusHistogramVec)
	p.summaries = make(map[string]*prometheusSummary)
	p.summaryVecs = make(map[string]*prometheusSummaryVec)

	return nil
}

// Health checks the health of the provider
func (p *PrometheusProvider) Health() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return metrics.ErrProviderClosed
	}

	// Try to gather metrics to check if everything is working
	_, err := p.gatherer.Gather()
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// Name returns the name of the provider
func (p *PrometheusProvider) Name() string {
	return "prometheus"
}

// Version returns the version of the provider
func (p *PrometheusProvider) Version() string {
	return "1.0.0"
}
