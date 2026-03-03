package dashboard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kescott027/diag-gateway/collector/api/listing"
	"github.com/kescott027/diag-gateway/collector/storage/layout"
	"github.com/kescott027/diag-gateway/collector/telemetry/agentmetrics"
	"github.com/kescott027/diag-gateway/collector/telemetry/lastseen"
)

const StatusUnknown = "unknown"

type Query struct {
	SourceIDs []string `json:"source_ids,omitempty"`
}

// Card is one per-source dashboard summary row.
type Card struct {
	SourceID                 string    `json:"source_id"`
	Status                   string    `json:"status"`
	LastSeenAt               time.Time `json:"last_seen_at,omitempty"`
	AgeSeconds               float64   `json:"age_seconds,omitempty"`
	StreamCount              int       `json:"stream_count"`
	ArtifactCount            int       `json:"artifact_count"`
	TotalBytes               int64     `json:"total_bytes"`
	TotalErrors              int64     `json:"total_errors"`
	QueueDepth               int64     `json:"queue_depth"`
	ThroughputBytesPerSecond float64   `json:"throughput_bytes_per_second"`
	ErrorRatePerSecond       float64   `json:"error_rate_per_second"`
}

type lastSeenSnapshotter interface {
	Snapshot(staleAfter time.Duration) []lastseen.Record
}

type metricsSnapshotter interface {
	Snapshot() []agentmetrics.Record
}

// Service builds source-level dashboard cards from listing and telemetry sources.
type Service struct {
	dataDir    string
	staleAfter time.Duration

	listing  *listing.Service
	lastSeen lastSeenSnapshotter
	metrics  metricsSnapshotter
}

func NewService(dataDir string, staleAfter time.Duration, lastSeenTracker lastSeenSnapshotter, metricsTracker metricsSnapshotter) *Service {
	if dataDir == "" {
		dataDir = "./collector-data"
	}
	if staleAfter <= 0 {
		staleAfter = 2 * time.Minute
	}
	return &Service{
		dataDir:    dataDir,
		staleAfter: staleAfter,
		listing:    listing.NewService(dataDir),
		lastSeen:   lastSeenTracker,
		metrics:    metricsTracker,
	}
}

func (s *Service) Cards(query Query) ([]Card, error) {
	catalog, err := s.listing.ListCatalog()
	if err != nil {
		return nil, err
	}

	streamCountBySource := make(map[string]int, len(catalog.Sources))
	sources := make(map[string]struct{}, len(catalog.Sources))
	for _, src := range catalog.Sources {
		streamCountBySource[src.SourceID] = len(src.Files)
		sources[src.SourceID] = struct{}{}
	}

	lastSeenBySource := make(map[string]lastseen.Record)
	if s.lastSeen != nil {
		for _, rec := range s.lastSeen.Snapshot(s.staleAfter) {
			lastSeenBySource[rec.SourceID] = rec
			sources[rec.SourceID] = struct{}{}
		}
	}

	metricsBySource := make(map[string]agentmetrics.Record)
	if s.metrics != nil {
		for _, rec := range s.metrics.Snapshot() {
			metricsBySource[rec.SourceID] = rec
			sources[rec.SourceID] = struct{}{}
		}
	}

	filter := make(map[string]struct{})
	if len(query.SourceIDs) > 0 {
		for _, sourceID := range query.SourceIDs {
			trimmed := strings.TrimSpace(sourceID)
			if trimmed == "" {
				continue
			}
			if _, err := layout.SourceRoot(s.dataDir, trimmed); err != nil {
				return nil, err
			}
			filter[trimmed] = struct{}{}
			sources[trimmed] = struct{}{}
		}
	}

	ids := make([]string, 0, len(sources))
	for sourceID := range sources {
		if len(filter) > 0 {
			if _, ok := filter[sourceID]; !ok {
				continue
			}
		}
		ids = append(ids, sourceID)
	}
	sort.Strings(ids)

	cards := make([]Card, 0, len(ids))
	for _, sourceID := range ids {
		artifactCount, err := s.countArtifacts(sourceID)
		if err != nil {
			return nil, err
		}

		card := Card{
			SourceID:      sourceID,
			Status:        StatusUnknown,
			StreamCount:   streamCountBySource[sourceID],
			ArtifactCount: artifactCount,
		}
		if liveness, ok := lastSeenBySource[sourceID]; ok {
			card.Status = string(liveness.Status)
			card.LastSeenAt = liveness.LastSeenAt
			card.AgeSeconds = liveness.AgeSeconds
		}
		if metrics, ok := metricsBySource[sourceID]; ok {
			card.TotalBytes = metrics.TotalBytes
			card.TotalErrors = metrics.TotalErrors
			card.QueueDepth = metrics.QueueDepth
			card.ThroughputBytesPerSecond = metrics.ThroughputBytesPerSecond
			card.ErrorRatePerSecond = metrics.ErrorRatePerSecond
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func (s *Service) countArtifacts(sourceID string) (int, error) {
	sourceRoot, err := layout.SourceRoot(s.dataDir, sourceID)
	if err != nil {
		return 0, err
	}
	artifactsDir := filepath.Join(sourceRoot, "artifacts")
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read artifacts dir for %s: %w", sourceID, err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := layout.ArtifactDir(s.dataDir, sourceID, entry.Name()); err != nil {
			continue
		}
		count++
	}
	return count, nil
}
