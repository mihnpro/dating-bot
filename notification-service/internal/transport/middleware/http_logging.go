package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Logging wraps a handler with structured JSON request logging.
func Logging(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			logrus.WithFields(logrus.Fields{
				"service":    serviceName,
				"method":     r.Method,
				"path":       r.URL.Path,
				"status":     rw.statusCode,
				"latency_ms": time.Since(start).Milliseconds(),
				"remote":     r.RemoteAddr,
			}).Info("http request")
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
