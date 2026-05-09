package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// gRPC infrastructure metrics — collected via UnaryServerInterceptor.
var (
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_server_request_duration_seconds",
			Help:    "Duration of gRPC requests by method and status.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "code"},
	)

	requestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_server_requests_total",
			Help: "Total number of gRPC requests by method and status.",
		},
		[]string{"method", "code"},
	)

	inFlightRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "grpc_server_in_flight_requests",
			Help: "Current number of in-flight gRPC requests.",
		},
	)
)

func init() {
	prometheus.MustRegister(requestDuration, requestTotal, inFlightRequests)
}

// Business metrics — domain events counted at the transport layer.
var (
	UsersRegisteredTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "dating_users_registered_total",
		Help: "Total number of users successfully registered.",
	})

	ProfilesCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "dating_profiles_created_total",
		Help: "Total number of user profiles created.",
	})

	ProfilesUpdatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "dating_profiles_updated_total",
		Help: "Total number of user profiles updated.",
	})

	ProfileFullness = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "dating_profile_fullness",
		Help:    "Distribution of profile fullness percent [0..1] at creation/update.",
		Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
	})
)

// UnaryServerInterceptor returns a gRPC UnaryServerInterceptor that collects Prometheus metrics.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		inFlightRequests.Inc()
		defer inFlightRequests.Dec()

		resp, err := handler(ctx, req)

		st := status.Convert(err)
		code := st.Code().String()
		method := info.FullMethod

		requestDuration.WithLabelValues(method, code).Observe(time.Since(start).Seconds())
		requestTotal.WithLabelValues(method, code).Inc()

		return resp, err
	}
}

// Registerer returns the prometheus.Registerer for use with HTTP handler.
func Registerer() prometheus.Registerer {
	return prometheus.DefaultRegisterer
}

// Gatherer returns the prometheus.Gatherer for use with HTTP handler.
func Gatherer() prometheus.Gatherer {
	return prometheus.DefaultGatherer
}
