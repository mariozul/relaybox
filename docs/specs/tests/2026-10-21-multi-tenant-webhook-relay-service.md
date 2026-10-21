# TestSpec: Relaybox — Multi-Tenant Webhook Relay Service Tests (YYYY-MM-DD)

This document outlines the test scenarios required for the Relaybox service, ensuring 100% RTM traceability to the functional requirements (FRs) and acceptance criteria (ACs) defined in the ReqSpec. Each test case includes a "Violation Signal" describing how the test would fail if a specific invariant or requirement is not met.

## 1. Traceability Matrix

| Test ID | Test Scenario | Mapping (FR & AC) | Category | Target Test File(s) |
| :-------------- | :--------------------------------------------------------------------------- | :-------------------------------------------------- | :---------- | :----------------------------------------------------------------- |
| `TC-ING-01-INT` | Successful event ingestion and idempotency for same `(tenant_id, dedup_key)` | `FR-ING-01`, `FR-ING-03`, `AC-01` | Integration | `pkg/handler/event_handler_test.go`, `pkg/service/ingestion_test.go` |
| `TC-ING-02-INT` | Event persistence and transactional outbox durability across restarts | `FR-ING-02`, `AC-02` | Integration | `pkg/service/ingestion_test.go` |
| `TC-SUB-01-INT` | CRUD operations for subscriptions (create, get, delete) | `FR-SUB-01` | Integration | `pkg/handler/subscription_handler_test.go` |
| `TC-SUB-02-INT` | Multi-tenant isolation for subscriptions | `FR-SUB-02`, `AC-04`, `RULE-SEC-01` | Integration | `pkg/handler/subscription_handler_test.go` |
| `TC-DEL-01-UNIT` | Dispatcher picks up outbox events and dispatches HTTP requests | `FR-DEL-01`, `RULE-RES-01`, `RULE-OBS-02` | Unit | `pkg/worker/dispatcher_test.go` |
| `TC-DEL-02-INT` | At-least-once delivery with retries, backoff, and dead-lettering | `FR-DEL-02`, `AC-03`, `RULE-RES-03` | Integration | `pkg/worker/dispatcher_test.go` |
| `TC-DEL-03-RACE` | Bounded worker pool for concurrent delivery and graceful shutdown | `FR-DEL-03`, `AC-05`, `RULE-RES-02` | Concurrency | `pkg/worker/dispatcher_test.go` |
| `TC-DEL-04-UNIT` | Idempotent delivery attempts (`X-Relaybox-Delivery-Id` header) | `FR-DEL-04`, `RULE-DATA-02` | Unit | `pkg/worker/dispatcher_test.go` |
| `TC-OBS-01-INT` | `/livez` and `/readyz` endpoints with DB reachability check | `FR-OBS-01`, `AC-06`, `RULE-OBS-04` | Integration | `pkg/handler/health_handler_test.go` |
| `TC-OBS-02-UNIT` | Structured logging and Prometheus metrics with bounded cardinality | `FR-OBS-02`, `RULE-OBS-01`, `RULE-OBS-03` | Unit | `pkg/observability/metrics_test.go`, `pkg/observability/logger_test.go` |
| `TC-DATA-03-UNIT` | Mandatory audit columns for data models | `RULE-DATA-03` | Unit | `pkg/domain/event_test.go`, `pkg/domain/subscription_test.go` |

## 2. Test Case Details & Violation Signals

### `TC-ING-01-INT`: Event Ingestion & Idempotency
- **Test Scenario:**
  1. Authenticate as `tenant_A`.
  2. Send a `POST /v1/events` request with a unique `dedup_key`. Verify a new event is created and an outbox entry is made.
  3. Send the exact same `POST /v1/events` request from `tenant_A`. Verify no new event is created, and the original event ID is returned.
  4. Send a `POST /v1/events` request with the same `dedup_key` but from `tenant_B`. Verify a new event is created for `tenant_B`.
