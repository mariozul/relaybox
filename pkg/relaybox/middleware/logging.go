// Package middleware implements HTTP middleware for structured logging
// (RULE-OBS-01) and Prometheus metrics (RULE-OBS-03) for the Relaybox service.
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code before delegating to the wrapped writer.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

// StructuredLogger returns an HTTP middleware that logs each request as a
// structured JSON log entry (RULE-OBS-01).
//
// Logged fields:
//   - method:      HTTP method
//   - path:        Request URL path
//   - query:       Raw query string (if present)
//   - status:      HTTP response status code
//   - latency:     Request processing duration
//   - ip:          Client IP address
//   - user_agent:  User-Agent header
//   - body_size:   Response body size in bytes
//
// When OpenTelemetry trace context is available in the request context,
// trace_id and span_id are automatically included by slog's context handler.
func StructuredLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the response writer to capture the status code.
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			slog.InfoContext(r.Context(), "HTTP Request",
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"status", rw.statusCode,
				"latency", duration.String(),
				"ip", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"body_size", rw.Size(),
			)
		})
	}
}

// Size returns the number of bytes written to the response.
// It delegates to the wrapped ResponseWriter's Size() if available, otherwise 0.
func (rw *responseWriter) Size() int {
	if s, ok := rw.ResponseWriter.(interface{ Size() int }); ok {
		return s.Size()
	}
	return 0
}
