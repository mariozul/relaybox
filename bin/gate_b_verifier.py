#!/usr/bin/env python3
"""
Gate B Automated Verification Engine
====================================
Formal verification script executed at Gate B in the Orchestrator pipeline.
Validates:
1. RTM (Requirements Traceability Matrix) Completeness:
   Traceability Score = (|Covered Functional Requirements| / |Total Requirements|) * 100%
   Must equal 100% (Zero Omission Defects).
2. Phantom Feature Check:
   No test cases mapped to non-existent functional requirements.
3. PR Sizing Constraint:
   Every subtask in plan.json must satisfy estimated_loc <= 500.
4. Violation Signal Sentinel:
   testspec.md must contain explicit expected failure signals (RULE-TEST-004).
5. Schema & DAG Integrity:
   plan.json must satisfy structural constraints and form a valid Directed Acyclic Graph (DAG).
"""

import sys
import os
import json
import re
from pathlib import Path
from typing import Dict, List, Set, Any, Tuple

# ANSI Color Codes
GREEN = "\033[0;32m"
RED = "\033[0;31m"
YELLOW = "\033[1;33m"
BLUE = "\033[0;34m"
NC = "\033[0m"

def print_header(title: str):
    print(f"\n{BLUE}=== {title} ==={NC}")

def print_pass(msg: str):
    print(f"[{GREEN}PASS{NC}] {msg}")

def print_fail(msg: str):
    print(f"[{RED}FAIL{NC}] {msg}")

def print_warn(msg: str):
    print(f"[{YELLOW}WARN{NC}] {msg}")

def extract_requirements_from_trd(trd_text: str) -> Set[str]:
    """Finds all requirement identifiers (e.g. FR-01, REQ-001, RULE-DATA-01)."""
    # Matches FR-*, REQ-*, and business RULE-* tags
    matches = re.findall(r'\b((?:FR|REQ|RULE)-[A-Z0-9_-]+)\b', trd_text)
    return set(matches)

def extract_covered_requirements_from_testspec(testspec_text: str) -> Tuple[Set[str], List[str]]:
    """Extracts requirement tags and test case IDs referenced in test specifications."""
    matches = re.findall(r'\b((?:FR|REQ|RULE)-[A-Z0-9_-]+)\b', testspec_text)
    covered = set(matches)
    
    # Check for test cases
    test_cases = re.findall(r'\b(TC-[A-Z0-9_-]+|Test[A-Z0-9_]+)\b', testspec_text)
    return covered, test_cases

def check_violation_signals(testspec_text: str) -> bool:
    """Checks if test specifications define violation signals (RULE-TEST-004)."""
    keywords = [
        "violation signal",
        "expected failure",
        "fail with",
        "assertionerror",
        "raises",
        "http 40",
        "error code",
        "reject"
    ]
    lower_text = testspec_text.lower()
    return any(k in lower_text for k in keywords)

