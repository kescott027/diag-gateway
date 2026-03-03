# agent

Lightweight data-plane worker for collection, parsing, fingerprinting, aggregation, and signal emission.

Current packages:

- `agent/spool` disk-backed FIFO queue for durable retry buffering.
- `agent/backpressure` deterministic pressure-level evaluation for spool saturation control.
- `agent/cursor` restart-safe persistent file cursor store.
