package metrics

import (
	"io"
)

// Counter represents a counter metric that only goes up
type Counter interface {
	// Inc increments the counter by 1
	Inc()
	
	// Add adds the given value to the counter
	// The value must be >= 0
	Add(delta float64)
	
	// Get returns the current value of the counter
	Get() float64
}

// Gauge represents a gauge metric that can go up and down
type Gauge interface {
	// Set sets the gauge to the given value
	Set(value float64)
	
	// Inc increments the gauge by 1
	Inc()
	
	// Dec decrements the gauge by 1
	Dec()
	
	// Add adds the given value to the gauge
	Add(delta float64)
	
	// Sub subtracts the given value from the gauge
	Sub(delta float64)
	
	// Get returns the current value of the gauge
	Get() float64
	
	// SetToCurrentTime sets the gauge to the current Unix time in seconds
	SetToCurrentTime()
}

// Histogram represents a histogram metric for observing distributions
type Histogram interface {
	// Observe adds a single observation to the histogram
	Observe(value float64)
	
	// GetCount returns the total number of observations
	GetCount() uint64
	
	// GetSum returns the sum of all observed values
	GetSum() float64
	
	// GetBuckets returns the histogram buckets
	GetBuckets() []Bucket
}

// Summary represents a summary metric for observing distributions with quantiles
type Summary interface {
	// Observe adds a single observation to the summary
	Observe(value float64)
	
	// GetCount returns the total number of observations
	GetCount() uint64
	
	// GetSum returns the sum of all observed values
	GetSum() float64
	
	// GetQuantiles returns the summary quantiles
	GetQuantiles() []Quantile
}

// CounterVec represents a vector of counters with different label values
type CounterVec interface {
	// WithLabelValues returns the Counter for the given slice of label values
	WithLabelValues(lvs ...string) Counter
	
	// With returns the Counter for the given Labels map
	With(labels map[string]string) Counter
	
	// GetMetricWithLabelValues returns the Counter for the given slice of label values
	GetMetricWithLabelValues(lvs ...string) (Counter, error)
	
	// GetMetricWith returns the Counter for the given Labels map
	GetMetricWith(labels map[string]string) (Counter, error)
	
	// Delete deletes the metric where the variable labels have the given values
	Delete(labels map[string]string) bool
	
	// DeleteLabelValues deletes the metric where the variable labels have the given values
	DeleteLabelValues(lvs ...string) bool
	
	// Reset deletes all metrics in this vector
	Reset()
}

// GaugeVec represents a vector of gauges with different label values
type GaugeVec interface {
	// WithLabelValues returns the Gauge for the given slice of label values
	WithLabelValues(lvs ...string) Gauge
	
	// With returns the Gauge for the given Labels map
	With(labels map[string]string) Gauge
	
	// GetMetricWithLabelValues returns the Gauge for the given slice of label values
	GetMetricWithLabelValues(lvs ...string) (Gauge, error)
	
	// GetMetricWith returns the Gauge for the given Labels map
	GetMetricWith(labels map[string]string) (Gauge, error)
	
	// Delete deletes the metric where the variable labels have the given values
	Delete(labels map[string]string) bool
	
	// DeleteLabelValues deletes the metric where the variable labels have the given values
	DeleteLabelValues(lvs ...string) bool
	
	// Reset deletes all metrics in this vector
	Reset()
}

// HistogramVec represents a vector of histograms with different label values
type HistogramVec interface {
	// WithLabelValues returns the Histogram for the given slice of label values
	WithLabelValues(lvs ...string) Histogram
	
	// With returns the Histogram for the given Labels map
	With(labels map[string]string) Histogram
	
	// GetMetricWithLabelValues returns the Histogram for the given slice of label values
	GetMetricWithLabelValues(lvs ...string) (Histogram, error)
	
	// GetMetricWith returns the Histogram for the given Labels map
	GetMetricWith(labels map[string]string) (Histogram, error)
	
	// Delete deletes the metric where the variable labels have the given values
	Delete(labels map[string]string) bool
	
	// DeleteLabelValues deletes the metric where the variable labels have the given values
	DeleteLabelValues(lvs ...string) bool
	
	// Reset deletes all metrics in this vector
	Reset()
}

// SummaryVec represents a vector of summaries with different label values
type SummaryVec interface {
	// WithLabelValues returns the Summary for the given slice of label values
	WithLabelValues(lvs ...string) Summary
	
	// With returns the Summary for the given Labels map
	With(labels map[string]string) Summary
	
	// GetMetricWithLabelValues returns the Summary for the given slice of label values
	GetMetricWithLabelValues(lvs ...string) (Summary, error)
	
	// GetMetricWith returns the Summary for the given Labels map
	GetMetricWith(labels map[string]string) (Summary, error)
	
	// Delete deletes the metric where the variable labels have the given values
	Delete(labels map[string]string) bool
	
	// DeleteLabelValues deletes the metric where the variable labels have the given values
	DeleteLabelValues(lvs ...string) bool
	
	// Reset deletes all metrics in this vector
	Reset()
}

// Collector represents a metric collector
type Collector interface {
	// Describe sends the super-set of all possible descriptors of metrics
	// collected by this Collector to the provided channel and returns once
	// the last descriptor has been sent
	Describe(chan<- *Desc)
	
	// Collect is called by the registry when collecting metrics
	Collect(chan<- Metric)
}

// Desc represents a metric descriptor
type Desc struct {
	FQName      string
	Help        string
	ConstLabels map[string]string
	VariableLabels []string
}

// Registerer is the interface for the part of a registry in charge of registering
// and unregistering
type Registerer interface {
	// Register registers a new Collector to be included in metrics collection
	Register(Collector) error
	
	// MustRegister works like Register but panics if an error occurs
	MustRegister(...Collector)
	
	// Unregister unregisters the Collector that equals the Collector passed in as parameter
	Unregister(Collector) bool
}

// Gatherer is the interface for the part of a registry in charge of gathering
// the collected metrics into a number of MetricFamilies
type Gatherer interface {
	// Gather calls the Collect method of the registered Collectors and then
	// gathers the collected metrics into a lexicographically sorted slice
	// of uniquely named MetricFamily protobufs
	Gather() ([]*MetricFamily, error)
	
	// GatherWithOptions gathers metrics with the given options
	GatherWithOptions(opts *GatherOptions) ([]*MetricFamily, error)
}

// Registry combines Registerer and Gatherer interfaces
type Registry interface {
	Registerer
	Gatherer
}

// Handler represents an HTTP handler for metrics
type Handler interface {
	// ServeHTTP serves metrics over HTTP
	ServeHTTP(w io.Writer, r interface{}) error
	
	// ContentType returns the content type for the metrics format
	ContentType() string
}
