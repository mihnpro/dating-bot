package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPMiddleware returns an http.Handler middleware that logs every request
// with method, path, status code, and latency using structured JSON logging.
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
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
				"user_agent": r.UserAgent(),
			}).Info("http request")
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code written
// by the handler, since the standard interface does not expose it after the
// fact.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
