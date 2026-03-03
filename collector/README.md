# collector

Ingest adapters, admission control, buffering, and durable append-only storage interfaces.

Current packages:

- `collector/storage/layout` deterministic source/stream/artifact path helpers.
- `collector/stream/reassembly` append-only stream reassembly and idempotent sequence processing primitives.
- `collector/security/bootstrap` local CA and server certificate bootstrap helpers.
- `collector/security/enrollment` single-use TTL token manager, credential exchange, and pull-based runtime renewal service.
- `collector/security/admission` certificate identity admission validator.
- `collector/security/rotation` server certificate renewal policy and rotation helper.
- `collector/security/revocation` immediate source/serial revocation service with append-only audit sink.
