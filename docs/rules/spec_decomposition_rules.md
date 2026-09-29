# Rules: Specification Decomposition (Spec-First Invariant Rules)

This document establishes the **Universal Engineering Rules** and canonical contracts that **must be strictly obeyed** by the **Planner Agent**, **Coding Agent**, and **Reviewer Agent**.
These rules adopt the specification decomposition standards from the Ads backend repository (`astro-ads-be` / `ads`), covering `reqspec`, `testspec`, and `implspec` to guarantee zero hallucination, deterministic TDD execution, and enterprise concurrency safety.

---

## 1. Rationale for Placement in `docs/rules/` (vs. `docs/guides/`)

1. **Agent Legal Authority**: LLMs treat documents in `docs/guides/` as narrative guidance or optional reference. In contrast, documents in `docs/rules/` constitute **Hard Invariants & Governance Contracts** enforced programmatically by linters, the Gate B reconciliation checker, and Reviewer Agent audits.
2. **Review Dimension Binding**: The Reviewer Agent explicitly audits whether `testspec` and `implspec` comply with requirement traceability, concurrency discipline, and PR sizing limits defined herein.
3. **Blocking Gate**: Agents are strictly prohibited from writing code before generating specifications that pass validation against these rules.

---

## 2. Specification Decomposition Hierarchy & Pipeline

```text
[ Approved TRD / Mini-TRD ]
           │
           ├────────────────────────────┐ (Blast Radius: LOW / Fast-Path)
           ▼ (Blast Radius: HIGH)       │
 ┌───────────────────────┐              │
 │ 1. docs/reqspecs/     │              │  ➔ FR-xx, NFR-xx, SC-xx, Given-When-Then AC-xx
 │    {domain}-{feat}.md │              │
 └───────────┬───────────┘              │
             ▼                          │
 ┌───────────────────────┐              │
 │ 2. docs/architecture/ │              │  ➔ Architecture RFC / ADR (Schema, Migration, Observability)
 │    rfcs/{feat}.md     │              │
 └───────────┬───────────┘              │
             │ 🛑 Human Gate 1          │
             ├──────────────────────────┘
             ▼
 ┌───────────────────────┐
 │ 3. docs/testspecs/    │  ➔ TC-xx-UNIT, TC-xx-INT, TC-xx-RACE, TC-xx-EDGE
 │    {domain}/spec.md   │  🛑 RULE-SPEC-01: ImplSpec is BLOCKED until TestSpec is locked!
 └───────────┬───────────┘
             ▼
 ┌───────────────────────┐
 │ 4. docs/implspecs/    │  ➔ 7-Layer Clean Architecture, Contracts, Telemetry, Migrations
 │    YYYY-MM-DD-*.md    │  🛑 RULE-PLAN-004: PR Sizing Hard Limit <= 500 LOC
 └───────────┬───────────┘
             ▼
 ┌───────────────────────┐
 │ 5. artifacts/plans/   │  ➔ Machine DAG Execution Manifest (plan.json)
 │    plan.json          │
 └───────────────────────┘
```

---

## 3. RULE-REQ: Requirements Specification Standard (`docs/reqspecs/`)

**Applicability (Dual-Function Input Rule):** `docs/reqspecs/{domain}-{feature}.md` is **MANDATORY only for large PRDs / multi-domain initiatives**. For **Mini-TRD inputs** (single feature; FR-xx/AC already written inside the TRD), the TRD serves dual duty as the requirements spec — `/req-to-reqspec` is skipped and the pipeline ingests the TRD directly; the FR/RULE tags inside the TRD are the RTM source of truth validated by Gate B.

Every business requirement from a full PRD must be decomposed into `docs/reqspecs/{domain}-{feature}.md` following these standards:

