package api

import (
	"context"
	"net/http"
)

// contextKey is an unexported type to prevent collisions in context (RULE-ARCH-03).
type contextKey string

const tenantIDKey contextKey = "relaybox-tenant-id"

// TenantMiddleware extracts the tenant identity from the X-Tenant-ID header
// and injects it into the request context. This simulates gateway-verified
// tenant identity (RULE-SEC-01).
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			http.Error(w, `{"error":"missing X-Tenant-ID header"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), tenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetTenantID extracts the tenant ID from the request context.
// Returns empty string if not found.
func GetTenantID(ctx context.Context) string {
	v, _ := ctx.Value(tenantIDKey).(string)
	return v
}
