package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/mariozul/relaybox/internal/domain"
	"go.opentelemetry.io/otel"
)

type Sender struct{ client *http.Client }

func NewSender(client *http.Client) *Sender { return &Sender{client: client} }

func (s *Sender) Send(ctx context.Context, delivery domain.Delivery) (int, error) {
	ctx, span := otel.Tracer("relaybox/dispatcher").Start(ctx, "webhook.delivery")
	defer span.End()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, delivery.TargetURL, bytes.NewReader(delivery.Payload))
	if err != nil {
		return 0, fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Relaybox-Delivery-Id", delivery.ID)
	otel.GetTextMapPropagator().Inject(ctx, propagationHeader(req.Header))
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, nil
}

type propagationHeader http.Header

func (h propagationHeader) Get(key string) string { return http.Header(h).Get(key) }
func (h propagationHeader) Set(key, value string) { http.Header(h).Set(key, value) }
func (h propagationHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
