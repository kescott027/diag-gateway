# Log Streaming & Diagnostics Platform

Local-first, secure log streaming and diagnostics platform.

## Repository Layout

- `agent/` data-plane worker for parsing, fingerprinting, aggregation, signal emission
- `collector/` ingest adapters and buffering interfaces
- `correlator/` stateful multi-event detection engine
- `shared/` schemas, protocol contracts, common utilities
- `ui/` observability and control-plane interface
- `tests/` load, burst, drift, and degradation simulations
- `docs/` architecture, security, protocol, operations, planning, AI, future roadmap
- `project_management/` backlog, active sprint, decision log, sprint ledger

## Development Stage

Current stage is `READY` preparation only. No production implementation starts until sprint framework and quality/security gates are satisfied.
