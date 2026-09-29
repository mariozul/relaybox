# 04. Planner Agent Specification (The Architect)

> **Architecture Navigation:**  
> • [01. Global Pipeline Flow](01-global-flow.md) (Macro SDLC Lifecycle)  
> • [02. Multi-Agent Interaction Flow](02-agent-interaction-flow.md) (Agent Interaction Matrix & Runtime Scenarios)  
> • [03. Inter-Agent Engine Contracts](03-engine-contracts.md) (Data Payload Schemas)  
> • **04. Planner Agent Specification** (Current - The Architect)  
> • [05. Coding Agent Specification](05-coding-agent.md) (The Engineer)  
> • [06. Reviewer Agent Specification](06-reviewer-agent.md) (The Auditor)  
> • [07. Closer & Scribe Agent Specification](07-closer-scribe-agent.md) (The Finisher)  
> • [08. Orchestrator Engine & Runtime Harness](08-orchestrator-engine.md) (The Control Plane)

---

## 1. Persona, Cognitive Mandate & Boundaries

* **Persona:** Principal Software Architect & Technical Lead (The Brain).
* **Primary Objective:** Ingest design-ready feature specifications (`docs/implspecs/*.md` or TRD), explore target codebase AST structures, identify affected module boundaries, and decompose work into an unambiguous, verifiable, and non-blocking execution plan.
* **Tool Permissions:** **Strictly Read-Only**:
  * **AST & Code Intelligence:** Symbol discovery, reference navigation, implementation lookup, and declaration tracing (LSP/AST).
  * **File Inspection:** Read-only file viewer (`read`, `grep`, `glob` — cannot edit, replace, or delete files).
  * **Subagent Delegation (`task`):** Confined exclusively to concurrent read-only exploration and deep dependency tracing. Any subagent spawned by the Planner inherits the exact same read-only mandate and is strictly prohibited from invoking mutating tools (`write`, `edit`, `bash`).
* **Prohibited Actions:** Strictly forbidden from creating files in the source workspace, modifying source code, running mutating shell commands, pushing git branches, or delegating tasks to mutating workers. (All plan specifications are piped to artifact output streams by the orchestrator).

---

## 2. The Planning Lifecycle (S1–S4 + Machine Gate B + Human Gate 2)

The Planner Agent operates across a structured, multi-stage pipeline designed to eliminate ambiguity before expensive coding compute is dispatched:

