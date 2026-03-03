package search

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/kescott027/diag-gateway/collector/api/listing"
	"github.com/kescott027/diag-gateway/collector/storage/layout"
)

const (
	defaultMaxResults = 100
	maxAllowedResults = 1000
)

// Query captures search behavior over collector stream logs.
type Query struct {
	Pattern       string `json:"pattern"`
	UseRegex      bool   `json:"use_regex"`
	CaseSensitive bool   `json:"case_sensitive"`
	MaxResults    int    `json:"max_results"`
}

// Result is one matching log line with deterministic source/stream context.
type Result struct {
	SourceID    string `json:"source_id"`
	StreamID    string `json:"stream_id"`
	LogicalPath string `json:"logical_path"`
	LineNumber  int    `json:"line_number"`
	ByteOffset  int64  `json:"byte_offset"`
	Line        string `json:"line"`
}

// Service provides search primitives over append-only stream logs.
type Service struct {
	dataDir string
	catalog *listing.Service
}

func NewService(dataDir string) *Service {
	return &Service{
		dataDir: dataDir,
		catalog: listing.NewService(dataDir),
	}
}

func (s *Service) Search(query Query) ([]Result, error) {
	matcher, limit, err := compileMatcher(query)
	if err != nil {
		return nil, err
	}

	sourceIDs, err := s.catalog.ListSources()
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, min(limit, 64))
	for _, sourceID := range sourceIDs {
		sourceResults, err := s.searchSourceWithMatcher(sourceID, matcher, limit-len(results))
		if err != nil {
			return nil, err
		}
		results = append(results, sourceResults...)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (s *Service) SearchSource(sourceID string, query Query) ([]Result, error) {
	matcher, limit, err := compileMatcher(query)
	if err != nil {
		return nil, err
	}
	return s.searchSourceWithMatcher(sourceID, matcher, limit)
}

type lineMatcher func(line string) bool

func compileMatcher(query Query) (lineMatcher, int, error) {
	pattern := strings.TrimSpace(query.Pattern)
	if pattern == "" {
		return nil, 0, fmt.Errorf("pattern is required")
	}

	limit := query.MaxResults
	if limit <= 0 {
		limit = defaultMaxResults
	}
	if limit > maxAllowedResults {
		limit = maxAllowedResults
	}

	if query.UseRegex {
		regexPattern := pattern
		if !query.CaseSensitive {
			regexPattern = "(?i)" + regexPattern
		}
		re, err := regexp.Compile(regexPattern)
		if err != nil {
			return nil, 0, fmt.Errorf("compile regex pattern: %w", err)
		}
		return func(line string) bool { return re.MatchString(line) }, limit, nil
	}

	if query.CaseSensitive {
		return func(line string) bool { return strings.Contains(line, pattern) }, limit, nil
	}

	p := strings.ToLower(pattern)
	return func(line string) bool { return strings.Contains(strings.ToLower(line), p) }, limit, nil
}

func (s *Service) searchSourceWithMatcher(sourceID string, matcher lineMatcher, limit int) ([]Result, error) {
	if limit <= 0 {
		return []Result{}, nil
	}

	files, err := s.catalog.ListSourceFiles(sourceID)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, min(limit, 32))
	for _, file := range files {
		streamLogPath, err := layout.StreamLogPath(s.dataDir, sourceID, file.StreamID)
		if err != nil {
			continue
		}
		fileResults, err := searchFile(streamLogPath, file, matcher, limit-len(results))
		if err != nil {
			return nil, err
		}
		results = append(results, fileResults...)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func searchFile(path string, file listing.FileEntry, matcher lineMatcher, limit int) ([]Result, error) {
	if limit <= 0 {
		return []Result{}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Result{}, nil
		}
		return nil, fmt.Errorf("open stream log %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	results := make([]Result, 0, min(limit, 16))
	lineNumber := 0
	var byteOffset int64
	for scanner.Scan() {
		line := scanner.Text()
		lineNumber++

		if matcher(line) {
			results = append(results, Result{
				SourceID:    file.SourceID,
				StreamID:    file.StreamID,
				LogicalPath: file.LogicalPath,
				LineNumber:  lineNumber,
				ByteOffset:  byteOffset,
				Line:        line,
			})
			if len(results) >= limit {
				break
			}
		}

		byteOffset += int64(len(line) + 1)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan stream log %s: %w", path, err)
	}
	return results, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

