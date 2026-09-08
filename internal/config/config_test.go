package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  webhook_url: https://chat.googleapis.com/v1/spaces/test/messages?key=k&token=t
agent:
  model: claude-sonnet-4-5-20250929
  max_tokens: 2048
monitor:
  enabled: true
  interval: 10m
log:
  level: debug
  format: json
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Sonarr.BaseURL != "http://sonarr:8989" {
		t.Errorf("Sonarr.BaseURL = %q, want %q", cfg.Sonarr.BaseURL, "http://sonarr:8989")
	}
	if cfg.Agent.MaxTokens != 2048 {
		t.Errorf("Agent.MaxTokens = %d, want 2048", cfg.Agent.MaxTokens)
	}
	if cfg.Monitor.Interval.Minutes() != 10 {
		t.Errorf("Monitor.Interval = %v, want 10m", cfg.Monitor.Interval)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
}

func TestLoadDefaults(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  webhook_url: https://chat.example.com/webhook
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Agent.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("default Agent.Model = %q, want claude-sonnet-4-5-20250929", cfg.Agent.Model)
	}
	if cfg.Agent.MaxTokens != 4096 {
		t.Errorf("default Agent.MaxTokens = %d, want 4096", cfg.Agent.MaxTokens)
	}
	if !cfg.Monitor.Enabled {
		t.Error("default Monitor.Enabled = false, want true")
	}
	if cfg.Remediation.Enabled || cfg.Remediation.MaxAttempts != 1 || cfg.Remediation.Cooldown.String() != "1h0m0s" {
		t.Errorf("unexpected remediation defaults: %+v", cfg.Remediation)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: yaml-key
chat:
  webhook_url: https://chat.example.com/webhook
`
	path := writeTemp(t, yaml)

	t.Setenv("SONARR_API_KEY", "env-key")
	t.Setenv("GOOGLE_CHAT_WEBHOOK_URL", "https://env.example.com/webhook")
	t.Setenv("EVIDENCE_PATH", "/tmp/reel-life-events.jsonl")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Sonarr.APIKey != "env-key" {
		t.Errorf("Sonarr.APIKey = %q, want %q (from env)", cfg.Sonarr.APIKey, "env-key")
	}
	if cfg.Chat.WebhookURL != "https://env.example.com/webhook" {
		t.Errorf("Chat.WebhookURL = %q, want env override", cfg.Chat.WebhookURL)
	}
	if cfg.Evidence.Path != "/tmp/reel-life-events.jsonl" {
		t.Errorf("Evidence.Path = %q, want env override", cfg.Evidence.Path)
	}
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "missing sonarr base_url",
			yaml: `
sonarr:
  api_key: key
chat:
  webhook_url: https://example.com
`,
			want: "sonarr.base_url is required",
		},
		{
			name: "missing sonarr api_key",
			yaml: `
sonarr:
  base_url: http://sonarr:8989
chat:
  webhook_url: https://example.com
`,
			want: "sonarr.api_key is required (set SONARR_API_KEY env var)",
		},
		{
			name: "missing chat config",
			yaml: `
sonarr:
  base_url: http://sonarr:8989
  api_key: key
`,
			want: "chat.webhook_url or chat.service_account_file + chat.space is required",
		},
		{
			name: "remediation without evidence",
			yaml: `
sonarr:
  base_url: http://sonarr:8989
  api_key: key
chat:
  webhook_url: https://example.com
remediation:
  enabled: true
`,
			want: "evidence.path is required when remediation is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, tt.yaml)
			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); got != "config validation: "+tt.want {
				t.Errorf("error = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestLoadAppMode(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  service_account_file: /path/to/sa.json
  space: spaces/AAAAA
  project_number: "12345"
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.UseAppMode() {
		t.Error("expected UseAppMode() = true when service_account_file and space set")
	}
	if cfg.Chat.ProjectNumber != "12345" {
		t.Errorf("Chat.ProjectNumber = %q, want %q", cfg.Chat.ProjectNumber, "12345")
	}
}

func TestLoadAppModeEnvOverride(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  space: spaces/AAAAA
`
	path := writeTemp(t, yaml)
	t.Setenv("GOOGLE_SERVICE_ACCOUNT_FILE", "/env/sa.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Chat.ServiceAccountFile != "/env/sa.json" {
		t.Errorf("ServiceAccountFile = %q, want env override", cfg.Chat.ServiceAccountFile)
	}
	if !cfg.UseAppMode() {
		t.Error("expected UseAppMode() = true with env override")
	}
}

func TestUseAppModeFalseWithWebhookOnly(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  webhook_url: https://chat.example.com/webhook
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.UseAppMode() {
		t.Error("expected UseAppMode() = false when only webhook_url set")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadTelegramBackend(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  backend: telegram
  telegram_chat_id: 123456
  telegram_allowed_users: [111, 222]
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Chat.Backend != "telegram" {
		t.Errorf("Chat.Backend = %q, want %q", cfg.Chat.Backend, "telegram")
	}
	if cfg.Chat.TelegramChatID != 123456 {
		t.Errorf("Chat.TelegramChatID = %d, want 123456", cfg.Chat.TelegramChatID)
	}
	if len(cfg.Chat.TelegramAllowedUsers) != 2 {
		t.Errorf("Chat.TelegramAllowedUsers len = %d, want 2", len(cfg.Chat.TelegramAllowedUsers))
	}
}

func TestLoadTelegramBackendNoWebhookRequired(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  backend: telegram
`
	path := writeTemp(t, yaml)
	_, err := Load(path)
	if err != nil {
		t.Fatalf("telegram backend should not require webhook_url, got: %v", err)
	}
}

func TestListenAddrDefault(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  webhook_url: https://chat.example.com/webhook
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.ListenAddr(); got != ":9090" {
		t.Errorf("ListenAddr() = %q, want %q", got, ":9090")
	}
}

func TestListenAddrCustom(t *testing.T) {
	yaml := `
sonarr:
  base_url: http://sonarr:8989
  api_key: test-key
chat:
  webhook_url: https://chat.example.com/webhook
server:
  port: 3000
`
	path := writeTemp(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got := cfg.ListenAddr(); got != ":3000" {
		t.Errorf("ListenAddr() = %q, want %q", got, ":3000")
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
