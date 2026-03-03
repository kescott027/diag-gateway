# Backlog (Normalized Priority Order)

## Story Completion Contract (Source of Truth)

A story is complete only when all are true:

- acceptance criteria in the story are met
- measurable performance targets are met where applicable (latency, EPS, burst tolerance)
- observable telemetry evidence is captured proving behavior
- architecture constraints take precedence over ambiguous wording

Global architecture constraints:

- TLS-only communication
- protocol idempotency
- append-only stream reconstruction
- disk-backed durability
- bounded memory usage
- feature flags for experimental capabilities
- backward protocol compatibility

## Phase A: READY Stage Governance and Structure

### 1. PM-01 - Documentation normalization and taxonomy
- Description: Move architecture/security/protocol/ops/planning/AI/future docs to stable `/docs` layout.
- Dependencies: none
- Risk: low
- Architectural impact: documentation topology, no runtime impact

### 2. PM-02 - Sprint management framework bootstrap
- Description: Create `project_management` control files and immutable completed sprint archive path.
- Dependencies: PM-01
- Risk: low
- Architectural impact: governs delivery process and decision traceability

### 3. PM-03 - READY gate definition
- Description: Define objective criteria for entering implementation stage.
- Dependencies: PM-01, PM-02
- Risk: medium
- Architectural impact: prevents uncontrolled implementation drift

### 4. PM-04 - Decision logging baseline
- Description: Record protocol/deployment/backlog/scaling decisions in Decision Matrix.
- Dependencies: PM-02
- Risk: medium
- Architectural impact: security/protocol/data-model decisions become auditable

### 5. PM-05 - Architecture coherence cadence
- Description: Enforce architecture review every 3 sprints in governance rules.
- Dependencies: PM-02
- Risk: medium
- Architectural impact: long-term consistency guardrail

## Phase B: Core Foundations (Repo + Contracts)

### 6. R0-01 - Monorepo source structure
- Description: Establish `/agent`, `/collector`, `/correlator`, `/shared`, `/ui`, `/tests` baseline.
- Dependencies: PM-03
- Risk: low
- Architectural impact: defines module boundaries and ownership

### 7. R0-07 - Deterministic collector storage layout
- Description: Implement fixed storage path model for streams/artifacts by source.
- Dependencies: R0-01
- Risk: medium
- Architectural impact: persistent data contract for all downstream tooling

### 8. R0-19 - Shared protocol contracts
- Description: Define shared schema contract package used by all runtime components.
- Dependencies: R0-01
- Risk: high
- Architectural impact: protocol compatibility and schema evolution base

### 9. R0-03 - Standardized build commands
- Description: Establish unified build/test/run command surface.
- Dependencies: R0-01
- Risk: low
- Architectural impact: build reproducibility

### 10. R0-04 - Lint and format CI enforcement
- Description: Enforce style and static quality checks in CI.
- Dependencies: R0-03
- Risk: low
- Architectural impact: maintainability and defect prevention

### 11. R0-13 - CI quality gates
- Description: Run unit tests, race detection, and linting automatically.
- Dependencies: R0-03, R0-04
- Risk: medium
- Architectural impact: early regression containment

### 12. R0-14 - Cross-platform build automation
- Description: Produce Linux/macOS/Windows builds from CI.
- Dependencies: R0-13
- Risk: medium
- Architectural impact: platform support guarantees

### 13. R0-15 - Release artifacts with checksums
- Description: Publish signed artifacts and checksums.
- Dependencies: R0-14
- Risk: medium
- Architectural impact: supply integrity baseline

### 14. R0-05 - Commit and versioning policy
- Description: Conventional commits + semantic versioning workflow.
- Dependencies: R0-03
- Risk: low
- Architectural impact: controlled release evolution

### 15. R0-06 - Git safety for data paths
- Description: Exclude collector data/spool artifacts from VCS.
- Dependencies: R0-01
- Risk: low
- Architectural impact: prevents sensitive data leakage to repo

### 16. R0-02 - ADR template and process
- Description: Standard ADR format for major design changes.
- Dependencies: PM-04
- Risk: low
- Architectural impact: institutional architectural memory

## Phase C: Security Baseline

