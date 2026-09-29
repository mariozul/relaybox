# AGENTS.md — Runtime binding for OpenSWE

> This file is NOT the source of truth. The rules, spec templates, and JSON
> contracts live in `docs/rules/*`, `docs/architecture/*`, and
> `docs/architecture/schemas/*`. This file only maps our quad-agent architecture
> onto the OpenSWE runtime. If anything here conflicts with `docs/rules/*`,
> `docs/rules/*` wins.

## Canonical sources (READ before acting)
- Domain invariants  → docs/rules/domain_invariants.md (RULE-ARCH/DATA/RES/SEC/EVT/OBS)
- Spec decomposition → docs/rules/spec_decomposition_rules.md (RULE-REQ/TEST/IMPL/PLAN + Gate B)
- Reviewer protocol  → docs/rules/reviewer-protocol.md (RULE-REV)
- Review dimensions  → docs/rules/review-dimensions.md
- Per-agent detail   → docs/architecture/04-planner-agent.md, 05-coding-agent.md, 06-reviewer-agent.md
- JSON contracts     → docs/architecture/schemas/*.json (used verbatim)

## Agent roles → OpenSWE
- Planner   → OpenSWE planning phase. Read-only over code. Output: req/test/impl
              specs + artifacts/plans/<feature>.json conforming to execution_plan.schema.json.
- Coder     → OpenSWE Programmer phase. TDD Red→Green. Obey domain_invariants.md.
- Reviewer  → Programmer subagent via the `task` tool. DISCOVERY-ONLY, never edits
              code. Output conforms to review_findings.schema.json. Mode A (audit)
              and Mode B (PR-comment triage) per reviewer-protocol.md.
- Closer & Scribe → SKIPPED for this demo (post-merge Jira/RFC/DAG). Do not run.

## Gate mapping
- Human Gate 2 (approve plan/spec)   → OpenSWE built-in plan-approval interrupt.
- Machine Gate B (traceability/DAG)  → CI .github/workflows/gate-b.yml (bin/gate_b_verifier.py).
- Machine Gate C (build/test/cover)  → CI .github/workflows/gate-c-ci.yml.
- Human Gate 3 (merge)               → GitHub branch protection (required checks: gate-b, gate-c).
- Session Continuation               → continue in the same thread.
- Fresh Session                      → start a new task for a new feature.

## Gate B input binding & Date-Prefixed Naming (REQUIRED)
To prevent naming collisions as features accumulate, ALL specification documents and execution plans MUST be prefixed with the generation date (`YYYY-MM-DD-`):
- TRD / ReqSpec: `specs/reqs/YYYY-MM-DD-<domain>-<feature>.md`
- TestSpec:     `specs/tests/<domain>/YYYY-MM-DD-spec.md` (or `specs/tests/YYYY-MM-DD-<domain>-spec.md`)
- ImplSpec:     `specs/impls/YYYY-MM-DD-<domain>-<feature>.md`
- ExecutionPlan: `artifacts/plans/YYYY-MM-DD-<feature>.json`

## Atomic Stage 1 PR Invariant (Protocol Rule)
A Stage 1 PR MUST NOT be opened incrementally with missing spec files. Opening a PR with missing files immediately causes Gate B CI failure.
The Planner Agent MUST:
1. Complete all 4 documents above in the local sandbox workspace.
2. Run local pre-PR verification:
   `python bin/gate_b_verifier.py specs/reqs/*.md specs/tests/*/*.md specs/impls/*.md artifacts/plans/*.json`
3. ONLY after local Gate B returns `PASS`, commit all 4 files and open the PR targeting `main`.

## Two-stage flow
1. Stage 1 (spec): branch `spec/<id>` → commit all 4 date-prefixed spec files atomically → open PR → gate-b + human review → merge.
2. Stage 2 (impl): branch `feat/<id>` → TDD implementation → open PR → gate-c + human review → merge.

## Runtime notes (no direct OpenSWE equivalent)
- orchestrator.sh STEP 2.0 auto-serialize → none. Rely on Planner discipline plus
  the File Collision Guard pre-check in Gate B.
- `.omp/wt/` worktree pruning → N/A; OpenSWE manages its own sandboxes.
