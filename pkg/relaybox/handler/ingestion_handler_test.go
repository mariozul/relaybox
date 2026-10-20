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
// Mock IngestionService for handler testing
// ============================================================================

type mockIngestionService struct {
	ingestFn func(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error)
}

func (m *mockIngestionService) IngestEvent(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error) {
	if m.ingestFn != nil {
		return m.ingestFn(ctx, tenantID, req, now)
	}
	return &service.IngestEventResponse{EventID: "evt-123"}, nil
}

// ============================================================================
// Test helpers
// ============================================================================

// contextWithTenant returns a context with a tenant ID injected, simulating
// an authenticated request context (RULE-SEC-01).
func contextWithTenant(tenantID string) context.Context {
	return context.WithValue(context.Background(), handler.CtxKeyTenantID, tenantID)
}

// newIngestRequest creates a new HTTP POST request to /v1/events with the
// given body and context.
func newIngestRequest(t *testing.T, ctx context.Context, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	return req
}

// newIngestRequestWithTenant is a convenience wrapper that creates a request
// with a tenant context already set.
func newIngestRequestWithTenant(t *testing.T, tenantID string, body []byte) *http.Request {
	t.Helper()
	return newIngestRequest(t, contextWithTenant(tenantID), body)
}

// ============================================================================
// TC-01-UNIT: Successful event ingestion via HTTP handler
// ============================================================================

func TestIngestionHandler_HandleIngestEvent_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := &mockIngestionService{
		ingestFn: func(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error) {
			if tenantID != "tenant-abc" {
				t.Errorf("expected tenantID=tenant-abc, got %q", tenantID)
			}
			if req.EventType != "order.created" {
				t.Errorf("expected EventType=order.created, got %q", req.EventType)
			}
			if string(req.Payload) != `{"order_id":"123"}` {
				t.Errorf("expected Payload={\"order_id\":\"123\"}, got %q", string(req.Payload))
			}
			if req.DedupKey != "dedup-001" {
				t.Errorf("expected DedupKey=dedup-001, got %q", req.DedupKey)
			}
			return &service.IngestEventResponse{EventID: "evt-abc-123"}, nil
		},
	}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "123"},
		"dedup_key":  "dedup-001",
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal body: %v", err)
	}

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	// Act
	h.HandleIngestEvent(rr, req)

	// Assert
	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["event_id"] != "evt-abc-123" {
		t.Errorf("expected event_id=evt-abc-123, got %q", resp["event_id"])
	}

	// Verify Content-Type header
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %q", ct)
	}
}

func TestIngestionHandler_HandleIngestEvent_Success_JSONArrayPayload(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{
		ingestFn: func(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error) {
			if string(req.Payload) != `[1,2,3]` {
				t.Errorf("expected Payload=[1,2,3], got %q", string(req.Payload))
			}
			return &service.IngestEventResponse{EventID: "evt-arr-001"}, nil
		},
	}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    []int{1, 2, 3},
		"dedup_key":  "dedup-arr",
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// TC-02-EDGE: Validation error responses
// ============================================================================

func TestIngestionHandler_HandleIngestEvent_MissingAuthContext(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "123"},
		"dedup_key":  "dedup-001",
	}
	bodyBytes, _ := json.Marshal(body)

	// Request WITHOUT tenant context (RULE-SEC-01 violation detection)
	req := newIngestRequest(t, context.Background(), bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 for missing tenant context, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp["code"] != "RELAYBOX_UNAUTHORIZED" {
		t.Errorf("expected code=RELAYBOX_UNAUTHORIZED, got %q", errResp["code"])
	}
}

