// Package handler implements HTTP transport handlers for the Relaybox
// multi-tenant webhook relay service.
//
// This file contains shared helper functions used across handler files:
// response writing, error mapping, and Prometheus metrics recording.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/metrics"
)

// ============================================================================
// Prometheus Metrics Middleware (FR-OBS-02, RULE-OBS-03)
// ============================================================================

// responseRecorder wraps http.ResponseWriter to capture the status code.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware records HTTP request duration and outcome as Prometheus
// metrics with bounded-cardinality labels (RULE-OBS-03). It wraps the
// provided http.HandlerFunc and records:
//   - relaybox_http_request_duration_seconds histogram with labels
//     {path, method, status_code}
//
// Usage:
//
//	mux.HandleFunc("/v1/events", handler.MetricsMiddleware("POST /v1/events",
//	    ingestionHandler.HandleIngestEvent))
func MetricsMiddleware(operationName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next(rec, r)

		duration := time.Since(start)
		statusCode := rec.statusCode

		// Record the HTTP request duration with bounded-cardinality labels
		// (RULE-OBS-03). We use the operation name (e.g., "POST /v1/events")
		// as the path label, which is bounded by the number of endpoints.
		metrics.RecordHTTPDuration(operationName, r.Method, statusCode, duration)
	}
}

// ============================================================================
// Shared Response Helpers
// ============================================================================

// writeJSON serializes the given value as JSON and writes it to the response
// with the specified HTTP status code (RULE-ARCH-03).
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response",
			"error", err.Error(),
			"component", "handler",
			"status_code", status,
		)
	}
}

// writeError writes a JSON error response with the given status code and
// domain error details (RULE-ARCH-03). It extracts the machine-readable
// error code and human-readable message from the domain error.
func writeError(w http.ResponseWriter, status int, err error) {
	var domainErr *domain.DomainError
	if errors.As(err, &domainErr) {
		writeJSON(w, status, map[string]string{
			"code":    domainErr.Code,
			"message": domainErr.Message,
		})
		return
	}
	writeJSON(w, status, map[string]string{
		"code":    "RELAYBOX_INTERNAL_ERROR",
		"message": "an unexpected internal server error occurred",
	})
}

// writeDomainError maps domain errors to HTTP status codes and writes the
// appropriate error response (RULE-ARCH-03).
func writeDomainError(w http.ResponseWriter, err error) {
	// Default to 500 Internal Server Error.
	status := http.StatusInternalServerError

	switch {
	case errors.Is(err, domain.ErrInvalidEventPayload):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrDuplicateEvent):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusForbidden
	case errors.Is(err, domain.ErrInternal):
		status = http.StatusInternalServerError
	}

	writeError(w, status, err)
}
