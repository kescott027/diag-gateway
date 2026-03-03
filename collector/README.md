# collector

Ingest adapters, admission control, buffering, and durable append-only storage interfaces.

Current packages:

- `collector/storage/layout` deterministic source/stream/artifact path helpers.
- `collector/security/bootstrap` local CA and server certificate bootstrap helpers.
- `collector/security/enrollment` single-use TTL enrollment token manager.
- `collector/security/admission` certificate identity admission validator.
- `collector/security/rotation` server certificate renewal policy and rotation helper.
