package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTenantAuth(t *testing.T) {
	t.Parallel()
	h := TenantAuth("secret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, tc := range []struct {
		token, tenant string
		want          int
	}{{"", "tenant-a", 401}, {"secret", "", 401}, {"secret", "tenant-a", 204}} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		r.Header.Set("X-Relaybox-Tenant-Id", tc.tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("got %d, want %d", w.Code, tc.want)
		}
	}
}
