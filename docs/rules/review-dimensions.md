# Code Review Dimensions & Severity Guidelines

This document serves as the **Single Source of Truth (SSOT)** for automated code auditing by the **Reviewer & Validator Agent**. All code evaluations—whether executed during the pre-PR verification gate or post-PR review loops—must strictly adhere to the dimensions, invariants, and severity scoring outlined herein.

> **Ads Repository Alignment:**  
> The 4 review dimensions and severity classification matrix herein directly codify the code review checklist and architectural best practices of the Ads engineering repository (`ads` / `astro-ads-be`), including specification traceability, N+1 query elimination, clean architectural separation, and telemetry cardinality budgets.

---

## 1. Review Dimensions

### Dimension 1: Business Correctness & Specification Alignment
This dimension evaluates whether the implementation satisfies functional requirements, preserves system integrity, and prevents data corruption.

* **Specification Traceability (Critical):**
  * Every Functional Requirement (`FR-xx`) defined in the task specification (`ImplSpec`) must map directly to concrete implementation lines in the code diff.
  * Any missing requirement or unfulfilled Acceptance Criteria scenario (Given-When-Then) constitutes a blocking defect.
* **Scope Boundary Discipline (High):**
  * Code changes must strictly stay within the file and module boundaries defined by the specification. Modifying unrelated modules without architectural justification is prohibited (*scope creep*).
* **Data Consistency & Transaction Boundaries (Critical):**
  * Related state mutations across multiple entities or tables must execute within an atomic transaction. Partial persistence that risks orphan or corrupted states is prohibited.
* **Context & Cancellation Propagation (High):**
  * Request contexts (`context.Context`) must be propagated across service, database, and downstream client calls. Hardcoded `context.Background()` or `context.TODO()` in business logic is forbidden.
* **Audit & Traceability Integrity (Critical):**
  * Mutations must track actor metadata (`created_by`, `updated_by`, `deleted_by`, or equivalent telemetry) to maintain a complete audit trail.
* **Defensive Error Handling (Medium):**
  * Errors must be handled explicitly and wrapped with descriptive context (`fmt.Errorf("action description: %w", err)`). Silently ignoring errors using `_` without an explicit clarifying comment is forbidden.

---

### Dimension 2: Performance & Scalability
This dimension ensures code performs efficiently under scale and does not introduce bottlenecks or resource starvation.

* **N+1 Query Detection (High):**
  * Executing database queries or remote network calls inside iterative loops instead of batching operations (e.g., using `WHERE id IN (...)`) is prohibited.
* **Unbounded Collections & Missing Pagination (Medium):**
  * Reading collections from databases or memory stores without explicit limits, offsets, or pagination boundaries is prohibited.
* **Algorithmic Inefficiency (Medium):**
  * Nested loops with $O(n^2)$ complexity where hash-map lookups ($O(1)$) or indexed queries are applicable.
* **Redundant Reads & Cache Waste (Medium):**
  * Fetching identical entity records multiple times within the same execution path instead of passing or reusing retrieved state.
* **Index Coverage (High):**
  * Adding filter clauses (`WHERE`), join conditions (`JOIN`), or sorting clauses (`ORDER BY`) on large tables without index backing.

---

### Dimension 3: Architecture, Maintainability & Clean Code
This dimension guarantees long-term maintainability, modularity, and adherence to clean architectural principles (e.g., Domain-Driven Design).

* **Go Idiom Conformance (Medium — FIX OR EXPLAIN):**
  * Full manifest: [`docs/architecture/05-coding-agent.md` §3](../architecture/05-coding-agent.md) (Go Proverbs binding). Checklist:
    * Error swallowing (`_ = err` / empty branch) — must wrap and bubble up (*Errors are values*).
    * Return-type abstraction abuse (returning interfaces) — accept interfaces, return concrete types.
    * Trivial third-party dependencies that duplicate stdlib (*a little copying…*).
    * Non-useful zero values (nil-panic-prone structs, missing safe defaults).
    * Shared mutable global state guarded by locks — prefer channels/immutable data.
    * `ctx context.Context` not first param of I/O methods (`RULE-RES-01`).
    * Getter with `Get` prefix or package stutter (`order.OrderService`) — naming violations.

* **Architectural Layer Boundary Isolation (High):**
  * Core domain layers must remain pure and decoupled from infrastructure details. Domain services must never import transport protocols (HTTP frameworks, gRPC status codes, or database drivers directly).
