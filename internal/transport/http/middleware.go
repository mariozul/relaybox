package http

import (
	"context"
	"net/http"
)

// Tenant header that trusted gateway injects (RULE-SEC-01).
const tenantHeader = "X-Tenant-ID"

type contextKey string

const tenantKey contextKey = "tenant_id"

// TenantID extracts the tenant identifier from the request context.
func TenantID(ctx context.Context) (string, bool) {
	tid, ok := ctx.Value(tenantKey).(string)
	return tid, ok
}

// TenantMiddleware extracts the tenant from X-Tenant-ID header (set by auth gateway).
// It never trusts tenant_id from the request body (RULE-SEC-01).
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := r.Header.Get(tenantHeader)
		if tid == "" {
			http.Error(w, `{"error":"tenant identity required"}`, http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), tenantKey, tid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
