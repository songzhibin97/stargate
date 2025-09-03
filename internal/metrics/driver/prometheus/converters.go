package prometheus

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/songzhibin97/stargate/pkg/metrics"
)

// convertMetricFamily converts a Prometheus MetricFamily to our metrics.MetricFamily
func convertMetricFamily(promFamily *dto.MetricFamily) *metrics.MetricFamily {
	family := &metrics.MetricFamily{
		Name:    promFamily.GetName(),
		Help:    promFamily.GetHelp(),
		Type:    convertMetricType(promFamily.GetType()),
		Metrics: make([]metrics.Metric, len(promFamily.GetMetric())),
	}
	
	for i, promMetric := range promFamily.GetMetric() {
		family.Metrics[i] = convertMetric(promMetric, family.Type)
	}
	
	return family
}

// convertMetricType converts a Prometheus MetricType to our metrics.MetricType
func convertMetricType(promType dto.MetricType) metrics.MetricType {
	switch promType {
	case dto.MetricType_COUNTER:
		return metrics.CounterType
	case dto.MetricType_GAUGE:
		return metrics.GaugeType
	case dto.MetricType_HISTOGRAM:
		return metrics.HistogramType
	case dto.MetricType_SUMMARY:
		return metrics.SummaryType
	default:
		return metrics.CounterType
	}
}

// convertMetric converts a Prometheus Metric to our metrics.Metric
func convertMetric(promMetric *dto.Metric, metricType metrics.MetricType) metrics.Metric {
	metric := metrics.Metric{
		Labels:    convertLabelPairs(promMetric.GetLabel()),
		Timestamp: time.Unix(0, promMetric.GetTimestampMs()*int64(time.Millisecond)),
	}
	
	switch metricType {
	case metrics.CounterType:
		if counter := promMetric.GetCounter(); counter != nil {
			metric.Value = counter.GetValue()
		}
	case metrics.GaugeType:
		if gauge := promMetric.GetGauge(); gauge != nil {
			metric.Value = gauge.GetValue()
		}
	case metrics.HistogramType:
		if histogram := promMetric.GetHistogram(); histogram != nil {
			metric.Count = histogram.GetSampleCount()
			metric.Sum = histogram.GetSampleSum()
			metric.Buckets = convertHistogramBuckets(histogram.GetBucket())
		}
	case metrics.SummaryType:
		if summary := promMetric.GetSummary(); summary != nil {
			metric.Count = summary.GetSampleCount()
			metric.Sum = summary.GetSampleSum()
			metric.Quantiles = convertSummaryQuantiles(summary.GetQuantile())
		}
	}
	
	return metric
}

// convertLabelPairs converts Prometheus LabelPairs to our metrics.LabelPair
func convertLabelPairs(promLabels []*dto.LabelPair) []metrics.LabelPair {
	labels := make([]metrics.LabelPair, len(promLabels))
	for i, promLabel := range promLabels {
		labels[i] = metrics.LabelPair{
			Name:  promLabel.GetName(),
			Value: promLabel.GetValue(),
		}
	}
	return labels
}

// convertHistogramBuckets converts Prometheus histogram buckets to our metrics.Bucket
func convertHistogramBuckets(promBuckets []*dto.Bucket) []metrics.Bucket {
	buckets := make([]metrics.Bucket, len(promBuckets))
	for i, promBucket := range promBuckets {
		buckets[i] = metrics.Bucket{
			UpperBound: promBucket.GetUpperBound(),
			Count:      promBucket.GetCumulativeCount(),
		}
	}
	return buckets
}

// convertSummaryQuantiles converts Prometheus summary quantiles to our metrics.Quantile
func convertSummaryQuantiles(promQuantiles []*dto.Quantile) []metrics.Quantile {
	quantiles := make([]metrics.Quantile, len(promQuantiles))
	for i, promQuantile := range promQuantiles {
		quantiles[i] = metrics.Quantile{
			Quantile: promQuantile.GetQuantile(),
			Value:    promQuantile.GetValue(),
		}
	}
	return quantiles
}

// collectorAdapter adapts metrics.Collector to prometheus.Collector
type collectorAdapter struct {
	collector metrics.Collector
}

func (ca *collectorAdapter) Describe(ch chan<- *prometheus.Desc) {
	// Convert metrics.Desc to prometheus.Desc
	descCh := make(chan *metrics.Desc, 10)
	go func() {
		defer close(descCh)
		ca.collector.Describe(descCh)
	}()
	
	for desc := range descCh {
		promDesc := prometheus.NewDesc(
			desc.FQName,
			desc.Help,
			desc.VariableLabels,
			desc.ConstLabels,
		)
		ch <- promDesc
	}
}

