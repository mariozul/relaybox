package adapter

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mariozul/relaybox/internal/service"
)

// HTTPForwarder implements service.Forwarder with OTel propagation and timeouts.
type HTTPForwarder struct {
	client  *http.Client
	timeout time.Duration
}

// NewHTTPForwarder creates a new HTTPForwarder.
func NewHTTPForwarder(client *http.Client, timeout time.Duration) *HTTPForwarder {
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &HTTPForwarder{client: client, timeout: timeout}
}

// Forward sends an HTTP POST to the subscriber URL (FR-DEL-01, RULE-RES-01).
func (f *HTTPForwarder) Forward(ctx context.Context, req service.ForwardRequest) (*service.ForwardResult, error) {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, &service.ForwardError{Message: fmt.Sprintf("create request: %v", err), Retryable: false}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Relaybox-Delivery-Id", req.DeliveryID)

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, &service.ForwardError{Message: fmt.Sprintf("request failed: %v", err), Retryable: true}
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)

	result := &service.ForwardResult{StatusCode: resp.StatusCode, Body: buf.String()}

	if service.IsPermanent(resp.StatusCode) {
		return result, &service.ForwardError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("permanent failure: %d", resp.StatusCode), Retryable: false}
	}
	if resp.StatusCode >= 500 {
		return result, &service.ForwardError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("server error: %d", resp.StatusCode), Retryable: true}
	}
	return result, nil
}
