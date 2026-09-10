# reel-life

An AI-powered media curation agent for home media servers. reel-life connects Claude to your *arr stack (Sonarr, Radarr, Prowlarr, Overseerr) and communicates through Telegram or Google Chat. It handles natural language requests, proactively monitors service health, and learns your preferences over time through a persistent notebook.

The product mission, measurable outcomes, runtime autonomy limits, and the
human approval boundary for self-improvement are defined in
[Mission, outcomes, and autonomy](docs/mission-and-autonomy.md).

Claude operates as a constrained agent: it can only call a defined set of media API tools. No filesystem access, no shell commands, no arbitrary network calls.

## Architecture

```
                                  ┌──────────────┐
┌─────────────┐     messages      │  reel-life   │     API calls     ┌──────────┐
│  Telegram   │ ←───────────────→ │              │ ───────────────→  │  Sonarr  │
│  / GChat    │                   │  ┌────────┐  │ ───────────────→  │  Radarr  │
└─────────────┘                   │  │ Claude │  │ ───────────────→  │ Prowlarr │
                                  │  │ Agent  │  │ ───────────────→  │ Overseerr│
                                  │  └────────┘  │                   └──────────┘
                                  │  ┌────────┐  │
                                  │  │Monitor │  │  polls health every 5m
                                  │  └────────┘  │
                                  │  ┌────────┐  │
                                  │  │Notebook│  │  persistent memory
                                  │  └────────┘  │
                                  └──────────────┘
```

## Features

- **Natural language**: "search for Breaking Bad", "what's downloading?", "add that movie", "approve the pending request"
- **Full *arr stack**: Sonarr (TV), Radarr (movies), Prowlarr (indexers), Overseerr (requests)
- **Proactive monitoring**: Automatic alerts for health issues, failed downloads, and indexer problems
- **Verified remediation**: Optional allowlisted recovery of failed Sonarr downloads with bounded attempts, blocklisting, and post-action verification
- **Conversation history**: Sliding window per chat — the agent remembers context within a conversation
- **Persistent notebook**: Pinned notes (always in context) and reference notes (on-demand lookup) that persist across restarts
- **Constrained**: The agent can only call defined media API tools — no filesystem, no shell, no arbitrary network
- **Chat backends**: Telegram (bidirectional, recommended) and Google Chat (webhook or Chat API)
- **NixOS module**: Declarative deployment with systemd hardening, agenix secrets, and sandboxing

## Agent tools

| Integration | Tools |
|-------------|-------|
| **Sonarr** | `search_series`, `add_series`, `get_queue`, `get_history`, `check_health`, `remove_failed` |
| **Radarr** | `search_movies`, `add_movie`, `get_movie_queue`, `get_movie_history`, `check_movie_health`, `remove_failed_movie` |
| **Prowlarr** | `list_indexers`, `test_indexer`, `get_indexer_stats`, `check_indexer_health`, `search_indexers` |
| **Overseerr** | `list_requests`, `approve_request`, `decline_request`, `get_request_count`, `search_media` |
| **Notebook** | `notebook_write`, `notebook_read`, `notebook_list`, `notebook_delete` |

## Quick start

### Telegram (recommended)

```bash
go build ./cmd/reel-life
export ANTHROPIC_API_KEY=your-key
export SONARR_API_KEY=your-key
export TELEGRAM_BOT_TOKEN=your-bot-token
./reel-life -config config.yaml
```

Set `chat.backend: telegram` in your config and add your Telegram user ID to `telegram_allowed_users`. See the [Telegram setup guide](docs/telegram-setup.md) for details.

### NixOS service

Add to your flake inputs, import the module, and configure:

```nix
services.reel-life = {
  enable = true;
  chatBackend = "telegram";
  sonarrUrl = "http://localhost:8989";
  radarrUrl = "http://localhost:7878";
  prowlarrUrl = "http://localhost:9696";
  overseerrUrl = "http://localhost:5055";
  chatTelegramAllowedUsers = [ 123456789 ];
  notebookEnabled = true;
  notebookPath = "/var/lib/reel-life/notebook.json";
  agentHistoryPath = "/var/lib/reel-life/history.json";
  environmentFiles = [
    config.age.secrets.anthropic-key.path
    config.age.secrets.sonarr-api-key.path
    config.age.secrets.radarr-api-key.path
    config.age.secrets.prowlarr-api-key.path
    config.age.secrets.reel-life-telegram-token.path
  ];
};
```

## Configuration

```yaml
sonarr:
  base_url: http://localhost:8989
radarr:
  base_url: http://localhost:7878
prowlarr:
  base_url: http://localhost:9696
overseerr:
  base_url: http://localhost:5055

chat:
  backend: telegram
  telegram_allowed_users: [123456789]
  # telegram_admin_chat_id: 0  # separate chat for health alerts (0 = use main chat)

agent:
  model: claude-sonnet-4-5-20250929
  max_tokens: 4096
  history_size: 40  # conversation turns per chat (0 = disabled)
  history_path: /var/lib/reel-life/history.json  # persist between restarts

notebook:
  enabled: true
  path: notebook.json

monitor:
  enabled: true
  interval: 5m

evidence:
  path: /var/lib/reel-life/events.jsonl

remediation:
  enabled: false  # opt in after reviewing docs/mission-and-autonomy.md
  max_attempts: 1
  cooldown: 1h

log:
  level: info
  format: text
```

Secrets via environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Claude API key |
| `SONARR_API_KEY` | Yes | Sonarr API key |
| `TELEGRAM_BOT_TOKEN` | Telegram | Bot token from @BotFather |
| `RADARR_API_KEY` | No | Radarr API key (enables Radarr tools) |
| `PROWLARR_API_KEY` | No | Prowlarr API key (enables Prowlarr tools) |
| `OVERSEERR_API_KEY` | No | Overseerr API key (enables Overseerr tools) |

URL overrides: `SONARR_URL`, `RADARR_URL`, `PROWLARR_URL`, `OVERSEERR_URL`.

## Documentation

- [Setup guide](docs/setup-guide.md) — Deployment instructions
- [Telegram setup](docs/telegram-setup.md) — Bot configuration
- [Google Chat setup](docs/google-chat-setup.md) — Webhook and API setup
- [Sonarr setup](docs/sonarr-setup.md) — Connecting to Sonarr
- [Troubleshooting](docs/troubleshooting.md) — Common issues

## Development

```bash
nix develop       # enter dev shell
go test ./...     # run tests
go build ./...    # verify compilation
go vet ./...      # lint
```
