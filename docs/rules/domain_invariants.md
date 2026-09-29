# System & Domain Invariants

This document establishes the **Universal Non-Negotiable Invariants** for the codebase. These rules represent foundational constraints that must **never** be violated. 

> **Ads Repository Alignment:**  
> These invariants are adapted directly from the production architecture and engineering standards of the Ads backend repository (`ads` / `astro-ads-be`). They strictly enforce Clean Architecture layer boundaries, unbroken context propagation, bounded concurrency, hot-path telemetry cardinality hygiene, database transaction safety, and idempotent messaging.

The **Reviewer & Validator Agent** evaluates all pull requests and code modifications against these rules. Any direct violation of a named invariant (`RULE-XXX-NN`) is automatically classified as a **`CRITICAL`** blocking issue.

---

## Invariant Numbering Taxonomy

Rules are categorized into five core domains:
* `RULE-ARCH-xx`: Architecture & Layer Isolation
* `RULE-DATA-xx`: Data Integrity & Transactional Consistency
* `RULE-RES-xx`: Concurrency & Resource Lifecycle Safety
* `RULE-SEC-xx`: Security, Multi-Tenancy & Access Boundary
* `RULE-EVT-xx`: Asynchronous Messaging & Event Integrity
* `RULE-OBS-xx`: Application System Observability & Telemetry

---

## 1. Architecture & Layer Isolation (`RULE-ARCH`)

### `RULE-ARCH-01`: Domain Layer Independence
The core business/domain layer must remain pure and agnostic of external frameworks, transport protocols, and database drivers.
* **Invariant:** Domain entities, domain services, and domain errors must never import HTTP frameworks (e.g., Gin, Echo), gRPC transport packages, or SQL/ORM drivers directly.
* **Mechanism:** All persistence and external communication must be accessed via abstract interfaces defined within the domain layer.

### `RULE-ARCH-02`: Inverted Dependency Direction
Dependencies must always point inward toward business rules.
* **Invariant:** Higher-level business modules must not depend on low-level implementation details. Handlers, controllers, and subscribers act as outer adapters that call domain services, never vice-versa.

### `RULE-ARCH-03`: Explicit Domain Error Taxonomy
Internal system failures must not leak raw database errors or third-party client errors directly across domain boundaries.
* **Invariant:** Domain operations must expose typed domain error sentinels (e.g., `ErrNotFound`, `ErrConflict`, `ErrInvalidState`). Downstream transport handlers map domain sentinels to appropriate protocol status codes.

---

## 2. Data Integrity & Transactional Consistency (`RULE-DATA`)

### `RULE-DATA-01`: Atomic Multi-Entity Mutations
Any business operation that mutates state across multiple tables, records, or entities must guarantee atomicity.
* **Invariant:** Partial commits are forbidden. Multi-step state changes (e.g., updating order status, recording a ledger journal, and updating user balances) must execute within a single database transaction.

### `RULE-DATA-02`: Idempotent State Transitions
Operations triggered by external requests or asynchronous events must be safely re-executable without causing duplicate mutations or side effects.
* **Invariant:** State transitions (e.g., payment capture, order settlement, quota consumption) must enforce idempotency via unique constraints, idempotency keys, or version checks (`乐观锁 / Optimistic Locking`).

### `RULE-DATA-03`: Mandatory Audit Columns
To preserve operational traceability and post-mortem auditability, all persistent database records must capture audit metadata.
* **Invariant:** Mutation schemas must track creation, modification, and soft-deletion actor metadata (`created_by`, `updated_by`, `deleted_by`, alongside corresponding UTC timestamps).

---

## 3. Concurrency & Resource Lifecycle Safety (`RULE-RES`)

### `RULE-RES-01`: Unbroken Context Propagation
ExecutionContext and deadlines must flow uninterrupted throughout the call graph.
* **Invariant:** `context.Background()` or `context.TODO()` is strictly prohibited within business service layers and event handlers. All I/O operations (database queries, cache lookups, HTTP/RPC calls) must accept and pass through `ctx context.Context`.

