package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusRecorder struct {
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*prometheus.HistogramVec
	gauges     map[string]*prometheus.GaugeVec
}

func NewPrometheusRecorder(namespace string) *PrometheusRecorder {
	return &PrometheusRecorder{
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*prometheus.HistogramVec),
		gauges:     make(map[string]*prometheus.GaugeVec),
	}
}

func (r *PrometheusRecorder) IncCounter(name string, labels ...Label) {
	vec := r.getCounter(name)
	if vec == nil {
		labelNames := extractLabelNames(labels)
		vec = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: name,
			Help: name,
		}, labelNames)
		r.counters[name] = vec
	}

	vec.With(labelsToProm(labels)).Inc()
}

func (r *PrometheusRecorder) ObserveHistogram(name string, value float64, labels ...Label) {
	vec := r.getHistogram(name)
	if vec == nil {
		labelNames := extractLabelNames(labels)
		vec = promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    name,
			Help:    name,
			Buckets: prometheus.DefBuckets,
		}, labelNames)
		r.histograms[name] = vec
	}

	vec.With(labelsToProm(labels)).Observe(value)
}

func (r *PrometheusRecorder) SetGauge(name string, value float64, labels ...Label) {
	vec := r.getGauge(name)
	if vec == nil {
		labelNames := extractLabelNames(labels)
		vec = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: name,
			Help: name,
		}, labelNames)
		r.gauges[name] = vec
	}

	vec.With(labelsToProm(labels)).Set(value)
}

func (r *PrometheusRecorder) getCounter(name string) *prometheus.CounterVec {
	return r.counters[name]
}

func (r *PrometheusRecorder) getHistogram(name string) *prometheus.HistogramVec {
	return r.histograms[name]
}

func (r *PrometheusRecorder) getGauge(name string) *prometheus.GaugeVec {
	return r.gauges[name]
}

func extractLabelNames(labels []Label) []string {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return names
}

func labelsToProm(labels []Label) prometheus.Labels {
	m := make(prometheus.Labels, len(labels))
	for _, l := range labels {
		m[l.Name] = l.Value
	}
	return m
}
