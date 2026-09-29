# AGENTS.md — Runtime binding for OpenSWE

> This file is NOT the source of truth. The rules, spec templates, and JSON
> contracts live in `docs/rules/*`, `docs/architecture/*`, and
> `docs/architecture/schemas/*`. This file only maps our quad-agent architecture
> onto the OpenSWE runtime. If anything here conflicts with `docs/rules/*`,
> `docs/rules/*` wins.

## Canonical sources (READ before acting)
- TRDs / Product Req → docs/trd/*.md (multi-version TRDs for roadmap & features)
- Domain invariants  → docs/rules/domain_invariants.md (RULE-ARCH/DATA/RES/SEC/EVT/OBS)
- Spec decomposition → docs/rules/spec_decomposition_rules.md (RULE-REQ/TEST/IMPL/PLAN + Gate B)
- Reviewer protocol  → docs/rules/reviewer-protocol.md (RULE-REV)
- Review dimensions  → docs/rules/review-dimensions.md
- Per-agent detail   → docs/architecture/04-planner-agent.md, 05-coding-agent.md, 06-reviewer-agent.md
- JSON contracts     → docs/architecture/schemas/*.json (used verbatim)

## Agent roles → OpenSWE
- Planner   → OpenSWE planning phase. Read-only over code. Output: req/test/impl
              specs + artifacts/plans/<feature>.json conforming to execution_plan.schema.json.
              Presents the plan at Human Gate 2 in chat and waits for approval. Does NOT open a PR.
- Coder     → OpenSWE Programmer phase. TDD Red→Green. Obey domain_invariants.md.
              Carries out implementation once the plan is approved, runs Gate B & Gate C, and opens the feature PR.
- Reviewer  → Reviewer skill or auditor. DISCOVERY-ONLY, never edits
              code. Output conforms to review_findings.schema.json. Mode A (audit)
              and Mode B (PR-comment triage) per reviewer-protocol.md.
- Closer & Scribe → SKIPPED for this demo (post-merge Jira/RFC/DAG). Do not run.

## Gate mapping
- Human Gate 2 (approve plan/spec)   → OpenSWE in-chat plan approval. Planner presents the plan card and awaits user sign-off.
- Machine Gate B (traceability/DAG)  → Pre-PR local check & CI .github/workflows/gate-b.yml (bin/gate_b_verifier.py).
- Machine Gate C (build/test/cover)  → Pre-PR local check & CI .github/workflows/gate-c-ci.yml.
- Human Gate 3 (merge)               → GitHub PR review & branch protection (required checks: gate-b, gate-c).
- Session Continuation               → continue in the same thread (Planner ➔ Coder handoff upon approval).
- Fresh Session                      → start a new task for a new feature.

## Gate B input binding & Date-Prefixed Naming (REQUIRED)
To prevent naming collisions as features accumulate, ALL specification documents and execution plans MUST be prefixed with the generation date (`YYYY-MM-DD-`):
- TRD / ReqSpec: `specs/reqs/YYYY-MM-DD-<domain>-<feature>.md`
- TestSpec:     `specs/tests/<domain>/YYYY-MM-DD-spec.md` (or `specs/tests/YYYY-MM-DD-<domain>-spec.md`)
- ImplSpec:     `specs/impls/YYYY-MM-DD-<domain>-<feature>.md`
- ExecutionPlan: `artifacts/plans/YYYY-MM-DD-<feature>.json`

## Continuous Single Responsibility PR Workflow
Instead of opening an intermediate spec-only PR that blocks progress, the lifecycle follows a seamless, continuous flow:
1. **Planning Phase (`/planner`)**:
   - Parse TRD, explore AST, and formulate date-prefixed specs and `execution_plan.json`.
   - Run local Gate B pre-check:
     `python bin/gate_b_verifier.py specs/reqs/*.md specs/tests/*/*.md specs/impls/*.md artifacts/plans/*.json`
   - Present the **Human Gate 2 Summary Card** (Scope, Domain, Test Scenarios with Violation Signals, and DAG) to the user in chat.
   - **DO NOT open a GitHub PR in the Planner phase.** Await user approval.
2. **Implementation Phase (`/coder`)**:
   - Triggered when user approves the plan ("Approved", "Setuju") or runs `/coder <plan_path>`.
   - Checkout branch `feat/<domain>-<feature>` (or stacked branches if > 500 LOC per RULE-PLAN-004).
   - Write spec files and proceed with strict TDD (Phase Red: scaffold failing tests ➔ Phase Green: implement minimal clean architecture).
   - Verify Gate B (`bin/gate_b_verifier.py`) and Gate C (`go test -v -race ./...` & `golangci-lint`).
   - Commit and open the Feature Pull Request (containing specs + implementation + tests).
   - Present PR link for Human Gate 3 merge review.

## Runtime notes
- Commit Identity: The repository sandbox is pre-configured with author Mario Zulkarnain <2088012+mariozul@users.noreply.github.com>. When transitioning across turns or executing TDD after plan approval, ALWAYS reuse this configured author identity or the thread's initial sender context. NEVER halt, block, or refuse execution due to deduplicated turn metadata.
- Subagents: Do NOT invoke external subagent tools like `task` if subagent provider API keys are unset; execute steps directly within the session.
- Collision Guard: Disjoint file sets enforced by Gate B verifier (`RULE-PLAN-007` and File Collision Guard).
- Worktrees: OpenSWE manages its own sandboxes.

