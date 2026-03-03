package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type contextKey string

const correlationIDKey contextKey = "correlation_id"

// Entry is the canonical structured log shape used by the platform.
type Entry struct {
	Timestamp     string         `json:"timestamp"`
	Level         string         `json:"level"`
	Component     string         `json:"component"`
	Message       string         `json:"message"`
	CorrelationID string         `json:"correlation_id"`
	Fields        map[string]any `json:"fields,omitempty"`
}

// Logger writes newline-delimited JSON log entries.
type Logger struct {
	component string
	out       io.Writer
	now       func() time.Time
	mu        sync.Mutex
}

// New creates a structured logger for a component.
func New(component string, out io.Writer) *Logger {
	if out == nil {
		out = os.Stdout
	}
	return &Logger{
		component: component,
		out:       out,
		now:       time.Now,
	}
}

// WithCorrelationID stores a correlation id in context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	if correlationID == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// CorrelationIDFromContext returns a correlation id if present.
func CorrelationIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	raw := ctx.Value(correlationIDKey)
	if raw == nil {
		return "", false
	}
	id, ok := raw.(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// NewCorrelationID returns a random hex correlation id.
func NewCorrelationID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("cid-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

// Log emits a structured log entry.
func (l *Logger) Log(ctx context.Context, level, message string, fields map[string]any) error {
	if l == nil {
		return fmt.Errorf("logger is nil")
	}
	correlationID, ok := CorrelationIDFromContext(ctx)
	if !ok {
		correlationID = NewCorrelationID()
	}
	entry := Entry{
		Timestamp:     l.now().UTC().Format(time.RFC3339Nano),
		Level:         normalizeLevel(level),
		Component:     l.component,
		Message:       message,
		CorrelationID: correlationID,
		Fields:        fields,
	}

	blob, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal log entry: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.out.Write(append(blob, '\n')); err != nil {
		return fmt.Errorf("write log entry: %w", err)
	}
	return nil
}

func (l *Logger) Debug(ctx context.Context, message string, fields map[string]any) error {
	return l.Log(ctx, "debug", message, fields)
}

func (l *Logger) Info(ctx context.Context, message string, fields map[string]any) error {
	return l.Log(ctx, "info", message, fields)
}

func (l *Logger) Warn(ctx context.Context, message string, fields map[string]any) error {
	return l.Log(ctx, "warn", message, fields)
}

func (l *Logger) Error(ctx context.Context, message string, fields map[string]any) error {
	return l.Log(ctx, "error", message, fields)
}

func normalizeLevel(level string) string {
	out := strings.ToLower(strings.TrimSpace(level))
	if out == "" {
		return "info"
	}
	return out
}