### 3.1. Atomic Requirement Tagging
- **`FR-xx` (Functional Requirement)**: Specific, atomic, verifiable functional requirements. Multiple discrete operations must never be combined into a single ID.
- **`NFR-xx` (Non-Functional Requirement)**: Performance thresholds (e.g., p99 latency < 50ms), data security, concurrency bounds, and system availability.
- **`SC-xx` (Success Criteria / KPI)**: Quantifiable engineering and business metrics (e.g., `SC-01`: Zero data drift across 100,000 synthetic operations; `SC-02`: p95 response time $\le$ 30ms at 500 RPS).
- **`ASSUMP-xx` (Explicit Assumptions)**: Technical assumptions or infrastructure constraints recorded by the engineering team.
- **`OUT-OF-SCOPE`**: Scenarios or features explicitly excluded from the current delivery cycle.

### 3.2. Acceptance Criteria (Gherkin Format)
Every `FR-xx` must have at least one Acceptance Criteria written in strict **Given-When-Then** format (`AC-xx`):
- **Given**: Initial system state, user balances, mock dependencies, or context prerequisites.
- **When**: The triggering action (endpoint invoked, event published, function executed).
- **Then**: Expected outcome, database state mutation, events emitted, or specific sentinel error returned.

### 3.3. Error Catalog & Protocol Mapping
Every requirement spec must define an explicit table mapping business domain errors to transport protocol codes:
| Error Sentinel | Error Code String | HTTP Status | gRPC Status | Description / Client Message |
| :--- | :--- | :--- | :--- | :--- |
| `ErrInsufficientFunds` | `WALLET_INSUFFICIENT_FUNDS` | 422 Unprocessable | FailedPrecondition | User balance is lower than deduction amount |
| `ErrInvalidAmount` | `INVALID_TRANSACTION_AMOUNT` | 400 Bad Request | InvalidArgument | Deduction amount is $\le 0$ or exceeds upper limit |
| `ErrWalletNotFound` | `WALLET_NOT_FOUND` | 404 Not Found | NotFound | Target wallet record does not exist |

### 3.4. Blast Radius Assessment Matrix
Every requirement spec must evaluate blast radius:

| Blast Radius Criterion | Evaluation Question | Status (Yes/No) |
| :--- | :--- | :--- |
| **Database Schema** | Does this add/alter tables, modify columns, or require migrations? | |
| **API Contract** | Does this alter public request/response payloads or break backward compatibility? | |
| **Multi-Service Boundary** | Does this affect cross-service RPCs, shared message brokers, or third-party APIs? | |
| **Data / Financial Integrity** | Does this mutate ledger balances, financial records, or critical state transitions? | |

- **If ALL answers are "No"** ➔ `Classification: LOW` (Eligible for Fast-Path directly to `testspec.md`).
- **If ANY answer is "Yes"** ➔ `Classification: HIGH` (Mandates an Architecture RFC and approval at **Human Gate 1**).

---

## 4. RULE-TEST: Test Specification Standard (`docs/testspecs/`)

The Test Specification defines **WHAT** must be proved before a single line of implementation code is written (Strict TDD).

### 4.1. [RULE-TEST-001] Test Case Taxonomy
Every scenario must be assigned a categorized identifier:
- **`TC-xx-UNIT`**: Isolated unit test (mocked external I/O, pure domain logic).
- **`TC-xx-INT`**: Integration test (live database transactions, HTTP transport, cache adapter).
- **`TC-xx-RACE`**: Concurrency race test (multi-goroutine / parallel requests).
- **`TC-xx-EDGE`**: Boundary and error handling test (overflow, zero values, timeouts, network partitions).

### 4.2. [RULE-TEST-002] Bidirectional Traceability Matrix
A traceability table must link each `TC-xx` directly to its parent `FR-xx` and `AC-xx`:

| Test ID | Test Scenario | Mapping (FR & AC) | Category | Target Test File |
| :--- | :--- | :--- | :--- | :--- |
| `TC-01-UNIT` | Successful balance deduction (Happy Path) | `FR-01`, `AC-01` | Unit Test | `pkg/domain/wallet_test.go` |
| `TC-02-EDGE` | Deduction amount exceeds balance (Insufficient) | `FR-02`, `AC-02` | Boundary | `pkg/domain/wallet_test.go` |
| `TC-03-RACE` | 20 concurrent parallel deductions on the same wallet | `FR-01`, `NFR-01` | Concurrency | `pkg/domain/wallet_test.go` |

