package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mariozul/relaybox/pkg/relaybox/domain"
)

// ============================================================================
// TC-01-UNIT: Core entity construction and validation
// ============================================================================

func TestEvent_NewEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tenantID   string
		eventType  string
		payload    []byte
		dedupKey   string
		createdBy  string
		wantErr    bool
		errSentinel error
	}{
		{
			name:      "valid event",
			tenantID:  "tenant-abc",
			eventType: "order.created",
			payload:   []byte(`{"order_id":"123"}`),
			dedupKey:  "dedup-001",
			createdBy: "system",
			wantErr:   false,
		},
		{
			name:       "missing tenant_id",
			tenantID:   "",
			eventType:  "order.created",
			payload:    []byte(`{}`),
			dedupKey:   "dedup-002",
			createdBy:  "system",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "missing event_type",
			tenantID:   "tenant-abc",
			eventType:  "",
			payload:    []byte(`{}`),
			dedupKey:   "dedup-003",
			createdBy:  "system",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "nil payload",
			tenantID:   "tenant-abc",
			eventType:  "order.created",
			payload:    nil,
			dedupKey:   "dedup-004",
			createdBy:  "system",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "empty payload",
			tenantID:   "tenant-abc",
			eventType:  "order.created",
			payload:    []byte{},
			dedupKey:   "dedup-005",
			createdBy:  "system",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "missing dedup_key",
			tenantID:   "tenant-abc",
			eventType:  "order.created",
			payload:    []byte(`{}`),
			dedupKey:   "",
			createdBy:  "system",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "missing created_by",
			tenantID:   "tenant-abc",
			eventType:  "order.created",
			payload:    []byte(`{}`),
			dedupKey:   "dedup-006",
			createdBy:  "",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			now := time.Now().UTC()
			evt, err := domain.NewEvent(tc.tenantID, tc.eventType, tc.payload, tc.dedupKey, tc.createdBy, now)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if !errors.Is(err, tc.errSentinel) {
					t.Fatalf("expected error %v, got %v", tc.errSentinel, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if evt == nil {
				t.Fatal("expected event, got nil")
			}

			if evt.ID == "" {
				t.Error("expected event ID to be set")
			}
			if evt.TenantID != tc.tenantID {
				t.Errorf("expected TenantID=%q, got %q", tc.tenantID, evt.TenantID)
			}
			if evt.EventType != tc.eventType {
				t.Errorf("expected EventType=%q, got %q", tc.eventType, evt.EventType)
			}
			if string(evt.Payload) != string(tc.payload) {
				t.Errorf("expected Payload=%q, got %q", string(tc.payload), string(evt.Payload))
			}
			if evt.DedupKey != tc.dedupKey {
				t.Errorf("expected DedupKey=%q, got %q", tc.dedupKey, evt.DedupKey)
			}
			if evt.CreatedBy != tc.createdBy {
				t.Errorf("expected CreatedBy=%q, got %q", tc.createdBy, evt.CreatedBy)
			}
			if !evt.CreatedAt.Equal(now) {
				t.Errorf("expected CreatedAt=%v, got %v", now, evt.CreatedAt)
			}
		})
	}
}

