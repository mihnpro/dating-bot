package http

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/dating-bot/notification-service/internal/transport/metrics"
	"github.com/dating-bot/notification-service/internal/transport/middleware"
)

// NewRouter builds and returns the HTTP mux for the notification service.
// Uses the Go 1.22+ ServeMux pattern syntax.
func NewRouter(h *Handler, serviceName string) http.Handler {
	mux := http.NewServeMux()

	// Observability
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Notification REST API
	mux.Handle("GET /api/v1/notifications/{user_id}",
		metrics.Instrument("/api/v1/notifications/{user_id}", http.HandlerFunc(h.GetNotifications)))
	mux.Handle("POST /api/v1/notifications/{id}/read",
		metrics.Instrument("/api/v1/notifications/{id}/read", http.HandlerFunc(h.MarkRead)))

	return middleware.Logging(serviceName)(mux)
}
