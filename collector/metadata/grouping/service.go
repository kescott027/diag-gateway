package grouping

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kescott027/diag-gateway/collector/metadata/store"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Assignment defines source grouping/tag metadata updates.
type Assignment struct {
	SourceID string   `json:"source_id"`
	GroupID  string   `json:"group_id,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// Service manages source group/tag metadata via the pluggable store.
type Service struct {
	store store.Store
	now   func() time.Time
}

func NewService(st store.Store) (*Service, error) {
	if st == nil {
		return nil, fmt.Errorf("metadata store is required")
	}
	return &Service{
		store: st,
		now:   time.Now,
	}, nil
}

// Assign applies group/tag metadata to one source.
func (s *Service) Assign(ctx context.Context, in Assignment) (store.SourceRecord, error) {
	sourceID, err := normalizeIdentifier("source_id", in.SourceID, false)
	if err != nil {
		return store.SourceRecord{}, err
	}
	groupID, err := normalizeIdentifier("group_id", in.GroupID, true)
	if err != nil {
		return store.SourceRecord{}, err
	}
	tags, err := normalizeTags(in.Tags)
	if err != nil {
		return store.SourceRecord{}, err
	}

	rec, err := s.store.GetSource(ctx, sourceID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return store.SourceRecord{}, err
		}
		rec = store.SourceRecord{
			SourceID: sourceID,
			Status:   "unknown",
		}
	}
	rec.GroupID = groupID
	rec.Tags = tags
	rec.UpdatedAt = s.now().UTC()
	if err := s.store.UpsertSource(ctx, rec); err != nil {
		return store.SourceRecord{}, err
	}
	return s.store.GetSource(ctx, sourceID)
}

// ListByGroup returns deterministic source records for a group.
func (s *Service) ListByGroup(ctx context.Context, groupID string) ([]store.SourceRecord, error) {
	normalized, err := normalizeIdentifier("group_id", groupID, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]store.SourceRecord, 0)
	for _, row := range rows {
		if row.GroupID == normalized {
			out = append(out, row)
		}
	}
	return out, nil
}

// ListByTag returns deterministic source records containing a tag.
func (s *Service) ListByTag(ctx context.Context, tag string) ([]store.SourceRecord, error) {
	normalizedTags, err := normalizeTags([]string{tag})
	if err != nil {
		return nil, err
	}
	if len(normalizedTags) != 1 {
		return nil, fmt.Errorf("tag is required")
	}
	target := normalizedTags[0]

	rows, err := s.store.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]store.SourceRecord, 0)
	for _, row := range rows {
		if hasTag(row.Tags, target) {
			out = append(out, row)
		}
	}
	return out, nil
}

func hasTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func normalizeIdentifier(name, value string, optional bool) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		if optional {
			return "", nil
		}
		return "", fmt.Errorf("%s is required", name)
	}
	if !idPattern.MatchString(trimmed) {
		return "", fmt.Errorf("%s contains invalid characters: %q", name, value)
	}
	return trimmed, nil
}

func normalizeTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	set := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		normalized, err := normalizeIdentifier("tag", tag, false)
		if err != nil {
			return nil, err
		}
		set[normalized] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out, nil
}
