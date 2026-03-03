# collector

Ingest adapters, admission control, buffering, and durable append-only storage interfaces.

Current packages:

- `collector/artifacts/download` signed, time-bounded artifact download-token generation and verification primitives.
- `collector/artifacts/upload` append-oriented artifact upload lifecycle with resume-state metadata and integrity hashing.
- `collector/bundles/exporter` deterministic debug-bundle export with embedded/sidecar manifest generation.
- `collector/configpush` remote configuration lifecycle primitives (submit/apply/fail) with audit event hooks.
- `collector/retention` per-source retention policy storage and deterministic stream/artifact pruning primitives.
- `collector/retention/jobs` bounded retention/compaction scheduler runner with overlap protection and per-source execution summaries.
- `collector/metadata/store` pluggable source/stream/artifact metadata store abstractions with memory and SQLite adapters.
- `collector/metadata/grouping` source group/tag assignment and deterministic filtering primitives backed by metadata store records.
- `collector/api/listing` deterministic source/file catalog listing primitives for UI integration.
- `collector/api/dashboard` per-source summary-card aggregation for dashboard/backend UX.
- `collector/api/livetail` bounded cursor-based stream polling primitives for live-tail UI integration.
- `collector/api/search` bounded substring/regex search primitives over stream logs.
- `collector/storage/layout` deterministic source/stream/artifact path helpers.
- `collector/stream/reassembly` append-only stream reassembly and idempotent sequence processing primitives.
- `collector/security/bootstrap` local CA and server certificate bootstrap helpers.
- `collector/security/enrollment` single-use TTL token manager, credential exchange, and pull-based runtime renewal service.
- `collector/security/admission` certificate identity admission validator.
- `collector/security/audittrail` append-only enrollment/config audit event logging with bounded query filters.
- `collector/security/crl` versioned certificate revocation-list manager with canonical serial handling.
- `collector/security/oidc` optional OIDC config/metadata validation and identity-to-role mapping primitives.
- `collector/security/rbac` control-plane role/action authorization primitives with deny-by-default semantics.
- `collector/security/rotation` server certificate renewal policy and rotation helper.
- `collector/security/revocation` immediate source/serial revocation service with append-only audit sink.
- `collector/telemetry/lastseen` concurrency-safe source liveness tracker for last-seen visibility.
- `collector/telemetry/agentmetrics` per-source bytes, queue depth, and error-rate tracker.
