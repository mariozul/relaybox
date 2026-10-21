# ImplSpec: Relaybox — Multi-Tenant Webhook Relay Service Implementation Plan

This document details the implementation plan for the Relaybox service, adhering to Clean Architecture principles and the "Contracts, Not Implementation" rule.

## 1. Core Principles

*   **Clean Architecture:** Strict separation of concerns into Domain, Repository, Service, and Transport/Adapter layers. Dependencies flow inward.
*   **Contracts First:** Define interfaces and method signatures before concrete implementations.
*   **Test-Driven Development:** Implementation will follow the TestSpec (YYYY-MM-DD-relaybox-testspec.md), ensuring full test coverage.
*   **Single Responsibility PRs:** Each PR will focus on a distinct, small, and cohesive set of changes, adhering to the 250 LOC declared estimate ceiling (`RULE-PLAN-004`).

## 2. Architectural Layers & Components

### 2.1. Foundation: Core Types, Errors, Context, and Configuration (`ST-01`)

*   **`pkg/relaybox/config.go`**: Defines the application-wide configuration struct (e.g., database connection string, HTTP server address, dispatcher settings).
    ```go
    package relaybox

    type Config struct {
        DatabaseURL string `env:"DATABASE_URL,required"`
        ListenAddr  string `env:"LISTEN_ADDR,default=:8080"`
        // ... dispatcher configs ...
    }
    ```
*   **`pkg/relaybox/errors.go`**: Defines common application-wide error sentinels.
    ```go
    package relaybox

    import "errors"

    var (
        ErrNotFound          = errors.New("not found")
        ErrConflict          = errors.New("conflict")
        ErrInvalidArgument   = errors.New("invalid argument")
        ErrUnauthorized      = errors.New("unauthorized")
        ErrInternal          = errors.New("internal server error")
        ErrTenantMismatch    = errors.New("tenant mismatch")
        ErrDedupKeyViolation = errors.New("deduplication key violation")
    )

    // ErrorCatalog mapping domain errors to HTTP/gRPC status codes (per ReqSpec §3.3)
    // type ErrorCatalog map[error]ProtocolError ...
    ```
*   **`pkg/relaybox/context.go`**: Defines functions for extracting/injecting `tenant_id` and OpenTelemetry trace context.
    ```go
    package relaybox

    import "context"

    type TenantID string

    func ContextWithTenantID(ctx context.Context, tenantID TenantID) context.Context
    func TenantIDFromContext(ctx context.Context) (TenantID, error)
    ```
*   **`pkg/domain/types.go`**: Common type definitions like `UUID`, `EventType`.
    ```go
    package domain

    type UUID string // Represents a unique identifier
    type EventType string
    ```

### 2.2. Domain: Entities, Repository Interfaces (`ST-02`)

*   **`pkg/domain/event.go`**: Defines the `Event` entity and associated business logic (e.g., validation).
    ```go
    package domain

    import (
        "time"
        "encoding/json"
    )

    type Event struct {
        ID        UUID
        TenantID  relaybox.TenantID
        EventType EventType
        Payload   json.RawMessage
        DedupKey  string // Optional, for idempotency
        CreatedAt time.Time
        CreatedBy string
        UpdatedAt time.Time
        UpdatedBy string
    }

    // Methods for validation, etc.
    func (e *Event) Validate() error
    ```
*   **`pkg/domain/subscription.go`**: Defines the `Subscription` entity.
    ```go
    package domain

    import (
        "time"
    )

    type Subscription struct {
        ID        UUID
        TenantID  relaybox.TenantID
        EventType EventType
        TargetURL string
        CreatedAt time.Time
        CreatedBy string
        UpdatedAt time.Time
        UpdatedBy string
    }

    // Methods for validation, etc.
    func (s *Subscription) Validate() error
    ```
*   **`pkg/domain/outbox.go`**: Defines the `OutboxEntry` entity.
    ```go
    package domain

    import "time"

    type OutboxStatus string

    const (
        OutboxStatusPending    OutboxStatus = "PENDING"
        OutboxStatusDelivered  OutboxStatus = "DELIVERED"
        OutboxStatusFailed     OutboxStatus = "FAILED"
        OutboxStatusDeadLetter OutboxStatus = "DEAD_LETTERED"
    )

    type OutboxEntry struct {
        ID               UUID
        EventID          UUID
        SubscriptionID   UUID
        Status           OutboxStatus
        Attempts         int
        NextAttemptAt    time.Time
        LastAttemptAt    time.Time
        ErrorMessage     string
        DeliveryTraceID  string // For FR-DEL-04, X-Relaybox-Delivery-Id
        CreatedAt        time.Time
        CreatedBy        string
        UpdatedAt        time.Time
        UpdatedBy        string
    }
    ```
