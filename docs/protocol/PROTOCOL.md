# Log Streaming & Diagnostics Protocol Specification

## Version 1.0 (Stable, Minimal, Extensible)

## 1. Scope

Protocol v1.0 governs secure transport between agent and collector for:

- enrollment and authentication
- log/event streaming
- artifact transfer
- signal emission
- heartbeat and metrics

Core design goals:

- TLS-only communication
- idempotent behavior under retries/replays
- append-only reconstruction for stream logs
- disk-backed durability before non-critical analysis
- backward-compatible evolution within v1.x

## 2. Non-Negotiable Invariants

1. No plaintext transport.
2. No unauthenticated ingest.
3. Duplicate deliveries must never corrupt output.
4. Stream reconstruction is append-only.
5. Unacknowledged data is never dropped silently.
6. Raw payload is preserved and never overwritten.
7. Protocol evolution must preserve v1 required fields.

## 3. Transport, Auth, and Integrity

- HTTPS required, TLS 1.2+ (TLS 1.3 preferred).
- Enrollment uses single-use TTL token over TLS.
- Post-enrollment connections require mTLS.
- `source_id` must match authenticated identity.
- Every chunk includes checksum; collector verifies before commit.
- Frame replay safety is enforced with idempotency keys and sequence tracking.

## 4. Versioning and Compatibility

All protocol messages include:

```json
{
  "protocol_version": "1.0",
  "message_type": "StreamChunk",
  "schema_version": "1.0"
}
```

Rules:

- major mismatch: reject.
- minor mismatch: allow only when required-field contract is preserved.
- unknown required field: reject.
- unknown optional field: ignore safely.
- producers may add optional fields only; never repurpose existing semantics.

## 5. Message Classes

- `EnrollmentRequest`, `EnrollmentResponse`
- `StreamOpen`, `StreamChunk`, `StreamClose`
- `ArtifactUploadStart`, `ArtifactChunk`, `ArtifactComplete`
- `SignalBatch`
- `Heartbeat`, `Metrics`
- reserved (future): `PluginManifest`, `PluginDownload`, `PluginActivation`

Reserved messages are schema-reserved only and are not active in v1 runtime.

## 6. Mandatory Log Event Envelope

No log event enters the system without this envelope.

```json
{
  "event_time": "2026-03-03T10:20:30Z",
  "ingest_time": "2026-03-03T10:20:31Z",
  "tenant_id": "tenant-1",
  "source_id": "agent-uuid",
  "source_type": "app|service|system|inferred",
  "raw_payload": "...", 
  "raw_pointer": "optional immutable object pointer",
  "fingerprint": "fp_hash",
  "fingerprint_version": "1",
  "structure_confidence": 0.91,
  "sampling_flag": "full|sampled|aggregated-only",
  "priority_flag": "critical|high|normal|low"
}
```

Requirements:

- `event_time`: original source time when available.
- `ingest_time`: collector-assigned timestamp.
- one of `raw_payload` or `raw_pointer` must be present.
- envelope fields above are required in v1.

## 7. Progressive Structuring Rules

Every event is classified as:

- `structured`
- `semi_structured`
- `unstructured`

Rules:

- extraction is additive only; never destructive.
- raw payload remains immutable source of truth.
- extracted fields are accepted only when parse confidence meets configured threshold.
- parse confidence is recorded per event.

## 8. Fingerprinting Rules

All events pass normalization + template hashing in hot path.

Normalization must:

- remove dynamic tokens (ids, numeric spans, uuids, volatile IPs where applicable)
- preserve stable token patterns
- be versioned (`fingerprint_version` required)

Fingerprint uses:

- drift detection
- novelty detection
- adaptive sampling eligibility
- correlation explainability

## 9. Sampling and Priority Policy

Always retain at 100%:

- security-critical categories
- events linked to anomaly/incidents
- first-seen fingerprints
- rare fingerprints below configured frequency threshold

Adaptive sampling:

- high-frequency stable fingerprints may be reduced
- aggregates are always retained at 100%
- novelty-biased policy increases retention when fingerprint drift rises

All sampling decisions must emit telemetry including effective sampling rate.

## 10. Signal Schema (Structured Summary)

Signals summarize derived conditions from events/aggregates.

```json
{
  "signal_id": "sig-uuid",
  "tenant_id": "tenant-1",
  "source_scope": ["agent-uuid"],
  "signal_type": "anomaly|drift|security|performance",
  "score": 0.87,
  "confidence": 0.82,
  "window": {
    "start": "2026-03-03T10:15:00Z",
    "end": "2026-03-03T10:20:00Z",
    "duration_seconds": 300
  },
  "contributing_fingerprints": ["fp_a", "fp_b"],
  "deviation": {
    "metric": "volume_zscore",
    "value": 3.2
  },
  "sampling_impact": "none|low|medium|high",
  "explainability": "short deterministic reason"
}
```

Required signal properties in v1:

- deterministic scoring field (`score`)
- bounded confidence (`confidence`, 0..1)
- explicit window metadata
- traceability to contributing fingerprints
- sampling impact indicator

## 11. Stream and Artifact Reliability

`StreamChunk` / `ArtifactChunk` rules:

- monotonic `sequence` per stream/artifact
- offset continuity validation
- checksum validation before append
- dedupe key: `(id, sequence)`
- duplicate replay returns ACK without rewrite

Gap handling:

- collector returns resync response with expected sequence
- agent retries from acknowledged boundary

Rotation/truncation:

- identity change or offset rollback requires new `stream_id`

## 12. Backpressure and Degradation Behavior

Overload responses:

- `429` or `503` with retry hint
- explicit backpressure reason code

Predictable degradation order:

1. preserve raw ingest durability
2. preserve fingerprinting + basic aggregates
3. reduce deep parsing
4. increase sampling of high-volume stable fingerprints
5. defer heavy ML/deep analysis

Additional rules:

- raw and signal streams must use isolated queues
- correlator consumes signals/structured events/aggregates; raw is on-demand only

## 13. Drift and Novelty Rules

System must detect:

- first-seen fingerprint emergence
- template mutation rate
- volume deviation beyond configured bounds

Drift response:

- elevate sampling
- mark parser retraining candidate
- optional alert emission

## 14. Failure Mode Handling

Required under failure:

- durable spool replay after restart
- no silent drops on network interruption
- explicit confidence scoring on detections
- explicit status for delayed/deferred analysis

## 15. Security and Audit Expectations

- revoked identities are rejected immediately
- certificate expiration terminates session
- enrollment, revocation, and config mutations are audited
- plugin/AI transport remains disabled unless separately feature-flagged and signed

## 16. Protocol Evolution Policy

Within `v1.x`:

- additive optional fields only
- existing required fields cannot be removed or redefined
- compatibility tests required for agent/collector mixed-minor versions

Breaking changes require:

- major version bump
- migration notes
- rollout compatibility matrix

## 17. v1 Stability Criteria

v1 is READY when test evidence shows:

- resumable delivery across interrupted networks
- idempotent replay safety
- append-only reconstruction correctness
- fingerprint and sampling rule enforcement
- backpressure/degradation predictability
- TLS/mTLS and revocation enforcement