func TestIngestionHandler_HandleIngestEvent_MissingAuthContext_NilContext(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "123"},
		"dedup_key":  "dedup-001",
	}
	bodyBytes, _ := json.Marshal(body)

	// Request with nil context (worst case)
	req := newIngestRequest(t, nil, bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 for nil context, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIngestionHandler_HandleIngestEvent_InvalidJSON(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	req := newIngestRequestWithTenant(t, "tenant-abc", []byte(`{invalid json`))
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for invalid JSON, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp["code"] != "RELAYBOX_INVALID_EVENT_PAYLOAD" {
		t.Errorf("expected code=RELAYBOX_INVALID_EVENT_PAYLOAD, got %q", errResp["code"])
	}
}

func TestIngestionHandler_HandleIngestEvent_EmptyBody(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	req := newIngestRequestWithTenant(t, "tenant-abc", nil)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for empty body, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIngestionHandler_HandleIngestEvent_MissingEventType(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"payload":   map[string]interface{}{"order_id": "123"},
		"dedup_key": "dedup-001",
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for missing event_type, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIngestionHandler_HandleIngestEvent_MissingPayload(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"dedup_key":  "dedup-001",
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for missing payload, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIngestionHandler_HandleIngestEvent_MissingDedupKey(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "123"},
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for missing dedup_key, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIngestionHandler_HandleIngestEvent_EmptyEventType(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "",
		"payload":    map[string]interface{}{"order_id": "123"},
		"dedup_key":  "dedup-001",
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for empty event_type, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIngestionHandler_HandleIngestEvent_EmptyDedupKey(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "123"},
		"dedup_key":  "",
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for empty dedup_key, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIngestionHandler_HandleIngestEvent_NullPayload(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    nil,
		"dedup_key":  "dedup-001",
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for null payload, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// TC-02-EDGE: Domain error to HTTP status mapping
// ============================================================================

func TestIngestionHandler_HandleIngestEvent_DomainErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		domainErr      error
		expectedStatus int
	}{
		{
			name:           "ErrInvalidEventPayload -> 400",
			domainErr:      domain.ErrInvalidEventPayload,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "ErrDuplicateEvent -> 409",
			domainErr:      domain.ErrDuplicateEvent,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "ErrUnauthorized -> 403",
			domainErr:      domain.ErrUnauthorized,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "ErrInternal -> 500",
			domainErr:      domain.ErrInternal,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "generic error -> 500",
			domainErr:      errors.New("unexpected database error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := &mockIngestionService{
				ingestFn: func(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error) {
					return nil, tc.domainErr
				},
			}
			h := handler.NewIngestionHandler(svc)

			body := map[string]interface{}{
				"event_type": "order.created",
				"payload":    map[string]interface{}{"order_id": "123"},
				"dedup_key":  "dedup-001",
			}
			bodyBytes, _ := json.Marshal(body)

			req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
			rr := httptest.NewRecorder()

			h.HandleIngestEvent(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("expected HTTP %d, got %d: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}

			// Verify error response body is valid JSON with code field
			var errResp map[string]interface{}
			if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
				t.Fatalf("error response must be valid JSON: %v", err)
			}
			if _, ok := errResp["code"]; !ok {
				t.Error("error response must contain a 'code' field")
			}
			if _, ok := errResp["message"]; !ok {
				t.Error("error response must contain a 'message' field")
			}
		})
	}
}

// ============================================================================
// TC-01-UNIT: Idempotent behavior - duplicate returns same event ID via HTTP
// ============================================================================

func TestIngestionHandler_HandleIngestEvent_DuplicateReturnsSameID(t *testing.T) {
	t.Parallel()

	callCount := 0
	svc := &mockIngestionService{
		ingestFn: func(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error) {
			callCount++
			return &service.IngestEventResponse{EventID: "evt-original-001"}, nil
		},
	}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "123"},
		"dedup_key":  "dedup-same",
	}
	bodyBytes, _ := json.Marshal(body)

	// First request
	req1 := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr1 := httptest.NewRecorder()
	h.HandleIngestEvent(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: expected HTTP 200, got %d: %s", rr1.Code, rr1.Body.String())
	}
	var resp1 map[string]string
	json.NewDecoder(rr1.Body).Decode(&resp1)

	// Second request with same dedup_key
	req2 := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr2 := httptest.NewRecorder()
	h.HandleIngestEvent(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("second request: expected HTTP 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
	var resp2 map[string]string
	json.NewDecoder(rr2.Body).Decode(&resp2)

	if resp2["event_id"] != resp1["event_id"] {
		t.Errorf("duplicate request should return same event_id: first=%q, second=%q", resp1["event_id"], resp2["event_id"])
	}
}

// ============================================================================
// TC-02-EDGE: HTTP method and content type validation
// ============================================================================

func TestIngestionHandler_HandleIngestEvent_WrongMethod(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "123"},
		"dedup_key":  "dedup-001",
	}
	bodyBytes, _ := json.Marshal(body)

	// GET request instead of POST
	req := httptest.NewRequest(http.MethodGet, "/v1/events", bytes.NewReader(bodyBytes))
	req = req.WithContext(contextWithTenant("tenant-abc"))
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	// The handler should still process the event data; this tests it doesn't panic
	// on wrong method (we don't enforce method check on the handler itself,
	// that's typically done by the HTTP router)
	_ = rr.Code // No strict assertion; handler responds based on body
}

// ============================================================================
// TC-02-EDGE: Nil service dependency
// ============================================================================

func TestIngestionHandler_NewIngestionHandler_NilService(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("NewIngestionHandler should panic when service is nil")
		}
	}()

	handler.NewIngestionHandler(nil)
}