### `RULE-RES-02`: Bounded Concurrency & Goroutine Lifecycle
Asynchronous workers, goroutines, or background tasks must never be spawned in an unmanaged, unbounded fashion.
* **Invariant:** Background routines must have a deterministic lifecycle tied to parent context cancellation, graceful shutdown signals (`sync.WaitGroup`), or bounded worker pools. Spawning unmanaged routines inside request handlers is forbidden.

### `RULE-RES-03`: Immediate Resource Deferral
Any acquired operating system resource (file handles, network responses, database rows/transactions) must have its cleanup bound immediately upon acquisition.
* **Invariant:** Cleanup routines (`defer resp.Body.Close()`, `defer rows.Close()`, `defer tx.Rollback()`) must be invoked immediately after verifying that acquisition succeeded without error.

---

## 4. Security, Multi-Tenancy & Access Boundary (`RULE-SEC`)

### `RULE-SEC-01`: Trusted Context Identity Extraction
Multi-tenant isolation and user identity must always originate from authenticated, cryptographic tokens or gateway-verified context headers.
* **Invariant:** Tenant identifiers (`tenant_id`, `org_id`, `merchant_id`) or user credentials must never be parsed or trusted directly from client request bodies or query parameters on public endpoints. They must be resolved from authenticated request context.

### `RULE-SEC-02`: Zero Sensitive Data Leakage in Telemetry
Protected data must never appear in unencrypted application logs, metrics, or error traces.
* **Invariant:** Personally Identifiable Information (PII), API secret keys, passwords, and sensitive financial payloads must be masked or excluded from logs, error strings, and metric labels.

---

## 5. Asynchronous Messaging & Event Integrity (`RULE-EVT`)

### `RULE-EVT-01`: Safe Subscriber Acknowledgment Semantics
Message broker consumers must differentiate between permanent application errors and transient infrastructure failures.
* **Invariant:** 
  * Permanent business validation failures must acknowledge (`Ack`) the message to prevent toxic infinite reprocessing loops.
  * Transient infrastructure failures (e.g., database network timeout) must reject or re-queue (`Nack`) the message to enable retry backoff.

### `RULE-EVT-02`: Transactional Outbox for Distributed Events
State persistence and corresponding event publishing must not suffer from dual-write consistency hazards.
* **Invariant:** If a database transaction commits, its related domain event must reliably publish. Direct remote message publication inside a pending database transaction before commit is prohibited.

---

## 6. Application System Observability (`RULE-OBS`)

All services and features synthesized by the Coding Agent must possess production-grade observability out of the box.

### `RULE-OBS-01`: Structured JSON Logging & Context Binding
Standardized structured logging is mandatory across all execution paths.
* **Invariant:** Raw `fmt.Println` or unformatted `log.Printf` is strictly forbidden. All logs must emit structured JSON (via `slog` or `zerolog`) bound to `ctx context.Context` and automatically extract `trace_id` and `span_id`.

### `RULE-OBS-02`: Distributed Tracing Context Propagation
Telemetry context must flow unbroken across process and network boundaries.
* **Invariant:** Every outbound network call (HTTP, gRPC, Redis, SQL) must inject or propagate OpenTelemetry trace metadata.

### `RULE-OBS-03`: Bounded Metric Cardinality & Standard Telemetry
All business mutators and transport endpoints must register standard Prometheus metrics.
* **Invariant:** 
  * Endpoints must expose request duration histograms and error counters.
  * Metric labels must strictly bound cardinality (e.g. `status="success|error"`, `method="POST"`). Never use dynamic values like user IDs or UUIDs as metric labels.

### `RULE-OBS-04`: Liveness & Readiness Probes
Every deployable microservice must expose standardized operational health endpoints.
* **Invariant:** Expose `/livez` (process alive) and `/readyz` (dependent caches/DB reachable) handlers for orchestration platforms.

---

## 7. How the Reviewer Agent Enforces Invariants

When the **Reviewer & Validator Agent** audits code:
1. It compares every line in the change diff against this catalog.
2. If any invariant is broken:
   * The finding is tagged with the exact rule ID (e.g., `RULE-DATA-01`).
   * The severity is set to **`CRITICAL`**.
   * The finding is added to `ReviewFindingsPayload` to block the PR gate and trigger the **Coding Agent** remediation loop.