### 4.3. [RULE-TEST-004] Mandatory Violation Signal (Sentinel Bug Trap)
Every test scenario in `testspec` must define a **Violation Signal**.
*Definition*: An explicit statement explaining what bug, code mutation, or missing logic **will reliably cause this test to fail (RED)**.
*Examples*:
- `TC-01-UNIT`: *"Removing the SQL transaction commit causes the function to return nil while balance data is not persisted."*
- `TC-03-RACE`: *"Accessing balance without `mu.Lock()` or serializable transactions triggers a data race detected by the `-race` flag."*

### 4.4. [RULE-TEST-003] Idiomatic Go Test Patterns & Flakiness Prevention
1. **Table-Driven Tests**: Group related scenarios into structured test tables using `t.Run(tc.name, func(t *testing.T) { ... })`.
2. **Deterministic Time / Clock Injection**: Forbidden from using `time.Now()` directly in domain logic. Clocks must be injected as interfaces or time providers to ensure zero test flakiness.
3. **Fixture Teardown**: Integration tests must utilize `t.Cleanup(func() { ... })` or transaction rollbacks to guarantee pristine test state without residual side effects.
4. **Deterministic Concurrency Discipline**:
   - Zero `time.Sleep`: All asynchronous synchronization must use deterministic primitives (`sync.WaitGroup`, buffered channels, or context deadlines).
   - Mandatory `-race` Execution: Test suites must execute with race detection enabled:
     ```bash
     go test -v -race -timeout 30s ./...
     ```
5. **Coverage Gates**:
   - Minimum **85%** statement coverage on domain and service logic.
   - Minimum **100%** branch coverage on sentinel error handling (`ErrNotFound`, `ErrConflict`, etc.).

---

## 5. RULE-IMPL: Implementation Specification Standard (`docs/implspecs/`)

The Implementation Specification defines **HOW** the system is built under Clean Architecture principles without writing full implementation logic.

### 5.1. Core Principle: Contracts, Not Implementation
- Only write struct definitions, interface method signatures, sentinel error constants, and godoc documentation.
- **NEVER** write full function implementation bodies or complete algorithms in ImplSpec. Implementation logic is the exclusive domain of the Coding Agent during the TDD Green phase.

### 5.2. Clean Architecture 7-Layer Component Mapping
Every component must be placed in its proper architectural layer:
1. **Entity**: Pure domain models & value objects (`pkg/domain/`). Zero external dependencies.
2. **Domain Service**: Domain business logic coordinating multiple entities.
3. **Use Case / Port Interface**: Application input/output contracts.
4. **Repository Interface**: Data persistence contracts (`pkg/repository/`).
5. **Adapter / Infrastructure**: Concrete implementations for DB, Redis, external APIs (`pkg/infrastructure/`).
6. **Transport / Handler**: HTTP router, gRPC server, DTO serialization (`pkg/handler/`).
7. **DTO / Mapper**: Request/Response transfer objects and converters to Domain Entities.

### 5.3. Database Schema Migration Plan (Expand/Contract)
If Blast Radius involves database mutations:
1. **Migration Scripts**: Explicit Up and Down SQL scripts must be specified:
   - `migrations/YYYYMMDDHHMMSS_create_wallet_table.up.sql`
   - `migrations/YYYYMMDDHHMMSS_create_wallet_table.down.sql`
2. **Zero-Downtime Expand/Contract Pattern**:
   - *Phase 1 (Expand)*: Add new columns as nullable or with defaults. Code dual-writes if necessary.
   - *Phase 2 (Migrate)*: Backfill historical data via asynchronous workers.
   - *Phase 3 (Contract)*: Make columns NOT NULL and remove legacy fallback code in a subsequent PR.

### 5.4. System Observability & Telemetry Blueprint (RULE-OBS)
Every implementation spec must explicitly define its telemetry footprint:
1. **Prometheus Metrics**:
   - Counter: `app_{domain}_{operation}_total{status="success|failure"}`
   - Histogram: `app_{domain}_{operation}_duration_seconds` (predefined latency buckets)
   - **Bounded Cardinality Invariant**: Unbounded identifiers (e.g., User ID, Order UUID, email, arbitrary client queries) are STRICTLY FORBIDDEN in Prometheus metric labels to prevent time-series database memory exhaustion (OOM).
