package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
	"github.com/mariozul/relaybox/pkg/relaybox/handler"
	"github.com/mariozul/relaybox/pkg/relaybox/service"
)

// ============================================================================
// Mock Subscription Service for handler-level tests
// ============================================================================

// mockSubscriptionService implements service.SubscriptionService for testing
// HTTP handlers without a real service layer.
type mockSubscriptionService struct {
	createFn func(ctx context.Context, tenantID string, req service.CreateSubscriptionRequest) (*domain.Subscription, error)
	getFn    func(ctx context.Context, tenantID, id string) (*domain.Subscription, error)
	listFn   func(ctx context.Context, tenantID string) ([]domain.Subscription, error)
	deleteFn func(ctx context.Context, tenantID, id string) error
}

func newMockSubscriptionService() *mockSubscriptionService {
	return &mockSubscriptionService{
		createFn: func(ctx context.Context, tenantID string, req service.CreateSubscriptionRequest) (*domain.Subscription, error) {
			sub, err := domain.NewSubscription(tenantID, req.EventType, req.TargetURL, "admin", time.Now().UTC())
			if err != nil {
				return nil, err
			}
			return sub, nil
		},
		getFn: func(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
			return nil, domain.ErrSubscriptionNotFound
		},
		listFn: func(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
			return []domain.Subscription{}, nil
		},
		deleteFn: func(ctx context.Context, tenantID, id string) error {
			return domain.ErrSubscriptionNotFound
		},
	}
}

func (m *mockSubscriptionService) CreateSubscription(ctx context.Context, tenantID string, req service.CreateSubscriptionRequest) (*domain.Subscription, error) {
	return m.createFn(ctx, tenantID, req)
}

func (m *mockSubscriptionService) GetSubscription(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
	return m.getFn(ctx, tenantID, id)
}

func (m *mockSubscriptionService) ListSubscriptions(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
	return m.listFn(ctx, tenantID)
}

func (m *mockSubscriptionService) DeleteSubscription(ctx context.Context, tenantID, id string) error {
	return m.deleteFn(ctx, tenantID, id)
}

// Ensure mock implements the interface at compile time.
var _ service.SubscriptionService = (*mockSubscriptionService)(nil)

// ============================================================================
// Tenant context helpers (RULE-SEC-01)
// Uses the handler's exported WithTenantContext to ensure context key
// compatibility between tests and production code.
// ============================================================================

// withTenantContext wraps handler.WithTenantContext for test convenience.
func withTenantContext(ctx context.Context, tenantID string) context.Context {
	return handler.WithTenantContext(ctx, tenantID)
}

// ============================================================================
// TC-04-UNIT: Subscription CRUD handlers
// ============================================================================

