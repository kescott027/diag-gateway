# Sprint Management Rules

## Sprint Cadence (Accelerated)

- Sprint duration uses minutes/hours, not multi-week windows.
- Target throughput is minimum 10 completed sprints per day.
- Every sprint record must include `Start Time`, `Actual Completion Time`, and `Duration`.
- Projected completion times must be recalculated after each sprint close.

Projection method:
- If fewer than 3 completed sprints exist, use provisional 60-minute sprint duration.
- When 3 or more completed sprints exist, use rolling average duration of the last 3 completed sprints.
- Next sprint projected completion time = next sprint start time + current projection duration.

## Sprint Lifecycle

1. Backlog review
2. Story selection
3. Dependency analysis
4. Architectural impact assessment
5. Risk scoring
6. Sprint definition
7. Implementation
8. Test additions
9. Documentation updates
10. Security impact assessment
11. Refactoring pass
12. Performance sanity check
13. Sprint close documentation
14. Move `Sprint_n.md` to `completed_sprints/`

## Close Criteria (Mandatory)

No sprint closes without:

- documentation updates
- security review summary
- test summary
- architectural review summary
- decision matrix updates for architectural-impact decisions
- start/completion timestamps captured and projection metrics updated

## Coherence Cadence

Every 3 sprints perform and log:

- architecture coherence review
- refactor debt evaluation
- naming consistency review
- config surface review
- plugin security boundary review (if plugin framework enabled)

## Change Governance

- No feature work outside selected sprint stories unless explicitly approved.
- No protocol invariant breaks.
- No remote code execution without signature validation.
- No AI/plugin implementation before core streaming stability.
