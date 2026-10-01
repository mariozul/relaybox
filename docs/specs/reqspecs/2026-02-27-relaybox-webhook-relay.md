# ReqSpec: Relaybox — Multi-Tenant Webhook Relay Service

Source TRD: `docs/trd/trd.md` · Date: 2026-02-27 · Domain(s): ingestion, subscription, delivery, observability

## 1. Atomic Requirements

### Ingestion domain
- **FR-ING-01**: `POST /v1/events` accepts `{ event_type, payload, dedup_key }`; tenant identity resolved from authenticated context (JWT claim / gateway header), never body (RULE-SEC-01).
- **FR-ING-02**: Event persist + outbox enqueue in a single DB transaction (RULE-DATA-01, RULE-EVT-02). No remote publish inside the open transaction.
- **FR-ING-03**: Idempotent per `(tenant_id, dedup_key)` via unique constraint (RULE-DATA-02); re-send returns original event id.

### Subscription domain
- **FR-SUB-01**: CRUD `POST/GET/DELETE /v1/subscriptions` mapping `event_type` → target URL for the authenticated tenant.
- **FR-SUB-02**: Strict tenant scoping; cross-tenant read/mutate impossible (RULE-SEC-01).

### Delivery domain
- **FR-DEL-01**: Background dispatcher reads pending outbox rows and POSTs to subscriber URLs; ctx + OTel propagation, per-attempt timeout, no `context.Background()` (RULE-RES-01, RULE-OBS-02).
- **FR-DEL-02**: At-least-once. 2xx → delivered; transient (timeout/5xx) → exponential backoff retry; permanent (4xx) → dead-letter after N attempts. Response bodies closed immediately (RULE-RES-03).
- **FR-DEL-03**: Bounded worker pool tied to ctx cancellation + graceful shutdown (RULE-RES-02).
- **FR-DEL-04**: Stable `X-Relaybox-Delivery-Id` header on every attempt (subscriber-side idempotency, RULE-DATA-02).

### Observability domain
- **FR-OBS-01**: `/livez` + `/readyz`; `/readyz` pings DB, 503 when unreachable (RULE-OBS-04).
- **FR-OBS-02**: Structured JSON logs bound to ctx (`slog`); Prometheus request-duration histograms and delivery-outcome counters with bounded labels (`status="delivered|failed|deadletter"`) (RULE-OBS-01, RULE-OBS-03).

### NFRs
- **NFR-01**: Race detector clean under concurrent ingestion + delivery (`go test -race`).
- **NFR-02**: p99 ingestion latency ≤ 50ms at 500 RPS on local Postgres.
- **NFR-03**: Zero event loss across process kill (outbox durability).

## 2. Acceptance Criteria (Given-When-Then)
- **AC-01** (FR-ING-03): Given an ingested `(tenant, dedup_key)`, when the same request is re-sent, then exactly one event row exists and the original event id is returned.
- **AC-02** (FR-ING-02): Given a committed event whose delivery has not started, when the process is killed and restarted, then the event is still delivered.
- **AC-03** (FR-DEL-02): Given a subscriber returning 500 then 200, the event is delivered via retry with backoff; given a subscriber returning 400, the outbox row is dead-lettered after N attempts.
- **AC-04** (FR-SUB-02): Given tenant A's subscription, when tenant B issues GET/DELETE for it, then the response is 404 and no mutation occurs.
- **AC-05** (NFR-01, FR-DEL-03): Given concurrent ingestion + delivery load, when run under `-race`, then zero data races.
- **AC-06** (FR-OBS-01): Given an unreachable DB, when `/readyz` is called, then 503.

## 3. Error Catalog & Protocol Mapping
| Sentinel | Error Code | HTTP | Description |
| :--- | :--- | :--- | :--- |
| `ErrEventDuplicate` | `EVENT_DUPLICATE` | 200 (idempotent replay) | Re-ingest of known `(tenant, dedup_key)` returns original id |
| `ErrInvalidEventPayload` | `INVALID_EVENT_PAYLOAD` | 400 | Missing/invalid `event_type`/`payload`/`dedup_key` |
| `ErrSubscriptionNotFound` | `SUBSCRIPTION_NOT_FOUND` | 404 | Subscription absent or owned by another tenant |
| `ErrTenantMissing` | `TENANT_CONTEXT_MISSING` | 401 | No authenticated tenant identity in context |

## 4. Blast Radius Assessment
| Criterion | Answer |
| :--- | :--- |
| Database Schema | **Yes** (new tables, migrations) |
| API Contract | **Yes** (new public `/v1/events`, `/v1/subscriptions`) |
| Multi-Service Boundary | Yes (external HTTP egress) |
| Data Integrity | **Yes** (durable outbox, at-least-once) |

**Classification: HIGH** → Architecture RFC + Human Gate 1 sign-off (folded into Gate 2 card below per MEDIUM/HIGH routing).

## 5. Assumptions
- **ASSUMP-01**: Tenant identity arrives as a verified header/claim (e.g. `X-Tenant-Id` set by an upstream gateway); Relaybox does not implement token issuance.
- **ASSUMP-02**: Postgres 16 via docker-compose for local/test.
- **ASSUMP-03**: Payload ≤ 256KB; larger rejected with 400.

## 6. Out of Scope
UI/dashboard, cross-type ordering, message transformation, The Closer & Scribe ticket flow.

## 7. PR Partition (RULE-PLAN-007, SRP-PR)
| PR Group | Domain | Content |
| :--- | :--- | :--- |
| `PR-INGEST-01` | ingestion | Event entity, ingest use case, HTTP handler, migrations, idempotency |
| `PR-SUB-01` | subscription | Subscription entity, repo, CRUD handler, tenant scoping |
| `PR-DELIVER-01` | delivery | Outbox dispatcher, bounded pool, retry/backoff/dead-letter, egress client |
| `PR-OBS-01` | observability | /livez /readyz, slog setup, Prometheus metrics, middleware |
