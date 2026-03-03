# Current Sprint

## Sprint
- Sprint 52
- Sprint Mode: Accelerated (minutes/hours)
- Start Date: 2026-03-03
- Start Time: 14:56:11 CST
- Target Throughput: minimum 10 sprints per day
- Target End Time (Projected): 2026-03-03 15:00:01 CST
- Actual End Time: Pending
- Status: Blocked (Decision Required)

## Sprint Goal
Implement routing rules by source/tag/path.

## Selected Stories
- R3-04 Routing rules

## Rationale for Selection
- Dependencies from grouping (R3-02) and config/audit foundations are complete; routing is the next prerequisite for forwarding stories.

## Acceptance Criteria Summary
- Rule model supports selectors over `source_id`, `group_id`, `tags`, and logical path patterns.
- Deterministic evaluation order and terminal action semantics are defined.
- Unit tests cover precedence, conflict resolution, and default-route behavior.

## Definition of Done (Sprint)
- Documentation updates complete.
- Security review summary captured.
- Test summary captured (or explicit no-code test note).
- Architectural review summary captured.
- Decision matrix updated for security/protocol/data-model-impacting choices.
- Sprint completion time logged for projection baseline updates.

## Risks
- Ambiguous rule precedence/default behavior can create unsafe or non-deterministic forwarding outcomes.

## Required Architectural Review Areas
- Rule language scope (glob vs regex) and validation boundaries.
- Conflict resolution policy (first-match vs last-match vs priority).
- Default action when no rule matches (drop vs local-only vs pass-through).
