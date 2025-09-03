package metrics

import (
	"context"
	"time"
)

// MetricType represents the type of metric
type MetricType int

const (
	// CounterType represents a counter metric
	CounterType MetricType = iota
	// GaugeType represents a gauge metric
	GaugeType
	// HistogramType represents a histogram metric
	HistogramType
	// SummaryType represents a summary metric
	SummaryType
)

// String returns the string representation of MetricType
func (mt MetricType) String() string {
	switch mt {
	case CounterType:
		return "counter"
	case GaugeType:
		return "gauge"
	case HistogramType:
		return "histogram"
	case SummaryType:
		return "summary"
	default:
		return "unknown"
	}
}

// LabelPair represents a key-value pair for metric labels
type LabelPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Metric represents a single metric with its metadata
type Metric struct {
	Name        string      `json:"name"`
	Help        string      `json:"help"`
	Type        MetricType  `json:"type"`
	Labels      []LabelPair `json:"labels,omitempty"`
	Value       float64     `json:"value"`
	Timestamp   time.Time   `json:"timestamp"`
	Count       uint64      `json:"count,omitempty"`       // For histograms
	Sum         float64     `json:"sum,omitempty"`         // For histograms
	Buckets     []Bucket    `json:"buckets,omitempty"`     // For histograms
	Quantiles   []Quantile  `json:"quantiles,omitempty"`   // For summaries
}

// Bucket represents a histogram bucket
type Bucket struct {
	UpperBound float64 `json:"upper_bound"`
	Count      uint64  `json:"count"`
}

// Quantile represents a summary quantile
type Quantile struct {
	Quantile float64 `json:"quantile"`
	Value    float64 `json:"value"`
}

// MetricFamily represents a family of metrics with the same name but different labels
type MetricFamily struct {
	Name    string     `json:"name"`
	Help    string     `json:"help"`
	Type    MetricType `json:"type"`
	Metrics []Metric   `json:"metrics"`
}

// MetricOptions represents options for creating metrics
type MetricOptions struct {
	Name        string            `json:"name"`
	Help        string            `json:"help"`
	Labels      []string          `json:"labels,omitempty"`
	ConstLabels map[string]string `json:"const_labels,omitempty"`
	Buckets     []float64         `json:"buckets,omitempty"`     // For histograms
	Objectives  map[float64]float64 `json:"objectives,omitempty"` // For summaries
	MaxAge      time.Duration     `json:"max_age,omitempty"`     // For summaries
	AgeBuckets  uint32            `json:"age_buckets,omitempty"` // For summaries
	BufCap      uint32            `json:"buf_cap,omitempty"`     // For summaries
}

// ProviderOptions represents options for creating a metrics provider
type ProviderOptions struct {
	Namespace   string            `json:"namespace,omitempty"`
	Subsystem   string            `json:"subsystem,omitempty"`
	ConstLabels map[string]string `json:"const_labels,omitempty"`
	Registerer  Registerer        `json:"-"`
	Gatherer    Gatherer          `json:"-"`
}

// CollectorOptions represents options for collectors
type CollectorOptions struct {
	Namespace   string            `json:"namespace,omitempty"`
	Subsystem   string            `json:"subsystem,omitempty"`
	ConstLabels map[string]string `json:"const_labels,omitempty"`
}

// GatherOptions represents options for gathering metrics
type GatherOptions struct {
	Format      string            `json:"format,omitempty"`       // text, json, protobuf
	Compression string            `json:"compression,omitempty"`  // gzip, none
	Timeout     time.Duration     `json:"timeout,omitempty"`
	Context     context.Context   `json:"-"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// DefaultBuckets provides default histogram buckets
var DefaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// ExponentialBuckets creates exponential histogram buckets
func ExponentialBuckets(start, factor float64, count int) []float64 {
	if count <= 0 {
		return nil
	}
	buckets := make([]float64, count)
	for i := range buckets {
		buckets[i] = start
		start *= factor
	}
	return buckets
}

// LinearBuckets creates linear histogram buckets
func LinearBuckets(start, width float64, count int) []float64 {
	if count <= 0 {
		return nil
	}
	buckets := make([]float64, count)
	for i := range buckets {
		buckets[i] = start
		start += width
	}
	return buckets
}

// DefaultObjectives provides default summary objectives
var DefaultObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}

// ValidateName validates a metric name
func ValidateName(name string) error {
	if name == "" {
		return ErrInvalidName
	}
	// Add more validation rules as needed
	return nil
}

// ValidateLabels validates metric labels
func ValidateLabels(labels []string) error {
	for _, label := range labels {
		if label == "" {
			return ErrInvalidLabel
		}
		// Add more validation rules as needed
	}
	return nil
}

// NormalizeLabels normalizes label values
func NormalizeLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	normalized := make(map[string]string, len(labels))
	for k, v := range labels {
		normalized[k] = v
	}
	return normalized
}
