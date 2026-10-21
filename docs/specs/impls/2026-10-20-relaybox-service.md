# ImplSpec: Relaybox - Multi-Tenant Webhook Relay Service Implementation Plan

This document outlines the implementation plan for the Relaybox service, adhering to Clean Architecture principles, PR sizing limits, and all architectural invariants. It specifies contracts, interfaces, data structures, and architectural components without providing full implementation logic.

## 1. Core Principle: Contracts, Not Implementation
All sections below will only define struct definitions, interface method signatures, sentinel error constants, and godoc documentation. Full function implementation bodies or complete algorithms are explicitly excluded from this specification (RULE-IMPL-5.1).

## 2. Clean Architecture Layer Mapping
The service will follow a 7-layer Clean Architecture, ensuring inverted dependencies (RULE-ARCH-02) and domain purity (RULE-ARCH-01).

## 3. PR Group: PR-RELAYBOX-01 (Domain & Repository Interfaces + Migrations)
This PR group establishes the core domain entities, errors, and the contracts for data persistence, along with initial database schema migrations.

### Subtask: ST-01 - Core Domain Entities & Errors
- **Layer**: `domain`
- **Description**: Defines the foundational data structures and error types for the Relaybox service.
- **Target Files**: `pkg/relaybox/domain/event.go`, `pkg/relaybox/domain/subscription.go`, `pkg/relaybox/domain/outbox.go`, `pkg/relaybox/domain/errors.go`
- **Contracts/Structures**:
  - `Event` struct: `ID`, `TenantID`, `EventType`, `Payload` (JSONB), `DedupKey`, `CreatedAt`, `CreatedBy` (RULE-DATA-03).
  - `Subscription` struct: `ID`, `TenantID`, `EventType`, `TargetURL`, `CreatedAt`, `CreatedBy` (RULE-DATA-03).
  - `OutboxEntry` struct: `ID`, `EventID`, `SubscriptionID`, `Status`, `Attempts`, `NextAttemptAt`, `CreatedAt`.
  - `ErrInvalidEventPayload`, `ErrDuplicateEvent`, `ErrSubscriptionNotFound`, `ErrUnauthorized`, `ErrInternal` (sentinel errors, RULE-ARCH-03).

### Subtask: ST-02 - Persistence Interfaces & Migrations
- **Layer**: `repository` (interfaces), `cross_cutting` (migrations)
- **Description**: Defines the abstract contracts for interacting with the data store and provides the necessary database schema definitions.
- **Target Files**: `pkg/relaybox/repository/interfaces.go`, `migrations/YYYYMMDDHHMMSS_create_relaybox_tables.up.sql`, `migrations/YYYYMMDDHHMMSS_create_relaybox_tables.down.sql`
- **Contracts/Structures**:
  - `EventRepository` interface: `Save(ctx context.Context, event *domain.Event) (*domain.Event, error)`, `FindByDedupKey(ctx context.Context, tenantID string, dedupKey string) (*domain.Event, error)`.
  - `SubscriptionRepository` interface: `Save(ctx context.Context, sub *domain.Subscription) (*domain.Subscription, error)`, `FindByID(ctx context.Context, tenantID, id string) (*domain.Subscription, error)`, `FindByTenantID(ctx context.Context, tenantID string) ([]domain.Subscription, error)`, `Delete(ctx context.Context, tenantID, id string) error`.
  - `OutboxRepository` interface: `Save(ctx context.Context, entry *domain.OutboxEntry) (*domain.OutboxEntry, error)`, `FindPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error)`, `UpdateStatus(ctx context.Context, id string, status domain.OutboxStatus) error`.
- **Database Schema Migration Plan (Expand/Contract)** (RULE-IMPL-5.3):
  - `migrations/YYYYMMDDHHMMSS_create_relaybox_tables.up.sql`:
    - Create `events` table with `id` (PK), `tenant_id`, `event_type`, `payload` (JSONB), `dedup_key`, `created_at` (UTC), `created_by`. Add `UNIQUE (tenant_id, dedup_key)` constraint (FR-ING-03, RULE-DATA-02, RULE-DATA-03).
    - Create `subscriptions` table with `id` (PK), `tenant_id`, `event_type`, `target_url`, `created_at` (UTC), `created_by` (RULE-DATA-03).
    - Create `outbox` table with `id` (PK), `event_id`, `subscription_id`, `status` (ENUM/TEXT), `attempts`, `next_attempt_at` (UTC), `created_at` (UTC).
  - `migrations/YYYYMMDDHHMMSS_create_relaybox_tables.down.sql`: Corresponding drop table statements.

