package httpclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/infrastructure/httpclient"
)

func TestHTTPClient_Post(t *testing.T) {
	t.Parallel()

	t.Run("sends headers including X-Relaybox-Delivery-Id", func(t *testing.T) {
		t.Parallel()
		var deliveryID string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deliveryID = r.Header.Get("X-Relaybox-Delivery-Id")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		client := httpclient.NewDefaultHTTPClient(5 * time.Second)
		resp, err := client.Post(context.Background(), srv.URL,
			map[string]string{"X-Relaybox-Delivery-Id": "del-uuid"}, []byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		if deliveryID != "del-uuid" {
			t.Errorf("expected header del-uuid, got %q", deliveryID)
		}
	})

	t.Run("context cancellation returns error", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		client := httpclient.NewDefaultHTTPClient(5 * time.Second)
		_, err := client.Post(ctx, "http://localhost:1", nil, []byte(`{}`))
		if err == nil {
			t.Fatal("expected error from cancelled context")
		}
	})

	t.Run("handles all HTTP status codes", func(t *testing.T) {
		t.Parallel()
		for _, code := range []int{200, 201, 400, 500} {
			code := code
			t.Run(http.StatusText(code), func(t *testing.T) {
				t.Parallel()
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(code)
				}))
				defer srv.Close()

				client := httpclient.NewDefaultHTTPClient(5 * time.Second)
				resp, err := client.Post(context.Background(), srv.URL, nil, []byte(`{}`))
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != code {
					t.Errorf("expected %d, got %d", code, resp.StatusCode)
				}
			})
		}
	})

	t.Run("timeout returns error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
		}))
		defer srv.Close()

		client := httpclient.NewDefaultHTTPClient(50 * time.Millisecond)
		_, err := client.Post(context.Background(), srv.URL, nil, []byte(`{}`))
		if err == nil {
			t.Fatal("expected timeout error")
		}
	})
}
