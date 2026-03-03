# Module Boundaries

## Purpose

Defines ownership boundaries for the initial repository layout.

## Modules

- `agent/`
  - data-plane worker runtime
  - parsing, fingerprinting, aggregation, signal emit
  - owns local spool interactions and source-side flow control

- `collector/`
  - ingest adapters and buffering interfaces
  - authentication/admission and durable append-only commit path

- `correlator/`
  - stateful multi-event detection
  - consumes signals, structured events, and aggregates
  - raw ingestion is on-demand only

- `shared/`
  - schemas, protocol contracts, and common utilities
  - versioning rules for cross-component compatibility

- `ui/`
  - observability and control-plane interfaces
  - no ingest-path authority; control actions are API mediated

- `tests/`
  - load, burst, drift, and degradation simulations

## Dependency Rule

Application modules (`agent`, `collector`, `correlator`, `ui`) may depend on `shared`.
`shared` must not depend on application modules.
