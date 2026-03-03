# collector

Ingest adapters, admission control, buffering, and durable append-only storage interfaces.

Current packages:

- `collector/api/listing` deterministic source/file catalog listing primitives for UI integration.
- `collector/api/search` bounded substring/regex search primitives over stream logs.
- `collector/storage/layout` deterministic source/stream/artifact path helpers.
- `collector/stream/reassembly` append-only stream reassembly and idempotent sequence processing primitives.
- `collector/security/bootstrap` local CA and server certificate bootstrap helpers.
- `collector/security/enrollment` single-use TTL token manager, credential exchange, and pull-based runtime renewal service.
- `collector/security/admission` certificate identity admission validator.
- `collector/security/rotation` server certificate renewal policy and rotation helper.
- `collector/security/revocation` immediate source/serial revocation service with append-only audit sink.
- `collector/telemetry/lastseen` concurrency-safe source liveness tracker for last-seen visibility.
- `collector/telemetry/agentmetrics` per-source bytes, queue depth, and error-rate tracker.