func TestSubscriptionHandler_CreateSubscription_Success(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	h := handler.NewSubscriptionHandler(mockSvc)

	body := map[string]string{
		"event_type": "order.created",
		"target_url": "https://example.com/webhook",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewReader(payload))
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.HandleCreateSubscription(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if id, ok := resp["id"]; !ok || id == "" {
		t.Error("expected non-empty id in response")
	}
	if tenantID, ok := resp["tenant_id"]; !ok || tenantID != "tenant-abc" {
		t.Errorf("expected tenant_id=%q, got %v", "tenant-abc", tenantID)
	}
	if eventType, ok := resp["event_type"]; !ok || eventType != "order.created" {
		t.Errorf("expected event_type=%q, got %v", "order.created", eventType)
	}
	if targetURL, ok := resp["target_url"]; !ok || targetURL != "https://example.com/webhook" {
		t.Errorf("expected target_url=%q, got %v", "https://example.com/webhook", targetURL)
	}
}

func TestSubscriptionHandler_CreateSubscription_EmptyTenant(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	h := handler.NewSubscriptionHandler(mockSvc)

	body := map[string]string{
		"event_type": "order.created",
		"target_url": "https://example.com/webhook",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewReader(payload))
	req = req.WithContext(withTenantContext(req.Context(), ""))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.HandleCreateSubscription(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for empty tenant, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSubscriptionHandler_CreateSubscription_MissingEventType(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	h := handler.NewSubscriptionHandler(mockSvc)

	body := map[string]string{
		"event_type": "",
		"target_url": "https://example.com/webhook",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewReader(payload))
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.HandleCreateSubscription(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for missing event_type, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSubscriptionHandler_CreateSubscription_InvalidJSON(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewReader([]byte(`{invalid json}`)))
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.HandleCreateSubscription(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid JSON, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSubscriptionHandler_GetSubscription_Success(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	// Override getFn to return a subscription.
	mockSvc.getFn = func(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
		sub, _ := domain.NewSubscription(tenantID, "order.created", "https://example.com/webhook", "admin", time.Now().UTC())
		sub.ID = id
		return sub, nil
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/sub-123", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleGetSubscription(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if id, ok := resp["id"]; !ok || id != "sub-123" {
		t.Errorf("expected id=%q, got %v", "sub-123", id)
	}
}

func TestSubscriptionHandler_GetSubscription_NotFound(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/nonexistent", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleGetSubscription(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for not found, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSubscriptionHandler_ListSubscriptions_Success(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	mockSvc.listFn = func(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
		sub1, _ := domain.NewSubscription(tenantID, "order.created", "https://example.com/webhook1", "admin", time.Now().UTC())
		sub2, _ := domain.NewSubscription(tenantID, "order.shipped", "https://example.com/webhook2", "admin", time.Now().UTC())
		return []domain.Subscription{*sub1, *sub2}, nil
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleListSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var subs []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&subs); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if len(subs) != 2 {
		t.Errorf("expected 2 subscriptions, got %d", len(subs))
	}
}

func TestSubscriptionHandler_ListSubscriptions_Empty(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleListSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var subs []interface{}
	if err := json.NewDecoder(rec.Body).Decode(&subs); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if len(subs) != 0 {
		t.Errorf("expected empty list, got %d element(s)", len(subs))
	}
}

func TestSubscriptionHandler_DeleteSubscription_Success(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	mockSvc.deleteFn = func(ctx context.Context, tenantID, id string) error {
		return nil
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/sub-123", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleDeleteSubscription(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestSubscriptionHandler_DeleteSubscription_NotFound(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/nonexistent", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleDeleteSubscription(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for not found, got %d", http.StatusNotFound, rec.Code)
	}
}

// ============================================================================
// TC-05-INT: Tenant isolation for subscription handlers
// Tenant A cannot access Tenant B's subscriptions (FR-SUB-02, RULE-SEC-01).
// ============================================================================

func TestSubscriptionHandler_TenantIsolation_GetCrossTenant(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	mockSvc.getFn = func(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
		// Simulate service-level tenant isolation: tenant-a's subscription
		// cannot be retrieved by tenant-b.
		if tenantID == "tenant-b" {
			return nil, domain.ErrSubscriptionNotFound
		}
		sub, _ := domain.NewSubscription(tenantID, "order.created", "https://tenant-a.com/webhook", "admin", time.Now().UTC())
		sub.ID = id
		return sub, nil
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	// Tenant B tries to access Tenant A's subscription.
	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/sub-123", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-b"))

	rec := httptest.NewRecorder()
	h.HandleGetSubscription(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for cross-tenant access, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSubscriptionHandler_TenantIsolation_DeleteCrossTenant(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	mockSvc.deleteFn = func(ctx context.Context, tenantID, id string) error {
		// Simulate service-level tenant isolation.
		if tenantID == "tenant-b" {
			return domain.ErrSubscriptionNotFound
		}
		return nil
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	// Tenant B tries to delete Tenant A's subscription.
	req := httptest.NewRequest(http.MethodDelete, "/v1/subscriptions/sub-123", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-b"))

	rec := httptest.NewRecorder()
	h.HandleDeleteSubscription(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for cross-tenant deletion, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSubscriptionHandler_TenantIsolation_ListIsScoped(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	mockSvc.listFn = func(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
		if tenantID == "tenant-a" {
			sub1, _ := domain.NewSubscription("tenant-a", "order.created", "https://tenant-a.com/webhook1", "admin", time.Now().UTC())
			sub2, _ := domain.NewSubscription("tenant-a", "order.shipped", "https://tenant-a.com/webhook2", "admin", time.Now().UTC())
			return []domain.Subscription{*sub1, *sub2}, nil
		} else if tenantID == "tenant-b" {
			sub, _ := domain.NewSubscription("tenant-b", "order.created", "https://tenant-b.com/webhook", "admin", time.Now().UTC())
			return []domain.Subscription{*sub}, nil
		}
		return []domain.Subscription{}, nil
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	// Tenant A lists - should see 2.
	reqA := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	reqA = reqA.WithContext(withTenantContext(reqA.Context(), "tenant-a"))
	recA := httptest.NewRecorder()
	h.HandleListSubscriptions(recA, reqA)

	if recA.Code != http.StatusOK {
		t.Errorf("expected status %d for tenant-a, got %d", http.StatusOK, recA.Code)
	}

	var subsA []interface{}
	if err := json.NewDecoder(recA.Body).Decode(&subsA); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if len(subsA) != 2 {
		t.Errorf("expected 2 subscriptions for tenant-a, got %d", len(subsA))
	}

	// Tenant B lists - should see 1.
	reqB := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	reqB = reqB.WithContext(withTenantContext(reqB.Context(), "tenant-b"))
	recB := httptest.NewRecorder()
	h.HandleListSubscriptions(recB, reqB)

	if recB.Code != http.StatusOK {
		t.Errorf("expected status %d for tenant-b, got %d", http.StatusOK, recB.Code)
	}

	var subsB []interface{}
	if err := json.NewDecoder(recB.Body).Decode(&subsB); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if len(subsB) != 1 {
		t.Errorf("expected 1 subscription for tenant-b, got %d", len(subsB))
	}
}

// ============================================================================
// Error mapping tests: verify domain errors map to correct HTTP status codes
// (RULE-ARCH-03).
// ============================================================================

func TestSubscriptionHandler_ErrorMapping_Unauthorized(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	mockSvc.getFn = func(ctx context.Context, tenantID, id string) (*domain.Subscription, error) {
		return nil, domain.ErrUnauthorized
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions/sub-123", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleGetSubscription(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d for unauthorized, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestSubscriptionHandler_ErrorMapping_InternalError(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	mockSvc.listFn = func(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
		return nil, domain.ErrInternal
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleListSubscriptions(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d for internal error, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestSubscriptionHandler_ErrorMapping_UnknownError(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	mockSvc.listFn = func(ctx context.Context, tenantID string) ([]domain.Subscription, error) {
		return nil, errors.New("some unknown error")
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/v1/subscriptions", nil)
	req = req.WithContext(withTenantContext(req.Context(), "tenant-abc"))

	rec := httptest.NewRecorder()
	h.HandleListSubscriptions(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d for unknown error, got %d", http.StatusInternalServerError, rec.Code)
	}
}

// ============================================================================
// Context propagation test: verify handler extracts tenant from context
// (RULE-SEC-01) rather than from request body/query parameters.
// ============================================================================

func TestSubscriptionHandler_TenantFromContext(t *testing.T) {
	t.Parallel()

	mockSvc := newMockSubscriptionService()
	capturedTenant := ""
	mockSvc.createFn = func(ctx context.Context, tenantID string, req service.CreateSubscriptionRequest) (*domain.Subscription, error) {
		capturedTenant = tenantID
		sub, _ := domain.NewSubscription(tenantID, req.EventType, req.TargetURL, "admin", time.Now().UTC())
		return sub, nil
	}
	h := handler.NewSubscriptionHandler(mockSvc)

	body := map[string]string{
		"event_type": "order.created",
		"target_url": "https://example.com/webhook",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/subscriptions", bytes.NewReader(payload))
	req = req.WithContext(withTenantContext(req.Context(), "tenant-from-context"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.HandleCreateSubscription(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	if capturedTenant != "tenant-from-context" {
		t.Errorf("expected tenant from context to be used, got %q", capturedTenant)
	}
}
