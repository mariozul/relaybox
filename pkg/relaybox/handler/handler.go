// Package handler implements HTTP transport handlers.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Context key type for tenant ID to avoid collisions.
type contextKey string

const tenantCtxKey contextKey = "tenant_id"

// WithTenant returns a new context with the tenant ID set.
// Used by auth middleware to inject the authenticated tenant identity
// into the request context (ASSUMP-01, RULE-SEC-01).
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantCtxKey, tenantID)
}

// tenantFromContext extracts the tenant ID from the request context.
// Returns empty string if not set.
func tenantFromContext(ctx context.Context) string {
	if tid, ok := ctx.Value(tenantCtxKey).(string); ok {
		return tid
	}
	return ""
}

// writeJSON marshals the payload and writes it as JSON with the given status code.
// Sets Content-Type header to application/json.
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// errorResponse is the standard API error envelope.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError writes a standard JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}
