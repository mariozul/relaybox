package http

import (
	"context"
	"net/http"
)

// TenantMiddleware extracts tenant identity from the request.
// In production, this would extract from JWT/gateway headers (RULE-SEC-01).
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), TenantCtxKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
