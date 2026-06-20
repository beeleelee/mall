package metrics

import (
	"testing"
)

func TestPrometheusRecorder_IncCounter(t *testing.T) {
	r := NewPrometheusRecorder("test")
	r.IncCounter("test_requests_total", NewLabel("path", "/test"), NewLabel("status", "200"))
	r.IncCounter("test_requests_total", NewLabel("path", "/test"), NewLabel("status", "200"))
	r.IncCounter("test_requests_total", NewLabel("path", "/test"), NewLabel("status", "404"))
}

func TestPrometheusRecorder_ObserveHistogram(t *testing.T) {
	r := NewPrometheusRecorder("test")
	r.ObserveHistogram("test_duration_seconds", 0.5, NewLabel("path", "/test"))
	r.ObserveHistogram("test_duration_seconds", 1.2, NewLabel("path", "/test"))
}

func TestPrometheusRecorder_SetGauge(t *testing.T) {
	r := NewPrometheusRecorder("test")
	r.SetGauge("test_active_sessions", 5, NewLabel("capability", "checkout"))
	r.SetGauge("test_active_sessions", 3, NewLabel("capability", "checkout"))
}