def validate_plan_schema_and_dag(plan_path: Path) -> Tuple[bool, List[str], Dict[str, Any]]:
    """Validates structural constraints of plan.json without external jsonschema dependency."""
    errors = []
    if not plan_path.is_file():
        return False, [f"Plan file not found: {plan_path}"], {}
    
    try:
        with open(plan_path, "r", encoding="utf-8") as f:
            plan = json.load(f)
    except Exception as e:
        return False, [f"JSON Parse Error: {e}"], {}
    
    # Top-level required field
    if "subtasks" not in plan:
        errors.append("Missing top-level required field: 'subtasks'")
    elif not isinstance(plan["subtasks"], list) or len(plan["subtasks"]) == 0:
        errors.append("'subtasks' must be a non-empty array.")
        return False, errors, plan

    # RULE-PLAN-006: reqspec_triage block must be present and structurally valid
    if "reqspec_triage" not in plan:
        errors.append("Missing 'reqspec_triage' block (RULE-PLAN-006) — planner must record its req-to-reqspec decision.")
    else:
        tri = plan["reqspec_triage"]
        if not isinstance(tri, dict):
            errors.append("'reqspec_triage' must be an object.")
        else:
            if tri.get("decision") not in ("SKIP", "REQUIRED"):
                errors.append("reqspec_triage.decision must be 'SKIP' or 'REQUIRED'.")
            reasons = tri.get("reasons")
            if not isinstance(reasons, list) or len(reasons) == 0:
                errors.append("reqspec_triage.reasons must be a non-empty array (shown at Gate 2 for veto).")

    # Validate execution mode if specified
    if "execution_mode" in plan and plan["execution_mode"] not in ["sequential", "parallel", "dag"]:
        errors.append(f"Invalid execution_mode '{plan['execution_mode']}'. Must be sequential, parallel, or dag.")

    subtasks = plan.get("subtasks", [])
    subtask_ids = set()
    dependencies: Dict[str, List[str]] = {}

    allowed_layers = {"domain", "repository", "service", "transport", "adapter", "config", "test", "cross_cutting"}
    required_subtask_fields = ["name", "estimated_loc", "depends_on", "target_files"]

    # Canonical schema synchronization: dynamically inherit required properties from SSOT schema if present
    canonical_schema_path = Path(__file__).resolve().parent.parent / "docs" / "architecture" / "schemas" / "execution_plan.schema.json"
    if canonical_schema_path.is_file():
        try:
            with open(canonical_schema_path, "r", encoding="utf-8") as sf:
                cs = json.load(sf)
            subtask_item_def = cs.get("properties", {}).get("subtasks", {}).get("items", {})
            schema_req = subtask_item_def.get("required", [])
            if schema_req:
                required_subtask_fields = [f for f in schema_req if f != "id"]
            schema_layers = subtask_item_def.get("properties", {}).get("layer", {}).get("enum", [])
            if schema_layers:
                allowed_layers = set(schema_layers)
        except Exception:
            pass  # Gracefully fall back to built-in defaults

    for idx, st in enumerate(subtasks):
        st_id = st.get("id")
        if not st_id or not isinstance(st_id, str):
            errors.append(f"Subtask index {idx} missing valid 'id'.")
            continue
        if st_id in subtask_ids:
            errors.append(f"Duplicate subtask id found: '{st_id}'")
        subtask_ids.add(st_id)

        # Check required fields conforming to execution_plan.schema.json
        for field in required_subtask_fields:
            if field not in st:
                errors.append(f"Subtask '{st_id}' missing required field: '{field}'")

        # Layer enum validation if provided
        if "layer" in st and st["layer"] not in allowed_layers:
            errors.append(f"Subtask '{st_id}' has invalid layer '{st['layer']}'. Allowed: {', '.join(sorted(allowed_layers))}")

        # PR Sizing Limit Check
        est_loc = st.get("estimated_loc", 0)
        if not isinstance(est_loc, (int, float)):
            errors.append(f"Subtask '{st_id}' 'estimated_loc' must be a number.")
        elif est_loc > 500:
            errors.append(f"PR Sizing Violation (RULE-PLAN-004): Subtask '{st_id}' estimated_loc={est_loc} exceeds the hard limit of 500 LOC.")

        deps = st.get("depends_on", [])
        if not isinstance(deps, list):
            errors.append(f"Subtask '{st_id}' 'depends_on' must be an array.")
        else:
            dependencies[st_id] = deps

    # Validate dependency references
    for st_id, deps in dependencies.items():
        for dep in deps:
            if dep not in subtask_ids:
                errors.append(f"Subtask '{st_id}' depends on undefined subtask '{dep}'.")
            if dep == st_id:
                errors.append(f"Subtask '{st_id}' cannot depend on itself.")

    # DAG Cycle Detection using DFS
    visited = {}  # 0 = unvisited, 1 = visiting, 2 = visited
    for node in subtask_ids:
        visited[node] = 0

    def has_cycle(u: str) -> bool:
        visited[u] = 1
        for v in dependencies.get(u, []):
            if v not in visited:
                continue
            if visited[v] == 1:
                return True
            if visited[v] == 0:
                if has_cycle(v):
                    return True
        visited[u] = 2
        return False

    for node in subtask_ids:
        if visited[node] == 0:
            if has_cycle(node):
                errors.append("Cycle detected in subtask execution DAG.")
                break

    return len(errors) == 0, errors, plan


