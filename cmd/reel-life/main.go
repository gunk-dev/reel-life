package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/patflynn/reel-life/internal/agent"
	"github.com/patflynn/reel-life/internal/chat"
	"github.com/patflynn/reel-life/internal/config"
	"github.com/patflynn/reel-life/internal/events"
	"github.com/patflynn/reel-life/internal/monitor"
	"github.com/patflynn/reel-life/internal/notebook"
	"github.com/patflynn/reel-life/internal/overseerr"
	"github.com/patflynn/reel-life/internal/prowlarr"
	"github.com/patflynn/reel-life/internal/radarr"
	"github.com/patflynn/reel-life/internal/remediation"
	"github.com/patflynn/reel-life/internal/sonarr"
	"github.com/patflynn/reel-life/internal/weather"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel(),
	}))
	if cfg.Log.Format == "json" {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: cfg.LogLevel(),
		}))
	}

	sonarrClient := sonarr.NewClient(cfg.Sonarr.BaseURL, cfg.Sonarr.APIKey)

	var radarrClient radarr.Client
	if cfg.Radarr.BaseURL != "" {
		radarrClient = radarr.NewClient(cfg.Radarr.BaseURL, cfg.Radarr.APIKey)
		logger.Info("radarr client configured", "url", cfg.Radarr.BaseURL)
	}

	// Build Prowlarr client (optional — use defaults if not configured).
	prowlarrURL := cfg.Prowlarr.BaseURL
	if prowlarrURL == "" {
		prowlarrURL = "http://localhost:9696"
	}
	var prowlarrClient prowlarr.Client
	if cfg.Prowlarr.APIKey != "" {
		prowlarrClient = prowlarr.NewClient(prowlarrURL, cfg.Prowlarr.APIKey)
		logger.Info("prowlarr integration enabled", "url", prowlarrURL)
	}

	// Overseerr client (optional — uses default URL if not configured).
	overseerrBaseURL := cfg.Overseerr.BaseURL
	if overseerrBaseURL == "" {
		overseerrBaseURL = "http://localhost:5055"
	}
	var overseerrClient overseerr.Client
	if cfg.Overseerr.APIKey != "" {
		overseerrClient = overseerr.NewClient(overseerrBaseURL, cfg.Overseerr.APIKey)
		logger.Info("overseerr client configured", "url", overseerrBaseURL)
	}

	// Build conversation history store.
	var historyStore *agent.HistoryStore
	if cfg.Agent.HistorySize > 0 {
		if cfg.Agent.HistoryPath != "" {
			historyStore = agent.NewPersistentHistoryStore(cfg.Agent.HistorySize, cfg.Agent.HistoryPath)
			logger.Info("persistent conversation history enabled", "max_turns", cfg.Agent.HistorySize, "path", cfg.Agent.HistoryPath)
		} else {
			historyStore = agent.NewHistoryStore(cfg.Agent.HistorySize)
			logger.Info("conversation history enabled", "max_turns", cfg.Agent.HistorySize)
		}
	}

	// Select notifier based on backend configuration.
	var notifier chat.Notifier
	var telegramBot *chat.Telegram
	switch cfg.Chat.Backend {
	case "telegram":
		tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
		if tgToken == "" {
			fmt.Fprintf(os.Stderr, "error: TELEGRAM_BOT_TOKEN environment variable is required for telegram backend\n")
			os.Exit(1)
		}
		tg, err := chat.NewTelegram(tgToken, cfg.Chat.TelegramChatID, cfg.Chat.TelegramAdminChatID, cfg.Chat.TelegramAllowedUsers, logger, historyStore)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating Telegram client: %v\n", err)
			os.Exit(1)
		}
		notifier = tg
		telegramBot = tg
		logger.Info("using Telegram notifier")
	case "googlechat", "":
		// Google Chat: Chat API (app mode) or webhook (legacy).
		if cfg.UseAppMode() {
			saKey, err := os.ReadFile(cfg.Chat.ServiceAccountFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading service account file: %v\n", err)
				os.Exit(1)
			}
			notifier, err = chat.NewGoogleChatApp(saKey, cfg.Chat.Space, logger)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error creating Chat API client: %v\n", err)
				os.Exit(1)
			}
			logger.Info("using Google Chat App (API) notifier", "space", cfg.Chat.Space)
		} else {
			notifier = chat.NewGoogleChat(cfg.Chat.WebhookURL, logger)
			logger.Info("using Google Chat webhook notifier")
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unsupported chat backend: %q\n", cfg.Chat.Backend)
		os.Exit(1)
	}

	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		fmt.Fprintf(os.Stderr, "error: ANTHROPIC_API_KEY environment variable is required\n")
		os.Exit(1)
	}

	// Build rate limiter from config (or defaults).
	var limiter *agent.RateLimiter
	rl := cfg.Agent.RateLimits
	maxPerMin, maxPerReq, maxMutative, maxDestructive := agent.DefaultMaxCallsPerMinute, agent.DefaultMaxCallsPerRequest, agent.DefaultMaxMutative, agent.DefaultMaxDestructive
	if rl != nil {
		if rl.MaxCallsPerMinute > 0 {
			maxPerMin = rl.MaxCallsPerMinute
		}
		if rl.MaxCallsPerRequest > 0 {
			maxPerReq = rl.MaxCallsPerRequest
		}
		if rl.MaxMutative > 0 {
			maxMutative = rl.MaxMutative
		}
		if rl.MaxDestructive > 0 {
			maxDestructive = rl.MaxDestructive
		}
	}
	limiter = agent.NewRateLimiter(maxPerMin, maxPerReq, maxMutative, maxDestructive)

	// Notebook (optional — enabled explicitly or when a path is set).
	var nb notebook.Notebook
	if cfg.Notebook.Enabled || cfg.Notebook.Path != "" {
		nbPath := cfg.Notebook.Path
		if nbPath == "" {
			nbPath = "notebook.json"
		}
		nb = notebook.NewFileNotebook(nbPath)
		logger.Info("notebook enabled", "path", nbPath)
	}

	// Weather client (optional — requires location name).
	var weatherClient *weather.Client
	if cfg.Location.Name != "" {
		weatherClient = weather.NewClient(cfg.Location.Latitude, cfg.Location.Longitude, cfg.Location.Name)
		logger.Info("weather context enabled", "location", cfg.Location.Name)
	}

	var eventRecorder events.Recorder
	if cfg.Evidence.Path != "" {
		eventRecorder = events.NewFileRecorder(cfg.Evidence.Path)
		logger.Info("product evidence ledger enabled", "path", cfg.Evidence.Path)
	}
	agentInstance := agent.New(anthropicKey, sonarrClient, radarrClient, prowlarrClient, overseerrClient, nb, weatherClient, cfg.Agent.Model, cfg.Agent.MaxTokens, logger, limiter)
	agentInstance.SetEventRecorder(eventRecorder)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start monitor loop
	if cfg.Monitor.Enabled {
		mon := monitor.New(sonarrClient, notifier, cfg.Monitor.Interval, logger)
		if cfg.Remediation.Enabled {
			mon.SetRemediator(remediation.New(sonarrClient, notifier, eventRecorder, logger, cfg.Remediation.MaxAttempts, cfg.Remediation.Cooldown))
			logger.Info("automatic remediation enabled", "max_attempts", cfg.Remediation.MaxAttempts, "cooldown", cfg.Remediation.Cooldown)
		}
		go func() {
			if err := mon.Run(ctx); err != nil && ctx.Err() == nil {
				logger.Error("monitor error", "error", err)
			}
		}()
		logger.Info("monitor started", "interval", cfg.Monitor.Interval)
	}

	// Start Telegram listener if using Telegram backend.
	if telegramBot != nil {
		go func() {
			if err := telegramBot.Listen(ctx, agentInstance); err != nil && ctx.Err() == nil {
				logger.Error("telegram listener error", "error", err)
			}
		}()
		logger.Info("telegram listener started")
	}

	// Health endpoint for container probes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// Webhook endpoint for incoming Google Chat events
	webhookHandler := chat.NewWebhookHandler(agentInstance, cfg.Chat.ProjectNumber, logger, historyStore)
	mux.Handle("POST /webhook", webhookHandler)

	server := &http.Server{
		Addr:    cfg.ListenAddr(),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down HTTP server")
		server.Close()
	}()

	logger.Info("reel-life started", "addr", server.Addr)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("HTTP server error", "error", err)
		os.Exit(1)
	}
}
