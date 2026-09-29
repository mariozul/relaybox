# Relaybox ReqSpec

## Triage and scope
REQUIRED; HIGH blast radius: database schema=yes, API contract=yes, multi-service=yes, financial integrity=no. One bounded context: relaybox. Human architecture and plan approval received.

## Functional requirements
- FR-ING-01: POST /v1/events accepts event_type, payload, dedup_key; tenant and actor come only from verified request context.
- FR-ING-02: Persist event and matching subscriber outbox rows atomically; no remote publish inside the transaction.
- FR-ING-03: Unique (tenant_id, dedup_key); duplicates return the original event ID without repeated fan-out.
- FR-SUB-01: Create/list subscriptions and delete /v1/subscriptions/{id}; map event_type to target_url.
- FR-SUB-02: All subscription access is tenant-scoped; foreign IDs return not-found.
- FR-DEL-01: Dispatch matching pending deliveries with context, timeout and OpenTelemetry propagation.
- FR-DEL-02: 2xx delivers; timeout/5xx retry with exponential backoff; 4xx dead-letters after N attempts; immediately defer response cleanup.
- FR-DEL-03: Use a bounded worker pool with parent cancellation and graceful shutdown.
- FR-DEL-04: Send stable X-Relaybox-Delivery-Id across retries and restarts.
- FR-OBS-01: Expose /livez and DB-backed /readyz.
- FR-OBS-02: Emit ctx-bound structured JSON logs and bounded-cardinality Prometheus request duration and delivery outcome metrics.

## Acceptance criteria (original IDs preserved; AC-07–11 complete coverage)
- AC-01 (FR-ING-03): Given a tenant/key already ingested; When the request is resent; Then one event exists and the same event ID is returned.
- AC-02 (FR-ING-02): Given a committed event with pending delivery; When the process is killed before delivery and restarted; Then the persisted delivery resumes without event loss.
- AC-03 (FR-DEL-02): Given subscribers returning 500 then 200 or consistently 400; When scheduled attempts execute; Then 500 retries after backoff and succeeds; 400 dead-letters at N.
- AC-04 (FR-SUB-02): Given subscriptions belonging to tenant B; When tenant A lists or deletes them; Then no B data is returned or changed.
- AC-05 (FR-DEL-03): Given concurrent ingestion and delivery load; When race-enabled tests cancel the service; Then no race occurs and all bounded workers terminate.
- AC-06 (FR-OBS-01): Given an unreachable DB; When readyz is requested; Then 503 is returned within its deadline.
- AC-07 (FR-ING-01): Given verified identity and valid input; When POST /v1/events is called; Then the context tenant owns the event; absent identity or malformed input is rejected.
- AC-08 (FR-SUB-01): Given a verified tenant; When subscriptions are created, listed and deleted; Then only active own subscriptions appear and deletion affects future fan-out.
- AC-09 (FR-DEL-01): Given pending delivery with stored trace metadata; When dispatch runs or its parent cancels; Then HTTP receives trace metadata and is bounded by parent and timeout.
- AC-10 (FR-DEL-04): Given a delivery requiring retry or restart replay; When another attempt is made; Then the delivery ID remains unchanged.
- AC-11 (FR-OBS-02): Given requests and delivery outcomes; When telemetry is collected; Then JSON logs have trace fields and redaction; metrics expose only bounded labels.

## Mandatory invariant catalog
RULE-ARCH-01, RULE-ARCH-02, RULE-ARCH-03, RULE-DATA-01, RULE-DATA-02, RULE-DATA-03, RULE-RES-01, RULE-RES-02, RULE-RES-03, RULE-SEC-01, RULE-SEC-02, RULE-EVT-01, RULE-EVT-02, RULE-OBS-01, RULE-OBS-02, RULE-OBS-03, RULE-OBS-04.

## Approved assumptions
ASSUMP-01: Gateway verification contract must be supplied before transport wiring; arbitrary public headers are never trusted.
ASSUMP-02: Active subscription snapshot at ingestion; zero matches means zero deliveries; later subscriptions are not retroactive. Deletion cannot remove accepted deliveries.
ASSUMP-03: Duplicate payload differences still return original ID. Public HTTPS only; reject credentials and non-public resolved IPs at connect time; disable redirects.
ASSUMP-04: Eight workers, 1 MiB payload limit, 10s delivery timeout; validate all limits. Retry delay starts at 1s, caps at 5min with injected jitter/clock. Transient failures have no terminal cap; permanent 3xx/4xx stop after N=3 total attempts.
ASSUMP-05: At-least-once can duplicate POSTs after lost acknowledgments; subscribers deduplicate stable delivery IDs.
NFR-01: Domain-service coverage >=85%, all sentinel branches tested; bounded concurrency and telemetry cardinality.
OUT-OF-SCOPE: UI, ordering, transformation, retention, replay API and Closer/Scribe.
