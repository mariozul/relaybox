# Relaybox requirements

Source: docs/trd/2026-09-29-relaybox.md. HIGH blast radius: schema/API/external services yes; financial integrity no. ReqSpec REQUIRED; single Relaybox context.



### Ingestion
- FR-ING-01: Expose `POST /v1/events` accepting a JSON event
  `{ event_type, payload, dedup_key }`. Tenant identity MUST be resolved from
  the authenticated request context, never the body (RULE-SEC-01).
- FR-ING-02: Persisting an event and enqueuing it for delivery MUST occur in a
  single database transaction (RULE-DATA-01), writing to an outbox table
  (RULE-EVT-02). No remote publish inside the open transaction.
- FR-ING-03: Ingestion MUST be idempotent per `(tenant_id, dedup_key)` via a
  unique constraint (RULE-DATA-02). Re-sends return the original event id.

### Subscriptions
- FR-SUB-01: Expose CRUD `POST/GET/DELETE /v1/subscriptions` mapping an
  `event_type` to a target URL for the authenticated tenant.
- FR-SUB-02: Subscriptions are scoped strictly to the owning tenant; a tenant
  can never read or mutate another tenant's subscriptions (RULE-SEC-01).

### Delivery (external endpoint access)
- FR-DEL-01: A background dispatcher reads pending outbox rows and sends an
  HTTP POST to each matching subscriber URL. All outbound calls MUST propagate
  `ctx` and OpenTelemetry trace metadata (RULE-RES-01, RULE-OBS-02) and set a
  timeout; `context.Background()` is forbidden in the dispatcher (RULE-RES-01).
- FR-DEL-02: Delivery is at-least-once. A 2xx response marks the row delivered;
  a transient failure (timeout, 5xx) is retried with exponential backoff; a
  permanent failure (4xx) is dead-lettered after N attempts. Response bodies
  MUST be closed immediately (RULE-RES-03).
- FR-DEL-03: The dispatcher MUST use a bounded worker pool tied to context
  cancellation and graceful shutdown; no unbounded goroutines (RULE-RES-02).
- FR-DEL-04: Delivery attempts MUST be idempotent from the subscriber's view:
  each attempt sends a stable `X-Relaybox-Delivery-Id` header (RULE-DATA-02).

### Observability & Health
- FR-OBS-01: Expose `/livez` and `/readyz`; `/readyz` checks DB reachability
  (RULE-OBS-04).
- FR-OBS-02: Emit structured JSON logs bound to ctx, and Prometheus metrics for
  request duration and delivery outcomes with bounded-cardinality labels
  (`status="delivered|failed|deadletter"`) (RULE-OBS-01, RULE-OBS-03).


## Acceptance refinements
- FR-ING-01 / AC-07: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then authenticated ingestion rejects malformed/oversized input and body/header tenant spoofing.
- FR-ING-02 / AC-02: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then rollback both writes on failure; recover after commit and after send before acknowledgment.
- FR-ING-03 / AC-01: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then concurrent duplicate keys yield one event and fan-out; keys are independent across tenants.
- FR-SUB-01 / AC-08: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then create/list/delete and pagination; deleting a subscription preserves accepted snapshots.
- FR-SUB-02 / AC-04: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then cross-tenant list/delete/fan-out never reveals or changes another tenant.
- FR-DEL-01 / AC-09: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then send all snapshots with deadline, cancellation, trace propagation and public https validation.
- FR-DEL-02 / AC-03: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then 500 then 200 retries with capped jitter; 400 and 429 deadletter at n.
- FR-DEL-02 / AC-03: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then expired claims recover; stale completion rejected; resources closed per attempt.
- FR-DEL-03 / AC-05: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then concurrent ingestion and delivery stay race-free; worker bound and cancellation join hold.
- FR-DEL-04 / AC-10: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then delivery id stays stable across attempts/restarts and differs across subscribers.
- FR-OBS-01 / AC-06: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then ready returns 503 on unavailable db; live remains independent.
- FR-OBS-02 / AC-11: Given isolated tenants and deterministic fixtures, when the scenario is exercised, then json logs redact secrets and bind trace; metrics have bounded labels.

## Approved assumptions and exclusions
Gateway verification remains an explicit integration input; fail closed until configured. No new authentication product. POST/GET collection and DELETE by ID only. Tenant identities never come from body. Subscription matching and URL snapshot occur on ingest; no retroactive fan-out. Duplicate payload changes return original event. No ordering, transformation, UI or Closer/Scribe.
All mutation tables record created/updated/deleted actor and UTC timestamp metadata. Enforce RULE-ARCH-01 RULE-ARCH-02 RULE-ARCH-03 RULE-DATA-03 RULE-SEC-02 RULE-EVT-01 in addition to TRD invariants.
ErrUnauthorized→401; ErrInvalid→400; ErrNotFound→404; ErrConflict→409; ErrUnavailable→503. Error responses exclude internal causes.