2. **Structured Logging (`slog`)**:
   - Context propagation: All logs must pass `ctx` (`slog.InfoContext(ctx, ...)`).
   - Standard attributes: `trace_id`, `span_id`, `component`, `error`.

### 5.5. Hard Rule: PR Sizing Limit ($\le$ 500 LOC) (`RULE-PLAN-004`)
- Every Pull Request or implementation subtask **MUST NOT** exceed 500 lines of code (LOC), including test files (`RULE-PLAN-004`).
- If a feature exceeds 500 LOC, the Planner Agent **MUST** decompose it into an ordered Directed Acyclic Graph (DAG) of PRs:
  - **PR-1 (Domain & Repository Interface)**: Entity + interfaces + domain unit tests.
  - **PR-2 (Database Adapter & Persistence)**: Concrete repository + integration tests.
  - **PR-3 (Use Case & Business Service)**: Orchestration service + service tests.
  - **PR-4 (HTTP/Transport & Observability)**: HTTP handler + routes + metrics.

---

## 6. RULE-PLAN: Machine Execution Manifest (`artifacts/plans/plan.json`)

The final output produced by the Planner Agent for direct execution by `bin/orchestrator.sh` and the Coding Agent must conform to valid JSON:

### 6.1. RULE-PLAN-005: File Collision Guard (Disjoint Concurrent File Sets)

Subtasks that may execute **concurrently** (no direct or transitive `depends_on` ordering between them) must have **100% disjoint `target_files`**:

$$\text{target\_files}(S_i) \cap \text{target\_files}(S_j) = \emptyset \quad \forall \; S_i \neq S_j \text{ concurrent}$$

**Enforcement (two layers):**
1. **Pre-dispatch (Gate B):** `bin/gate_b_verifier.py` Section 4 FAILS the plan when concurrent subtasks share files.
2. **Runtime dispatch:** `bin/orchestrator.sh` STEP 2.0 re-checks at dispatch time and auto-serializes overlapping pairs by injecting a synthetic `depends_on` edge (Kahn wave demotion).

**Planner obligation:** when decomposing into subtasks, overlapping file sets MUST be either disjointed or explicitly ordered via `depends_on` — never left concurrent.

### 6.2. RULE-PLAN-006: Reqspec Triage (Decision Delegated to the Planner Agent)

The Planner Agent decides whether `/req-to-reqspec` (Step 1, `docs/reqspecs/`) is required before planning, and records the decision in the structured output (`reqspec_triage` in `execution_plan.schema.json`):

**Indicators → `REQUIRED`:**
- Input spans **≥2 bounded contexts** or needs decomposition into **>1 feature**.
- Database schema migrations, public API contract changes, cross-service boundaries, or financial/ledger integrity are involved (Blast Radius Matrix §3.4: any "yes").
- FRs are ambiguous — they cannot be mapped 1-FR→1-TC without human interpretation.

**Indicators → `SKIP`:**
- Single bounded context; FR-xx/AC complete inside the TRD (dual-duty input, doc 01 Step 1); all four matrix answers "no".

**Fail-safe rule:** when uncertain, the Planner MUST choose `REQUIRED` (a redundant reqspec wastes one review; a skipped reqspec leaks loose FRs into coding and causes repeated Gate B omission failures).

**Human veto:** `decision` + `reasons` + `matrix_answers` are presented at Human Gate 2 (and in the `/orchestrator-plan` flow) — the human may override either direction before coding starts. The machine gate (Gate B) validates the triage block is present and structurally valid; it does not second-guess the decision itself.

