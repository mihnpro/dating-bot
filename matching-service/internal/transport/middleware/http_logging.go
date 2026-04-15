package middleware

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// LoggingResponseWriter wraps http.ResponseWriter to capture status codes.
type LoggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *LoggingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware returns an HTTP middleware handler that adds structured logging.
func HTTPMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lw := &LoggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			fields := logrus.Fields{
				"service":  serviceName,
				"method":   r.Method,
				"path":     r.URL.Path,
				"remote":   r.RemoteAddr,
				"user_agent": r.UserAgent(),
			}

			logrus.WithFields(fields).Info("HTTP request started")

			next.ServeHTTP(lw, r)

			fields["status"] = lw.statusCode
			fields["latency_ms"] = time.Since(start).Milliseconds()

			if lw.statusCode >= 500 {
				logrus.WithFields(fields).Error("HTTP request failed")
			} else {
				logrus.WithFields(fields).Info("HTTP request completed")
			}
		})
	}
}
