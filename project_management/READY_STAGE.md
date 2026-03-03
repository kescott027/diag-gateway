# READY Stage Definition

Project is READY for production implementation only when all checks pass.

## Required Conditions

- documentation organized under `/docs` taxonomy
- normalized backlog exists in strict priority order
- sprint framework exists in `/project_management`
- active sprint defined in `Current_Sprint.md`
- development rules documented and enforced
- quality guardrails documented and measurable

## Mandatory Technical Guardrails

- TLS-only communication
- idempotent protocol behavior
- append-only log reconstruction
- disk-backed durability
- bounded memory/state usage
- feature flags for experimental capabilities
- backward compatibility for protocol evolution

## Additional Mandatory Success Conditions

### Performance Guardrails
- sustain 20k EPS mixed workload
- absorb 100k EPS burst without data loss
- p95 detection latency < 3 seconds under sustained load
- graceful degradation under overload

### State Management
- bounded in-memory state per key
- configurable window retention
- explicit eviction policy

### Schema Strategy
- progressive structuring classification
- mandatory hot-path fingerprinting
- versioned schema evolution

### Sampling Policy
- always retain anomalies and rare fingerprints
- novelty-biased adaptive sampling
- telemetry on effective sampling rate

### Observability
- per-plane EPS
- signal reduction ratio
- correlation latency
- state utilization
- backpressure indicators

### Failure Mode Handling
- defined overload behavior
- queue isolation between raw and signal streams
- explicit confidence scoring for detections
