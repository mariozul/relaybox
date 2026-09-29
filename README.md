# Relaybox

Reference demo of a Planner → Coder → Reviewer agent pipeline running on **OpenSWE**.

The rules, spec templates, and JSON contracts are **engine-agnostic** and reused
verbatim from our omp setup. Only the runtime engine differs — here it is OpenSWE.

## Layout

| Path | Purpose | Ownership |
|------|---------|-----------|
| `docs/rules/` | Domain invariants & agent protocols (single source of truth) | Verbatim — agents must not edit |
| `docs/architecture/04·05·06-*.md` | Per-agent behavior specs | Verbatim |
| `docs/architecture/schemas/*.json` | JSON contracts (plan, review, coder, remediation, telemetry) | Verbatim |
| `bin/gate_b_verifier.py` | Gate B verifier (traceability / DAG / PR-sizing) — pure stdlib | Verbatim |
| `AGENTS.md` | Maps the quad-agent architecture onto OpenSWE | OpenSWE layer |
| `.github/workflows/gate-b.yml` | Spec traceability gate (Machine Gate B) | OpenSWE layer |
| `.github/workflows/gate-c-ci.yml` | Build / test / coverage gate (Machine Gate C) | OpenSWE layer |
| `docs/reqspecs·testspecs·implspecs/`, `artifacts/plans/` | Planner outputs (Stage 1) | Generated |

## Two-stage flow

1. **Stage 1 — spec** on `spec/<id>`: Planner emits req/test/impl specs +
   `artifacts/plans/<feature>.json`, writes the ingested TRD to
   `docs/reqspecs/<domain>-<feature>.md`, opens a PR → `gate-b` + human review → merge.
2. **Stage 2 — implementation** on `feat/<id>`: Coder implements via TDD (Red→Green);
   Reviewer (a `task` subagent, discovery-only) audits against the schemas →
   PR → `gate-c` + human review → merge.

The only real merge gate is **GitHub branch protection** (required checks:
`gate-b`, `gate-c`) — this is Human Gate 3. Closer & Scribe (Jira/RFC/DAG) is
out of scope for this demo.

## Local development

```bash
docker compose up -d      # Postgres
make test                 # go test -race -timeout 90s ./...
make lint                 # golangci-lint run ./...
make build