func TestSubscription_NewSubscription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tenantID   string
		eventType  string
		targetURL  string
		createdBy  string
		wantErr    bool
		errSentinel error
	}{
		{
			name:      "valid subscription",
			tenantID:  "tenant-abc",
			eventType: "order.created",
			targetURL: "https://example.com/webhook",
			createdBy: "admin",
			wantErr:   false,
		},
		{
			name:       "missing tenant_id",
			tenantID:   "",
			eventType:  "order.created",
			targetURL:  "https://example.com/webhook",
			createdBy:  "admin",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "missing event_type",
			tenantID:   "tenant-abc",
			eventType:  "",
			targetURL:  "https://example.com/webhook",
			createdBy:  "admin",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "invalid url",
			tenantID:   "tenant-abc",
			eventType:  "order.created",
			targetURL:  "not-a-url",
			createdBy:  "admin",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "empty url",
			tenantID:   "tenant-abc",
			eventType:  "order.created",
			targetURL:  "",
			createdBy:  "admin",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
		{
			name:       "missing created_by",
			tenantID:   "tenant-abc",
			eventType:  "order.created",
			targetURL:  "https://example.com/webhook",
			createdBy:  "",
			wantErr:    true,
			errSentinel: domain.ErrInvalidEventPayload,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			now := time.Now().UTC()
			sub, err := domain.NewSubscription(tc.tenantID, tc.eventType, tc.targetURL, tc.createdBy, now)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if !errors.Is(err, tc.errSentinel) {
					t.Fatalf("expected error %v, got %v", tc.errSentinel, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if sub == nil {
				t.Fatal("expected subscription, got nil")
			}

			if sub.ID == "" {
				t.Error("expected subscription ID to be set")
			}
			if sub.TenantID != tc.tenantID {
				t.Errorf("expected TenantID=%q, got %q", tc.tenantID, sub.TenantID)
			}
			if sub.EventType != tc.eventType {
				t.Errorf("expected EventType=%q, got %q", tc.eventType, sub.EventType)
			}
			if sub.TargetURL != tc.targetURL {
				t.Errorf("expected TargetURL=%q, got %q", tc.targetURL, sub.TargetURL)
			}
			if sub.CreatedBy != tc.createdBy {
				t.Errorf("expected CreatedBy=%q, got %q", tc.createdBy, sub.CreatedBy)
			}
			if !sub.CreatedAt.Equal(now) {
				t.Errorf("expected CreatedAt=%v, got %v", now, sub.CreatedAt)
			}
		})
	}
}

func TestOutboxEntry_NewOutboxEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		eventID        string
		subscriptionID string
		wantErr        bool
		errSentinel    error
	}{
		{
			name:           "valid outbox entry",
			eventID:        "evt-001",
			subscriptionID: "sub-001",
			wantErr:        false,
		},
		{
			name:           "missing event id",
			eventID:        "",
			subscriptionID: "sub-001",
			wantErr:        true,
			errSentinel:    domain.ErrInvalidEventPayload,
		},
		{
			name:           "missing subscription id",
			eventID:        "evt-001",
			subscriptionID: "",
			wantErr:        true,
			errSentinel:    domain.ErrInvalidEventPayload,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			now := time.Now().UTC()
			entry, err := domain.NewOutboxEntry(tc.eventID, tc.subscriptionID, now)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if !errors.Is(err, tc.errSentinel) {
					t.Fatalf("expected error %v, got %v", tc.errSentinel, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if entry == nil {
				t.Fatal("expected outbox entry, got nil")
			}

			if entry.ID == "" {
				t.Error("expected outbox entry ID to be set")
			}
			if entry.EventID != tc.eventID {
				t.Errorf("expected EventID=%q, got %q", tc.eventID, entry.EventID)
			}
			if entry.SubscriptionID != tc.subscriptionID {
				t.Errorf("expected SubscriptionID=%q, got %q", tc.subscriptionID, entry.SubscriptionID)
			}
			if entry.Status != domain.OutboxStatusPending {
				t.Errorf("expected Status=%q, got %q", domain.OutboxStatusPending, entry.Status)
			}
			if entry.Attempts != 0 {
				t.Errorf("expected Attempts=0, got %d", entry.Attempts)
			}
			if !entry.CreatedAt.Equal(now) {
				t.Errorf("expected CreatedAt=%v, got %v", now, entry.CreatedAt)
			}
		})
	}
}

func TestOutboxEntry_AdvanceAttempt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func() *domain.OutboxEntry
		newStatus   domain.OutboxStatus
		maxAttempts int
		wantErr     bool
		errSentinel error
	}{
		{
			name: "delivered stops advancement",
			setup: func() *domain.OutboxEntry {
				return &domain.OutboxEntry{Status: domain.OutboxStatusDelivered}
			},
			newStatus:   domain.OutboxStatusDelivered,
			maxAttempts: 5,
			wantErr:     false,
		},
		{
			name: "advance from pending to failed",
			setup: func() *domain.OutboxEntry {
				entry, _ := domain.NewOutboxEntry("evt-001", "sub-001", time.Now().UTC())
				return entry
			},
			newStatus:   domain.OutboxStatusFailed,
			maxAttempts: 3,
			wantErr:     false,
		},
		{
			name: "deadletter after max attempts",
			setup: func() *domain.OutboxEntry {
				return &domain.OutboxEntry{
					Status:    domain.OutboxStatusFailed,
					Attempts:  domain.MaxDeliveryAttempts,
				}
			},
			newStatus:   domain.OutboxStatusFailed,
			maxAttempts: domain.MaxDeliveryAttempts,
			wantErr:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			entry := tc.setup()

			err := entry.AdvanceAttempt(tc.newStatus, tc.maxAttempts)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if !errors.Is(err, tc.errSentinel) {
					t.Fatalf("expected error %v, got %v", tc.errSentinel, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.newStatus == domain.OutboxStatusDelivered {
				if entry.Status != domain.OutboxStatusDelivered {
					t.Errorf("expected Status=delivered, got %v", entry.Status)
				}
			} else if entry.Attempts > 0 && entry.Attempts >= tc.maxAttempts {
				if entry.Status != domain.OutboxStatusDeadLetter {
					t.Errorf("expected Status=deadletter after max attempts, got %v", entry.Status)
				}
			} else {
				if entry.Status != tc.newStatus {
					t.Errorf("expected Status=%v, got %v", tc.newStatus, entry.Status)
				}
			}
		})
	}
}

