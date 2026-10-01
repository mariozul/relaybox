package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Client struct {
	transport http.RoundTripper
	timeout   time.Duration
}

func New(timeout time.Duration) *Client {
	return &Client{
		transport: otelhttp.NewTransport(http.DefaultTransport),
		timeout:   timeout,
	}
}

type DeliveryRequest struct {
	URL        string
	Body       []byte
	DeliveryID string
	AttemptNum int
}

type DeliveryResponse struct {
	StatusCode int
	Body       string
}

func (c *Client) Deliver(ctx context.Context, req *DeliveryRequest) (*DeliveryResponse, error) {
	if req.URL == "" {
		return nil, fmt.Errorf("deliver: URL is required")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Relaybox-Delivery-Id", req.DeliveryID)

	transport := c.transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	client := &http.Client{
		Timeout:   c.timeout,
		Transport: transport,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close() // RULE-RES-03

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	return &DeliveryResponse{StatusCode: resp.StatusCode, Body: string(body)}, nil
}
