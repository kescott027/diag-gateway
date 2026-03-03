# STORAGE_LAYOUT.md

# Collector Storage Layout

## 1. Root Directory

Configured via:

collector.data_dir

Default:

./data

---

## 2. Directory Structure

/data/
/sources/
/{source_id}/
/streams/
/{stream_id}/
stream.log
metadata.json
/artifacts/
/{artifact_id}/
artifact.bin
metadata.json
metrics.json

---

## 3. Streams

stream.log

* Append-only file
* UTF-8 text (binary-safe)
* No rewrites allowed

metadata.json

* logical_path
* created_at
* last_offset
* status

---

## 4. Artifacts

artifact.bin

* Raw uploaded file

metadata.json

* name
* size
* checksum
* upload_timestamp

---

## 5. Database (SQLite)

Tables:

sources
streams
artifacts
enrollments
agent_metrics

Database contains:

* Lookup data
* Stream status
* Enrollment records

Database does NOT store log contents.

---

## 6. Retention

Retention policy:

* Streams older than retention_days archived or deleted
* Artifacts older than retention_days deleted
* Metrics compacted periodically

Deletion rules:

* Only completed streams eligible
* Active streams protected

---

## 7. Disk Safety

Collector enforces:

* Max disk usage threshold (configurable future feature)
* Refuses new artifact uploads when threshold exceeded

---

End Storage Layout
