package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/patflynn/reel-life/internal/sonarr"
)

func TestRateLimiterAllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, 3, 2, 1)
	if err := rl.Allow("search_series", false, false); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	if err := rl.Allow("add_series", true, false); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestRateLimiterBlocksPerRequest(t *testing.T) {
	rl := NewRateLimiter(100, 2, 100, 100)
	rl.Allow("search_series", false, false)
	rl.Allow("get_queue", false, false)
	err := rl.Allow("check_health", false, false)
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if !strings.Contains(err.Error(), "per request") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRateLimiterBlocksPerMinute(t *testing.T) {
	rl := NewRateLimiter(2, 100, 100, 100)
	rl.Allow("search_series", false, false)
	rl.Allow("get_queue", false, false)
	err := rl.Allow("check_health", false, false)
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if !strings.Contains(err.Error(), "per minute") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRateLimiterBlocksDestructive(t *testing.T) {
	rl := NewRateLimiter(100, 100, 100, 1)
	rl.Allow("remove_failed", true, true)
	err := rl.Allow("remove_failed", true, true)
	if err == nil {
		t.Fatal("expected rate limit error for destructive action")
	}
	if !strings.Contains(err.Error(), "destructive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRateLimiterBlocksMutative(t *testing.T) {
	rl := NewRateLimiter(100, 100, 3, 100)
	for i := 0; i < 3; i++ {
		if err := rl.Allow("add_movie", true, false); err != nil {
			t.Fatalf("expected allow on call %d, got %v", i+1, err)
		}
	}
	err := rl.Allow("add_movie", true, false)
	if err == nil {
		t.Fatal("expected rate limit error for mutative action")
	}
	if !strings.Contains(err.Error(), "content changes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMutativeToolsAllowedUpTo20(t *testing.T) {
	rl := NewRateLimiter(100, 100, 20, 5)
	for i := 0; i < 20; i++ {
		if err := rl.Allow("add_movie", true, false); err != nil {
			t.Fatalf("expected allow on call %d, got %v", i+1, err)
		}
	}
	err := rl.Allow("add_movie", true, false)
	if err == nil {
		t.Fatal("expected rate limit error after 20 mutative calls")
	}
}

func TestDestructiveCountsTowardBothLimits(t *testing.T) {
	rl := NewRateLimiter(100, 100, 5, 100)
	// Use up mutative limit with destructive calls (which are implicitly mutative)
	for i := 0; i < 5; i++ {
		if err := rl.Allow("remove_failed", true, true); err != nil {
			t.Fatalf("expected allow on call %d, got %v", i+1, err)
		}
	}
	// Now even a mutative-only call should be blocked by the mutative limit
	err := rl.Allow("add_movie", true, false)
	if err == nil {
		t.Fatal("expected mutative limit error")
	}
	if !strings.Contains(err.Error(), "content changes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRateLimiterReset(t *testing.T) {
	rl := NewRateLimiter(100, 1, 1, 1)
	rl.Allow("search_series", false, false)
	if err := rl.Allow("search_series", false, false); err == nil {
		t.Fatal("expected rate limit before reset")
	}
	rl.Reset()
	if err := rl.Allow("search_series", false, false); err != nil {
		t.Fatalf("expected allow after reset, got %v", err)
	}
}

func TestIsMutative(t *testing.T) {
	if !IsMutative("add_series") {
		t.Error("add_series should be mutative")
	}
	if !IsMutative("add_movie") {
		t.Error("add_movie should be mutative")
	}
	if !IsMutative("remove_failed") {
		t.Error("remove_failed should be mutative (destructive implies mutative)")
	}
	if IsMutative("search_series") {
		t.Error("search_series should not be mutative")
	}
}

func TestIsDestructive(t *testing.T) {
	if IsDestructive("add_series") {
		t.Error("add_series should NOT be destructive (it's mutative)")
	}
	if IsDestructive("add_movie") {
		t.Error("add_movie should NOT be destructive (it's mutative)")
	}
	if !IsDestructive("remove_failed") {
		t.Error("remove_failed should be destructive")
	}
	if !IsDestructive("delete_series") {
		t.Error("delete_series should be destructive")
	}
	if !IsDestructive("delete_movie") {
		t.Error("delete_movie should be destructive")
	}
	if !IsDestructive("remove_blocklist_item") {
		t.Error("remove_blocklist_item should be destructive")
	}
	if IsDestructive("search_series") {
		t.Error("search_series should not be destructive")
	}
	if IsDestructive("get_queue") {
		t.Error("get_queue should not be destructive")
	}
}

func TestRateLimiterDenialReturnedAsToolError(t *testing.T) {
	rl := NewRateLimiter(100, 100, 0, 0) // zero mutative/destructive allowed
	mock := &mockSonarr{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a := &Agent{sonarr: mock, radarr: &mockRadarr{}, logger: logger, limiter: rl}

	input, _ := json.Marshal(addSeriesInput{
		Title: "Test", TVDBID: 1, QualityProfileID: 1, RootFolderPath: "/tv",
	})
	result := a.executeToolWithAudit(context.Background(), "add_series", input, 0, "req-1")
	if result.Success {
		t.Fatal("expected error from rate-limited tool")
	}
	if !strings.Contains(result.Error, "content changes") {
		t.Errorf("expected content changes limit message, got %s", result.Error)
	}
	if result.ErrorKind != "rate_limited" {
		t.Errorf("expected error_kind=rate_limited, got %q", result.ErrorKind)
	}
}

func TestAuditLogging(t *testing.T) {
	mock := &mockSonarr{
		searchResult: []sonarr.Series{{ID: 1, Title: "Test", TVDBID: 123}},
	}
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a := &Agent{sonarr: mock, radarr: &mockRadarr{}, logger: logger}

	input, _ := json.Marshal(searchSeriesInput{Term: "test"})
	a.executeToolWithAudit(context.Background(), "search_series", input, 0, "req-42")

	logs := buf.String()
	if !strings.Contains(logs, "tool invocation") {
		t.Error("missing 'tool invocation' log entry")
	}
	if !strings.Contains(logs, "tool result") {
		t.Error("missing 'tool result' log entry")
	}
	if !strings.Contains(logs, "req-42") {
		t.Error("missing request_id in log")
	}
	if !strings.Contains(logs, "search_series") {
		t.Error("missing tool name in log")
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldHave  []string
		shouldNotHave []string
	}{
		{
			name:        "exact key api_key",
			input:       `{"term":"test","api_key":"secret123"}`,
			shouldHave:  []string{"REDACTED", "test"},
			shouldNotHave: []string{"secret123"},
		},
		{
			name:        "exact key secret",
			input:       `{"name":"foo","secret":"hunter2"}`,
			shouldHave:  []string{"REDACTED", "foo"},
			shouldNotHave: []string{"hunter2"},
		},
		{
			name:        "exact key password",
			input:       `{"user":"admin","password":"p@ss"}`,
			shouldHave:  []string{"REDACTED", "admin"},
			shouldNotHave: []string{"p@ss"},
		},
		{
			name:        "exact key token",
			input:       `{"token":"abc123","query":"hello"}`,
			shouldHave:  []string{"REDACTED", "hello"},
			shouldNotHave: []string{"abc123"},
		},
		{
			name:        "substring match sonarr_api_key",
			input:       `{"sonarr_api_key":"xyz","term":"show"}`,
			shouldHave:  []string{"REDACTED", "show"},
			shouldNotHave: []string{"xyz"},
		},
		{
			name:        "substring match user_token",
			input:       `{"user_token":"tok99","id":1}`,
			shouldHave:  []string{"REDACTED"},
			shouldNotHave: []string{"tok99"},
		},
		{
			name:        "case insensitive API_KEY",
			input:       `{"API_KEY":"upper","value":"safe"}`,
			shouldHave:  []string{"REDACTED", "safe"},
			shouldNotHave: []string{"upper"},
		},
		{
			name:       "invalid json",
			input:      `not json`,
			shouldHave: []string{"<invalid json>"},
		},
		{
			name:       "no sensitive fields",
			input:      `{"term":"test","count":5}`,
			shouldHave: []string{"test"},
			shouldNotHave: []string{"REDACTED"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeInput(json.RawMessage(tt.input))
			for _, s := range tt.shouldHave {
				if !strings.Contains(result, s) {
					t.Errorf("expected result to contain %q, got %s", s, result)
				}
			}
			for _, s := range tt.shouldNotHave {
				if strings.Contains(result, s) {
					t.Errorf("expected result NOT to contain %q, got %s", s, result)
				}
			}
		})
	}
}
