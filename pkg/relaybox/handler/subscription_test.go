// Package handler implements HTTP transport handlers.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/dto"
)

// ---------------------------------------------------------------------------
// Mock SubscriptionService
// ---------------------------------------------------------------------------

type mockSubscriptionService struct {
	createFn func(ctx context.Context, tenantID string, req *dto.CreateSubscriptionRequest) (string, error)
	getFn    func(ctx context.Context, tenantID, id string) (*domain.Subscription, error)
	listFn   func(ctx context.Context, tenantID string) ([]domain.Subscription, error)
	updateFn func(ctx context.Context, tenantID, id string, req *dto.UpdateSubscriptionRequest) error
	deleteFn func(ctx context.Context, tenantID, id string) error
}

func (m *mockSubscriptionService) CreateSubscription(ctx context.Context, tenantID string, req *dto.CreateSubscriptionRequest) (string, error) {
	return m.createFn(ctx, tenantID, req)
}

func (m *mockSubscriptionService) GetSubscription(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	return m.getFn(ctx, tenantID, id)
}

func (m *mockSubscriptionService) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	return m.listFn(ctx, tenantID)
}

func (m *mockSubscriptionService) UpdateSubscription(ctx context.Context, tenantID, id string, req *dto.UpdateSubscriptionRequest) error {
	return m.updateFn(ctx, tenantID, id, req)
}

func (m *mockSubscriptionService) DeleteSubscription(ctx context.Context, tenantID, id string) error {
	return m.deleteFn(ctx, tenantID, id)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSubscriptionHandler_Create_Success(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		createFn: func(_ context.Context, tenantID string, req *dto.CreateSubscriptionRequest) (string, error) {
			if tenantID != "tenant-alpha" {
				return "", domain.ErrUnauthorized
			}
			return "sub-123", nil
		},
	}

	h := NewSubscriptionHandler(svc)

	body, _ := json.Marshal(map[string]string{
		"event_type": "order.created",
		"target_url": "https://example.com/webhook",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewReader(body))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.CreateSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", res.StatusCode)
	}

	var resp map[string]string
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["subscription_id"] != "sub-123" {
		t.Errorf("expected subscription_id sub-123, got %s", resp["subscription_id"])
	}
}

func TestSubscriptionHandler_Create_MissingTenant(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		createFn: func(_ context.Context, _ string, _ *dto.CreateSubscriptionRequest) (string, error) {
			return "", domain.ErrUnauthorized
		},
	}

	h := NewSubscriptionHandler(svc)

	body, _ := json.Marshal(map[string]string{
		"event_type": "order.created",
		"target_url": "https://example.com/webhook",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.CreateSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 for missing tenant, got %d", res.StatusCode)
	}
}

func TestSubscriptionHandler_Create_InvalidInput(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		createFn: func(_ context.Context, _ string, _ *dto.CreateSubscriptionRequest) (string, error) {
			return "", domain.ErrInvalidInput
		},
	}

	h := NewSubscriptionHandler(svc)

	body, _ := json.Marshal(map[string]string{
		"event_type": "",
		"target_url": "",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewReader(body))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.CreateSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid input, got %d", res.StatusCode)
	}
}

func TestSubscriptionHandler_Get_Success(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		getFn: func(_ context.Context, tenantID, id string) (*domain.Subscription, error) {
			if tenantID != "tenant-alpha" {
				return nil, domain.ErrUnauthorized
			}
			return &domain.Subscription{
				ID:        id,
				TenantID:  tenantID,
				EventType: "order.created",
				TargetURL: "https://example.com/webhook",
			}, nil
		},
	}

	h := NewSubscriptionHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/sub-123", nil)
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.SetPathValue("id", "sub-123")

	w := httptest.NewRecorder()
	h.GetSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["id"] != "sub-123" {
		t.Errorf("expected id sub-123, got %s", resp["id"])
	}
}

func TestSubscriptionHandler_Get_NotFound(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		getFn: func(_ context.Context, tenantID, id string) (*domain.Subscription, error) {
			return nil, domain.ErrSubscriptionNotFound
		},
	}

	h := NewSubscriptionHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/sub-unknown", nil)
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.SetPathValue("id", "sub-unknown")

	w := httptest.NewRecorder()
	h.GetSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", res.StatusCode)
	}
}

func TestSubscriptionHandler_List_Success(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		listFn: func(_ context.Context, tenantID string) ([]domain.Subscription, error) {
			if tenantID != "tenant-alpha" {
				return nil, domain.ErrUnauthorized
			}
			return []domain.Subscription{
				{ID: "sub-1", TenantID: tenantID, EventType: "order.created", TargetURL: "https://a.com/webhook"},
				{ID: "sub-2", TenantID: tenantID, EventType: "payment.completed", TargetURL: "https://b.com/webhook"},
			}, nil
		},
	}

	h := NewSubscriptionHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))

	w := httptest.NewRecorder()
	h.ListSubscriptionsHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}

	var resp dto.SubscriptionListResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Subscriptions) != 2 {
		t.Errorf("expected 2 subscriptions, got %d", len(resp.Subscriptions))
	}
}

func TestSubscriptionHandler_Update_Success(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		updateFn: func(_ context.Context, tenantID, id string, req *dto.UpdateSubscriptionRequest) error {
			if tenantID != "tenant-alpha" {
				return domain.ErrUnauthorized
			}
			if req.TargetURL == nil || *req.TargetURL != "https://updated.com/webhook" {
				return domain.ErrInvalidInput
			}
			return nil
		},
	}

	h := NewSubscriptionHandler(svc)

	body, _ := json.Marshal(map[string]string{
		"target_url": "https://updated.com/webhook",
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/subscriptions/sub-123", bytes.NewReader(body))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.SetPathValue("id", "sub-123")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.UpdateSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}
}

func TestSubscriptionHandler_Update_NotFound(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		updateFn: func(_ context.Context, _, _ string, _ *dto.UpdateSubscriptionRequest) error {
			return domain.ErrSubscriptionNotFound
		},
	}

	h := NewSubscriptionHandler(svc)

	body, _ := json.Marshal(map[string]string{
		"target_url": "https://updated.com/webhook",
	})
	req := httptest.NewRequest(http.MethodPut, "/v1/subscriptions/sub-unknown", bytes.NewReader(body))
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.SetPathValue("id", "sub-unknown")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.UpdateSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", res.StatusCode)
	}
}

func TestSubscriptionHandler_Delete_Success(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		deleteFn: func(_ context.Context, tenantID, id string) error {
			if tenantID != "tenant-alpha" {
				return domain.ErrUnauthorized
			}
			return nil
		},
	}

	h := NewSubscriptionHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/sub-123", nil)
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.SetPathValue("id", "sub-123")

	w := httptest.NewRecorder()
	h.DeleteSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", res.StatusCode)
	}
}

func TestSubscriptionHandler_Delete_NotFound(t *testing.T) {
	t.Parallel()

	svc := &mockSubscriptionService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return domain.ErrSubscriptionNotFound
		},
	}

	h := NewSubscriptionHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/sub-unknown", nil)
	req = req.WithContext(WithTenant(req.Context(), "tenant-alpha"))
	req.SetPathValue("id", "sub-unknown")

	w := httptest.NewRecorder()
	h.DeleteSubscriptionHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", res.StatusCode)
	}
}
