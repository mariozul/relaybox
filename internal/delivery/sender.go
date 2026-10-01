package delivery

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mariozul/relaybox/internal/domain"
)

// SenderConfig configures the HTTP delivery sender.
type SenderConfig struct {
	HTTPClient      *http.Client
	RequestTimeout  time.Duration
	DeliveryIDHeader string
}

// DefaultSenderConfig returns a sane default configuration.
func DefaultSenderConfig() SenderConfig {
	return SenderConfig{
		HTTPClient:       &http.Client{Timeout: 30 * time.Second},
		RequestTimeout:   10 * time.Second,
		DeliveryIDHeader: "X-Relaybox-Delivery-Id",
	}
}

// HTTPSender sends an outbox message to the subscriber's target URL.
type HTTPSender struct {
	cfg    SenderConfig
	client *http.Client
}

// NewHTTPSender creates an HTTPSender.
func NewHTTPSender(cfg SenderConfig) *HTTPSender {
	return &HTTPSender{
		cfg:    cfg,
		client: cfg.HTTPClient,
	}
}

// SendResult holds the outcome of a delivery attempt.
type SendResult struct {
	Status      string // "delivered", "retry", "deadletter"
	NextAttempt time.Time
	Error       error
}

// Send delivers a payload to a target URL with context propagation and stable delivery ID.
// RULE-RES-01: ctx propagated throughout.
// RULE-RES-03: resp.Body.Close() deferred immediately.
// RULE-DATA-02: stable X-Relaybox-Delivery-Id header for idempotency.
func (s *HTTPSender) Send(ctx context.Context, msg domain.OutboxMessage, payload []byte, targetURL string) SendResult {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return SendResult{Status: "deadletter", Error: fmt.Errorf("build request: %w", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(s.cfg.DeliveryIDHeader, msg.DeliveryID)

	resp, err := s.client.Do(req)
	if err != nil {
		// Network errors are transient.
		delay, ok := Backoff(msg.Attempts)
		if !ok {
			return SendResult{Status: "deadletter", Error: err}
		}
		return SendResult{Status: "retry", NextAttempt: time.Now().Add(delay), Error: err}
	}
	defer resp.Body.Close() // RULE-RES-03

	// 2xx: success.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return SendResult{Status: "delivered"}
	}

	// 4xx (except 429): permanent, dead-letter.
	if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
		return SendResult{Status: "deadletter", Error: fmt.Errorf("permanent error: HTTP %d", resp.StatusCode)}
	}

	// 5xx or 429: transient, retry.
	delay, ok := Backoff(msg.Attempts)
	if !ok {
		return SendResult{Status: "deadletter", Error: fmt.Errorf("max attempts reached after HTTP %d", resp.StatusCode)}
	}
	return SendResult{Status: "retry", NextAttempt: time.Now().Add(delay), Error: fmt.Errorf("transient error: HTTP %d", resp.StatusCode)}
}
