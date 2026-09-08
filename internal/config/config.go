package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Sonarr      SonarrConfig      `yaml:"sonarr"`
	Radarr      RadarrConfig      `yaml:"radarr"`
	Prowlarr    ProwlarrConfig    `yaml:"prowlarr"`
	Overseerr   OverseerrConfig   `yaml:"overseerr"`
	Chat        ChatConfig        `yaml:"chat"`
	Agent       AgentConfig       `yaml:"agent"`
	Monitor     MonitorConfig     `yaml:"monitor"`
	Log         LogConfig         `yaml:"log"`
	Server      ServerConfig      `yaml:"server"`
	Notebook    NotebookConfig    `yaml:"notebook"`
	Location    LocationConfig    `yaml:"location"`
	Evidence    EvidenceConfig    `yaml:"evidence"`
	Remediation RemediationConfig `yaml:"remediation"`
}

// EvidenceConfig controls the append-only product outcome ledger.
type EvidenceConfig struct {
	Path string `yaml:"path"`
}

// RemediationConfig controls deterministic, allowlisted recovery policies.
type RemediationConfig struct {
	Enabled     bool          `yaml:"enabled"`
	MaxAttempts int           `yaml:"max_attempts"`
	Cooldown    time.Duration `yaml:"cooldown"`
}

// LocationConfig holds coordinates and display name for weather lookups.
type LocationConfig struct {
	Name      string  `yaml:"name"`
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
}

// NotebookConfig controls the persistent notebook for agent memory.
type NotebookConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type SonarrConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type RadarrConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type ProwlarrConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type OverseerrConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type ChatConfig struct {
	Backend              string  `yaml:"backend"`
	WebhookURL           string  `yaml:"webhook_url"`
	ServiceAccountFile   string  `yaml:"service_account_file"`
	Space                string  `yaml:"space"`
	ProjectNumber        string  `yaml:"project_number"`
	TelegramChatID       int64   `yaml:"telegram_chat_id"`
	TelegramAdminChatID  int64   `yaml:"telegram_admin_chat_id"`
	TelegramAllowedUsers []int64 `yaml:"telegram_allowed_users"`
}

type AgentConfig struct {
	Model       string           `yaml:"model"`
	MaxTokens   int              `yaml:"max_tokens"`
	HistorySize int              `yaml:"history_size"`
	HistoryPath string           `yaml:"history_path"`
	RateLimits  *RateLimitConfig `yaml:"rate_limits,omitempty"`
}

// RateLimitConfig controls how many tool calls the agent can make.
type RateLimitConfig struct {
	MaxCallsPerMinute  int `yaml:"max_calls_per_minute"`
	MaxCallsPerRequest int `yaml:"max_calls_per_request"`
	MaxMutative        int `yaml:"max_mutative"`
	MaxDestructive     int `yaml:"max_destructive"`
}

type MonitorConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Agent: AgentConfig{
			Model:       "claude-sonnet-4-5-20250929",
			MaxTokens:   4096,
			HistorySize: 40,
		},
		Monitor: MonitorConfig{
			Enabled:  true,
			Interval: 5 * time.Minute,
		},
		Remediation: RemediationConfig{
			MaxAttempts: 1,
			Cooldown:    time.Hour,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Chat: ChatConfig{
			Backend: "googlechat",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyEnvOverrides(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SONARR_API_KEY"); v != "" {
		cfg.Sonarr.APIKey = v
	}
	if v := os.Getenv("SONARR_URL"); v != "" {
		cfg.Sonarr.BaseURL = v
	}
	if v := os.Getenv("RADARR_API_KEY"); v != "" {
		cfg.Radarr.APIKey = v
	}
	if v := os.Getenv("RADARR_URL"); v != "" {
		cfg.Radarr.BaseURL = v
	}
	if v := os.Getenv("PROWLARR_API_KEY"); v != "" {
		cfg.Prowlarr.APIKey = v
	}
	if v := os.Getenv("PROWLARR_URL"); v != "" {
		cfg.Prowlarr.BaseURL = v
	}
	if v := os.Getenv("OVERSEERR_API_KEY"); v != "" {
		cfg.Overseerr.APIKey = v
	}
	if v := os.Getenv("OVERSEERR_URL"); v != "" {
		cfg.Overseerr.BaseURL = v
	}
	if v := os.Getenv("GOOGLE_CHAT_WEBHOOK_URL"); v != "" {
		cfg.Chat.WebhookURL = v
	}
	if v := os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE"); v != "" {
		cfg.Chat.ServiceAccountFile = v
	}
	if v := os.Getenv("NOTEBOOK_PATH"); v != "" {
		cfg.Notebook.Path = v
	}
	if v := os.Getenv("HISTORY_PATH"); v != "" {
		cfg.Agent.HistoryPath = v
	}
	if v := os.Getenv("EVIDENCE_PATH"); v != "" {
		cfg.Evidence.Path = v
	}
	if v := os.Getenv("REEL_LIFE_LATITUDE"); v != "" {
		if lat, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Location.Latitude = lat
		}
	}
	if v := os.Getenv("REEL_LIFE_LONGITUDE"); v != "" {
		if lon, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Location.Longitude = lon
		}
	}
	if v := os.Getenv("REEL_LIFE_LOCATION_NAME"); v != "" {
		cfg.Location.Name = v
	}
}

// UseAppMode returns true when the Chat API (service account) notifier should be used
// instead of the legacy incoming webhook.
func (cfg *Config) UseAppMode() bool {
	return cfg.Chat.ServiceAccountFile != "" && cfg.Chat.Space != ""
}

func validate(cfg *Config) error {
	if cfg.Sonarr.BaseURL == "" {
		return fmt.Errorf("sonarr.base_url is required")
	}
	if cfg.Sonarr.APIKey == "" {
		return fmt.Errorf("sonarr.api_key is required (set SONARR_API_KEY env var)")
	}
	if cfg.Radarr.BaseURL != "" && cfg.Radarr.APIKey == "" {
		return fmt.Errorf("radarr.api_key is required when radarr.base_url is set (set RADARR_API_KEY env var)")
	}
	if cfg.Remediation.Enabled && !cfg.Monitor.Enabled {
		return fmt.Errorf("monitor.enabled must be true when remediation is enabled")
	}
	if cfg.Remediation.Enabled && cfg.Evidence.Path == "" {
		return fmt.Errorf("evidence.path is required when remediation is enabled")
	}
	// Telegram backend only needs the bot token (checked at startup via env var).
	if cfg.Chat.Backend == "telegram" {
		return nil
	}
	// Either webhook URL or service account + space must be configured.
	if cfg.Chat.WebhookURL == "" && (cfg.Chat.ServiceAccountFile == "" || cfg.Chat.Space == "") {
		return fmt.Errorf("chat.webhook_url or chat.service_account_file + chat.space is required")
	}
	return nil
}

func (c *Config) ListenAddr() string {
	port := c.Server.Port
	if port == 0 {
		port = 9090
	}
	return fmt.Sprintf(":%d", port)
}

func (cfg *Config) LogLevel() slog.Level {
	switch cfg.Log.Level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