*   **`pkg/repository/event.go`**: Interface for Event persistence.
    ```go
    package repository

    import (
        "context"
        "github.com/mariozul/relaybox/pkg/domain"
        "github.com/mariozul/relaybox/pkg/relaybox"
    )

    type EventRepository interface {
        CreateEvent(ctx context.Context, event *domain.Event) (*domain.Event, error)
        FindEventByDedupKey(ctx context.Context, tenantID relaybox.TenantID, dedupKey string) (*domain.Event, error)
        // Add more methods as needed for domain operations
    }
    ```
*   **`pkg/repository/subscription.go`**: Interface for Subscription persistence.
    ```go
    package repository

    import (
        "context"
        "github.com/mariozul/relaybox/pkg/domain"
        "github.com/mariozul/relaybox/pkg/relaybox"
    )

    type SubscriptionRepository interface {
        CreateSubscription(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error)
        GetSubscriptionByID(ctx context.Context, tenantID relaybox.TenantID, id domain.UUID) (*domain.Subscription, error)
        ListSubscriptions(ctx context.Context, tenantID relaybox.TenantID, eventType domain.EventType) ([]domain.Subscription, error)
        DeleteSubscription(ctx context.Context, tenantID relaybox.TenantID, id domain.UUID) error
    }
    ```
*   **`pkg/repository/outbox.go`**: Interface for Outbox persistence.
    ```go
    package repository

    import (
        "context"
        "github.com/mariozul/relaybox/pkg/domain"
        "time"
    )

    type OutboxRepository interface {
        CreateOutboxEntry(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error)
        FindPendingEntries(ctx context.Context, limit int) ([]domain.OutboxEntry, error)
        UpdateOutboxEntryStatus(ctx context.Context, id domain.UUID, status domain.OutboxStatus, attempts int, nextAttemptAt time.Time, errorMessage string) error
        // Add more methods for transactional updates with event creation
    }
    ```

### 2.3. Persistence: PostgreSQL Migrations & Repository Implementations (`ST-03`)

*   **`migrations/YYYYMMDDHHMMSS_create_events_table.up.sql` / `down.sql`**: SQL scripts for `events` table.
    -   Includes `tenant_id`, `dedup_key`, audit columns (`created_at`, `created_by`, `updated_at`, `updated_by`).
    -   Unique constraint on `(tenant_id, dedup_key)`.
*   **`migrations/YYYYMMDDHHMMSS_create_subscriptions_table.up.sql` / `down.sql`**: SQL scripts for `subscriptions` table.
    -   Includes `tenant_id`, `event_type`, `target_url`, audit columns.
*   **`migrations/YYYYMMDDHHMMSS_create_outbox_table.up.sql` / `down.sql`**: SQL scripts for `outbox` table.
    -   Includes FKs to `events` and `subscriptions`, `status`, `attempts`, `next_attempt_at`, `delivery_trace_id`, audit columns.
*   **`pkg/infrastructure/persistence/pg_event_repository.go`**: Concrete implementation of `EventRepository` using `pgx` or `database/sql`.
    -   Methods: `CreateEvent`, `FindEventByDedupKey`.
    -   `CreateEvent` will implement the `RULE-DATA-01` transactional outbox pattern.
*   **`pkg/infrastructure/persistence/pg_subscription_repository.go`**: Concrete implementation of `SubscriptionRepository`.
    -   Methods: `CreateSubscription`, `GetSubscriptionByID`, `ListSubscriptions`, `DeleteSubscription`.
    -   Includes tenant filtering per `RULE-SEC-01`.
*   **`pkg/infrastructure/persistence/pg_outbox_repository.go`**: Concrete implementation of `OutboxRepository`.
    -   Methods: `CreateOutboxEntry`, `FindPendingEntries`, `UpdateOutboxEntryStatus`.
    -   Will also include a method for atomic creation of event + outbox entry within a single transaction.

### 2.4. Ingestion Service & HTTP Handler (`ST-04`)

*   **`pkg/service/ingestion.go`**: Application service for event ingestion.
    ```go
    package service

    import (
        "context"
        "github.com/mariozul/relaybox/pkg/domain"
        "github.com/mariozul/relaybox/pkg/repository"
        "github.com/mariozul/relaybox/pkg/relaybox"
    )

    type IngestionService struct {
        EventRepo repository.EventRepository
        OutboxRepo repository.OutboxRepository
        // ... other dependencies like transaction manager ...
    }

    func (s *IngestionService) IngestEvent(ctx context.Context, event *domain.Event, dedupKey string) (*domain.Event, error)
    ```
