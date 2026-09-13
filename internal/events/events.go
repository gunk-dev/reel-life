// Package events provides a durable, redacted record of product outcomes.
package events

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const SchemaVersion = 1

// Event is one immutable observation in the product evidence ledger.
type Event struct {
	SchemaVersion int            `json:"schema_version"`
	ID            string         `json:"id"`
	Timestamp     time.Time      `json:"timestamp"`
	Type          string         `json:"type"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Component     string         `json:"component,omitempty"`
	Operation     string         `json:"operation,omitempty"`
	Outcome       string         `json:"outcome,omitempty"`
	DurationMS    int64          `json:"duration_ms,omitempty"`
	ErrorKind     string         `json:"error_kind,omitempty"`
	Attributes    map[string]any `json:"attributes,omitempty"`
}

// Recorder accepts evidence events. Implementations must be safe for concurrent use.
type Recorder interface {
	Record(context.Context, Event) error
}

// FileRecorder stores one JSON event per line and syncs it before returning.
type FileRecorder struct {
	path string
	mu   sync.Mutex
}

func NewFileRecorder(path string) *FileRecorder { return &FileRecorder{path: path} }

// Replay reads complete ledger entries under the recorder's lock. Unlike an
// analytics report, recovery must reject damaged or unsupported history rather
// than silently forget an action. A missing file represents a first run.
// The visitor must not call back into this recorder while the lock is held.
func (r *FileRecorder) Replay(visit func(Event) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, err := os.Open(r.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open event ledger for recovery: %w", err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if atEOF && len(data) > 0 && !bytes.ContainsRune(data, '\n') {
			return 0, nil, fmt.Errorf("unterminated ledger entry")
		}
		return bufio.ScanLines(data, atEOF)
	})
	line := 0
	for scanner.Scan() {
		line++
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("decode ledger entry %d: %w", line, err)
		}
		if event.SchemaVersion != SchemaVersion || event.Type == "" {
			return fmt.Errorf("invalid or unsupported ledger entry %d", line)
		}
		if err := visit(event); err != nil {
			return fmt.Errorf("recover ledger entry %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read ledger after entry %d: %w", line, err)
	}
	return nil
}

func (r *FileRecorder) Record(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(event.Type) == "" {
		return fmt.Errorf("event type is required")
	}
	if event.SchemaVersion == 0 {
		event.SchemaVersion = SchemaVersion
	}
	if event.ID == "" {
		id, err := newID()
		if err != nil {
			return err
		}
		event.ID = id
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.Attributes = redactMap(event.Attributes)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	data = append(data, '\n')

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return fmt.Errorf("create event directory: %w", err)
	}
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open event ledger: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("append event: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync event ledger: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close event ledger: %w", err)
	}
	return nil
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate event ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func redactMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		if sensitiveKey(key) {
			result[key] = "[REDACTED]"
			continue
		}
		switch nested := value.(type) {
		case map[string]any:
			result[key] = redactMap(nested)
		case []any:
			result[key] = redactSlice(nested)
		default:
			result[key] = value
		}
	}
	return result
}

func redactSlice(values []any) []any {
	result := make([]any, len(values))
	for i, value := range values {
		switch nested := value.(type) {
		case map[string]any:
			result[i] = redactMap(nested)
		case []any:
			result[i] = redactSlice(nested)
		default:
			result[i] = value
		}
	}
	return result
}

func sensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, fragment := range []string{"api_key", "apikey", "authorization", "credential", "password", "secret", "token"} {
		if strings.Contains(k, fragment) {
			return true
		}
	}
	return false
}
