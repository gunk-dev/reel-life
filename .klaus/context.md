# Repository context

## Purpose
`reel-life` is an AI-powered chatops agent that connects Anthropic's Claude to a home-media *arr stack (Sonarr, Radarr, Prowlarr, Overseerr) and exposes it via Telegram or Google Chat. Claude is run as a constrained tool-use agent: it can only invoke a defined set of media-API and notebook tools — no shell, no filesystem, no arbitrary HTTP. A background monitor polls Sonarr for health/queue issues and pushes proactive alerts to the chat backend.

## Tech stack
- Go `1.25.0` (see `go.mod`).
- Direct dependencies (`go.mod`):
  - `github.com/anthropics/anthropic-sdk-go v1.26.0` — Claude API client.
  - `github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1` — Telegram bot.
  - `github.com/golang-jwt/jwt/v5 v5.3.1` and `golang.org/x/oauth2 v0.36.0` — used by the Google Chat App (Chat API) mode for service-account auth.
  - `github.com/invopop/jsonschema v0.13.0` — generates JSON Schema for tool definitions.
  - `gopkg.in/yaml.v3 v3.0.1` — YAML config parsing.
- Standard library only for HTTP, logging (`log/slog`), and the *arr REST clients.
- Build/runtime tooling: `Makefile`, a `Dockerfile` (multi-stage, `golang:1.25-alpine` → `gcr.io/distroless/static-debian12:nonroot`), and a Nix `flake.nix` + `nix/` module for NixOS deployment.

## Entry points
- `cmd/reel-life/main.go` — the only binary. Loads config, wires the *arr clients, chat notifier, optional notebook + weather, conversation-history store, rate limiter, monitor loop, Telegram listener, and an HTTP server exposing `GET /healthz` and `POST /webhook` (Google Chat events).

## Layout
- `cmd/reel-life/` — main entrypoint.
- `internal/config/` — YAML config loader with env-var overrides for secrets (`config.go`, `config_test.go`).
- `internal/chat/` — chat adapters: `google.go` (webhook), `google_app.go` (Chat API with service-account JWT), `telegram.go` (bidirectional bot), `webhook_handler.go` (HTTP handler for Google Chat events), and a `Notifier` interface in `chat.go`.
- `internal/sonarr/`, `internal/radarr/`, `internal/prowlarr/`, `internal/overseerr/` — REST clients for each *arr service: `client.go` (interface + HTTP impl) and `types.go` (DTOs), each with `client_test.go`.
- `internal/agent/` — the Claude tool-use agent. `agent.go` drives the model loop; `tools_*.go` declare tool schemas per integration; `dispatch_*.go` route tool calls to the right client; `history.go` / `history_store.go` manage per-chat sliding-window conversation history (optionally persistent); `ratelimit.go` enforces per-minute / per-request / mutative / destructive call caps.
- `internal/monitor/` — polling loop that calls Sonarr health/queue endpoints and pushes alerts via the `Notifier`.
- `internal/notebook/` — file-backed JSON notebook (pinned + reference notes) exposed to the agent as `notebook_*` tools.
- `internal/weather/` — optional Open-Meteo-style weather client; injected into the agent when a `location` is configured.
- `docs/` — setup guides: `setup-guide.md`, `telegram-setup.md`, `google-chat-setup.md`, `sonarr-setup.md`, `troubleshooting.md`.
- `nix/` — `package.nix` and `module.nix` (NixOS service with systemd hardening; surface documented in README).
- `assets/` — `avatar.png` only.
- `.github/workflows/ci.yml` — CI workflow.
- Top-level files: `config.yaml.example`, `dev.yaml` (local dev config), `.env.example`, `Dockerfile`, `Makefile`, `flake.nix`, `flake.lock`.

## Build, test, run
Per `Makefile`, `CLAUDE.md`, and `README.md`:

```bash
# Build the binary
go build ./cmd/reel-life          # or: make build  (outputs bin/reel-life)

# Tests / vet
go test ./...                     # or: make test
go vet ./...                      # or: make vet

# Run locally (Makefile loads ./.env then runs against dev.yaml)
make run

# Quick smoke test (starts the binary, curls /healthz, kills it)
make smoke

# Nix dev shell
nix develop
```

Required env vars (`README.md` table, enforced in `cmd/reel-life/main.go`):
- `ANTHROPIC_API_KEY` — always required.
- `SONARR_API_KEY` — always required (Sonarr is the only non-optional *arr).
- `TELEGRAM_BOT_TOKEN` — required when `chat.backend: telegram`.
- Optional: `RADARR_API_KEY`, `PROWLARR_API_KEY`, `OVERSEERR_API_KEY` enable the corresponding tools; `SONARR_URL`, `RADARR_URL`, `PROWLARR_URL`, `OVERSEERR_URL` override base URLs.

The binary takes one flag: `-config <path>` (defaults to `config.yaml`). HTTP listen port comes from `server.port` in config.

## Conventions
Verified from the code and `CLAUDE.md`:
- **Secrets via env vars only.** `config.yaml` carries URLs and behavior; API keys/tokens are read from the environment (see `internal/config/config.go` and `cmd/reel-life/main.go`).
- **HTTP clients are real, not mocked.** Tests use `net/http/httptest` to stand up fake *arr / Telegram / Chat servers. There are no interface-mocked HTTP clients (per `CLAUDE.md` testing conventions and confirmed by `*_test.go` files).
- **Each *arr integration follows the same shape:** an interface + HTTP implementation in `internal/<svc>/client.go`, DTOs in `types.go`, tool schemas in `internal/agent/tools_<svc>.go`, and dispatch in `internal/agent/dispatch_<svc>.go`. New integrations should match this pattern.
- **Optional integrations gate on config.** Radarr/Prowlarr/Overseerr/Notebook/Weather clients are only constructed when their respective config is present; `main.go` logs which ones are enabled.
- **Agent is sandboxed by tool surface.** All capabilities Claude can exercise live in `internal/agent/tools_*.go`; do not give the agent direct shell, filesystem, or arbitrary HTTP access.
- **Rate limiting is mandatory.** A `RateLimiter` (per-minute, per-request, mutative, destructive caps) wraps every tool invocation — see `internal/agent/ratelimit.go` and how it is built in `main.go`.
- **Logging uses `log/slog`** with text or JSON handler selected by `log.format`.

## Gotchas
- `make smoke` runs the binary in the background with `&`, sleeps 2 seconds, then `curl`s `/healthz` and kills the PID. It requires a populated `./.env` and `dev.yaml` and will leak the process if `kill` fails — be aware when iterating on it.
- The Google Chat backend has two modes: legacy `webhook_url` and "App mode" (Chat API with a service-account JSON file + `space`). `cfg.UseAppMode()` in `main.go` selects between them — misconfiguring one will surface as a startup error, not a runtime one.
- Conversation history is only persisted when both `agent.history_size > 0` *and* `agent.history_path` is set; otherwise it stays in-memory and is lost on restart (see `cmd/reel-life/main.go`).

## External dependencies
At runtime the binary talks to:
- Anthropic Messages API (`ANTHROPIC_API_KEY`).
- Sonarr v3 HTTP API (required).
- Radarr, Prowlarr, Overseerr HTTP APIs (optional; enabled per-service via their `*_API_KEY`).
- Telegram Bot API (when `chat.backend: telegram`) or Google Chat — either an incoming webhook URL or the Google Chat REST API via OAuth2 service-account JWT.
- Open-Meteo-style weather service via `internal/weather` when a `location` is configured.

No database, queue, or other persistent backing store: state lives in JSON files (`notebook.json`, `history.json`) on the local filesystem.