*   **`pkg/handler/dto/event_dto.go`**: DTOs for event ingestion.
    ```go
    package dto

    import "encoding/json"

    type IngestEventRequest struct {
        EventType string          `json:"event_type"`
        Payload   json.RawMessage `json:"payload"`
        DedupKey  string          `json:"dedup_key,omitempty"`
    }

    type EventResponse struct {
        ID        string    `json:"id"`
        EventType string    `json:"event_type"`
        // ... other fields ...
    }
    ```
*   **`pkg/handler/event_handler.go`**: HTTP handler for `POST /v1/events`.
    ```go
    package handler

    import (
        "net/http"
        "github.com/gin-gonic/gin" // Example HTTP framework
        "github.com/mariozul/relaybox/pkg/service"
        "github.com/mariozul/relaybox/pkg/relaybox"
    )

    type EventHandler struct {
        IngestionService *service.IngestionService
        // ... other dependencies ...
    }

    func (h *EventHandler) IngestEventHandler(c *gin.Context)
    ```

### 2.5. Subscription Service & HTTP Handler (`ST-05`)

*   **`pkg/service/subscription.go`**: Application service for subscription management.
    ```go
    package service

    import (
        "context"
        "github.com/mariozul/relaybox/pkg/domain"
        "github.com/mariozul/relaybox/pkg/repository"
        "github.com/mariozul/relaybox/pkg/relaybox"
    )

    type SubscriptionService struct {
        SubscriptionRepo repository.SubscriptionRepository
    }

    func (s *SubscriptionService) CreateSubscription(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error)
    func (s *SubscriptionService) GetSubscription(ctx context.Context, tenantID relaybox.TenantID, id domain.UUID) (*domain.Subscription, error)
    func (s *SubscriptionService) ListSubscriptions(ctx context.Context, tenantID relaybox.TenantID, eventType domain.EventType) ([]domain.Subscription, error)
    func (s *SubscriptionService) DeleteSubscription(ctx context.Context, tenantID relaybox.TenantID, id domain.UUID) error
    ```
*   **`pkg/handler/dto/subscription_dto.go`**: DTOs for subscription management.
    ```go
    package dto

    type CreateSubscriptionRequest struct {
        EventType string `json:"event_type"`
        TargetURL string `json:"target_url"`
    }

    type SubscriptionResponse struct {
        ID        string `json:"id"`
        EventType string `json:"event_type"`
        TargetURL string `json:"target_url"`
        // ...
    }
    ```
*   **`pkg/handler/subscription_handler.go`**: HTTP handler for `CRUD /v1/subscriptions`.
    ```go
    package handler

    import (
        "net/http"
        "github.com/gin-gonic/gin"
        "github.com/mariozul/relaybox/pkg/service"
        "github.com/mariozul/relaybox/pkg/relaybox"
    )

    type SubscriptionHandler struct {
        SubscriptionService *service.SubscriptionService
    }

    func (h *SubscriptionHandler) CreateSubscriptionHandler(c *gin.Context)
    func (h *SubscriptionHandler) GetSubscriptionHandler(c *gin.Context)
    func (h *SubscriptionHandler) ListSubscriptionsHandler(c *gin.Context)
    func (h *SubscriptionHandler) DeleteSubscriptionHandler(c *gin.Context)
    ```

### 2.6. Delivery Dispatcher (Background Worker Core Logic) (`ST-06`)

*   **`pkg/worker/dispatcher.go`**: The core background worker.
    ```go
    package worker

    import (
        "context"
        "github.com/mariozul/relaybox/pkg/repository"
        "github.com/mariozul/relaybox/pkg/infrastructure/httpclient"
        "github.com/mariozul/relaybox/pkg/observability"
        "go.opentelemetry.io/otel/trace"
        "time"
    )

    type Dispatcher struct {
        OutboxRepo          repository.OutboxRepository
        SubscriptionRepo    repository.SubscriptionRepository
        HTTPClient          httpclient.Client
        Logger              *observability.Logger
        Tracer              trace.Tracer
        Config              *relaybox.Config
        WorkerPool          chan struct{} // Bounded worker pool (RULE-RES-02)
        // ... metrics, retry strategy ...
    }

    // NewDispatcher creates and returns a configured Dispatcher.
    func NewDispatcher( /* deps */ ) *Dispatcher

    // Run starts the dispatcher's main loop. It must respect context cancellation.
    func (d *Dispatcher) Run(ctx context.Context) error

    // dispatchEntry processes a single outbox entry.
    func (d *Dispatcher) dispatchEntry(ctx context.Context, entry *domain.OutboxEntry) error
    ```
