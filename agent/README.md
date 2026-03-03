# agent

Lightweight data-plane worker for collection, parsing, fingerprinting, aggregation, and signal emission.

Current packages:

- `agent/spool` disk-backed FIFO queue for durable retry buffering.
- `agent/backpressure` deterministic pressure-level evaluation for spool saturation control.
- `agent/cursor` restart-safe persistent file cursor store.
- `agent/config` validated atomic persistence for local watch-path/pattern configuration.
- `agent/fileid` cross-platform file identity detection with native and fallback modes.
- `agent/rotation` deterministic rotation/truncation cursor reassignment decisions.
- `agent/tailer` near-real-time append tailing with max-size guardrails and bounded initial tail-context extraction helpers.
- `agent/watcher` OS-native notification abstraction with deterministic native-to-polling fallback and adaptive polling tiers.
