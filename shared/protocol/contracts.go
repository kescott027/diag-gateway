package protocol

import "time"

// Version identifies the protocol schema version emitted by this package.
const Version = "1.0"

// SamplingFlag captures retention handling for an event.
type SamplingFlag string

const (
	SamplingFull           SamplingFlag = "full"
	SamplingSampled        SamplingFlag = "sampled"
	SamplingAggregatedOnly SamplingFlag = "aggregated-only"
)

// PriorityFlag captures event processing urgency.
type PriorityFlag string

const (
	PriorityCritical PriorityFlag = "critical"
	PriorityHigh     PriorityFlag = "high"
	PriorityNormal   PriorityFlag = "normal"
	PriorityLow      PriorityFlag = "low"
)

// StructureClass indicates parser confidence class for raw log payload.
type StructureClass string

const (
	StructureStructured     StructureClass = "structured"
	StructureSemiStructured StructureClass = "semi_structured"
	StructureUnstructured   StructureClass = "unstructured"
)

// MessageType identifies wire message type.
type MessageType string

const (
	MessageEnrollmentRequest  MessageType = "EnrollmentRequest"
	MessageEnrollmentResponse MessageType = "EnrollmentResponse"
	MessageStreamOpen         MessageType = "StreamOpen"
	MessageStreamChunk        MessageType = "StreamChunk"
	MessageStreamClose        MessageType = "StreamClose"
	MessageArtifactStart      MessageType = "ArtifactUploadStart"
	MessageArtifactChunk      MessageType = "ArtifactChunk"
	MessageArtifactComplete   MessageType = "ArtifactComplete"
	MessageSignalBatch        MessageType = "SignalBatch"
	MessageHeartbeat          MessageType = "Heartbeat"
	MessageMetrics            MessageType = "Metrics"
)

// Metadata is embedded into protocol messages for compatibility checks.
type Metadata struct {
	ProtocolVersion string      `json:"protocol_version"`
	SchemaVersion   string      `json:"schema_version"`
	MessageType     MessageType `json:"message_type"`
}

// EventEnvelope is mandatory for all event payloads entering the system.
type EventEnvelope struct {
	EventTime           time.Time      `json:"event_time"`
	IngestTime          time.Time      `json:"ingest_time"`
	TenantID            string         `json:"tenant_id"`
	SourceID            string         `json:"source_id"`
	SourceType          string         `json:"source_type"`
	RawPayload          string         `json:"raw_payload,omitempty"`
	RawPointer          string         `json:"raw_pointer,omitempty"`
	Fingerprint         string         `json:"fingerprint"`
	FingerprintVersion  string         `json:"fingerprint_version"`
	StructureClass      StructureClass `json:"structure_class"`
	StructureConfidence float64        `json:"structure_confidence"`
	SamplingFlag        SamplingFlag   `json:"sampling_flag"`
	PriorityFlag        PriorityFlag   `json:"priority_flag"`
}

// SignalWindow defines the temporal scope used for derived detection outputs.
type SignalWindow struct {
	Start           time.Time `json:"start"`
	End             time.Time `json:"end"`
	DurationSeconds int       `json:"duration_seconds"`
}

// SignalDeviation captures the measured metric and its deviation value.
type SignalDeviation struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"`
}

// Signal is the structured summary format for correlator outputs.
type Signal struct {
	SignalID                 string          `json:"signal_id"`
	TenantID                 string          `json:"tenant_id"`
	SourceScope              []string        `json:"source_scope"`
	SignalType               string          `json:"signal_type"`
	Score                    float64         `json:"score"`
	Confidence               float64         `json:"confidence"`
	Window                   SignalWindow    `json:"window"`
	ContributingFingerprints []string        `json:"contributing_fingerprints"`
	Deviation                SignalDeviation `json:"deviation"`
	SamplingImpact           string          `json:"sampling_impact"`
	Explainability           string          `json:"explainability"`
}

// StreamOpen declares the start of a file stream.
type StreamOpen struct {
	Metadata
	SourceID               string            `json:"source_id"`
	Timestamp              time.Time         `json:"timestamp"`
	StreamID               string            `json:"stream_id"`
	LogicalPath            string            `json:"logical_path"`
	FileIdentity           string            `json:"file_identity"`
	FileIdentityConfidence string            `json:"file_identity_confidence,omitempty"`
	InitialOffset          int64             `json:"initial_offset"`
	FileSizeAtOpen         int64             `json:"file_size_at_open"`
	Attributes             map[string]string `json:"attributes,omitempty"`
}

// StreamChunk transports append-only data increments.
type StreamChunk struct {
	Metadata
	SourceID      string    `json:"source_id"`
	Timestamp     time.Time `json:"timestamp"`
	StreamID      string    `json:"stream_id"`
	Sequence      int64     `json:"sequence"`
	Offset        int64     `json:"offset"`
	PayloadLength int       `json:"payload_length"`
	Checksum      string    `json:"checksum"`
	Compressed    bool      `json:"compressed"`
	Payload       []byte    `json:"payload,omitempty"`
}

// StreamClose marks completion of a stream segment.
type StreamClose struct {
	Metadata
	SourceID    string    `json:"source_id"`
	Timestamp   time.Time `json:"timestamp"`
	StreamID    string    `json:"stream_id"`
	FinalOffset int64     `json:"final_offset"`
	Reason      string    `json:"reason"`
}

// ArtifactUploadStart starts artifact transfer.
type ArtifactUploadStart struct {
	Metadata
	SourceID   string            `json:"source_id"`
	Timestamp  time.Time         `json:"timestamp"`
	ArtifactID string            `json:"artifact_id"`
	Name       string            `json:"name"`
	TotalSize  int64             `json:"total_size"`
	Checksum   string            `json:"checksum"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ArtifactChunk transports a portion of artifact bytes.
type ArtifactChunk struct {
	Metadata
	SourceID      string    `json:"source_id"`
	Timestamp     time.Time `json:"timestamp"`
	ArtifactID    string    `json:"artifact_id"`
	Sequence      int64     `json:"sequence"`
	Offset        int64     `json:"offset"`
	PayloadLength int       `json:"payload_length"`
	Checksum      string    `json:"checksum"`
	Payload       []byte    `json:"payload,omitempty"`
}

// ArtifactComplete completes artifact transfer.
type ArtifactComplete struct {
	Metadata
	SourceID   string    `json:"source_id"`
	Timestamp  time.Time `json:"timestamp"`
	ArtifactID string    `json:"artifact_id"`
}

// SignalBatch carries derived signal output.
type SignalBatch struct {
	Metadata
	SourceID  string    `json:"source_id"`
	Timestamp time.Time `json:"timestamp"`
	Signals   []Signal  `json:"signals"`
}

// Heartbeat carries liveness and lightweight runtime metrics.
type Heartbeat struct {
	Metadata
	SourceID       string             `json:"source_id"`
	Timestamp      time.Time          `json:"timestamp"`
	AgentVersion   string             `json:"agent_version"`
	UptimeSeconds  int64              `json:"uptime_seconds"`
	QueueDepth     int                `json:"queue_depth"`
	MemoryUsageMB  float64            `json:"memory_usage_mb"`
	CPUPercent     float64            `json:"cpu_percent"`
	OptionalMetric map[string]float64 `json:"optional_metric,omitempty"`
}
