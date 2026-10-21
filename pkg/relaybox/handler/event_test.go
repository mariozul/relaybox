// Package handler implements HTTP transport handlers (Clean Architecture transport layer).
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/dto"
)

// ---------------------------------------------------------------------------
// Mock EventIngestionService
// ---------------------------------------------------------------------------

type mockIngestionService struct {
	ingestFn func(ctx context.Context, tenantID string, req *dto.IngestEventRequest) (string, error)
}

func (m *mockIngestionService) IngestEvent(ctx context.Context, tenantID string, req *dto.IngestEventRequest) (string, error) {
	return m.ingestFn(ctx, tenantID, req)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestEventHandler_IngestEvent_Success(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{
		ingestFn: func(_ context.Context, tenantID string, req *dto.IngestEventRequest) (string, error) {
			if tenantID != "tenant-alpha" {
				return "", domain.ErrUnauthorized
			}
			if req.EventType != "order.created" {
				return "", domain.ErrInvalidInput
			}
			return "evt-123", nil
		},
	}

	h := NewEventHandler(svc)

	body, _ := json.Marshal(map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "ord-1"},
		"dedup_key":  "dedup-001",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.IngestEventHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}

	var resp map[string]string
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["event_id"] != "evt-123" {
		t.Errorf("expected event_id evt-123, got %s", resp["event_id"])
	}
}

func TestEventHandler_IngestEvent_MissingTenant(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{
		ingestFn: func(_ context.Context, _ string, _ *dto.IngestEventRequest) (string, error) {
			return "", domain.ErrUnauthorized
		},
	}

	h := NewEventHandler(svc)

	body, _ := json.Marshal(map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "ord-1"},
		"dedup_key":  "dedup-001",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	// No tenant in context (RULE-SEC-01)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.IngestEventHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 for missing tenant, got %d", res.StatusCode)
	}
}

func TestEventHandler_IngestEvent_Duplicate(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{
		ingestFn: func(_ context.Context, tenantID string, req *dto.IngestEventRequest) (string, error) {
			if tenantID == "tenant-alpha" && req.DedupKey == "dedup-001" {
				return "evt-original", domain.ErrEventAlreadyExists
			}
			return "evt-new", nil
		},
	}

	h := NewEventHandler(svc)

	body, _ := json.Marshal(map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "ord-1"},
		"dedup_key":  "dedup-001",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.IngestEventHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	// Idempotent: should return original event_id (FR-ING-03)
	if res.StatusCode != http.StatusConflict {
		t.Errorf("expected status 409 for duplicate event, got %d", res.StatusCode)
	}

	var resp map[string]string
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["event_id"] != "evt-original" {
		t.Errorf("expected original event_id evt-original for idempotent resend, got %s", resp["event_id"])
	}
}

func TestEventHandler_IngestEvent_InvalidJSON(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := NewEventHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader([]byte(`{invalid json`)))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.IngestEventHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", res.StatusCode)
	}
}

func TestEventHandler_IngestEvent_InvalidInput(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{
		ingestFn: func(_ context.Context, _ string, _ *dto.IngestEventRequest) (string, error) {
			return "", domain.ErrInvalidInput
		},
	}

	h := NewEventHandler(svc)

	body, _ := json.Marshal(map[string]interface{}{
		"event_type": "",
		"payload":    nil,
		"dedup_key":  "",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.IngestEventHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid input, got %d", res.StatusCode)
	}
}

func TestEventHandler_IngestEvent_InternalError(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{
		ingestFn: func(_ context.Context, _ string, _ *dto.IngestEventRequest) (string, error) {
			return "", errors.New("unexpected database error")
		},
	}

	h := NewEventHandler(svc)

	body, _ := json.Marshal(map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "ord-1"},
		"dedup_key":  "dedup-001",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.IngestEventHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 for internal error, got %d", res.StatusCode)
	}
}
