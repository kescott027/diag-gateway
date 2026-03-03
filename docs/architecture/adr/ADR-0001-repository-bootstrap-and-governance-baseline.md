# ADR-0001: Repository Bootstrap and Governance Baseline

- Date: 2026-03-03
- Status: Accepted
- Owners: Platform Engineering

## Context

The repository started without stable documentation taxonomy, sprint governance files, or module boundaries.
Implementation without these controls creates protocol/security drift and low decision traceability.

## Decision

Adopt normalized documentation structure under `docs/`, formal sprint governance under `project_management/`, and explicit source boundaries (`agent`, `collector`, `correlator`, `shared`, `ui`, `tests`).

## Options Considered

1. Start implementation immediately with ad-hoc planning
2. Bootstrap governance and architecture docs first

## Consequences

- Positive: stronger delivery discipline and architecture continuity
- Tradeoff: initial setup time before code implementation
- Risk: sprint overhead if governance artifacts are not kept current

## Validation

- Required docs are discoverable in normalized paths
- Sprint artifacts exist and are updated per sprint lifecycle rules
- Decision matrix logs architecture-impact choices

## References

- ../ARCHITECTURE.md
- ../PROJECT_VISION.md
- ../../project_management/SPRINT_RULES.md
