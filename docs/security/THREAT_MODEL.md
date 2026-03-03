# Threat Model (Baseline)

## Scope

Applies to agent, collector, correlator, and control-plane/UI paths.

## Primary Assumptions

- Private-network deployment with potentially compromised test nodes.
- Adversary may observe/modify network traffic if transport is weak.
- Log payloads may include sensitive operational details.

## Key Threats

- Man-in-the-middle during enrollment or ingestion.
- Replay and duplicate-delivery abuse.
- Credential theft and long-lived unauthorized access.
- Integrity loss via payload tampering or reordering.
- Resource exhaustion through burst traffic and unbounded state.

## Mandatory Controls

- TLS-only transport; mTLS after enrollment.
- Single-use, TTL-bound enrollment tokens.
- Idempotent sequence processing and checksum verification.
- Disk-backed durability before expensive analysis.
- Bounded memory/state with deterministic eviction.
- Append-only audit trail for enrollment, revocation, and config changes.

## Out of Scope (Current Stage)

- Autonomous AI actions without explicit operator enablement.
- Remote code execution or unsigned plugin loading.