### 17. R0-08 - Local CA and TLS bootstrap
- Description: Collector auto-generates local CA/certs on first boot.
- Dependencies: R0-19
- Risk: high
- Architectural impact: foundational transport security
- Performance target: no measurable ingest regression >5% at 20k EPS profile
- Telemetry evidence: handshake success/failure rates and cert issuance metrics

### 18. R0-09 - Single-use TTL enrollment tokens
- Description: Token issuance with one-time and expiry semantics.
- Dependencies: R0-08
- Risk: high
- Architectural impact: onboarding trust boundary
- Telemetry evidence: token issuance, redemption, expiry, reject counters

### 19. R1-15 - Enrollment exchange for long-lived credentials
- Description: Exchange valid token for operational identity credentials.
- Dependencies: R0-09
- Risk: high
- Architectural impact: transitions trust from bootstrap to operational mode

### 20. R1-01 - Authenticated agent connections
- Description: Accept ingest only from enrolled approved identities.
- Dependencies: R1-15
- Risk: high
- Architectural impact: ingest admission control

### 21. R0-10 - Secure credential storage conventions
- Description: Enforce keychain/DPAPI or strict file permissions fallback.
- Dependencies: R1-15
- Risk: high
- Architectural impact: credential theft resistance

### 22. R0-12 - Key rotation policies and implementation
- Description: Scheduled certificate/key rotation without reinstall.
- Dependencies: R1-01
- Risk: high
- Architectural impact: long-term trust hygiene

### 23. R1-16 - Runtime credential rotation support
- Description: Agent/collector rotation execution path.
- Dependencies: R0-12
- Risk: high
- Architectural impact: live identity continuity

### 24. R1-17 - Immediate agent revocation
- Description: Disable compromised agents instantly.
- Dependencies: R1-01
- Risk: high
- Architectural impact: compromise containment

### 25. R0-11 - STRIDE threat model formalization
- Description: Keep threat model current with implementation changes.
- Dependencies: PM-01
- Risk: medium
- Architectural impact: explicit risk model governance

## Phase D: Streaming Reliability Core

### 26. R1-02 - Deterministic stream path mapping
- Description: Map stream writes to deterministic source/stream paths.
- Dependencies: R0-07, R1-01
- Risk: medium
- Architectural impact: storage and retrieval determinism

### 27. R1-03 - Append-only stream reassembly
- Description: Reassemble chunks only via append semantics.
- Dependencies: R1-02
- Risk: high
- Architectural impact: core data integrity invariant

### 28. R1-07 - Idempotent duplicate-safe chunk processing
- Description: Dedupe and ACK replayed chunks without rewrite.
- Dependencies: R1-03
- Risk: high
- Architectural impact: retry safety and corruption prevention
- Performance target: sustain 20k EPS with duplicate replay load while keeping p95 detection latency < 3s
- Telemetry evidence: dedupe hit rate, out-of-order rejects, write amplification ratio

### 29. R1-12 - Disk-backed spool queue
- Description: Persist unsent data locally for downtime tolerance.
- Dependencies: R1-07
- Risk: high
- Architectural impact: durability during transport outages

### 30. R1-13 - Backpressure handling with memory bounds
- Description: Enforce bounded queues and controlled retry/degradation.
- Dependencies: R1-12
- Risk: high
- Architectural impact: overload safety
- Performance target: absorb 100k EPS burst without data loss
- Telemetry evidence: queue depth, spill-to-disk counts, backpressure durations

### 31. R1-10 - Cursor persistence
- Description: Persist file offsets and recovery checkpoints.
- Dependencies: R1-12
- Risk: medium
- Architectural impact: restart resume correctness

### 32. R1-11 - Rotation/truncation safety
- Description: Detect file identity changes and reopen stream safely.
- Dependencies: R1-10
- Risk: high
- Architectural impact: prevents stream corruption after rotations

### 33. R1-09 - Near-real-time append streaming
- Description: Send appended file data with low-latency tailing.
- Dependencies: R1-11
- Risk: medium
- Architectural impact: real-time diagnostics value
- Performance target: maintain p95 detection latency < 3s at 20k EPS
- Telemetry evidence: ingest-to-availability latency histograms