## 4. PR Group: PR-RELAYBOX-02 (PostgreSQL Repository Implementation)
This PR group provides the concrete PostgreSQL implementations for the repository interfaces.

### Subtask: ST-03 - PostgreSQL Repository Implementation
- **Layer**: `adapter` (persistence)
- **Description**: Implements the actual data access logic using PostgreSQL for events, subscriptions, and outbox entries.
- **Target Files**: `pkg/relaybox/adapter/postgresql/event_repository.go`, `pkg/relaybox/adapter/postgresql/subscription_repository.go`, `pkg/relaybox/adapter/postgresql/outbox_repository.go`
- **Contracts/Structures**:
  - `pgEventRepository` struct: Implements `domain.EventRepository`.
  - `pgSubscriptionRepository` struct: Implements `domain.SubscriptionRepository`.
  - `pgOutboxRepository` struct: Implements `domain.OutboxRepository`.
  - Methods will accept `ctx context.Context` (RULE-RES-01).
  - Transactional wrapper for `EventRepository.Save` to ensure `events` and `outbox` writes are atomic (FR-ING-02, RULE-DATA-01).

## 5. PR Group: PR-RELAYBOX-03 (Service Layer)
This PR group implements the core business logic services for event ingestion and subscription management.

### Subtask: ST-04 - Ingestion Service
- **Layer**: `service`
- **Description**: Orchestrates event ingestion, including validation, persistence, and outbox enqueueing, ensuring idempotency and transactionality.
- **Target Files**: `pkg/relaybox/service/ingestion.go`, `pkg/relaybox/service/dtos.go`
- **Contracts/Structures**:
  - `IngestionService` interface: `IngestEvent(ctx context.Context, tenantID string, req IngestEventRequest) (*IngestEventResponse, error)`.
  - `IngestEventRequest` struct: `EventType`, `Payload`, `DedupKey`.
  - `IngestEventResponse` struct: `EventID`.
  - Service will inject `domain.EventRepository` and `domain.OutboxRepository` interfaces.

### Subtask: ST-05 - Subscription Service
- **Layer**: `service`
- **Description**: Provides CRUD operations for subscriptions, enforcing tenant-level access control.
- **Target Files**: `pkg/relaybox/service/subscription.go`
- **Contracts/Structures**:
  - `SubscriptionService` interface: `CreateSubscription(ctx context.Context, tenantID string, req CreateSubscriptionRequest) (*Subscription, error)`, `GetSubscription(ctx context.Context, tenantID, id string) (*Subscription, error)`, `ListSubscriptions(ctx context.Context, tenantID string) ([]Subscription, error)`, `DeleteSubscription(ctx context.Context, tenantID, id string) error`.
  - `CreateSubscriptionRequest` struct: `EventType`, `TargetURL`.
  - Service will inject `domain.SubscriptionRepository` interface.

## 6. PR Group: PR-RELAYBOX-04 (HTTP Ingestion Handler)
This PR group implements the HTTP API endpoint for event ingestion.

### Subtask: ST-06 - HTTP Ingestion Handler
- **Layer**: `transport` (handler)
- **Description**: Exposes the `POST /v1/events` endpoint, handling request parsing, tenant context extraction, and error mapping.
- **Target Files**: `pkg/relaybox/handler/ingestion_handler.go`
- **Contracts/Structures**:
  - `IngestionHandler` struct: Depends on `service.IngestionService`.
  - `HandleIngestEvent(w http.ResponseWriter, r *http.Request)` function: Parses request body into `service.IngestEventRequest`, extracts `tenantID` from request context (RULE-SEC-01), calls `IngestionService.IngestEvent`, and writes appropriate HTTP responses, mapping domain errors to HTTP status codes (RULE-ARCH-03).

## 7. PR Group: PR-RELAYBOX-05 (HTTP Subscription Handlers)
This PR group implements the HTTP API endpoints for subscription management.

### Subtask: ST-07 - HTTP Subscription Handlers
- **Layer**: `transport` (handler)
- **Description**: Exposes CRUD HTTP endpoints for `/v1/subscriptions`, handling request/response mapping and tenant context.
- **Target Files**: `pkg/relaybox/handler/subscription_handler.go`
- **Contracts/Structures**:
  - `SubscriptionHandler` struct: Depends on `service.SubscriptionService`.
  - `HandleCreateSubscription(w http.ResponseWriter, r *http.Request)`, `HandleGetSubscription(w http.ResponseWriter, r *http.Request)`, `HandleListSubscriptions(w http.ResponseWriter, r *http.Request)`, `HandleDeleteSubscription(w http.ResponseWriter, r *http.Request)`: Functions for respective CRUD operations, extracting `tenantID` from context (RULE-SEC-01), parsing parameters, calling `SubscriptionService`, and mapping errors to HTTP responses.

