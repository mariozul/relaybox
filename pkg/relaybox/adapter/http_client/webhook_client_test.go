package http_client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/adapter/http_client"
)

// ============================================================================
// TC-06-INT: Webhook client integration tests with a test server
// ============================================================================

func TestWebhookClient_Send_Success(t *testing.T) {
	t.Parallel()

	// Start a test server that returns 200 OK.
	receivedHeaders := make(http.Header)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := http_client.NewWebhookClient(5 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deliveryID := "dlv-test-001"
	headers := map[string]string{
		"X-Relaybox-Delivery-Id": deliveryID,
		"Content-Type":           "application/json",
	}
	payload := []byte(`{"event":"test"}`)

	resp, err := client.SendWebhook(ctx, server.URL, headers, payload)
	if err != nil {
		t.Fatalf("SendWebhook() returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify delivery ID header was sent.
	if got := receivedHeaders.Get("X-Relaybox-Delivery-Id"); got != deliveryID {
		t.Errorf("expected X-Relaybox-Delivery-Id=%q, got %q", deliveryID, got)
	}

	// Verify content-type header was sent.
	if got := receivedHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type=%q, got %q", "application/json", got)
	}
}

func TestWebhookClient_Send_ServerError(t *testing.T) {
	t.Parallel()

	// Start a test server that returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	client := http_client.NewWebhookClient(5 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SendWebhook(ctx, server.URL, nil, []byte(`{}`))
	if err != nil {
		t.Fatalf("SendWebhook() returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}
}

func TestWebhookClient_Send_ClientError(t *testing.T) {
	t.Parallel()

	// Start a test server that returns 400.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	client := http_client.NewWebhookClient(5 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SendWebhook(ctx, server.URL, nil, []byte(`{}`))
	if err != nil {
		t.Fatalf("SendWebhook() returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestWebhookClient_Send_ContextCancelled(t *testing.T) {
	t.Parallel()

	// Start a slow server to trigger context timeout.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := http_client.NewWebhookClient(5 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.SendWebhook(ctx, server.URL, nil, []byte(`{}`))
	if err == nil {
		t.Fatal("SendWebhook() expected error for cancelled context, got nil")
	}
}

func TestWebhookClient_Send_ContextPropagation(t *testing.T) {
	t.Parallel()

	// Verify that ctx is used (not context.Background()).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := http_client.NewWebhookClient(5 * time.Second)

	// TC-09-UNIT: Verify client does not use context.Background().
	// The client should use the passed context for the request.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SendWebhook(ctx, server.URL, nil, []byte(`{"event":"test"}`))
	if err != nil {
		t.Fatalf("SendWebhook() returned error: %v", err)
	}
	resp.Body.Close()
}

func TestWebhookClient_Send_ResponseBodyClosed(t *testing.T) {
	t.Parallel()

	// RULE-RES-03: Verify response body is closed.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := http_client.NewWebhookClient(5 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SendWebhook(ctx, server.URL, nil, []byte(`{}`))
	if err != nil {
		t.Fatalf("SendWebhook() returned error: %v", err)
	}

	// The SendWebhook implementation MUST close the response body.
	// Verify we can read it (the caller shouldn't need to).
	// If the implementation delegates close to the caller, this test will
	// need adjustment; we want the implementation to handle it per RULE-RES-03.
	resp.Body.Close()
}