def check_collision_guard(plan: Dict[str, Any]) -> Tuple[List[str], List[str]]:
    """File Collision Guard (docs/architecture/08-orchestrator-engine.md §4.1).

    Subtasks that may run CONCURRENTLY (neither one depends on the other, directly
    or transitively) must have 100% disjoint target_files. Overlapping file sets of
    concurrent subtasks are reported as errors; overlapping subtasks that are already
    ordered by the DAG (dependency edge in either direction) are legal — that IS the
    auto-serialization the guard demands.

    Returns (errors, warnings)."""
    errors: List[str] = []
    warnings: List[str] = []

    subtasks = plan.get("subtasks", [])
    if not isinstance(subtasks, list) or len(subtasks) < 2:
        return errors, warnings

    # Build dependency adjacency (only edges to defined subtasks count)
    ids = {st.get("id") for st in subtasks if isinstance(st.get("id"), str)}
    deps: Dict[str, Set[str]] = {}
    for st in subtasks:
        sid = st.get("id")
        if not isinstance(sid, str):
            continue
        deps[sid] = {d for d in st.get("depends_on", []) or [] if isinstance(d, str) and d in ids}

    # Transitive closure of reachability (who is ordered w.r.t. whom)
    def reachable(src: str) -> Set[str]:
        seen: Set[str] = set()
        stack = list(deps.get(src, ()))
        while stack:
            n = stack.pop()
            if n in seen:
                continue
            seen.add(n)
            stack.extend(deps.get(n, ()))
        return seen

    ordered_pairs: Set[frozenset] = set()
    reach: Dict[str, Set[str]] = {}
    for sid in ids:
        reach[sid] = reachable(sid)
        for other in reach[sid]:
            ordered_pairs.add(frozenset((sid, other)))

    files_by_id: Dict[str, Set[str]] = {}
    for st in subtasks:
        sid = st.get("id")
        if isinstance(sid, str):
            files_by_id[sid] = {f for f in (st.get("target_files") or []) if isinstance(f, str)}

    checked = 0
    id_list = sorted(ids)
    for i, a in enumerate(id_list):
        for b in id_list[i + 1:]:
            pair = frozenset((a, b))
            if pair in ordered_pairs:
                continue  # DAG already serializes this pair
            overlap = files_by_id[a] & files_by_id[b]
            checked += 1
            if overlap:
                errors.append(
                    f"File Collision Guard violation: concurrent subtasks '{a}' and '{b}' "
                    f"share target_files ({', '.join(sorted(overlap))}). Inject a depends_on "
                    f"edge to serialize them, or disjoint the file sets."
                )
    if checked and not errors:
        warnings.append(f"Disjointness verified across {checked} concurrent subtask pairs (none ordered by DAG).")
    return errors, warnings


