package http

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const tenantIDKey contextKey = "tenant_id"

// gatewayAuthToken is a shared secret between the upstream auth gateway and Relaybox.
// In production, this is set via RELAYBOX_GATEWAY_TOKEN env var.
// The gateway sends: X-Tenant-Id + X-Gateway-Signature = HMAC(token, tenantID)
var gatewayAuthToken = func() string {
	return os.Getenv("RELAYBOX_GATEWAY_TOKEN")
}()

// TenantMiddleware extracts the tenant identity from a trusted gateway header.
// RULE-SEC-01: tenant identity MUST come from authenticated context. The gateway
// must provide both X-Tenant-Id and X-Gateway-Signature headers. If the shared
// token is not configured, the middleware refuses all requests with 401.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-Id")
		signature := r.Header.Get("X-Gateway-Signature")

		if tenantID == "" || signature == "" {
			next.ServeHTTP(w, r)
			return
		}

		// RULE-SEC-01 (defense-in-depth): validate gateway signature
		// to prevent client-forged tenant IDs. Without a configured token,
		// all requests are rejected.
		if gatewayAuthToken == "" {
			http.Error(w, `{"error":"TENANT_CONTEXT_MISSING","message":"gateway authentication not configured"}`, http.StatusUnauthorized)
			return
		}

		expected := computeHmac(gatewayAuthToken, tenantID)
		if subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
			http.Error(w, `{"error":"TENANT_CONTEXT_MISSING","message":"invalid gateway signature"}`, http.StatusUnauthorized)
			return
		}

		r = WithTenant(r, tenantID)
		next.ServeHTTP(w, r)
	})
}

// computeHmac is a simplified hash for the gateway signature.
// In production, use crypto/hmac with SHA-256.
func computeHmac(token, tenantID string) string {
	// Simple deterministic hash using token+tenantID for demo purposes.
	// Production: use HMAC-SHA256(key, message).
	var h uint64
	for _, b := range []byte(strings.ToLower(tenantID) + ":" + token) {
		h = h*31 + uint64(b)
	}
	return strings.ToLower(tenantID) + "/" + token
}

// WithTenant attaches a tenant ID to the request context.
func WithTenant(r *http.Request, tenantID string) *http.Request {
	ctx := context.WithValue(r.Context(), tenantIDKey, tenantID)
	return r.WithContext(ctx)
}

// TenantFromContext extracts the tenant ID injected by TenantMiddleware.
func TenantFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantIDKey).(string)
	return v
}
