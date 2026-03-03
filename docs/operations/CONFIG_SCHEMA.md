# CONFIG_SCHEMA.md

# Configuration Schema

All configuration files are YAML. JSON equivalent supported.

Precedence:

1. CLI flags
2. Environment variables
3. Config file
4. Defaults

---

# Agent Configuration

agent:
source_id: auto-generated
collector_url: https://collector.local:8443
data_dir: ./agent-data
spool:
max_size_mb: 512
max_memory_mb: 64
transport:
compression: gzip
retry_backoff_seconds: 5
max_retry_seconds: 300
security:
cert_path: ./certs/client.pem
key_path: ./certs/client.key

watch:

* path: /var/log/app
  include: ["*.log"]
  exclude: ["*.gz"]
  send_last_n_lines: 200
  max_file_size_mb: 200

adaptive_polling:
enabled: true
tiers:
- within_minutes: 1
interval_seconds: 1
- within_minutes: 15
interval_seconds: 15
- within_minutes: 60
interval_seconds: 30
- default_interval_seconds: 60

---

# Collector Configuration

collector:
bind_address: 0.0.0.0:8443
data_dir: ./data
db_path: ./collector.db
retention_days: 14
compression: gzip

security:
ca_path: ./ca
enrollment_token_ttl_minutes: 30
cert_rotation_days: 30

ui:
bind_address: 0.0.0.0:8080
enable_auth: false

---

Validation Rules

* max_file_size_mb must be > 0
* retry_backoff_seconds < max_retry_seconds
* retention_days >= 1
* collector_url must be HTTPS

---

End Configuration Schema
