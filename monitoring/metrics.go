package monitoring

import (
	"distore/replication"
	"distore/storage"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	requestDuration *prometheus.HistogramVec
	requestCount    *prometheus.CounterVec
	errorCount      *prometheus.CounterVec
	storageSize     prometheus.Gauge
	replicationLag  prometheus.Gauge
	nodesOnline     prometheus.Gauge
}

type ResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

func NewMetrics() *Metrics {
	return &Metrics{
		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
		}, []string{"method", "path", "status"}),

		requestCount: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "path", "status"}),

		errorCount: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP errors",
		}, []string{"method", "path", "error_type"}),

		storageSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "storage_size_bytes",
			Help: "Total size of stored data in bytes",
		}),

		replicationLag: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "replication_lag_seconds",
			Help: "Replication lag in seconds",
		}),

		nodesOnline: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "nodes_online_total",
			Help: "Number of online replica nodes",
		}),
	}
}

func (m *Metrics) ObserveRequest(method, path string, status int, duration time.Duration) {
	m.requestDuration.WithLabelValues(method, path, fmt.Sprintf("%d", status)).Observe(duration.Seconds())
	m.requestCount.WithLabelValues(method, path, fmt.Sprintf("%d", status)).Inc()
}

func (m *Metrics) ObserveError(method, path, errorType string) {
	m.errorCount.WithLabelValues(method, path, errorType).Inc()
}

func (m *Metrics) UpdateStorageMetrics(storage storage.Storage) {
	go func() {
		items, err := storage.GetAll()
		if err == nil {
			// Simple size metric
			// TODO: improve the metric
			m.storageSize.Set(float64(len(items)))
		}
	}()
}

func (m *Metrics) UpdateReplicationMetrics(replicator *replication.Replicator) {
	go func() {
		// Logic for measuring lag can be added here
		nodes := replicator.GetNodes()
		m.nodesOnline.Set(float64(len(nodes)))
	}()
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

func (rw *ResponseWriter) WriteHeader(code int) {
	rw.StatusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