```text
┌────────────────────────────────────────────────────────────────────────┐
│ [S1] CLASSIFY: Scope & Blast-Radius Triage                             │
│ • Ingest approved TRD / ImplSpec (FR-xx)                               │
│ • Classify task tier (T1–T5) & blast radius (LOW vs MEDIUM/HIGH)       │
│ • Map cross-service dependencies and database migration requirements   │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ [S2] GEN-TESTSPEC: Test-Driven Specification                           │
│ • Ingest Invariants Registry (docs/rules/domain_invariants.md)         │
│ • Formulate test cases (TC-xx) mapped to FR-xx                         │
│ • Embed explicit "Violation Signal" for each TC-xx (sentinel trap)     │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ 🛑 BLOCKING GATE: S3 is BLOCKED
                                    │ until S2 test scenarios are locked!
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ [S3] GEN-EXECSPEC: Task Decomposition & PR Slicing                     │
│ • Deconstruct feature using 7-Layer Clean Architecture reference       │
│ • Enforce "Contracts, Not Implementation" (struct/signature only)      │
│ • Slice into atomic, reviewable PRs (<= 500 LOC per PR)                │
│ • Build execution DAG with explicit depends_on edges                   │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ [S4] EXEC-READINESS: Concurrency & Collision Guard                     │
│ • Enforce File Collision Guard: isolate target_files whitelist         │
│ • Verify disjoint mutable file sets for parallel tasks                 │
│ • Auto-serialize conflicting file tasks into sequential DAG edges      │
│ • Compile execution_plan.json payload                                  │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ [S4.B] MACHINE GATE B: Spec Drift & Traceability Engine                │
│ • Execute bin/gate_b_verifier.py                                       │
│ • Enforce 100% RTM Traceability Score (zero omission defects)          │
│ • Enforce PR Sizing limit (<= 500 LOC per subtask)                     │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ Passed
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│ HUMAN GATE 2: Specification & Plan Approval Sign-Off                   │
│ • Render interactive plan summary card (Scope, Impact, Tests, DAG)     │
│ • Await human sign-off before dispatching compute to Coding Agent      │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Foundational Planning Principles (BRD-Aligned)

To prevent the failure modes observed in naive LLM planning, the Planner Agent enforces three mandatory architectural constraints derived from production BRD standards:

### 3.1 The S2 ➔ S3 Blocking Gate (TDD-First Planning)
* **The Rule:** Stage S3 (`gen-execspec`) is **STRICTLY BLOCKED** until Stage S2 (`gen-testspec`) is completed and validated.
* **Rationale:** *"You must know what you have to prove before planning how to build it."* Planning implementation steps before test cases induces cognitive confirmation bias.
* **The Violation Signal Requirement:** Every test case (`TC-xx`) formulated by the Planner must include a mandatory **Violation Signal**—a single explicit sentence stating:
  > *"What subtle code change or omitted safeguard would silently invalidate this test case?"*  
  *(Example: `"Removing the SELECT ... FOR UPDATE lock or moving balance deduction outside the SQL transaction silently breaks balance integrity."`)*  
  This signal serves as the primary sentinel utilized by the Reviewer Agent during PR evaluation.

### 3.2 "Contracts, Not Implementation"
* **The Rule:** When decomposing tasks in `execution_plan.json` or `ImplSpec`, the Planner specifies **abstract contracts, entity structs, and interface method signatures only**.
* **Prohibited Detail:** The Planner is **strictly forbidden** from writing concrete method bodies, internal business algorithms, or raw SQL queries inside the plan.
* **Boundary Heuristic:**
  > *"If it can be deduced from architectural contracts, interfaces, and data models ➔ include it. If an engineer or Coding Agent must decide internal logic during local red-green testing ➔ omit it."*

### 3.3 7-Layer Clean Architecture PR Sizing Reference
When slicing medium or large initiatives into multiple atomic Pull Requests, the Planner aligns with the standard Clean Architecture layering used in enterprise Go services (`ads` / `commercial-be`):

```text
Layer 1: Database Migrations (Schema DDL, indexes, table alters)
Layer 2: Domain Entities & Repository Interfaces (pure contracts, no external imports)
Layer 3: Repository Implementation & Data Stores (SQL queries, testcontainers)
Layer 4: Domain Service & Business Logic (pure domain use-cases, unit tests)
Layer 5: Transport Handlers (HTTP routes, gRPC servers, request/response DTOs)
Layer 6: Event Subscribers & Background Workers (PubSub/Kafka consumers, idempotency)
Layer 7: Application Wiring & Dependency Injection (cmd/server/main.go)
```

* **Sizing Boundary:** Each PR must be $\le 500$ LOC and must compile and pass tests independently (*green at each step*).

---

## 4. AST Exploration Methodology & Blast Radius Analysis

To prevent hallucinated symbol usage, the Planner Agent explores the target codebase using structured AST Code Intelligence:

1. **Symbol Inventory:** Scans target packages to map existing interfaces, structs, function signatures, and context parameters.
2. **Implementation Lookup:** Locates existing implementations of abstract interfaces to mimic established repository patterns (e.g., error wrapping, transaction management, logger injection).
3. **Reference Mapping & Blast Radius:** Traces callers of modified symbols across the repository to determine whether downstream consumers are impacted.
4. **Dependency Direction Guard:** Confirms that proposed changes point dependencies inward toward business logic (`RULE-ARCH-02`).

---

## 5. Concurrency Hygiene: File Collision Guard

When decomposing multi-task features or evaluating parallel DAG dispatchability:
* **Disjoint File Sets Extraction:** The Planner explicitly isolates mutable target files in `subtasks[].target_files` for every subtask.
* **Collision Detection:** If two planned tasks contain overlapping mutable files, the Planner flags a collision risk.
* **Automatic DAG Edge Serialization:** Rather than permitting hazardous race conditions across parallel worktrees, the Planner injects a strict `depends_on` dependency edge, guaranteeing sequential execution for conflicting file sets while allowing orthogonal tasks to run in parallel.

---

## 6. Standard Artifact Output: `execution_plan.json`

The Planner Agent always outputs a strictly typed JSON contract conforming to [`docs/architecture/03-engine-contracts.md`](03-engine-contracts.md) and [`docs/architecture/schemas/execution_plan.schema.json`](schemas/execution_plan.schema.json), consumed directly by the Coding Agent:

```json
{
  "task_id": "COMTECH-123",
  "feature_name": "order-v2",
  "blast_radius": "LOW",
  "total_subtasks": 2,
  "subtasks": [
    {
      "id": "ST-01",
      "name": "Order repository implementation",
      "layer": "repository",
      "estimated_loc": 120,
      "depends_on": [],
      "test_cases": ["TC-ORDER-001"],
      "target_files": [
        "internal/repository/order.go",
        "internal/repository/order_test.go"
      ],
      "verification_command": "go test -race -v ./internal/repository/..."
    },
    {
      "id": "ST-02",
      "name": "Order domain service implementation",
      "layer": "service",
      "estimated_loc": 180,
      "depends_on": ["ST-01"],
      "test_cases": ["TC-ORDER-002"],
      "target_files": [
        "internal/service/order.go",
        "internal/service/order_test.go"
      ],
      "verification_command": "go test -race -v ./internal/service/..."
    }
  ],
  "verification_commands": [
    "go test -race -v ./internal/service/...",
    "golangci-lint run ./internal/service/..."
  ]
}
```

---

## 7. Machine Gate B & Human Gate 2: Specification & Plan Sign-Off

Before compute is dispatched to the Coding Agent, the pipeline executes a two-tier verification checkpoint:

### 7.1. Machine Gate B (Automated Spec Drift & Traceability Engine)
Executed automatically via `bin/gate_b_verifier.py`:
* **100% RTM Traceability Score:** Enforces $\text{Score} = \frac{|\{FR \in \text{TRD} \mid \exists TC \in \text{TestSpec}\}|}{|\text{TRD}_{FR}|} \times 100\% = 100\%$. Any omission defect halts execution.
* **PR Sizing Guard (RULE-PLAN-004):** Validates all subtasks satisfy $\text{estimated\_loc} \le 500$.
* **Violation Signal Sentinel (RULE-TEST-004):** Validates presence of expected failure traps in `testspec.md`.
* **DAG Acyclicity:** Verifies topological validity without circular dependencies.

### 7.2. Human Gate 2 (Team Member Plan Sign-Off)
Following Machine Gate B success, the Planner presents an interactive summary approval card in the communication channel (e.g. Google Chat or Terminal CLI):

* **Feature Scope:** Target `FR-xx` requirements addressed.
* **Impact Matrix:** Whitelisted mutable files (`target_files`) and layer classification.
* **Test Plan & Invariants:** Target `TC-xx` scenarios and corresponding `RULE-xx` invariants enforced.
* **Dependency DAG:** Execution sequence showing parallel vs serialized PR subtasks.
* **Approval Decision:**
  * **Approved:** Dispatches `execution_plan.json` to Phase 2 (Coding Agent).
  * **Revision Requested:** Loops back with human feedback to refine the plan without burning coding compute.
