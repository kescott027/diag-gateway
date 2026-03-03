# Threat Model (STRIDE)

## Scope

Applies to agent, collector, correlator, shared security services, and control-plane/UI paths.

Implemented security components referenced by this model:

- `collector/security/bootstrap`
- `collector/security/enrollment`
- `collector/security/admission`
- `collector/security/rotation`
- `collector/security/revocation`
- `shared/security/secretstore`

## Primary Assumptions

- Deployments are local-first and private-network capable, but compromised endpoints are possible.
- Network-path attackers may attempt interception, replay, or downgrade attacks.
- Collector is trusted root for enrollment and certificate issuance.
- Agent and collector clocks may drift within small operational bounds.

## Security-Critical Assets

- CA private key and certificate chain.
- Server TLS keys/certs.
- Agent client credentials.
- Enrollment tokens and redemption state.
- Revocation state (source and serial).
- Append-only audit logs.

## Trust Boundaries

1. Untrusted endpoint to collector enrollment boundary.
2. Authenticated mTLS ingest boundary.
3. Credential-at-rest boundary on agent/collector nodes.
4. Operator actions boundary (revocation/rotation events).

## STRIDE Threats and Controls

## Spoofing

Threats:

- Fake agent identity using forged credentials.
- Impersonation via replayed enrollment material.

Controls:

- TLS-only + mTLS for operational traffic.
- Single-use TTL enrollment tokens.
- Per-source client certificate issuance.
- Admission validator checks source status and cert posture.

Residual risk:

- Source identity status is currently in-memory for runtime packages; persistence hardening remains.

## Tampering

Threats:

- Payload manipulation in transit.
- Unauthorized mutation of credential/audit files.

Controls:

- TLS transport protections.
- Protocol checksum/idempotency requirements.
- Restricted file permissions for key/cert/secret storage.
- Append-only audit sink for revocation events.

Residual risk:

- File-backed audit sink is local; remote tamper-evident replication is future work.

## Repudiation

Threats:

- Operator denies revocation or security actions.
- Enrollment/rotation actions lack traceability.

Controls:

- Append-only audit events for source and serial revocation.
- Decision matrix and sprint logs for architecture-impacting policy changes.

Residual risk:

- Enrollment and rotation runtime actions need consolidated audit sink integration in future ingest service wiring.

## Information Disclosure

Threats:

- Secret leakage from credential/token storage.
- Exposure of sensitive logs via insecure transport or storage.

Controls:

- File secret store requires restrictive permissions and rejects insecure modes.
- Token manager stores SHA-256 digests instead of plaintext tokens.
- TLS-only transport with no plaintext fallback.

Residual risk:

- OS-native keychain adapters are not yet implemented.

## Denial of Service

Threats:

- Exhaustion via burst traffic, token spam, renewal spam.
- Admission bypass attempts for disabled/revoked sources.

Controls:

- Bounded-state and degradation policies defined in protocol/deployment docs.
- Min issue gap in pull renewal path to reduce cert churn abuse.
- Deterministic reject paths for unknown/revoked/disabled identities.

Residual risk:

- Token and revocation stores are currently in-memory and should gain durable-backed scaling paths.

## Elevation of Privilege

Threats:

- Unauthorized source gains active ingest access.
- Revoked credentials continue to operate.

Controls:

- Source status lifecycle (`active/revoked/disabled/unknown`) in admission validator.
- Serial revocation checks integrated with admission validator.
- Pull renewal requires authenticated certificate and renewal-window compliance.

Residual risk:

- Rollover orchestration is package-level and not yet fully wired into network service handlers.

## Out of Scope (Current Stage)

- AI/plugin runtime execution and remote plugin distribution.
- Remote code execution pathways.
- Multi-tenant policy isolation enforcement.

## Required Follow-Up Security Work

- Persist enrollment token state and revocation state durably.
- Integrate audit sinks across enrollment and rotation workflows.
- Add OS-native secure store adapters for credentials.
- Wire admission/renewal/revocation services into collector network handlers.