## 8. PR Group: PR-RELAYBOX-06 (Delivery Dispatcher)
This PR group implements the background worker responsible for event delivery.

### Subtask: ST-08 - Delivery Dispatcher & Webhook Client
- **Layer**: `service` (dispatcher), `adapter` (HTTP client)
- **Description**: Implements the background service that reads from the outbox, dispatches events to external webhooks, and manages retry/dead-letter logic.
- **Target Files**: `pkg/relaybox/service/delivery_dispatcher.go`, `pkg/relaybox/adapter/http_client/webhook_client.go`
- **Contracts/Structures**:
  - `DeliveryDispatcher` struct: Depends on `domain.OutboxRepository`, `domain.SubscriptionRepository`, and `WebhookClient` interface.
  - `Start(ctx context.Context)`: Method to start the dispatcher, using a bounded worker pool (`sync.WaitGroup` or similar, RULE-RES-02) and respecting `ctx.Done()` for graceful shutdown.
  - `WebhookClient` interface: `SendWebhook(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error)`.
  - `httpWebhookClient` struct: Implements `WebhookClient`, setting a timeout, propagating OpenTelemetry trace context via headers (FR-DEL-01, RULE-OBS-02), and immediately closing `resp.Body` (RULE-RES-03).
  - Logic for exponential backoff on 5xx and dead-lettering on 4xx (FR-DEL-02).
  - Ensures `X-Relaybox-Delivery-Id` header is stable per attempt (FR-DEL-04, RULE-DATA-02).

## 9. PR Group: PR-RELAYBOX-07 (Observability & Health)
This PR group integrates cross-cutting observability concerns and health endpoints.

### Subtask: ST-09 - Observability & Health Endpoints
- **Layer**: `transport` (health), `cross_cutting` (metrics, logging), `config` (wiring)
- **Description**: Implements health check endpoints, structured logging, and Prometheus metrics for the Relaybox service.
- **Target Files**: `pkg/relaybox/handler/health_handler.go`, `pkg/relaybox/metrics/metrics.go`, `pkg/relaybox/log/logger.go`, `cmd/relaybox/main.go`
- **Contracts/Structures**:
  - `HealthHandler` struct: Depends on a `DBChecker` interface.
  - `HandleLivez(w http.ResponseWriter, r *http.Request)`, `HandleReadyz(w http.ResponseWriter, r *http.Request)`: Implement health endpoints (FR-OBS-01, RULE-OBS-04).
  - `DBChecker` interface: `CheckDB(ctx context.Context) error`.
  - `pkg/relaybox/log/logger.go`: Global `slog.Logger` instance, configured for JSON output and `context.Context` binding (FR-OBS-02, RULE-OBS-01).
  - `pkg/relaybox/metrics/metrics.go`: Prometheus `Counter` and `Histogram` metrics for HTTP request duration, delivery outcomes (e.g., `relaybox_http_request_duration_seconds`, `relaybox_delivery_outcome_total`). Labels like `status="delivered|failed|deadletter"` (FR-OBS-02, RULE-OBS-03). Explicitly avoid high-cardinality labels (RULE-OBS-03).
  - `cmd/relaybox/main.go`: Main application entry point for wiring up handlers, services, repositories, logger, and metrics.

## 10. System Observability & Telemetry Blueprint (RULE-OBS)
- **Structured Logging (`slog`)**: All log statements will use `slog` with `slog.InfoContext(ctx, ...)` for context propagation. Standard attributes will include `trace_id`, `span_id`, `component`, `error` (RULE-OBS-01).
- **Prometheus Metrics**: Defined for key operations, ensuring bounded cardinality:
  - `http_request_duration_seconds` (histogram): records HTTP request latency, with labels like `path`, `method`, `status_code`.
  - `event_ingestion_total` (counter): total events ingested, with labels `status="success|error"`.
  - `event_delivery_total` (counter): total delivery attempts, with labels `outcome="delivered|retried|deadletter|failed"`.
  - `delivery_worker_pool_size` (gauge): current size of the dispatcher worker pool.
- **Distributed Tracing**: OpenTelemetry `trace_id` and `span_id` will be propagated across all internal service calls and outbound HTTP requests (FR-DEL-01, RULE-OBS-02).