func (ca *collectorAdapter) Collect(ch chan<- prometheus.Metric) {
	// Convert metrics.Metric to prometheus.Metric
	metricCh := make(chan metrics.Metric, 10)
	go func() {
		defer close(metricCh)
		ca.collector.Collect(metricCh)
	}()
	
	for metric := range metricCh {
		promMetric := convertToPrometheusMetric(metric)
		if promMetric != nil {
			ch <- promMetric
		}
	}
}

// convertToPrometheusMetric converts our metrics.Metric to prometheus.Metric
func convertToPrometheusMetric(metric metrics.Metric) prometheus.Metric {
	// This is a simplified implementation
	// In a real implementation, you would need to create proper prometheus.Metric instances
	// based on the metric type and data
	return nil
}

// gathererAdapter adapts metrics.Gatherer to prometheus.Gatherer
type gathererAdapter struct {
	gatherer metrics.Gatherer
}

func (ga *gathererAdapter) Gather() ([]*dto.MetricFamily, error) {
	families, err := ga.gatherer.Gather()
	if err != nil {
		return nil, err
	}
	
	promFamilies := make([]*dto.MetricFamily, len(families))
	for i, family := range families {
		promFamilies[i] = convertToPrometheusMetricFamily(family)
	}
	
	return promFamilies, nil
}

// convertToPrometheusMetricFamily converts our metrics.MetricFamily to dto.MetricFamily
func convertToPrometheusMetricFamily(family *metrics.MetricFamily) *dto.MetricFamily {
	promType := convertToPrometheusMetricType(family.Type)
	
	promFamily := &dto.MetricFamily{
		Name:   &family.Name,
		Help:   &family.Help,
		Type:   &promType,
		Metric: make([]*dto.Metric, len(family.Metrics)),
	}
	
	for i, metric := range family.Metrics {
		promFamily.Metric[i] = convertToPrometheusMetricDTO(metric, family.Type)
	}
	
	return promFamily
}

// convertToPrometheusMetricType converts our metrics.MetricType to dto.MetricType
func convertToPrometheusMetricType(metricType metrics.MetricType) dto.MetricType {
	switch metricType {
	case metrics.CounterType:
		return dto.MetricType_COUNTER
	case metrics.GaugeType:
		return dto.MetricType_GAUGE
	case metrics.HistogramType:
		return dto.MetricType_HISTOGRAM
	case metrics.SummaryType:
		return dto.MetricType_SUMMARY
	default:
		return dto.MetricType_COUNTER
	}
}

// convertToPrometheusMetricDTO converts our metrics.Metric to dto.Metric
func convertToPrometheusMetricDTO(metric metrics.Metric, metricType metrics.MetricType) *dto.Metric {
	promMetric := &dto.Metric{
		Label: make([]*dto.LabelPair, len(metric.Labels)),
	}
	
	// Convert labels
	for i, label := range metric.Labels {
		promMetric.Label[i] = &dto.LabelPair{
			Name:  &label.Name,
			Value: &label.Value,
		}
	}
	
	// Set timestamp if available
	if !metric.Timestamp.IsZero() {
		timestampMs := metric.Timestamp.UnixNano() / int64(time.Millisecond)
		promMetric.TimestampMs = &timestampMs
	}
	
	// Set metric value based on type
	switch metricType {
	case metrics.CounterType:
		promMetric.Counter = &dto.Counter{
			Value: &metric.Value,
		}
	case metrics.GaugeType:
		promMetric.Gauge = &dto.Gauge{
			Value: &metric.Value,
		}
	case metrics.HistogramType:
		promBuckets := make([]*dto.Bucket, len(metric.Buckets))
		for i, bucket := range metric.Buckets {
			promBuckets[i] = &dto.Bucket{
				UpperBound:      &bucket.UpperBound,
				CumulativeCount: &bucket.Count,
			}
		}
		promMetric.Histogram = &dto.Histogram{
			SampleCount: &metric.Count,
			SampleSum:   &metric.Sum,
			Bucket:      promBuckets,
		}
	case metrics.SummaryType:
		promQuantiles := make([]*dto.Quantile, len(metric.Quantiles))
		for i, quantile := range metric.Quantiles {
			promQuantiles[i] = &dto.Quantile{
				Quantile: &quantile.Quantile,
				Value:    &quantile.Value,
			}
		}
		promMetric.Summary = &dto.Summary{
			SampleCount: &metric.Count,
			SampleSum:   &metric.Sum,
			Quantile:    promQuantiles,
		}
	}
	
	return promMetric
}