* **Thin Transport Handlers & Subscribers (High):**
  * API handlers and event consumers must remain thin adapters: their only role is payload deserialization, parameter validation, and delegation to domain services.
* **Message Consumption Idempotency (Critical):**
  * Asynchronous event handlers must handle acknowledgment (`Ack`) vs negative acknowledgment (`Nack` / retry) safely. Non-recoverable errors must not enter infinite retry loops, and recoverable errors must not be silently discarded.
* **Reuse Discipline & Duplication Prevention (High):**
  * **Same-Diff Reuse:** Introducing a redundant dependency or field when an existing injected client already provides equivalent capability.
  * **Cross-File Duplication:** Creating utility functions or abstractions that duplicate capabilities already present in shared libraries.
  * **Reinventing Standard Capabilities:** Writing custom logic (e.g., retry backoff, string formatting, slice manipulation) when equivalent functionality exists in standard libraries.
* **Parameter Creep (High):**
  * Adding parameters (especially boolean control flags) that cause a function to handle multiple unrelated responsibilities, violating the Single Responsibility Principle.
* **Code Complexity Constraints (Medium):**
  * Cognitive complexity $> 15$ per function.
  * Functions exceeding 80 lines or files exceeding 500 lines without modular decomposition.

---

### Dimension 4: Reliability & Operational Observability
This dimension safeguards infrastructure resilience and operational visibility without causing telemetry cost explosions.

* **Telemetry Tag Cardinality Budget (High):**
  * High-cardinality values (e.g., dynamic UUIDs, customer emails, timestamps) must never be used as metric tags or labels, as they cause exponential timeseries explosions and severe cloud billing spikes.
* **Logging Hygiene on Hot Paths (High):**
  * Emitting verbose logging (`INFO`, `DEBUG`) on high-throughput execution paths causes disk I/O exhaustion and log-storage bloat. Only `ERROR` logs are justified on hot paths.
* **Standardized Metric Naming (Medium):**
  * Telemetry metric keys must follow standardized hierarchical naming: `{domain}_{subsystem}_{noun}_{unit}` (e.g., `order_processing_latency_ms`).

---

## 2. Severity Classification Matrix

Every finding identified by the Reviewer Agent must be classified into one of four severity levels:

| Severity | Definition & Trigger Criteria | Pipeline & Merge Impact |
| :--- | :--- | :--- |
| **`CRITICAL`** | Violates domain invariants, risks data loss/corruption, compromises security, introduces infinite retry loops, drops audit tracking, or leaves a Functional Requirement (`FR-xx`) unimplemented. | **BLOCKING (Must Fix)** |
| **`HIGH`** | N+1 queries, layer boundary violations, unhandled contexts, parameter creep, duplicating available capabilities, or metric tag cardinality explosions. | **BLOCKING (Must Fix)** |
| **`MEDIUM`** | Cognitive complexity $> 15$, functions $> 80$ lines, missing pagination, suboptimal $O(n^2)$ loops, missing error wrapping, or inconsistent metric naming. | **NON-BLOCKING (FIX OR EXPLAIN — requires technical justification if deferred)** |
| **`LOW`** | Magic literals, import reordering, minor code deduplication, or missing documentation comments. | **NON-BLOCKING (ADVISORY — optional polish)** |

---

## 3. Deduplication & Output Protocol

When compiling findings:
1. **Deduplication:** Multiple findings reporting on the same line and file must be merged into a single entry, retaining the **highest severity**.
2. **Schema Compliance:** Output findings must strictly conform to the **`ReviewFindingsPayload`** JSON schema defined in [`docs/architecture/03-engine-contracts.md`](file:///Users/mario.zulkarnain/Workouts/agent-orchestration/docs/architecture/03-engine-contracts.md).
3. **Tri-State Verdict Resolution:**
   * **`APPROVED`** (or **`SAFE_TO_MERGE`**): Zero `CRITICAL` (P0) and zero `HIGH` (P1) findings.
   * **`NEEDS_FIX`**: One or more `CRITICAL` (P0) or `HIGH` (P1) findings remain unaddressed.
   * **`REJECTED`** (or **`NEEDS_HUMAN_DISCUSSION`**): Valid technical trade-off, architectural dispute, or fundamental invariant violation that requires human intervention.
   *(Note: Full semantic-to-numeric priority mapping is defined in [`docs/architecture/03-engine-contracts.md`](file:///Users/mario.zulkarnain/Workouts/agent-orchestration/docs/architecture/03-engine-contracts.md) §4.2).*
