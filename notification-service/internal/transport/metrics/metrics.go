package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "path", "status"},
	)

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	notificationsDeliveredTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_delivered_total",
			Help: "Total notifications delivered via Telegram by type.",
		},
		[]string{"type"},
	)

	notificationsFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_failed_total",
			Help: "Total notification delivery failures by type.",
		},
		[]string{"type"},
	)

	notificationsCreatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_created_total",
			Help: "Total notifications saved to the database by type.",
		},
		[]string{"type"},
	)

	eventsConsumedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_events_consumed_total",
			Help: "Total RabbitMQ events consumed by event type.",
		},
		[]string{"event_type"},
	)

	eventsErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notification_events_errors_total",
			Help: "Total RabbitMQ events that failed processing by event type.",
		},
		[]string{"event_type"},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestDuration,
		httpRequestsTotal,
		notificationsDeliveredTotal,
		notificationsFailedTotal,
		notificationsCreatedTotal,
		eventsConsumedTotal,
		eventsErrorsTotal,
	)
}

// Handler returns the Prometheus HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// IncDelivered increments the delivered counter for the given notification type.
func IncDelivered(notifType string) {
	notificationsDeliveredTotal.WithLabelValues(notifType).Inc()
}

// IncFailed increments the failure counter for the given notification type.
func IncFailed(notifType string) {
	notificationsFailedTotal.WithLabelValues(notifType).Inc()
}

// IncCreated increments the created-in-DB counter for the given notification type.
func IncCreated(notifType string) {
	notificationsCreatedTotal.WithLabelValues(notifType).Inc()
}

// IncEventsConsumed increments the RabbitMQ consumed counter for the given event type.
func IncEventsConsumed(eventType string) {
	eventsConsumedTotal.WithLabelValues(eventType).Inc()
}

// IncEventsError increments the RabbitMQ processing error counter for the given event type.
func IncEventsError(eventType string) {
	eventsErrorsTotal.WithLabelValues(eventType).Inc()
}

// InstrumentedResponseWriter wraps http.ResponseWriter to capture the status code.
type InstrumentedResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

func NewInstrumentedResponseWriter(w http.ResponseWriter) *InstrumentedResponseWriter {
	return &InstrumentedResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
}

func (w *InstrumentedResponseWriter) WriteHeader(code int) {
	w.StatusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Instrument wraps a handler and records Prometheus metrics.
func Instrument(path string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		iw := NewInstrumentedResponseWriter(w)
		next.ServeHTTP(iw, r)

		code := strconv.Itoa(iw.StatusCode)
		dur := time.Since(start).Seconds()
		httpRequestDuration.WithLabelValues(r.Method, path, code).Observe(dur)
		httpRequestsTotal.WithLabelValues(r.Method, path, code).Inc()
	})
}
