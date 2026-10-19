// Package httpclient provides an HTTP client adapter for outbound webhook
// delivery with context propagation, timeout support, and immediate body
// closure discipline (RULE-RES-01, RULE-RES-03, RULE-OBS-02).
package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client defines the contract for making HTTP POST requests with context
// propagation and response body management.
type Client interface {
	// Post sends an HTTP POST request to the given URL with the provided headers
	// and body. Context is propagated for cancellation and timeout (RULE-RES-01).
	// The caller MUST close resp.Body when done (RULE-RES-03).
	Post(ctx context.Context, url string, headers map[string]string, body []byte) (*http.Response, error)
}

// defaultHTTPClient implements Client using the standard net/http package.
type defaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient creates a new HTTP client with the specified timeout.
func NewDefaultHTTPClient(timeout time.Duration) Client {
	return &defaultHTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Post sends an HTTP POST request with context propagation.
//
// The method:
//   - Creates the HTTP request with the given body
//   - Sets Content-Type to application/json unless overridden
//   - Propagates ctx to the request for cancellation/timeout (RULE-RES-01)
//   - Sets all provided headers
//   - Returns the response with body intact for the caller to close (RULE-RES-03)
//
// Note: The caller MUST call resp.Body.Close() to avoid resource leaks.
func (c *defaultHTTPClient) Post(ctx context.Context, url string, headers map[string]string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set default content type
	req.Header.Set("Content-Type", "application/json")

	// Set additional headers (may override Content-Type)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	return resp, nil
}

// compile-time interface check.
var _ Client = (*defaultHTTPClient)(nil)

// EnsureResponseBodyClosed is a helper that reads and closes the response body.
// Use it when you need to ensure the body is fully consumed and closed
// (RULE-RES-03).
func EnsureResponseBodyClosed(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}