*   **`pkg/infrastructure/httpclient/client.go`**: An abstract HTTP client for outbound calls, allowing easy mocking.
    ```go
    package httpclient

    import (
        "context"
        "net/http"
    )

    type Client interface {
        Post(ctx context.Context, url string, headers map[string]string, body []byte) (*http.Response, error)
    }

    type defaultClient struct { /* http.Client instance */ }

    func NewDefaultClient(timeout time.Duration) Client
    ```
    -   `defaultClient` will ensure `resp.Body.Close()` is called (`RULE-RES-03`).

### 2.7. Observability & Health Endpoints (`ST-07`)

*   **`pkg/handler/health_handler.go`**: HTTP handlers for `/livez` and `/readyz`.
    ```go
    package handler

    import (
        "net/http"
        "github.com/gin-gonic/gin"
        "github.com/mariozul/relaybox/pkg/repository" // For DB health check
    )

    type HealthHandler struct {
        DBPing func(ctx context.Context) error // Abstract DB ping
    }

    func (h *HealthHandler) LivezHandler(c *gin.Context)
    func (h *h HealthHandler) ReadyzHandler(c *gin.Context) // Implements FR-OBS-01, AC-06, RULE-OBS-04
    ```
*   **`pkg/observability/metrics.go`**: Prometheus metrics setup.
    ```go
    package observability

    import (
        "github.com/prometheus/client_golang/prometheus"
        "github.com/prometheus/client_golang/prometheus/promauto"
    )

    var (
        IngestionRequestsTotal = promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "relaybox_ingestion_requests_total",
                Help: "Total number of ingestion requests",
            },
            []string{"status"}, // "success", "failure"
        )
        DeliveryDurationSeconds = promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "relaybox_delivery_duration_seconds",
                Help: "Duration of event deliveries",
                Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 5, 10},
            },
            []string{"event_type", "status"}, // status="delivered|failed|deadletter" (bounded cardinality per RULE-OBS-03)
        )
        // ... other metrics ...
    )

    // InitMetrics initializes and registers all application metrics.
    func InitMetrics()
    ```
*   **`pkg/observability/logger.go`**: Structured logging initialization (`slog`).
    ```go
    package observability

    import (
        "log/slog"
        "context"
        "os"
    )

    var globalLogger *slog.Logger

    // InitLogger initializes a structured JSON logger (RULE-OBS-01).
    func InitLogger(level slog.Level)

    // NewContextWithLogger creates a new context with logger.
    func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context

    // LoggerFromContext retrieves logger from context.
    func LoggerFromContext(ctx context.Context) *slog.Logger

    // LogContextInfo logs info with context (propagates trace_id, span_id).
    func LogContextInfo(ctx context.Context, msg string, args ...any)
    ```

## 3. Database Schema Migration Plan (Expand/Contract)

All migrations will follow the Expand/Contract pattern where applicable, or simple additions for new tables.

*   **Phase 1 (Initial PR-RELAYBOX-02):**
    *   `create_events_table.up.sql`: Create `events` table with all specified columns, including audit and unique constraint.
    *   `create_subscriptions_table.up.sql`: Create `subscriptions` table with all specified columns, including audit.
    *   `create_outbox_table.up.sql`: Create `outbox` table with all specified columns, including FKs and audit.
*   **Down scripts:** Corresponding `down.sql` scripts will drop tables.

## 4. Error Catalog & Protocol Mapping

| Error Sentinel | Error Code String | HTTP Status | Description / Client Message |
| :----------------------------- | :------------------------ | :------------------ | :--------------------------------------------------------------- |
| `relaybox.ErrNotFound` | `RELAYBOX_NOT_FOUND` | 404 Not Found | Resource not found. |
| `relaybox.ErrConflict` | `RELAYBOX_CONFLICT` | 409 Conflict | A resource with the specified identifier already exists. |
| `relaybox.ErrInvalidArgument` | `RELAYBOX_INVALID_ARGUMENT` | 400 Bad Request | The provided input is invalid. |
| `relaybox.ErrUnauthorized` | `RELAYBOX_UNAUTHORIZED` | 401 Unauthorized | Authentication required. |
| `relaybox.ErrTenantMismatch` | `RELAYBOX_FORBIDDEN` | 403 Forbidden | Access to this resource is forbidden for your tenant. |
| `relaybox.ErrDedupKeyViolation`| `RELAYBOX_DEDUP_KEY_VIOLATION` | 409 Conflict | An event with this deduplication key already exists for this tenant. |
| `relaybox.ErrInternal` | `RELAYBOX_INTERNAL_ERROR` | 500 Internal Server | An unexpected internal error occurred. |