### 34. R1-20 - Protocol fuzz and integrity tests
- Description: Fuzz sequence/checksum/offset behaviors for resilience.
- Dependencies: R1-07
- Risk: medium
- Architectural impact: protocol correctness hardening

## Phase E: Observability and Operator Visibility

### 35. R0-17 - Structured logging with correlation IDs
- Description: Uniform structured logs across components.
- Dependencies: R0-01
- Risk: low
- Architectural impact: diagnosability across planes

### 36. R0-18 - Health and metrics endpoints
- Description: Expose `/health` and `/metrics` across core services.
- Dependencies: R0-17
- Risk: medium
- Architectural impact: operational monitoring baseline

### 37. R1-18 - Agent last-seen visibility
- Description: Track and show per-agent liveness.
- Dependencies: R0-18
- Risk: low
- Architectural impact: operator confidence and incident response

### 38. R1-19 - Bytes/queue/error metrics
- Description: Show throughput, queue depth, error rate per agent.
- Dependencies: R0-18
- Risk: medium
- Architectural impact: overload diagnosis and SLO verification

### 39. R1-04 - Collector source/file listing UI
- Description: Minimal UI to browse sources and files.
- Dependencies: R1-02, R1-18
- Risk: low
- Architectural impact: usability surface

### 40. R1-05 - Live tail UI
- Description: Real-time tail display for selected stream.
- Dependencies: R1-04, R1-09
- Risk: medium
- Architectural impact: operational feedback loop

### 41. R1-06 - Substring/regex search
- Description: Basic search over recent logs.
- Dependencies: R1-04
- Risk: medium
- Architectural impact: triage efficiency

## Phase F: Agent UX and Monitoring Enhancements

### 42. R1-08 - Agent local configuration UI
- Description: Configure watch paths/patterns locally.
- Dependencies: R1-09
- Risk: low
- Architectural impact: onboarding and operator ergonomics

### 43. R2-05 - OS-native notification watchers
- Description: Use inotify/FSEvents/ReadDirectoryChangesW where available.
- Dependencies: R1-09
- Risk: medium
- Architectural impact: change-detection efficiency

### 44. R2-06 - Polling fallback
- Description: Adaptive fallback when native watchers fail.
- Dependencies: R2-05
- Risk: medium
- Architectural impact: cross-platform resilience

### 45. R2-07 - Adaptive polling tiers
- Description: Poll based on recent activity profile.
- Dependencies: R2-06
- Risk: low
- Architectural impact: resource efficiency

### 46. R2-08 - Include/exclude path patterns
- Description: Per-path filtering for collection scope.
- Dependencies: R1-08
- Risk: low
- Architectural impact: data minimization and relevance

### 47. R2-09 - Initial tail context send
- Description: Send last N lines on new file discovery.
- Dependencies: R2-08
- Risk: medium
- Architectural impact: context-rich diagnostics

### 48. R2-10 - Max file size controls
- Description: Enforce per-stream size guardrails.
- Dependencies: R2-08
- Risk: medium
- Architectural impact: bounded resource protection

## Phase G: Artifacts and Retention

### 49. R2-01 - Artifact upload support
- Description: Transfer crash dumps and bundles.
- Dependencies: R1-12
- Risk: medium
- Architectural impact: evidence completeness

### 50. R2-02 - Resumable artifact uploads
- Description: Resume large transfers after interruption.
- Dependencies: R2-01
- Risk: high
- Architectural impact: large-file durability

### 51. R2-03 - Artifact metadata model
- Description: Persist source/tags/time/checksum metadata.
- Dependencies: R2-01
- Risk: medium
- Architectural impact: retrieval and auditability

### 52. R2-04 - Artifact download links in UI
- Description: Provide controlled artifact retrieval.
- Dependencies: R2-03
- Risk: medium
- Architectural impact: operator workflow completeness

### 53. R2-12 - Retention policy per source
- Description: Configure stream/artifact retention by source.
- Dependencies: R2-03
- Risk: medium
- Architectural impact: storage lifecycle controls

### 54. R2-14 - Compression options
- Description: none/gzip/zstd transport/storage selection.
- Dependencies: R1-03
- Risk: medium
- Architectural impact: throughput vs CPU tradeoff control

