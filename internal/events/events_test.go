package events

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFileRecorderAppendsRedactedEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "events.jsonl")
	recorder := NewFileRecorder(path)
	event := Event{Type: "tool.result", Attributes: map[string]any{
		"service": "sonarr", "api_token": "do-not-store",
		"nested": map[string]any{"password": "also-secret"},
		"list":   []any{map[string]any{"secret": "list-secret"}},
	}}
	if err := recorder.Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.ID == "" || got.Timestamp.IsZero() {
		t.Fatalf("missing envelope fields: %+v", got)
	}
	if got.Attributes["api_token"] != "[REDACTED]" {
		t.Fatalf("token was not redacted: %#v", got.Attributes)
	}
	nested := got.Attributes["nested"].(map[string]any)
	if nested["password"] != "[REDACTED]" {
		t.Fatalf("nested password was not redacted: %#v", nested)
	}
	list := got.Attributes["list"].([]any)
	if list[0].(map[string]any)["secret"] != "[REDACTED]" {
		t.Fatalf("secret in list was not redacted: %#v", list)
	}
}

func TestFileRecorderConcurrentWritesRemainValidJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	recorder := NewFileRecorder(path)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := recorder.Record(context.Background(), Event{Type: "test"}); err != nil {
				t.Errorf("record: %v", err)
			}
		}()
	}
	wg.Wait()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("invalid JSON line: %v", err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 20 {
		t.Fatalf("got %d events, want 20", count)
	}
}

func TestFileRecorderRejectsMissingType(t *testing.T) {
	err := NewFileRecorder(filepath.Join(t.TempDir(), "events.jsonl")).Record(context.Background(), Event{})
	if err == nil {
		t.Fatal("expected missing type error")
	}
}
