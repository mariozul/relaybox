// Package http_client provides concrete HTTP implementations of the
// service.WebhookClient interface for delivering webhook payloads to
// subscriber endpoints (FR-DEL-01).
package http_client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

// httpWebhookClient implements the WebhookClient interface (defined in the
// service package) using the standard net/http package. It propagates context
// and enforces client-side timeouts (FR-DEL-01, RULE-RES-01).
type httpWebhookClient struct {
	httpClient *http.Client
}

// NewWebhookClient creates a new HTTP-backed WebhookClient with the specified
// request timeout. The timeout is applied to each outbound HTTP request
// (FR-DEL-01, RULE-RES-01).
func NewWebhookClient(timeout time.Duration) *httpWebhookClient {
	return &httpWebhookClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// compile-time interface compliance check.
var _ interface {
	SendWebhook(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error)
} = (*httpWebhookClient)(nil)

// SendWebhook sends an HTTP POST request to the specified URL with the given
// headers and payload. It propagates ctx context.Context and returns the
// *http.Response for status code inspection (FR-DEL-01, FR-DEL-02, RULE-RES-01).
//
// The caller is responsible for closing resp.Body after processing the
// response (RULE-RES-03).
func (c *httpWebhookClient) SendWebhook(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	// Build the request body; use nil body if payload is empty.
	var body io.Reader
	if len(payload) > 0 {
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	// Set headers.
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Set default Content-Type if not provided.
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute the request.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Note: The response body is NOT closed here; the caller is responsible
	// for closing it (RULE-RES-03). This allows the caller to read the body
	// to determine error details if needed.

	return resp, nil
}

// Ensure io is imported (used for io.Discard in potential future expansion).
var _ = io.Discard