// ============================================================================
// TC-01-UNIT: CtxKeyTenantID is exported and usable
// ============================================================================

func TestCtxKeyTenantID_Exported(t *testing.T) {
	t.Parallel()

	// Verify the context key is exported and works
	ctx := context.WithValue(context.Background(), handler.CtxKeyTenantID, "test-tenant")
	val, ok := ctx.Value(handler.CtxKeyTenantID).(string)
	if !ok {
		t.Fatal("CtxKeyTenantID context value should be a string")
	}
	if val != "test-tenant" {
		t.Errorf("expected tenant=test-tenant, got %q", val)
	}
}

// ============================================================================
// TC-01-UNIT: IngestionHandler interface compliance
// ============================================================================

func TestIngestionHandler_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// Compile-time check that the handler can be instantiated
	svc := &mockIngestionService{}
	h := handler.NewIngestionHandler(svc)
	if h == nil {
		t.Fatal("NewIngestionHandler returned nil")
	}
}

// ============================================================================
// TC-02-EDGE: Large payload handling
// ============================================================================

func TestIngestionHandler_HandleIngestEvent_LargePayload(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{
		ingestFn: func(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error) {
			if len(req.Payload) == 0 {
				t.Error("expected non-empty payload")
			}
			return &service.IngestEventResponse{EventID: "evt-large-001"}, nil
		},
	}
	h := handler.NewIngestionHandler(svc)

	// Create a larger nested JSON payload
	largePayload := make(map[string]interface{})
	largePayload["data"] = make([]int, 1000)
	for i := 0; i < 1000; i++ {
		largePayload["data"].([]int)[i] = i
	}

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    largePayload,
		"dedup_key":  "dedup-large",
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for large payload, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// TC-02-EDGE: Error response structure consistency
// ============================================================================

func TestIngestionHandler_ErrorResponseStructure(t *testing.T) {
	t.Parallel()

	svc := &mockIngestionService{
		ingestFn: func(ctx context.Context, tenantID string, req service.IngestEventRequest, now time.Time) (*service.IngestEventResponse, error) {
			return nil, domain.ErrInvalidEventPayload
		},
	}
	h := handler.NewIngestionHandler(svc)

	body := map[string]interface{}{
		"event_type": "order.created",
		"payload":    map[string]interface{}{"order_id": "123"},
		"dedup_key":  "dedup-001",
	}
	bodyBytes, _ := json.Marshal(body)

	req := newIngestRequestWithTenant(t, "tenant-abc", bodyBytes)
	rr := httptest.NewRecorder()

	h.HandleIngestEvent(rr, req)

	var errResp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// Verify all required fields in error response
	if _, ok := errResp["code"]; !ok {
		t.Error("error response must contain 'code'")
	}
	if _, ok := errResp["message"]; !ok {
		t.Error("error response must contain 'message'")
	}
}
