package http

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(h *Handler) http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/health", h.Health).Methods(http.MethodGet)
	r.Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
	r.HandleFunc("/ws", h.WebSocket)

	api := r.PathPrefix("/api/v1/chat").Subrouter()
	api.HandleFunc("/token", h.GenerateToken).Methods(http.MethodGet)
	api.HandleFunc("/conversations", h.ListUserConversations).Methods(http.MethodGet)
	api.HandleFunc("/conversations", h.CreateOrGetConversation).Methods(http.MethodPost)
	api.HandleFunc("/conversations/{id}", h.GetConversation).Methods(http.MethodGet)
	api.HandleFunc("/conversations/{id}/messages", h.GetMessages).Methods(http.MethodGet)

	// CORS for browser clients
	r.Use(corsMiddleware)

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
