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

## Gate B input binding (REQUIRED so CI has the files)
Stage 1 MUST write the ingested TRD to specs/reqs/<domain>-<feature>.md
(even for a Mini-TRD / dual-duty input), and write the plan to
artifacts/plans/<feature>.json, so gate-b CI can resolve all four inputs
(TRD, testspec, implspec, plan) matching .github/workflows/gate-b.yml:
- TRD: specs/reqs/<domain>-<feature>.md
- TestSpec: specs/tests/<domain>/spec.md
- ImplSpec: specs/impls/<domain>-<feature>.md
- Plan: artifacts/plans/<feature>.json

## Two-stage flow
1. Stage 1 (spec): branch `spec/<id>` → commit specs/reqs + specs/tests + specs/impls +
   artifacts/plans/<feature>.json → open PR → gate-b + human review → merge.
2. Stage 2 (impl): branch `feat/<id>` → TDD implementation → open PR → gate-c +
   human review → merge.

## Runtime notes (no direct OpenSWE equivalent)
- orchestrator.sh STEP 2.0 auto-serialize → none. Rely on Planner discipline plus
  the File Collision Guard pre-check in Gate B.
- `.omp/wt/` worktree pruning → N/A; OpenSWE manages its own sandboxes.