### 55. R2-13 - Exportable debug bundles
- Description: Create deterministic debug bundle exports.
- Dependencies: R2-04, R2-03
- Risk: medium
- Architectural impact: reproducibility and external sharing workflows

### 56. R2-11 - Per-source dashboard UX
- Description: Unified source-level status panel.
- Dependencies: R1-19
- Risk: low
- Architectural impact: operational visibility

## Phase H: Scale-Out and Enterprise Hardening

### 57. R3-07 - Pluggable metadata store
- Description: SQLite-to-Postgres metadata abstraction.
- Dependencies: R1 core stability
- Risk: high
- Architectural impact: persistence architecture expansion

### 58. R3-08 - Retention/compaction jobs
- Description: Automated cleanup and compaction jobs.
- Dependencies: R3-07
- Risk: medium
- Architectural impact: storage efficiency at scale

### 59. R3-09 - HA collector deployment patterns
- Description: Document and implement HA reference topology.
- Dependencies: R3-07
- Risk: high
- Architectural impact: availability guarantees

### 60. R3-01 - RBAC
- Description: admin/operator/viewer roles for control plane.
- Dependencies: R3-07
- Risk: high
- Architectural impact: authorization model expansion

### 61. R3-12 - Optional OIDC auth
- Description: OIDC-backed UI authentication.
- Dependencies: R3-01
- Risk: medium
- Architectural impact: identity federation support

### 62. R3-02 - Agent grouping and tagging
- Description: Organize agents by project/team/context.
- Dependencies: R3-01
- Risk: low
- Architectural impact: fleet management model

### 63. R3-03 - Remote config push with audit
- Description: Push controlled config changes to agents.
- Dependencies: R3-01, R3-11
- Risk: high
- Architectural impact: high-impact control path, requires strict auditing

### 64. R3-10 - Certificate revocation lists
- Description: Formal CRL support and enforcement.
- Dependencies: R1-17
- Risk: high
- Architectural impact: trust revocation robustness

### 65. R3-11 - Enrollment/config audit trails
- Description: Immutable audit for security-critical operations.
- Dependencies: R1-17
- Risk: medium
- Architectural impact: compliance and forensics baseline

### 66. R3-04 - Routing rules
- Description: Route streams by source/tag/path.
- Dependencies: R3-02
- Risk: medium
- Architectural impact: ingest topology complexity

### 67. R3-06 - Multi-collector forwarding
- Description: Forward data across collector nodes.
- Dependencies: R3-04
- Risk: high
- Architectural impact: distributed consistency concerns

### 68. R3-05 - External sink forwarding
- Description: Optional forward to S3/Loki/Elasticsearch.
- Dependencies: R3-04
- Risk: medium
- Architectural impact: integration surface growth

## Phase I: Future Capabilities (Feature-Flagged)

### 69. RN-01 - Full-text indexed search
- Description: Indexed search across log corpus.
- Dependencies: R2 maturity
- Risk: medium
- Architectural impact: indexing/storage overhead

### 70. RN-02 - Session timeline model
- Description: Aggregate logs/artifacts/metrics by session.
- Dependencies: R2-13
- Risk: medium
- Architectural impact: domain model expansion

### 71. RN-03 - IDE integration
- Description: Integrate diagnostics flow into IDE tooling.
- Dependencies: RN-02
- Risk: low
- Architectural impact: developer experience extension

### 72. RN-04 - Client-side PII redaction rules
- Description: Local redaction policies before transfer.
- Dependencies: R1 stability
- Risk: high
- Architectural impact: privacy/security critical transformation path

### 73. RN-05 - Path policy engine
- Description: Allow/deny file collection policies.
- Dependencies: R1-08
- Risk: medium
- Architectural impact: security boundary control

### 74. RN-06 - Code-signed agent binaries
- Description: Signed releases with signature verification.
- Dependencies: R0-15
- Risk: medium
- Architectural impact: software supply-chain security

### 75. RN-07 - Zero-trust networking integration
- Description: WireGuard/Tailscale compatibility patterns.
- Dependencies: R3 network maturity
- Risk: medium
- Architectural impact: deployment/security model extension

