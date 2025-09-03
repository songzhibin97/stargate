package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/songzhibin97/stargate/pkg/metrics"
)

// prometheusCounter adapts prometheus.Counter to metrics.Counter
type prometheusCounter struct {
	counter prometheus.Counter
}

func (c *prometheusCounter) Inc() {
	c.counter.Inc()
}

func (c *prometheusCounter) Add(delta float64) {
	c.counter.Add(delta)
}

func (c *prometheusCounter) Get() float64 {
	metric := &dto.Metric{}
	if err := c.counter.Write(metric); err != nil {
		return 0
	}
	return metric.GetCounter().GetValue()
}

// prometheusCounterVec adapts prometheus.CounterVec to metrics.CounterVec
type prometheusCounterVec struct {
	counterVec *prometheus.CounterVec
}

func (cv *prometheusCounterVec) WithLabelValues(lvs ...string) metrics.Counter {
	return &prometheusCounter{counter: cv.counterVec.WithLabelValues(lvs...)}
}

func (cv *prometheusCounterVec) With(labels map[string]string) metrics.Counter {
	promLabels := prometheus.Labels(labels)
	return &prometheusCounter{counter: cv.counterVec.With(promLabels)}
}

func (cv *prometheusCounterVec) GetMetricWithLabelValues(lvs ...string) (metrics.Counter, error) {
	counter, err := cv.counterVec.GetMetricWithLabelValues(lvs...)
	if err != nil {
		return nil, err
	}
	return &prometheusCounter{counter: counter}, nil
}

func (cv *prometheusCounterVec) GetMetricWith(labels map[string]string) (metrics.Counter, error) {
	promLabels := prometheus.Labels(labels)
	counter, err := cv.counterVec.GetMetricWith(promLabels)
	if err != nil {
		return nil, err
	}
	return &prometheusCounter{counter: counter}, nil
}

func (cv *prometheusCounterVec) Delete(labels map[string]string) bool {
	promLabels := prometheus.Labels(labels)
	return cv.counterVec.Delete(promLabels)
}

func (cv *prometheusCounterVec) DeleteLabelValues(lvs ...string) bool {
	return cv.counterVec.DeleteLabelValues(lvs...)
}

func (cv *prometheusCounterVec) Reset() {
	cv.counterVec.Reset()
}

// prometheusGauge adapts prometheus.Gauge to metrics.Gauge
type prometheusGauge struct {
	gauge prometheus.Gauge
}

func (g *prometheusGauge) Set(value float64) {
	g.gauge.Set(value)
}

func (g *prometheusGauge) Inc() {
	g.gauge.Inc()
}

func (g *prometheusGauge) Dec() {
	g.gauge.Dec()
}

func (g *prometheusGauge) Add(delta float64) {
	g.gauge.Add(delta)
}

func (g *prometheusGauge) Sub(delta float64) {
	g.gauge.Sub(delta)
}

func (g *prometheusGauge) Get() float64 {
	metric := &dto.Metric{}
	if err := g.gauge.Write(metric); err != nil {
		return 0
	}
	return metric.GetGauge().GetValue()
}

func (g *prometheusGauge) SetToCurrentTime() {
	g.gauge.SetToCurrentTime()
}

// prometheusGaugeVec adapts prometheus.GaugeVec to metrics.GaugeVec
type prometheusGaugeVec struct {
	gaugeVec *prometheus.GaugeVec
}

func (gv *prometheusGaugeVec) WithLabelValues(lvs ...string) metrics.Gauge {
	return &prometheusGauge{gauge: gv.gaugeVec.WithLabelValues(lvs...)}
}

func (gv *prometheusGaugeVec) With(labels map[string]string) metrics.Gauge {
	promLabels := prometheus.Labels(labels)
	return &prometheusGauge{gauge: gv.gaugeVec.With(promLabels)}
}

func (gv *prometheusGaugeVec) GetMetricWithLabelValues(lvs ...string) (metrics.Gauge, error) {
	gauge, err := gv.gaugeVec.GetMetricWithLabelValues(lvs...)
	if err != nil {
		return nil, err
	}
	return &prometheusGauge{gauge: gauge}, nil
}

func (gv *prometheusGaugeVec) GetMetricWith(labels map[string]string) (metrics.Gauge, error) {
	promLabels := prometheus.Labels(labels)
	gauge, err := gv.gaugeVec.GetMetricWith(promLabels)
	if err != nil {
		return nil, err
	}
	return &prometheusGauge{gauge: gauge}, nil
}