- **Mapping:** `FR-ING-01`, `FR-ING-03`, `AC-01`
- **Violation Signal:** If the second request from `tenant_A` creates a new event, returns an error other than "already exists" (or similar idempotent success), or returns a different event ID. If `tenant_id` is parsed from the request body.

### `TC-ING-02-INT`: Transactional Persistence & Durability
- **Test Scenario:**
  1. Start the Relaybox service and a mock HTTP subscriber.
  2. Send a `POST /v1/events` request.
  3. Immediately kill the Relaybox process *after* the event is persisted to the `events` table but *before* its corresponding entry is confirmed in the `outbox` table (simulated crash between transactional steps).
  4. Restart the Relaybox service.
  5. Verify that the event is eventually delivered to the mock subscriber, proving the transactional outbox's durability.
- **Mapping:** `FR-ING-02`, `AC-02`
- **Violation Signal:** If the event is lost and never delivered after the restart, indicating a non-atomic persistence of event and outbox entry (violates `RULE-DATA-01`, `RULE-EVT-02`).

### `TC-SUB-01-INT`: Subscription CRUD
- **Test Scenario:**
  1. Authenticate as `tenant_A`.
  2. `POST /v1/subscriptions` to create a subscription. Verify 201 Created and response contains the new subscription ID.
  3. `GET /v1/subscriptions/{id}` for the created ID. Verify 200 OK and correct details.
  4. `GET /v1/subscriptions` to list all. Verify the new subscription is present.
  5. `DELETE /v1/subscriptions/{id}`. Verify 204 No Content.
  6. `GET /v1/subscriptions/{id}` again. Verify 404 Not Found.
- **Mapping:** `FR-SUB-01`
- **Violation Signal:** Any CRUD operation fails or returns incorrect status/data.

### `TC-SUB-02-INT`: Multi-Tenant Subscription Isolation
- **Test Scenario:**
  1. Authenticate as `tenant_A` and create `sub_A`.
  2. Authenticate as `tenant_B` and create `sub_B`.
  3. As `tenant_A`, attempt `GET /v1/subscriptions/{id_of_sub_B}`. Verify 403 Forbidden or 404 Not Found.
  4. As `tenant_B`, attempt `DELETE /v1/subscriptions/{id_of_sub_A}`. Verify 403 Forbidden or 404 Not Found.
- **Mapping:** `FR-SUB-02`, `AC-04`, `RULE-SEC-01`
- **Violation Signal:** If `tenant_A` can successfully retrieve or modify `sub_B`'s details, `RULE-SEC-01` is violated.

### `TC-DEL-01-UNIT`: Dispatcher Outbox Processing & Context Propagation
- **Test Scenario:**
  1. Mock `OutboxRepository` to return pending events.
  2. Mock `HTTPClient` to capture outgoing requests.
  3. Create a `context.Context` with OpenTelemetry trace injected.
  4. Initialize the dispatcher with the mocked dependencies.
  5. Invoke a single dispatch cycle.
  6. Verify the `HTTPClient` received a request with the correct URL, method, and payload, and that the `X-Relaybox-Delivery-Id` header is present. Verify that the outgoing HTTP request context contains the OpenTelemetry trace.
- **Mapping:** `FR-DEL-01`, `RULE-RES-01`, `RULE-OBS-02`
- **Violation Signal:** If the dispatcher fails to construct the correct HTTP request, or if the outbound HTTP call does not propagate `ctx` and OpenTelemetry trace metadata, or if `context.Background()` is detected.

### `TC-DEL-02-INT`: At-Least-Once Delivery with Retry & Dead-Letter
- **Test Scenario (Retry):**
  1. Setup mock subscriber endpoint to return `500 Internal Server Error` once, then `200 OK`.
  2. Ingest an event and a subscription.
  3. Start the dispatcher.
  4. Verify the event is delivered after the retry, and the outbox status is `DELIVERED`.
- **Test Scenario (Dead-Letter):**
  1. Setup mock subscriber endpoint to consistently return `400 Bad Request` for N+1 attempts.
  2. Ingest an event and a subscription.
  3. Start the dispatcher.
  4. Verify the event is attempted N times, then its outbox status changes to `DEAD_LETTERED`.