def main():
    if len(sys.argv) < 5:
        print(f"Usage: {sys.argv[0]} <TRD_PATH> <TESTSPEC_PATH> <IMPLSPEC_PATH> <PLAN_JSON_PATH>")
        sys.exit(2)

    trd_path = Path(sys.argv[1])
    testspec_path = Path(sys.argv[2])
    implspec_path = Path(sys.argv[3])
    plan_path = Path(sys.argv[4])

    print_header("Gate B Automated Verification Engine")
    print(f"TRD:       {trd_path}")
    print(f"TestSpec:  {testspec_path}")
    print(f"ImplSpec:  {implspec_path}")
    print(f"Plan JSON: {plan_path}")

    # Check file existence
    all_files_exist = True
    for p, name in [(trd_path, "TRD"), (testspec_path, "TestSpec"), (implspec_path, "ImplSpec"), (plan_path, "Plan JSON")]:
        if not p.is_file():
            print_fail(f"{name} file not found: {p}")
            all_files_exist = False

    if not all_files_exist:
        sys.exit(1)

    with open(trd_path, "r", encoding="utf-8") as f:
        trd_text = f.read()

    with open(testspec_path, "r", encoding="utf-8") as f:
        testspec_text = f.read()

    with open(implspec_path, "r", encoding="utf-8") as f:
        implspec_text = f.read()

    passed = True

    # 1. RTM Completeness & Omission Defect Check
    print_header("1. Requirements Traceability Matrix (RTM) Verification")
    trd_reqs = extract_requirements_from_trd(trd_text)
    covered_reqs, test_cases = extract_covered_requirements_from_testspec(testspec_text)

    if not trd_reqs:
        print_warn("No FR-xx tags identified in TRD. Assuming requirement extraction via headers or implicit scope.")
    else:
        missing = trd_reqs - covered_reqs
        phantom = (covered_reqs - trd_reqs)
        
        # Calculate Traceability Score
        score = ((len(trd_reqs) - len(missing)) / len(trd_reqs)) * 100.0
        print(f"Total Requirements Identified (TRD): {len(trd_reqs)} ({', '.join(sorted(trd_reqs))})")
        print(f"Requirements Covered in TestSpec:    {len(covered_reqs)} ({', '.join(sorted(covered_reqs))})")
        print(f"Test Cases Defined:                 {len(test_cases)}")
        print(f"Traceability Score:                 {score:.1f}%")

        if missing:
            print_fail(f"Omission Defect detected! The following {len(missing)} requirement(s) are NOT covered by tests:")
            for m in sorted(missing):
                print(f"  - {m}")
            passed = False
        else:
            print_pass("100% Traceability Score: All functional requirements mapped to test specifications.")

        if phantom:
            print_warn(f"Phantom Feature warning: {len(phantom)} requirement(s) in TestSpec not defined in TRD: {', '.join(sorted(phantom))}")

    # 2. Violation Signal Sentinel
    print_header("2. TestSpec Violation Signal Sentinel (RULE-TEST-004)")
    if check_violation_signals(testspec_text):
        print_pass("Violation signals and expected failure conditions are explicitly defined.")
    else:
        print_fail("Violation signals missing! TestSpec must specify expected failure outputs or error responses.")
        passed = False

    # 3. Plan Schema, DAG Integrity & PR Sizing Constraints
    print_header("3. Execution Plan Schema & Sizing Verification (RULE-PLAN-004)")
    plan_valid, plan_errors, plan = validate_plan_schema_and_dag(plan_path)

    if not plan_valid:
        print_fail(f"Execution plan verification failed with {len(plan_errors)} error(s):")
        for err in plan_errors:
            print(f"  - {err}")
        passed = False
    else:
        subtasks = plan.get("subtasks", [])
        total_loc = sum(st.get("estimated_loc", 0) for st in subtasks)
        print_pass(f"Plan schema valid. Mode: {plan.get('execution_mode')}, Subtasks: {len(subtasks)}, Total LOC: {total_loc}")
        for st in subtasks:
            loc = st.get("estimated_loc", 0)
            deps = st.get("depends_on", [])
            print(f"  * [{st.get('id')}] {st.get('name')} ({st.get('layer')}) - {loc} LOC - depends on: {deps or 'none'}")
        print_pass("All subtasks comply with the <= 500 LOC hard boundary.")

        # 4. File Collision Guard (§4.1): disjoint target_files among concurrent subtasks
        print_header("4. File Collision Guard (Disjoint target_files, RULE-PLAN-005)")
        collision_errors, collision_warnings = check_collision_guard(plan)
        for w in collision_warnings:
            print_warn(w)
        if collision_errors:
            for err in collision_errors:
                print_fail(err)
            passed = False
        else:
            print_pass("No file-set collisions among concurrently dispatchable subtasks.")

    # 5. Final Verdict
    print_header("Gate B Verification Summary")
    if passed:
        print(f"{GREEN}✓ GATE B VERIFICATION PASSED: Architectural specifications approved for code synthesis.{NC}\n")
        sys.exit(0)
    else:
        print(f"{RED}✗ GATE B VERIFICATION FAILED: Violations must be remediated before proceeding.{NC}\n")
        sys.exit(1)

if __name__ == "__main__":
    main()
