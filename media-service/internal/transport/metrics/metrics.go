package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	UploadsTotal    prometheus.Counter
	UploadErrors    prometheus.Counter
}

func New(serviceName string) *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total number of HTTP requests",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}, []string{"method", "path", "status"}),

		RequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request duration in seconds",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"service": serviceName},
		}, []string{"method", "path"}),

		UploadsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "media_uploads_total",
			Help:        "Total number of successful photo uploads",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}),

		UploadErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "media_upload_errors_total",
			Help:        "Total number of failed photo uploads",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}),
	}
}