// ============================================================================
// TC-10-UNIT: X-Relaybox-Delivery-Id header stability (FR-DEL-04, RULE-DATA-02)
// ============================================================================

func TestDeliveryID_Deterministic(t *testing.T) {
	t.Parallel()

	// The same (eventID, subscriptionID) pair MUST produce the same DeliveryID.
	eventID := "evt-abc-123"
	subID := "sub-xyz-789"

	id1 := domain.GenerateDeliveryID(eventID, subID)
	id2 := domain.GenerateDeliveryID(eventID, subID)

	if id1 != id2 {
		t.Errorf("DeliveryID must be deterministic: first=%q, second=%q", id1, id2)
	}

	if id1 == "" {
		t.Error("DeliveryID must not be empty")
	}
}

func TestDeliveryID_DifferentInputs(t *testing.T) {
	t.Parallel()

	// Different (eventID, subscriptionID) pairs MUST produce different DeliveryIDs.
	id1 := domain.GenerateDeliveryID("evt-001", "sub-001")
	id2 := domain.GenerateDeliveryID("evt-002", "sub-001")
	id3 := domain.GenerateDeliveryID("evt-001", "sub-002")

	if id1 == id2 {
		t.Error("different eventIDs should produce different DeliveryIDs")
	}
	if id1 == id3 {
		t.Error("different subscriptionIDs should produce different DeliveryIDs")
	}
}

// ============================================================================
// Error sentinel tests
// ============================================================================

func TestErrorSentinels_AreSentinelErrors(t *testing.T) {
	t.Parallel()

	// Verify all sentinel errors are defined and exported.
	if domain.ErrInvalidEventPayload == nil {
		t.Error("ErrInvalidEventPayload must be defined")
	}
	if domain.ErrDuplicateEvent == nil {
		t.Error("ErrDuplicateEvent must be defined")
	}
	if domain.ErrSubscriptionNotFound == nil {
		t.Error("ErrSubscriptionNotFound must be defined")
	}
	if domain.ErrUnauthorized == nil {
		t.Error("ErrUnauthorized must be defined")
	}
	if domain.ErrInternal == nil {
		t.Error("ErrInternal must be defined")
	}
}

func TestErrorSentinels_UniqueMessages(t *testing.T) {
	t.Parallel()

	errorSentinels := []struct {
		name    string
		err     error
		errCode string
	}{
		{"ErrInvalidEventPayload", domain.ErrInvalidEventPayload, "RELAYBOX_INVALID_EVENT_PAYLOAD"},
		{"ErrDuplicateEvent", domain.ErrDuplicateEvent, "RELAYBOX_DUPLICATE_EVENT"},
		{"ErrSubscriptionNotFound", domain.ErrSubscriptionNotFound, "RELAYBOX_SUBSCRIPTION_NOT_FOUND"},
		{"ErrUnauthorized", domain.ErrUnauthorized, "RELAYBOX_UNAUTHORIZED"},
		{"ErrInternal", domain.ErrInternal, "RELAYBOX_INTERNAL_ERROR"},
	}

	// Check error.Error() returns a non-empty string.
	for _, se := range errorSentinels {
		if se.err.Error() == "" {
			t.Errorf("%s must have a non-empty Error() string", se.name)
		}
	}
}

func TestDomainError_ErrCodeAccessor(t *testing.T) {
	t.Parallel()

	err := &domain.DomainError{
		Code:    "RELAYBOX_TEST_ERROR",
		Message: "This is a test error",
	}

	if err.Code != "RELAYBOX_TEST_ERROR" {
		t.Errorf("expected Code=%q, got %q", "RELAYBOX_TEST_ERROR", err.Code)
	}
	if err.Message != "This is a test error" {
		t.Errorf("expected Message=%q, got %q", "This is a test error", err.Message)
	}
	if err.Error() != "RELAYBOX_TEST_ERROR: This is a test error" {
		t.Errorf("unexpected Error() string: %q", err.Error())
	}
}
