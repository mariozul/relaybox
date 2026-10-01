package service

import (
	"context"
	"net/http"
)

// ForwardRequest is the input for forwarding a webhook to a subscriber.
type ForwardRequest struct {
	URL         string
	Body        []byte
	DeliveryID  string
	EventType   string
}

// ForwardResult is the outcome of a forward attempt.
type ForwardResult struct {
	StatusCode int
	Body       string
}

// Forwarder is the contract for delivering events to external HTTP endpoints.
// Implementations must inject OTel trace metadata and set timeouts (RULE-RES-01, RULE-OBS-02).
type Forwarder interface {
	// Forward sends an HTTP POST to the subscriber URL with
	// X-Relaybox-Delivery-Id header set to req.DeliveryID (FR-DEL-04).
	Forward(ctx context.Context, req ForwardRequest) (*ForwardResult, error)
}

// ForwardError represents a transient delivery failure (5xx, timeout).
type ForwardError struct {
	StatusCode int
	Message    string
	Retryable  bool
}

func (e *ForwardError) Error() string {
	return "forward: " + e.Message
}

// IsPermanent checks if the forward error is non-retryable (4xx).
func IsPermanent(statusCode int) bool {
	return statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError
}
