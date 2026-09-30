package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestSenderUsesStableDeliveryID(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Relaybox-Delivery-Id") != "delivery-a" {
			t.Error("missing delivery id")
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	sender := NewSender(&http.Client{Timeout: time.Second})
	status, err := sender.Send(context.Background(), domain.Delivery{ID: "delivery-a", TargetURL: server.URL, Payload: []byte(`{"ok":true}`)})
	if err != nil || status != http.StatusNoContent {
		t.Fatalf("status=%d err=%v", status, err)
	}
}
