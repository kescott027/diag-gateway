# Deployment Guide

## 1. Deployment Objective

This guide defines the minimum deployment standard for the local-first diagnostics platform.

Reference performance tier for READY-stage planning:

- sustained ingest: 20,000 events per second (EPS)
- burst absorb: 100,000 EPS
- p95 detection latency: < 3 seconds under sustained reference load
- overload behavior: graceful degradation (sampling/depth reduction), never crash/drop silently

## 2. Multi-Plane Topology

The platform is deployed in logical planes.

### 2.1 Data Plane (Required)

Responsibilities:

- ingest raw events and artifacts
- validate envelope, auth, and checksums
- durable queueing/spooling
- append-only storage commit

### 2.2 Correlation Plane (Required)

Responsibilities:

- consume signals, structured events, and aggregates
- perform stateful multi-event detection
- emit scored incidents/signals with traceability

### 2.3 Deep Analysis Plane (Optional)

Responsibilities:

- heavy parsing, model-assisted analysis, retrospective enrichment
- deferred processing under load

Rules:

- Data Plane remains authoritative for durability.
- Correlation and Deep Analysis must not compromise ingest guarantees.
- Optional planes must be feature-flagged.

## 3. Capacity Unit Model

One reference capacity unit (CU) equals resources sized to support:

- 20k EPS sustained mixed workload
- 100k EPS burst absorption window
- bounded memory/state and durable disk buffering

Sizing model:

- start at 1 CU for single-team baseline
- scale Data Plane first for EPS growth
- scale Correlation Plane for latency/SLO pressure
- scale Deep Analysis only when enabled and isolated

## 4. Horizontal Scaling and Partitioning

Partition key order:

1. `tenant_id`
2. `source_id`
3. `fingerprint` (analysis workloads)

Rules:

- preserve per-stream ordering within a partition
- maintain idempotency key scope per partition
- avoid cross-partition state joins on hot path
- replicate only derived aggregates/signals when needed

## 5. High Availability Minimums

Minimum production-like HA profile:

- Data Plane: 3 nodes
- Correlation Plane: 3 nodes
- Deep Analysis: 2 nodes (if enabled)
- Metadata store: SQLite for single-node mode only; HA mode requires replicated store strategy before scaling beyond one collector host

Availability rules:

- no single node failure may cause data loss
- degraded mode must remain ingest-capable
- control/UI outage must not block ingest

### 5.1 Supported HA Topology Patterns

Pattern A: single-node local baseline (non-HA)

- planes co-located on one host
- SQLite metadata store allowed
- intended for development and local diagnostics only

Pattern B: active-active reference HA (recommended)

- 3 Data Plane nodes behind load-balancer
- 3 Correlation Plane nodes with partition ownership
- replicated metadata store backend
- pull-based worker ownership of partitions with lease/heartbeat semantics

Pattern C: active-active with optional deep analysis

- Pattern B plus isolated Deep Analysis workers (minimum 2 nodes)
- Deep Analysis receives replicated derived events only
- Deep Analysis failure must not affect Data Plane ingest durability

### 5.2 Partition Ownership and Rebalance Rules

- partition assignment must be deterministic from `(tenant_id, source_id)` keyspace
- one active owner per partition at a time; ownership transitions require lease expiry or explicit revoke
- rebalance operations must preserve per-partition ordering and idempotency-key scope
- replay on ownership handoff must be safe under duplicate delivery assumptions

### 5.3 Failure-Domain Rules

- place Data Plane replicas across at least 3 independent failure domains (host/zone/rack)
- avoid co-locating all metadata replicas with one Data Plane node group
- correlate node-loss alarms with ingest lag and queue-depth alarms for safe automation

## 6. Backpressure and Degradation Policies

Backpressure strategy:

- explicit 429/503 with retry hint
- bounded in-memory queues
- disk-backed overflow buffers

Backpressure thresholds (minimum operating contract):

- warning: queue utilization >= 70%
- critical: queue utilization >= 85%
- saturated: queue utilization >= 95%

Required control response by level:

- warning: increase telemetry sampling granularity and emit early pressure signals
- critical: increase adaptive sampling on stable high-volume fingerprints; defer expensive parsing
- saturated: admit only high-priority classes and preserve raw durability + fingerprinting first

Degradation order under overload:

1. preserve raw ingest durability
2. preserve fingerprinting + base aggregates
3. reduce deep parsing
4. increase adaptive sampling for stable high-volume patterns
5. defer heavy ML/deep analysis

Required behavior:

- queue isolation between raw and signal streams
- deterministic throttling policy
- no silent dropping of high-priority anomalies
- degraded behavior must be observable with explicit mode/state telemetry

## 7. State Management Requirements

- bounded in-memory state per key (`tenant_id/source_id/fingerprint`)
- configurable window retention
- explicit eviction policy (LRU+time-window or equivalent deterministic policy)
- eviction and state pressure must emit telemetry

## 8. Observability Requirements

Must expose plane-level telemetry:

- per-plane EPS
- ingest-to-correlation lag
- signal reduction ratio
- correlation latency (p50/p95/p99)
- state store utilization and eviction rate
- queue depth and backpressure indicators
- effective sampling rate by fingerprint class
- partition ownership churn rate
- lease/heartbeat freshness (for HA partition owners)
- replay volume after failover

SLO observability:

- validate p95 detection latency < 3s at sustained 20k EPS profile
- track burst recovery time after 100k EPS shock

HA observability:

- alert if a partition has no active owner for > 2 lease intervals
- alert if Data Plane durable queue replay exceeds configured recovery budget
- alert on repeated owner flapping for the same partition key range

## 9. Security Baseline for Deployment

- TLS-only endpoints
- mTLS for operational agent traffic
- single-use TTL enrollment token flow
- secure key material storage with restricted permissions
- revocation enforcement and immutable audit logs

## 10. Upgrade and Rollout Strategy

- use minor-version rolling upgrades for additive schema changes
- verify mixed-version protocol compatibility before full rollout
- pause deep-analysis modules first during constrained maintenance windows
- never deploy protocol-breaking changes without major-version plan

HA rollout constraints:

- do not upgrade more than one Data Plane node in the same partition ownership cohort at once
- verify lease handoff and replay-idempotency behavior during canary
- block rollout if ingest lag or replay error rates exceed safety thresholds

## 11. Validation Before Production Implementation

A deployment profile is READY only when validated by simulations in `tests/`:

- sustained-load test at 20k EPS
- burst test at 100k EPS
- drift/novelty behavior test
- degradation behavior test with deterministic outcomes
- failover and replay integrity test