func (gv *prometheusGaugeVec) Delete(labels map[string]string) bool {
	promLabels := prometheus.Labels(labels)
	return gv.gaugeVec.Delete(promLabels)
}

func (gv *prometheusGaugeVec) DeleteLabelValues(lvs ...string) bool {
	return gv.gaugeVec.DeleteLabelValues(lvs...)
}

func (gv *prometheusGaugeVec) Reset() {
	gv.gaugeVec.Reset()
}

// prometheusHistogram adapts prometheus.Histogram to metrics.Histogram
type prometheusHistogram struct {
	histogram prometheus.Histogram
}

func (h *prometheusHistogram) Observe(value float64) {
	h.histogram.Observe(value)
}

func (h *prometheusHistogram) GetCount() uint64 {
	metric := &dto.Metric{}
	if err := h.histogram.Write(metric); err != nil {
		return 0
	}
	return metric.GetHistogram().GetSampleCount()
}

func (h *prometheusHistogram) GetSum() float64 {
	metric := &dto.Metric{}
	if err := h.histogram.Write(metric); err != nil {
		return 0
	}
	return metric.GetHistogram().GetSampleSum()
}

func (h *prometheusHistogram) GetBuckets() []metrics.Bucket {
	metric := &dto.Metric{}
	if err := h.histogram.Write(metric); err != nil {
		return nil
	}
	
	promBuckets := metric.GetHistogram().GetBucket()
	buckets := make([]metrics.Bucket, len(promBuckets))
	for i, promBucket := range promBuckets {
		buckets[i] = metrics.Bucket{
			UpperBound: promBucket.GetUpperBound(),
			Count:      promBucket.GetCumulativeCount(),
		}
	}
	return buckets
}

// prometheusHistogramVec adapts prometheus.HistogramVec to metrics.HistogramVec
type prometheusHistogramVec struct {
	histogramVec *prometheus.HistogramVec
}

func (hv *prometheusHistogramVec) WithLabelValues(lvs ...string) metrics.Histogram {
	observer := hv.histogramVec.WithLabelValues(lvs...)
	if histogram, ok := observer.(prometheus.Histogram); ok {
		return &prometheusHistogram{histogram: histogram}
	}
	// Fallback: create a wrapper that only supports Observe
	return &prometheusHistogramObserver{observer: observer}
}

func (hv *prometheusHistogramVec) With(labels map[string]string) metrics.Histogram {
	promLabels := prometheus.Labels(labels)
	observer := hv.histogramVec.With(promLabels)
	if histogram, ok := observer.(prometheus.Histogram); ok {
		return &prometheusHistogram{histogram: histogram}
	}
	// Fallback: create a wrapper that only supports Observe
	return &prometheusHistogramObserver{observer: observer}
}

func (hv *prometheusHistogramVec) GetMetricWithLabelValues(lvs ...string) (metrics.Histogram, error) {
	observer, err := hv.histogramVec.GetMetricWithLabelValues(lvs...)
	if err != nil {
		return nil, err
	}
	if histogram, ok := observer.(prometheus.Histogram); ok {
		return &prometheusHistogram{histogram: histogram}, nil
	}
	// Fallback: create a wrapper that only supports Observe
	return &prometheusHistogramObserver{observer: observer}, nil
}

func (hv *prometheusHistogramVec) GetMetricWith(labels map[string]string) (metrics.Histogram, error) {
	promLabels := prometheus.Labels(labels)
	observer, err := hv.histogramVec.GetMetricWith(promLabels)
	if err != nil {
		return nil, err
	}
	if histogram, ok := observer.(prometheus.Histogram); ok {
		return &prometheusHistogram{histogram: histogram}, nil
	}
	// Fallback: create a wrapper that only supports Observe
	return &prometheusHistogramObserver{observer: observer}, nil
}

func (hv *prometheusHistogramVec) Delete(labels map[string]string) bool {
	promLabels := prometheus.Labels(labels)
	return hv.histogramVec.Delete(promLabels)
}

func (hv *prometheusHistogramVec) DeleteLabelValues(lvs ...string) bool {
	return hv.histogramVec.DeleteLabelValues(lvs...)
}

func (hv *prometheusHistogramVec) Reset() {
	hv.histogramVec.Reset()
}

// prometheusSummary adapts prometheus.Summary to metrics.Summary
type prometheusSummary struct {
	summary prometheus.Summary
}

func (s *prometheusSummary) Observe(value float64) {
	s.summary.Observe(value)
}

func (s *prometheusSummary) GetCount() uint64 {
	metric := &dto.Metric{}
	if err := s.summary.Write(metric); err != nil {
		return 0
	}
	return metric.GetSummary().GetSampleCount()
}

