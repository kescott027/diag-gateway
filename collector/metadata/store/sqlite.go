package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a sqlite-backed metadata adapter.
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	if path == "" {
		path = "./collector-data/metadata/metadata.db"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	store := &SQLiteStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) init() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS sources (
			source_id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			last_seen_at TEXT,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS streams (
			source_id TEXT NOT NULL,
			stream_id TEXT NOT NULL,
			logical_path TEXT,
			status TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(source_id, stream_id)
		);`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			source_id TEXT NOT NULL,
			artifact_id TEXT NOT NULL,
			status TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			checksum TEXT,
			completed_at TEXT,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(source_id, artifact_id)
		);`,
	}
	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("init sqlite schema: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) UpsertSource(ctx context.Context, rec SourceRecord) error {
	if err := validateID("source_id", rec.SourceID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sources (source_id, status, last_seen_at, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(source_id) DO UPDATE SET
		   status=excluded.status,
		   last_seen_at=excluded.last_seen_at,
		   updated_at=excluded.updated_at`,
		rec.SourceID, rec.Status, timeToText(rec.LastSeenAt), timeToText(rec.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert source: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSource(ctx context.Context, sourceID string) (SourceRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return SourceRecord{}, err
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT source_id, status, last_seen_at, updated_at FROM sources WHERE source_id = ?`, sourceID)
	var rec SourceRecord
	var lastSeenText string
	var updatedText string
	if err := row.Scan(&rec.SourceID, &rec.Status, &lastSeenText, &updatedText); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SourceRecord{}, ErrNotFound
		}
		return SourceRecord{}, fmt.Errorf("get source: %w", err)
	}
	rec.LastSeenAt = parseTime(lastSeenText)
	rec.UpdatedAt = parseTime(updatedText)
	return rec, nil
}

func (s *SQLiteStore) ListSources(ctx context.Context) ([]SourceRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT source_id, status, last_seen_at, updated_at FROM sources ORDER BY source_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	out := make([]SourceRecord, 0)
	for rows.Next() {
		var rec SourceRecord
		var lastSeenText string
		var updatedText string
		if err := rows.Scan(&rec.SourceID, &rec.Status, &lastSeenText, &updatedText); err != nil {
			return nil, fmt.Errorf("scan source row: %w", err)
		}
		rec.LastSeenAt = parseTime(lastSeenText)
		rec.UpdatedAt = parseTime(updatedText)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) UpsertStream(ctx context.Context, rec StreamRecord) error {
	if err := validateID("source_id", rec.SourceID); err != nil {
		return err
	}
	if err := validateID("stream_id", rec.StreamID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO streams (source_id, stream_id, logical_path, status, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(source_id, stream_id) DO UPDATE SET
		   logical_path=excluded.logical_path,
		   status=excluded.status,
		   updated_at=excluded.updated_at`,
		rec.SourceID, rec.StreamID, rec.LogicalPath, rec.Status, timeToText(rec.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert stream: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetStream(ctx context.Context, sourceID, streamID string) (StreamRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return StreamRecord{}, err
	}
	if err := validateID("stream_id", streamID); err != nil {
		return StreamRecord{}, err
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT source_id, stream_id, logical_path, status, updated_at
		 FROM streams WHERE source_id = ? AND stream_id = ?`, sourceID, streamID)
	var rec StreamRecord
	var updatedText string
	if err := row.Scan(&rec.SourceID, &rec.StreamID, &rec.LogicalPath, &rec.Status, &updatedText); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StreamRecord{}, ErrNotFound
		}
		return StreamRecord{}, fmt.Errorf("get stream: %w", err)
	}
	rec.UpdatedAt = parseTime(updatedText)
	return rec, nil
}

func (s *SQLiteStore) ListStreamsBySource(ctx context.Context, sourceID string) ([]StreamRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT source_id, stream_id, logical_path, status, updated_at
		 FROM streams WHERE source_id = ? ORDER BY stream_id ASC`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list streams: %w", err)
	}
	defer rows.Close()

	out := make([]StreamRecord, 0)
	for rows.Next() {
		var rec StreamRecord
		var updatedText string
		if err := rows.Scan(&rec.SourceID, &rec.StreamID, &rec.LogicalPath, &rec.Status, &updatedText); err != nil {
			return nil, fmt.Errorf("scan stream row: %w", err)
		}
		rec.UpdatedAt = parseTime(updatedText)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) UpsertArtifact(ctx context.Context, rec ArtifactRecord) error {
	if err := validateID("source_id", rec.SourceID); err != nil {
		return err
	}
	if err := validateID("artifact_id", rec.ArtifactID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO artifacts (source_id, artifact_id, status, size_bytes, checksum, completed_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(source_id, artifact_id) DO UPDATE SET
		   status=excluded.status,
		   size_bytes=excluded.size_bytes,
		   checksum=excluded.checksum,
		   completed_at=excluded.completed_at,
		   updated_at=excluded.updated_at`,
		rec.SourceID, rec.ArtifactID, rec.Status, rec.SizeBytes, rec.Checksum, timeToText(rec.CompletedAt), timeToText(rec.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert artifact: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetArtifact(ctx context.Context, sourceID, artifactID string) (ArtifactRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return ArtifactRecord{}, err
	}
	if err := validateID("artifact_id", artifactID); err != nil {
		return ArtifactRecord{}, err
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT source_id, artifact_id, status, size_bytes, checksum, completed_at, updated_at
		 FROM artifacts WHERE source_id = ? AND artifact_id = ?`, sourceID, artifactID)
	var rec ArtifactRecord
	var completedText, updatedText string
	if err := row.Scan(&rec.SourceID, &rec.ArtifactID, &rec.Status, &rec.SizeBytes, &rec.Checksum, &completedText, &updatedText); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ArtifactRecord{}, ErrNotFound
		}
		return ArtifactRecord{}, fmt.Errorf("get artifact: %w", err)
	}
	rec.CompletedAt = parseTime(completedText)
	rec.UpdatedAt = parseTime(updatedText)
	return rec, nil
}

func (s *SQLiteStore) ListArtifactsBySource(ctx context.Context, sourceID string) ([]ArtifactRecord, error) {
	if err := validateID("source_id", sourceID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT source_id, artifact_id, status, size_bytes, checksum, completed_at, updated_at
		 FROM artifacts WHERE source_id = ? ORDER BY artifact_id ASC`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	defer rows.Close()

	out := make([]ArtifactRecord, 0)
	for rows.Next() {
		var rec ArtifactRecord
		var completedText, updatedText string
		if err := rows.Scan(&rec.SourceID, &rec.ArtifactID, &rec.Status, &rec.SizeBytes, &rec.Checksum, &completedText, &updatedText); err != nil {
			return nil, fmt.Errorf("scan artifact row: %w", err)
		}
		rec.CompletedAt = parseTime(completedText)
		rec.UpdatedAt = parseTime(updatedText)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func timeToText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return ts
	}
	ts, err = time.Parse(time.RFC3339, value)
	if err == nil {
		return ts
	}
	return time.Time{}
}