```json
{
  "task_id": "FEATURE-01",
  "feature_name": "wallet-deduction",
  "blast_radius": "LOW",
  "total_subtasks": 2,
  "subtasks": [
    {
      "id": "ST-01",
      "name": "Domain Model and In-Memory Thread-Safe Wallet",
      "layer": "domain",
      "estimated_loc": 140,
      "depends_on": [],
      "test_cases": ["TC-01-UNIT", "TC-02-EDGE", "TC-03-RACE"],
      "target_files": [
        "pkg/domain/wallet.go",
        "pkg/domain/wallet_test.go"
      ],
      "verification_command": "go test -v -race ./pkg/domain/..."
    },
    {
      "id": "ST-02",
      "name": "HTTP Transport & Observability Handler",
      "layer": "transport",
      "estimated_loc": 180,
      "depends_on": ["ST-01"],
      "test_cases": ["TC-04-INT"],
      "target_files": [
        "pkg/handler/wallet_handler.go",
        "pkg/handler/wallet_handler_test.go"
      ],
      "verification_command": "go test -v -race ./pkg/handler/..."
    }
  ],
  "verification_commands": [
    "go test -v -race ./..."
  ]
}
```

### 6.3. RULE-PLAN-007: Single Responsibility Pull Request Invariant (SRP-PR)

Every Pull Request produced under this architecture **MUST strictly adhere to the Single Responsibility Principle (SRP)**:

1. **One Bounded Context / Domain per PR (MANDATORY)**:
   - A single PR must have **exactly one conceptual domain or functional topic** to change.
   - Mingling changes across multiple distinct bounded contexts (e.g., `auth` and `relaybox`, or `billing` and `notification`) in a single PR is **STRICTLY PROHIBITED**, even if the total lines of code is very small ($< 500$ LOC).
   - If a TRD spans multiple bounded contexts, the Planner Agent MUST decompose the execution plan into distinct domain PR tracks.
2. **Reviewer Veto**:
   - The Reviewer Agent MUST fail any PR audit that introduces cross-domain coupling or mixes unrelated functional concerns in a single changeset.
3. **Rollback & Blast Radius Guarantee**:
   - Isolating PRs per domain guarantees that any domain-specific bug or rollback can be executed without impacting independent subsystems.

---

## 7. Gate B: Machine Drift & Traceability Reconciliation Formula

The automated Gate B verification in `bin/orchestrator.sh` mathematically validates Requirements Traceability (RTM) before coding begins:

$$\text{Traceability Score} = \frac{|\{FR \in \text{TRD} \mid \exists TC \in \text{TestSpec}\}|}{|\text{TRD}_{FR}|} \times 100\% = 100\%$$

- **Omission Defect**: If any $FR$ from the TRD lacks a corresponding test scenario in `testspec.md`, the pipeline **fails immediately**.
- **Phantom Feature Defect**: If `testspec.md` introduces test scenarios for scope not approved in the TRD, the pipeline **fails immediately**.

---

## 8. Reviewer Agent Audit Checklist
The Reviewer Agent must **REJECT** any PR if:
1. [ ] Any `FR-xx` in the TRD lacks a corresponding `TC-xx` in `testspec.md` (Traceability Gap).
2. [ ] Concurrency test scenarios (`TC-xx-RACE`) lack a Violation Signal or rely on `time.Sleep`.
3. [ ] Any subtask in `implspec.md` or `plan.json` has `estimated_loc > 500` (`RULE-PLAN-004`).
4. [ ] ImplSpec contains full function implementation bodies (violating "Contracts, Not Implementation").
5. [ ] Domain logic calls `time.Now()` directly instead of using an injected clock provider.
6. [ ] Prometheus metric labels include unbounded cardinality fields (e.g., UUIDs, user IDs).
7. [ ] Database schema changes lack down migration scripts or violate the Expand/Contract pattern.

---

## 9. Severity Mapping Reference (`P0`–`P3`)

| Level | Semantic Severity | Definition | Pipeline Action |
| :---: | :---: | :--- | :--- |
| **P0** | **CRITICAL** | Invariant break (`RULE-DATA`, `RULE-RES`), race condition, data corruption, panic. | **BLOCKS MERGE** |
| **P1** | **HIGH** | Acceptance criteria failure (`TC-xx`), missing error branch, spec omission. | **MANDATORY FIX** |
| **P2** | **MEDIUM** | Code maintainability smell, sub-optimal query, missing documentation. | **FIX OR EXPLAIN** |
| **P3** | **LOW** | Minor style, nit, variable naming. | **ADVISORY** |