func (s *prometheusSummary) GetSum() float64 {
	metric := &dto.Metric{}
	if err := s.summary.Write(metric); err != nil {
		return 0
	}
	return metric.GetSummary().GetSampleSum()
}

func (s *prometheusSummary) GetQuantiles() []metrics.Quantile {
	metric := &dto.Metric{}
	if err := s.summary.Write(metric); err != nil {
		return nil
	}
	
	promQuantiles := metric.GetSummary().GetQuantile()
	quantiles := make([]metrics.Quantile, len(promQuantiles))
	for i, promQuantile := range promQuantiles {
		quantiles[i] = metrics.Quantile{
			Quantile: promQuantile.GetQuantile(),
			Value:    promQuantile.GetValue(),
		}
	}
	return quantiles
}

// prometheusSummaryVec adapts prometheus.SummaryVec to metrics.SummaryVec
type prometheusSummaryVec struct {
	summaryVec *prometheus.SummaryVec
}

func (sv *prometheusSummaryVec) WithLabelValues(lvs ...string) metrics.Summary {
	observer := sv.summaryVec.WithLabelValues(lvs...)
	if summary, ok := observer.(prometheus.Summary); ok {
		return &prometheusSummary{summary: summary}
	}
	// Fallback: create a wrapper that only supports Observe
	return &prometheusSummaryObserver{observer: observer}
}

func (sv *prometheusSummaryVec) With(labels map[string]string) metrics.Summary {
	promLabels := prometheus.Labels(labels)
	observer := sv.summaryVec.With(promLabels)
	if summary, ok := observer.(prometheus.Summary); ok {
		return &prometheusSummary{summary: summary}
	}
	// Fallback: create a wrapper that only supports Observe
	return &prometheusSummaryObserver{observer: observer}
}

func (sv *prometheusSummaryVec) GetMetricWithLabelValues(lvs ...string) (metrics.Summary, error) {
	observer, err := sv.summaryVec.GetMetricWithLabelValues(lvs...)
	if err != nil {
		return nil, err
	}
	if summary, ok := observer.(prometheus.Summary); ok {
		return &prometheusSummary{summary: summary}, nil
	}
	// Fallback: create a wrapper that only supports Observe
	return &prometheusSummaryObserver{observer: observer}, nil
}

func (sv *prometheusSummaryVec) GetMetricWith(labels map[string]string) (metrics.Summary, error) {
	promLabels := prometheus.Labels(labels)
	observer, err := sv.summaryVec.GetMetricWith(promLabels)
	if err != nil {
		return nil, err
	}
	if summary, ok := observer.(prometheus.Summary); ok {
		return &prometheusSummary{summary: summary}, nil
	}
	// Fallback: create a wrapper that only supports Observe
	return &prometheusSummaryObserver{observer: observer}, nil
}

func (sv *prometheusSummaryVec) Delete(labels map[string]string) bool {
	promLabels := prometheus.Labels(labels)
	return sv.summaryVec.Delete(promLabels)
}

func (sv *prometheusSummaryVec) DeleteLabelValues(lvs ...string) bool {
	return sv.summaryVec.DeleteLabelValues(lvs...)
}

func (sv *prometheusSummaryVec) Reset() {
	sv.summaryVec.Reset()
}

// prometheusHistogramObserver is a fallback adapter for histogram observers
type prometheusHistogramObserver struct {
	observer prometheus.Observer
}

func (h *prometheusHistogramObserver) Observe(value float64) {
	h.observer.Observe(value)
}

func (h *prometheusHistogramObserver) GetCount() uint64 {
	// Cannot get count from observer interface
	return 0
}

func (h *prometheusHistogramObserver) GetSum() float64 {
	// Cannot get sum from observer interface
	return 0
}

func (h *prometheusHistogramObserver) GetBuckets() []metrics.Bucket {
	// Cannot get buckets from observer interface
	return nil
}

// prometheusSummaryObserver is a fallback adapter for summary observers
type prometheusSummaryObserver struct {
	observer prometheus.Observer
}

func (s *prometheusSummaryObserver) Observe(value float64) {
	s.observer.Observe(value)
}

func (s *prometheusSummaryObserver) GetCount() uint64 {
	// Cannot get count from observer interface
	return 0
}

func (s *prometheusSummaryObserver) GetSum() float64 {
	// Cannot get sum from observer interface
	return 0
}

func (s *prometheusSummaryObserver) GetQuantiles() []metrics.Quantile {
	// Cannot get quantiles from observer interface
	return nil
}
