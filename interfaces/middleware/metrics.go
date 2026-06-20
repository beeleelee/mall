package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/beeleelee/mall/infrastructure/metrics"
)

type MetricsMiddleware struct {
	recorder metrics.MetricsRecorder
}

func NewMetricsMiddleware(recorder metrics.MetricsRecorder) *MetricsMiddleware {
	return &MetricsMiddleware{recorder: recorder}
}

func (m *MetricsMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		duration := time.Since(start).Seconds()
		path := r.URL.Path
		method := r.Method

		m.recorder.IncCounter("ucp_capability_requests_total",
			metrics.NewLabel("path", path),
			metrics.NewLabel("method", method),
			metrics.NewLabel("status", strconv.Itoa(sw.status)),
		)
		m.recorder.ObserveHistogram("ucp_capability_duration_seconds", duration,
			metrics.NewLabel("path", path),
			metrics.NewLabel("method", method),
		)
		if sw.status >= 400 {
			m.recorder.IncCounter("ucp_capability_error_total",
				metrics.NewLabel("path", path),
				metrics.NewLabel("method", method),
				metrics.NewLabel("error_code", strconv.Itoa(sw.status)),
			)
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