### 76. RN-08 - Multi-tenant project isolation
- Description: Isolate data and controls by tenant/project.
- Dependencies: R3-01
- Risk: high
- Architectural impact: data model and auth partitioning

### 77. RN-09 - Differential transfer support
- Description: Delta-based structured log transfer.
- Dependencies: R2-14
- Risk: high
- Architectural impact: protocol complexity increase

### 78. RN-10 - AI-ready debug packaging
- Description: Build deterministic AI-consumable diagnostic packets.
- Dependencies: R2-13
- Risk: medium
- Architectural impact: packaging schema and governance

## Phase J: Optional AI and Plugin Framework (After Core Streaming Stability)

### 79. AI-A1 - Plugin runtime core
- Description: Client plugin runtime with explicit capability scopes.
- Dependencies: R1 core stability, feature flags
- Risk: high
- Architectural impact: runtime boundary and security surface increase

### 80. AI-A2 - Signed plugin distribution
- Description: Signed bundle distribution and signature verification.
- Dependencies: AI-A1
- Risk: high
- Architectural impact: supply-chain trust model extension

### 81. AI-A3 - Dual enablement gate
- Description: Server and client must both enable remote plugins.
- Dependencies: AI-A2
- Risk: high
- Architectural impact: explicit activation boundary

### 82. AI-A4 - Sandboxed execution with resource caps
- Description: Restrict plugin file/network access and enforce CPU/memory bounds.
- Dependencies: AI-A1
- Risk: high
- Architectural impact: containment model and scheduler impact

### 83. AI-A5 - AI action audit logging
- Description: Audit all AI-triggered actions in collector UI.
- Dependencies: AI-A4
- Risk: high
- Architectural impact: security/forensics obligations

### 84. AI-B1 - Environment snapshot engine
- Description: Capture non-sensitive environment context with redaction.
- Dependencies: AI-A5
- Risk: medium
- Architectural impact: data collection scope expansion

### 85. AI-B2 - Error-triggered context capture
- Description: Capture context on repeated error patterns.
- Dependencies: AI-B1
- Risk: medium
- Architectural impact: event-triggered collection pipeline

### 86. AI-B3 - Environment diffing
- Description: Detect meaningful environment changes over time.
- Dependencies: AI-B1
- Risk: medium
- Architectural impact: comparison and state history model

### 87. AI-C1 - Performance sampler
- Description: Periodic CPU/memory/disk sampling with bounds.
- Dependencies: AI-A4
- Risk: medium
- Architectural impact: telemetry pipeline expansion

### 88. AI-C2 - Performance anomaly detection
- Description: Detect metric spikes correlated with error logs.
- Dependencies: AI-C1
- Risk: medium
- Architectural impact: correlation model complexity

### 89. AI-C3 - Performance trend reporting
- Description: Summarize performance trends in UI.
- Dependencies: AI-C2
- Risk: low
- Architectural impact: UX/reporting enhancement

### 90. AI-D1 - Session observation framework
- Description: Opt-in metadata-only test session observation.
- Dependencies: AI-A4
- Risk: high
- Architectural impact: privacy-sensitive event capture layer

### 91. AI-D2 - Test pattern detection
- Description: Identify repeated validation workflows.
- Dependencies: AI-D1
- Risk: medium
- Architectural impact: behavior modeling

### 92. AI-D3 - Test protocol generation
- Description: Generate reproducible test scripts/descriptions.
- Dependencies: AI-D2
- Risk: medium
- Architectural impact: artifact generation and trust implications

### 93. AI-E1 - Aggregated usage metrics
- Description: Optional anonymized usage pattern aggregation.
- Dependencies: governance/policy approval
- Risk: high
- Architectural impact: privacy/legal boundary

### 94. AI-E2 - Feature friction reporting
- Description: Detect repetitive workflow failures.
- Dependencies: AI-E1
- Risk: medium
- Architectural impact: analytics pipeline expansion

### 95. AI-E3 - Cross-agent pattern aggregation
- Description: Fleet-level insight aggregation.
- Dependencies: AI-E2
- Risk: high
- Architectural impact: multi-agent privacy + scalability concerns
