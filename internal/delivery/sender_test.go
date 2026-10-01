package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

func TestHTTPSender_Delivered(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Relaybox-Delivery-Id") != "del-1" {
			t.Errorf("delivery-id header mismatch")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := DefaultSenderConfig()
	cfg.HTTPClient = server.Client()
	sender := NewHTTPSender(cfg)
	msg := domain.NewOutboxMessage("evt-1", "sub-1", "del-1", "system", time.Now())
	result := sender.Send(context.Background(), msg, []byte(`{"x":1}`), server.URL)
	if result.Status != "delivered" {
		t.Fatalf("expected delivered, got %s: %v", result.Status, result.Error)
	}
}

func TestHTTPSender_RetryOn500(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	cfg := DefaultSenderConfig()
	cfg.HTTPClient = server.Client()
	sender := NewHTTPSender(cfg)
	msg := domain.NewOutboxMessage("evt-1", "sub-1", "del-1", "system", time.Now())
	result := sender.Send(context.Background(), msg, []byte(`{"x":1}`), server.URL)
	if result.Status != "retry" {
		t.Fatalf("expected retry on 500, got %s", result.Status)
	}
}

func TestHTTPSender_DeadLetterOn400(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	cfg := DefaultSenderConfig()
	cfg.HTTPClient = server.Client()
	sender := NewHTTPSender(cfg)
	msg := domain.NewOutboxMessage("evt-1", "sub-1", "del-1", "system", time.Now())
	result := sender.Send(context.Background(), msg, []byte(`{"x":1}`), server.URL)
	if result.Status != "deadletter" {
		t.Fatalf("expected deadletter on 400, got %s", result.Status)
	}
}

func TestHTTPSender_MaxRetriesDeadLetter(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	cfg := DefaultSenderConfig()
	cfg.HTTPClient = server.Client()
	sender := NewHTTPSender(cfg)
	msg := domain.NewOutboxMessage("evt-1", "sub-1", "del-1", "system", time.Now())
	msg.Attempts = 5
	result := sender.Send(context.Background(), msg, []byte(`{"x":1}`), server.URL)
	if result.Status != "deadletter" {
		t.Fatalf("expected deadletter at max attempts, got %s", result.Status)
	}
}