- **Mapping:** `FR-DEL-02`, `AC-03`, `RULE-RES-03`
- **Violation Signal:** If retries do not occur for transient failures, or permanent failures are retried indefinitely, or `resp.Body.Close()` is not called for outbound HTTP requests.

### `TC-DEL-03-RACE`: Bounded Concurrency & Race Detection
- **Test Scenario:**
  1. Ingest 100 events for different subscribers.
  2. Configure the dispatcher with a small, bounded worker pool (e.g., 5 workers).
  3. Start the dispatcher concurrently with another load-generating process (e.g., event ingestion).
  4. Run `go test -race` on the entire service.
  5. Verify all events are eventually delivered, and no race conditions are reported.
- **Mapping:** `FR-DEL-03`, `AC-05`, `RULE-RES-02`
- **Violation Signal:** If the `go test -race` flag reports any data races during concurrent operation, or if the dispatcher spawns an unbounded number of goroutines that consume excessive resources.

### `TC-DEL-04-UNIT`: Idempotent Delivery ID Header
- **Test Scenario:**
  1. Mock `OutboxRepository` to return a pending event.
  2. Mock `HTTPClient` to capture outgoing requests.
  3. Invoke a dispatch attempt for an event.
  4. Capture the `X-Relaybox-Delivery-Id` header.
  5. Simulate a retry by re-invoking the dispatch for the *same* outbox entry.
  6. Capture the `X-Relaybox-Delivery-Id` header again.
  7. Verify both captured headers are identical.
- **Mapping:** `FR-DEL-04`, `RULE-DATA-02`
- **Violation Signal:** If the `X-Relaybox-Delivery-Id` header changes between retry attempts for the same delivery.

### `TC-OBS-01-INT`: Health Endpoints
- **Test Scenario (Livez):**
  1. Send `GET /livez`. Verify `200 OK`.
- **Test Scenario (Readyz - DB Up):**
  1. Ensure the database is reachable. Send `GET /readyz`. Verify `200 OK`.
- **Test Scenario (Readyz - DB Down):**
  1. Shut down the database. Send `GET /readyz`. Verify `503 Service Unavailable` (`AC-06`).
- **Mapping:** `FR-OBS-01`, `AC-06`, `RULE-OBS-04`
- **Violation Signal:** If `/livez` or `/readyz` return incorrect status codes, or if `/readyz` returns `200 OK` when the database is unreachable (`AC-06`).

### `TC-OBS-02-UNIT`: Structured Logging & Bounded Metrics
- **Test Scenario (Logging):**
  1. Configure `slog` to capture logs in memory.
  2. Call a service function with a `context.Context` containing a `trace_id`.
  3. Verify the captured log output is valid JSON and includes `trace_id` and `component` attributes.
- **Test Scenario (Metrics):**
  1. Call a handler/service function that emits Prometheus metrics for request duration and delivery outcomes.
  2. Verify that the emitted metrics have `status="delivered|failed|deadletter"` labels and do NOT contain unbounded cardinality labels (e.g., event ID, tenant ID).
- **Mapping:** `FR-OBS-02`, `RULE-OBS-01`, `RULE-OBS-03`
- **Violation Signal:** If logs are not structured JSON or `ctx` attributes are not propagated, or if Prometheus metrics use high-cardinality labels, violating `RULE-OBS-01` or `RULE-OBS-03`.

### `TC-DATA-03-UNIT`: Mandatory Audit Columns
- **Test Scenario:**
  1. Inspect `pkg/domain/event.go` and `pkg/domain/subscription.go` (or generated ORM models).
  2. Verify that `Event` and `Subscription` structs (and their corresponding database schema) include `created_at`, `created_by`, `updated_at`, `updated_by` fields.
- **Mapping:** `RULE-DATA-03`
- **Violation Signal:** If any persistent data model struct or its corresponding migration lacks the mandatory audit columns, `RULE-DATA-03` is violated.