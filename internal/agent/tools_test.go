package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/patflynn/reel-life/internal/radarr"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantKind      string
		wantRetryable bool
	}{
		{
			name:     "nil",
			err:      nil,
			wantKind: "",
		},
		{
			name:     "auth 401",
			err:      errors.New("get queue: API error 401: unauthorized"),
			wantKind: "auth",
		},
		{
			name:     "auth 403",
			err:      errors.New("delete movie: API error 403: forbidden"),
			wantKind: "auth",
		},
		{
			name:     "not_found 404",
			err:      errors.New("get movie: API error 404: not found"),
			wantKind: "not_found",
		},
		{
			name:          "rate_limited 429",
			err:           errors.New("search movies: API error 429: too many requests"),
			wantKind:      "rate_limited",
			wantRetryable: true,
		},
		{
			name:     "invalid_input 400",
			err:      errors.New("add movie: API error 400: bad payload"),
			wantKind: "invalid_input",
		},
		{
			name:          "network 503",
			err:           errors.New("get queue: API error 503: service unavailable"),
			wantKind:      "network",
			wantRetryable: true,
		},
		{
			name:     "decode response",
			err:      errors.New("manual search: decode response: unexpected EOF"),
			wantKind: "decode",
		},
		{
			name:     "marshal payload",
			err:      errors.New("update movie: marshal movie: json: unsupported type"),
			wantKind: "decode",
		},
		{
			name:          "deadline exceeded",
			err:           context.DeadlineExceeded,
			wantKind:      "network",
			wantRetryable: true,
		},
		{
			name:          "canceled",
			err:           context.Canceled,
			wantKind:      "network",
			wantRetryable: false,
		},
		{
			name:          "connection refused",
			err:           errors.New("dial tcp 127.0.0.1:7878: connect: connection refused"),
			wantKind:      "network",
			wantRetryable: true,
		},
		{
			name:     "unknown",
			err:      errors.New("something weird happened"),
			wantKind: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKind, gotRetryable := classifyError(tt.err)
			if gotKind != tt.wantKind {
				t.Errorf("classifyError kind = %q, want %q", gotKind, tt.wantKind)
			}
			if gotRetryable != tt.wantRetryable {
				t.Errorf("classifyError retryable = %v, want %v", gotRetryable, tt.wantRetryable)
			}
		})
	}
}

func TestToolResultMarshalSuccess(t *testing.T) {
	r := successResult(`{"foo":"bar"}`)
	if r.Marshal() != `{"foo":"bar"}` {
		t.Errorf("success Marshal = %q, want raw content passthrough", r.Marshal())
	}
}

func TestToolResultMarshalError(t *testing.T) {
	r := errorResult("network", "boom", true)
	var got map[string]any
	if err := json.Unmarshal([]byte(r.Marshal()), &got); err != nil {
		t.Fatalf("Marshal error result not valid JSON: %v", err)
	}
	if got["success"] != false {
		t.Errorf("expected success=false, got %v", got["success"])
	}
	if got["error_kind"] != "network" {
		t.Errorf("expected error_kind=network, got %v", got["error_kind"])
	}
	if got["error"] != "boom" {
		t.Errorf("expected error=boom, got %v", got["error"])
	}
	if got["retryable"] != true {
		t.Errorf("expected retryable=true, got %v", got["retryable"])
	}
}

// failingRadarr returns a configurable error from ManualSearch to exercise
// the dispatcher's error-classification path.
type failingRadarr struct {
	mockRadarr
	manualSearchErr error
}

func (f *failingRadarr) ManualSearch(_ context.Context, _ int) ([]radarr.Release, error) {
	return nil, f.manualSearchErr
}

func TestDispatchManualMovieSearchSurfacesClassifiedError(t *testing.T) {
	mock := &failingRadarr{
		manualSearchErr: fmt.Errorf("manual search: API error 404: movie not found"),
	}
	a := &Agent{radarr: mock, sonarr: &mockSonarr{}, prowlarr: &mockProwlarr{}, overseerr: &mockOverseerr{}, logger: slog.Default()}

	input, _ := json.Marshal(manualMovieSearchInput{MovieID: 999})
	result := a.dispatchTool(context.Background(), "manual_movie_search", input)

	if result.Success {
		t.Fatal("expected dispatch to return Success=false on client error")
	}
	if result.ErrorKind != "not_found" {
		t.Errorf("expected ErrorKind=not_found, got %q", result.ErrorKind)
	}
	if result.Error == "" {
		t.Error("expected Error message to be populated")
	}
	if result.Retryable {
		t.Error("expected Retryable=false for not_found")
	}

	// Verify the wire form (what the model sees) carries the structured fields.
	var wire map[string]any
	if err := json.Unmarshal([]byte(result.Marshal()), &wire); err != nil {
		t.Fatalf("Marshal output not valid JSON: %v", err)
	}
	if wire["success"] != false {
		t.Errorf("wire success = %v, want false", wire["success"])
	}
	if wire["error_kind"] != "not_found" {
		t.Errorf("wire error_kind = %v, want not_found", wire["error_kind"])
	}
}

func TestDispatchInvalidInputClassifiedAsDecode(t *testing.T) {
	a := &Agent{radarr: &mockRadarr{}, sonarr: &mockSonarr{}, prowlarr: &mockProwlarr{}, overseerr: &mockOverseerr{}, logger: slog.Default()}

	// Trailing brace makes this invalid JSON; the dispatcher should classify as decode.
	result := a.dispatchTool(context.Background(), "manual_movie_search", json.RawMessage(`{not json}`))
	if result.Success {
		t.Fatal("expected dispatch to return Success=false on bad input")
	}
	if result.ErrorKind != "decode" {
		t.Errorf("expected ErrorKind=decode, got %q", result.ErrorKind)
	}
}
