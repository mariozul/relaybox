// Package service defines the core business logic interfaces and implementations
// for the Relaybox multi-tenant webhook relay service. It orchestrates domain
// operations using injected repository interfaces, following Clean Architecture
// principles (RULE-ARCH-01, RULE-ARCH-02).
package service

import (
	"context"
	"net/http"
)

// WebhookClient defines the contract for sending HTTP POST webhook requests
// to subscriber endpoints. Implementations MUST:
//   - Propagate ctx context.Context to the outbound HTTP request (RULE-RES-01).
//   - Set a timeout on the HTTP client to prevent hanging requests (FR-DEL-01).
//   - Propagate OpenTelemetry trace metadata via headers (RULE-OBS-02).
//   - Return the full *http.Response for status code inspection by the caller.
//   - The caller is responsible for closing resp.Body (RULE-RES-03).
type WebhookClient interface {
	// SendWebhook sends an HTTP POST request to the specified URL with the
	// given headers and payload. It returns the HTTP response for the caller
	// to inspect the status code.
	SendWebhook(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error)
}
