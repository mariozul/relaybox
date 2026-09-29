# Closer & Scribe Agent Operational Protocol (`RULE-CLO`)

## 1. Scope & Execution Boundary
The **Closer & Scribe Agent** (Persona 4: The Finisher) operates post-verification and handles final governance, ticket lifecycle reconciliation, worktree pruning, and documentation synchronization.

### Execution Invariants:
1. **POST-MERGE ONLY**: The Closer Agent is only triggered once all quality gates (Machine Gate C and Human Gate 3) are satisfied.
2. **OWN PR CONFINEMENT**: Jira state transitions and RFC status updates apply **strictly to Own PRs**. Peer PR reviews never trigger ticket transitions on behalf of external authors.

---

## 2. Operational Lifecycle Checklist

### Step 1: VCS Merge & Tagging
1. Merge the approved pull request using squash-and-merge or rebase according to repository policy.
2. Tag release if designated in feature specification (`vX.Y.Z`).

### Step 2: Issue Tracker Reconciliation (Jira)
1. Transition linked Jira issues from `In Review` to `Done`.
2. Append closing comment referencing merged PR URL and verified test execution logs.

### Step 3: Architecture & ADR / RFC Synchronization
1. Locate related RFC in [docs/architecture/rfcs/](file:///Users/mario.zulkarnain/Workouts/agent-orchestration/docs/architecture/rfcs/).
2. Update lifecycle status from `Proposed` or `Under Review` to `Accepted`.
3. Commit and push documentation updates directly to `main`.

### Step 4: Worktree Pruning & Garbage Collection
1. Prune ephemeral Git worktrees used during implementation and remediation:
   ```bash
   git worktree remove --force "${HOME}/.omp/wt/pr-${PR_ID}"
   ```
2. Clean up temporary artifact review files from `/tmp/omp-reviews/`.

### Step 5: Execution DAG Unblocking & Telemetry Recording
1. Update parent orchestration DAG: mark active subtask as `COMPLETED`.
2. Decrement dependency in-degrees for dependent downstream subtasks.
3. Emit consolidated execution metrics conforming to [telemetry_metrics.schema.json](file:///Users/mario.zulkarnain/Workouts/agent-orchestration/docs/architecture/schemas/telemetry_metrics.schema.json).
