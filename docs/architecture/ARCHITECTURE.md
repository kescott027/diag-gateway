# ARCHITECTURE.MD

# Log Streaming & Diagnostics Platform

## System Architecture

---

# 1. Overview

The system consists of two primary executables:

1. **Agent (Source Utility)** – Runs on monitored machines and streams log and diagnostic data.
2. **Collector (Receiver / Home System)** – Accepts streams, reassembles files, stores artifacts, and provides UI and APIs.

The architecture is designed to be:

* Secure by default (TLS, authenticated agents)
* Resumable and idempotent
* Disk-backed and durable
* Horizontally extensible in later releases
* Suitable for local-first deployment with optional future scale-out

---

# 2. High-Level Architecture

```
+-------------------+        mTLS        +----------------------+
|     Agent         |  ----------------> |      Collector       |
|-------------------|                    |----------------------|
| File Watcher      |                    | Ingest API           |
| Cursor Manager    |                    | Stream Reassembler   |
| Spool Queue       |                    | Artifact Handler     |
| Local Web UI      |                    | Storage Layer        |
| Metrics Exporter  |                    | Search & Tail UI     |
+-------------------+                    | Metrics & Dashboard  |
                                         +----------------------+
```

---

# 3. Technology Stack (Initial Target)

## Language

* **Go** (agent + collector)

  * Cross-platform binaries
  * Strong TLS support
  * Lightweight deployment
  * Efficient file IO

## Storage

* Log streams: Plain append-only text files
* Artifacts: Stored as files with metadata records
* Metadata: SQLite (pluggable in future releases)

## UI

* Collector: Embedded HTTP server
* Frontend: Minimal SPA (React/Vite) or server-rendered templates
* Agent: Local-only configuration UI

## Metrics

* Prometheus-compatible `/metrics`
* Internal metrics registry

---

# 4. Agent Architecture

## 4.1 Components

### File Watcher

* Uses OS-native notification systems:

  * inotify (Linux)
  * FSEvents (macOS)
  * ReadDirectoryChangesW (Windows)
* Polling fallback with adaptive interval logic.

### Adaptive Monitoring Strategy

Primary: Event-driven notifications
Fallback: Sliding detection intervals

Example adaptive logic:

* Changes within 1 minute → check every 1 second
* Changes within 15 minutes → check every 15 seconds
* Changes within 1 hour → check every 30 seconds
* No changes within 1 hour → check every 60 seconds

### Cursor Manager

Maintains:

* File identity (inode or platform equivalent)
* Current offset
* Rotation detection logic
* Persisted cursor state

### Spool Queue

* Disk-backed queue
* Bounded size
* Chunk-based transmission
* Retries with backoff

### Transport Layer

* HTTPS with mutual TLS
* Chunked streaming protocol
* Idempotent message processing
* Sequence-numbered frames

---

# 5. Collector Architecture

## 5.1 Ingest Layer

* Authenticates agents
* Validates message integrity
* Handles chunk deduplication
* Sends acknowledgments

Message types:

* StreamOpen
* StreamChunk
* StreamClose
* ArtifactUploadStart
* ArtifactChunk
* ArtifactComplete
* Heartbeat
* Metrics

---

## 5.2 Stream Reassembly

Each stream has:

* Unique Stream ID
* Source ID
* Target path mapping

Reassembly guarantees:

* Append-only behavior
* Idempotent writes
* Duplicate chunk protection
* Rotation-safe handling

---

## 5.3 Storage Layout

Example structure:

```
/data/
  /source-A/
    /logs/
      app.log
      service.log
    /artifacts/
      crash-2026-03-01.zip
```

Metadata schema includes:

* Sources
* Streams
* Artifacts
* Enrollment records
* Agent metrics snapshots

---

# 6. Security Model

## Enrollment Flow

1. Operator generates short-lived enrollment token.
2. Agent uses token to authenticate initial request.
3. Collector issues:

   * Client certificate OR
   * Long-lived API credential
4. Future connections use mutual TLS.

## Key Rotation

* Scheduled rotation interval
* Agent requests renewal before expiry
* Revocation list supported

## Secure Storage

* OS-native secure storage where available
* File permission restrictions as fallback

---

# 7. Protocol Design Principles

* Chunk-based
* Sequence-numbered
* Idempotent
* Backpressure-aware
* Compression-optional (gzip/zstd)
* Checksummed frames

Failure handling:

* Partial upload resumes
* Retry-safe writes
* Network interruption tolerant

---

# 8. Observability

Collector exposes:

* `/health`
* `/metrics`
* Agent status dashboard

Metrics include:

* Bytes received per source
* Queue sizes
* Last seen timestamp
* Error rates
* Upload latency

---

# 9. Scalability Path

Release 1:

* Single collector
* SQLite metadata

Release 3:

* Pluggable metadata store (Postgres)
* Multi-collector routing
* Forwarding to external sinks
* Role-based access control

---

# 10. Design Constraints

* Must tolerate collector downtime
* Must not consume unbounded memory
* Must avoid log corruption on retries
* Must operate in private/local network scenarios
* Must remain simple to deploy

---

This architecture intentionally favors reliability and clarity over early complexity. Scale features are additive, not foundational.

