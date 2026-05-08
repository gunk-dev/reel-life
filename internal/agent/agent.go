package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/patflynn/reel-life/internal/notebook"
	"github.com/patflynn/reel-life/internal/overseerr"
	"github.com/patflynn/reel-life/internal/prowlarr"
	"github.com/patflynn/reel-life/internal/radarr"
	"github.com/patflynn/reel-life/internal/sonarr"
	"github.com/patflynn/reel-life/internal/weather"
)

const systemPrompt = `You are a media curation assistant for a home media server. You help users manage their TV series library through Sonarr, their movie library through Radarr, and handle media requests through Overseerr.

Your capabilities:
- Search for TV series and movies, and provide concise summaries of results
- Add series or movies to the library for monitoring and automatic downloading
- Check the download queue for active and pending downloads (both TV and movies)
- Review download history for recent activity
- Monitor system health and report any issues
- Remove failed downloads and manage the blocklist
- Get detailed series info and episode status
- Update season monitoring settings (enable/disable monitoring per season)
- Search for available releases and see why they were accepted/rejected
- View Sonarr logs for debugging
- Check quality profiles, blocklist, root folders, and download client status
- Manage indexers via Prowlarr: list, test, enable/disable, update priority, delete, check stats and health, search across indexers
- List, approve, decline, delete, and retry media requests from Overseerr
- Get detailed information about specific requests
- Search for movies and TV shows in Overseerr's media database
- Get request statistics (pending, approved, declined counts)

Guidelines:
- When searching, present results concisely with title, year, and a brief description
- Always confirm with the user before adding a new series or movie, or approving/declining requests
- When reporting health issues, clearly explain what each issue means and suggest fixes
- Be direct and helpful — avoid unnecessary pleasantries
- Only use the tools provided — do not make up information
- You have a persistent notebook for memory across conversations. Save useful observations: user preferences, recurring issues, operational patterns
- Pinned notes are always visible to you; reference notes need to be looked up with notebook_read
- Keep pinned notes concise and high-signal; use reference type for detailed information
- Before creating a new note, check existing notes to avoid duplicates — update instead if a similar note exists

Tool discipline — NEVER fabricate tool results:
- Do NOT report past-tense success ("added", "queued", "imported", "requested", "updated", "approved") for any library operation unless a tool call for that operation succeeded in the current turn.
- If you intend to add or request multiple items, call the tool for each one and only report what the tool results actually say succeeded.
- If a tool fails or you didn't call it, say so plainly. "I tried to add X but the tool returned an error" or "I didn't call add_movie for Y" is far better than implying it happened.
- The right pattern is: "I'll add these now" → call tools → "X added successfully (per add_movie result), Y failed because <error>, Z still pending".`

const maxToolRounds = 10

// Agent handles natural language interactions using Claude with Sonarr, Radarr, Prowlarr, and Overseerr tools.
type Agent struct {
	client    *anthropic.Client
	sonarr    sonarr.Client
	radarr    radarr.Client
	prowlarr  prowlarr.Client
	overseerr overseerr.Client
	notebook  notebook.Notebook
	weather   *weather.Client
	model     string
	maxTok    int64
	logger    *slog.Logger
	limiter   *RateLimiter
}

func New(apiKey string, sonarrClient sonarr.Client, radarrClient radarr.Client, prowlarrClient prowlarr.Client, overseerrClient overseerr.Client, nb notebook.Notebook, weatherClient *weather.Client, model string, maxTokens int, logger *slog.Logger, limiter *RateLimiter) *Agent {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Agent{
		client:    &client,
		sonarr:    sonarrClient,
		radarr:    radarrClient,
		prowlarr:  prowlarrClient,
		overseerr: overseerrClient,
		notebook:  nb,
		weather:   weatherClient,
		model:     model,
		maxTok:    int64(maxTokens),
		logger:    logger,
		limiter:   limiter,
	}
}

// NewWithClient creates an Agent with a pre-configured Anthropic client (for testing).
func NewWithClient(client *anthropic.Client, sonarrClient sonarr.Client, radarrClient radarr.Client, prowlarrClient prowlarr.Client, overseerrClient overseerr.Client, nb notebook.Notebook, weatherClient *weather.Client, model string, maxTokens int, logger *slog.Logger, limiter *RateLimiter) *Agent {
	return &Agent{
		client:    client,
		sonarr:    sonarrClient,
		radarr:    radarrClient,
		prowlarr:  prowlarrClient,
		overseerr: overseerrClient,
		notebook:  nb,
		weather:   weatherClient,
		model:     model,
		maxTok:    int64(maxTokens),
		logger:    logger,
		limiter:   limiter,
	}
}

// requestIDKey is the context key for the per-request identifier used in audit logs.
type requestIDKey struct{}

// WithRequestID returns a child context carrying the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func requestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// buildSystemPrompt returns the system prompt with pinned notebook notes appended.
func (a *Agent) buildSystemPrompt(ctx context.Context) string {
	dateLine := fmt.Sprintf("Today's date is %s.", time.Now().Format("2006-01-02"))

	var locationLine string
	if a.weather != nil {
		if cond := a.weather.Current(ctx); cond != nil {
			locationLine = fmt.Sprintf("Current location: %s. Weather: %.0f°C, %s.", a.weather.Location(), cond.Temperature, cond.Description)
		} else {
			locationLine = fmt.Sprintf("Current location: %s.", a.weather.Location())
		}
	}

	var prompt string
	if locationLine != "" {
		prompt = fmt.Sprintf("%s\n%s\n\n%s", dateLine, locationLine, systemPrompt)
	} else {
		prompt = fmt.Sprintf("%s\n\n%s", dateLine, systemPrompt)
	}

	if a.notebook == nil {
		return prompt
	}

	pinned, err := a.notebook.Pinned(ctx)
	if err != nil {
		a.logger.Warn("failed to load pinned notes", "error", err)
		return prompt
	}
	if len(pinned) == 0 {
		return prompt
	}

	var sb strings.Builder
	sb.WriteString(prompt)
	sb.WriteString("\n\n## Notebook (always loaded)\n")
	for _, n := range pinned {
		sb.WriteString("### ")
		sb.WriteString(n.Title)
		sb.WriteString("\n")
		sb.WriteString(n.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}

// Process runs the agentic tool-use loop for a user message and returns the final text response.
// History turns are prepended to provide conversational context.
func (a *Agent) Process(ctx context.Context, userMessage string, history []Turn) (string, error) {
	tools := toolDefinitions()

	var messages []anthropic.MessageParam
	for _, t := range history {
		switch t.Role {
		case "user":
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(t.Content)))
		case "assistant":
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(t.Content)))
		}
	}
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)))

	if a.limiter != nil {
		a.limiter.Reset()
	}

	reqID := requestID(ctx)
	sysPrompt := a.buildSystemPrompt(ctx)

	for round := range maxToolRounds {
		resp, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(a.model),
			MaxTokens: a.maxTok,
			System: []anthropic.TextBlockParam{
				{Text: sysPrompt},
			},
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			return "", fmt.Errorf("claude API call: %w", err)
		}

		// Collect tool uses from the response
		var toolResults []anthropic.ContentBlockParamUnion
		var textResponse string

		for _, block := range resp.Content {
			switch v := block.AsAny().(type) {
			case anthropic.TextBlock:
				textResponse += v.Text
			case anthropic.ToolUseBlock:
				result, isErr := a.executeToolWithAudit(ctx, v.Name, v.Input, round, reqID)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, result, isErr))
			}
		}

		// If no tool calls, we're done
		if len(toolResults) == 0 {
			return textResponse, nil
		}

		// Add assistant response and tool results to conversation
		messages = append(messages, resp.ToParam())
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return "", fmt.Errorf("exceeded maximum tool rounds (%d)", maxToolRounds)
}

// executeToolWithAudit wraps tool dispatch with rate limiting and audit logging.
func (a *Agent) executeToolWithAudit(ctx context.Context, name string, rawInput json.RawMessage, round int, reqID string) (string, bool) {
	// Audit log: invocation
	a.logger.Info("tool invocation",
		"tool", name,
		"input", sanitizeInput(rawInput),
		"round", round,
		"request_id", reqID,
	)

	// Rate limit check
	if a.limiter != nil {
		if err := a.limiter.Allow(name, IsMutative(name), IsDestructive(name)); err != nil {
			a.logger.Warn("tool rate limited",
				"tool", name,
				"error", err,
				"round", round,
				"request_id", reqID,
			)
			return jsonError(err.Error()), true
		}
	}

	start := time.Now()
	result, isErr := a.dispatchTool(ctx, name, rawInput)
	duration := time.Since(start)

	// Audit log: result
	a.logger.Info("tool result",
		"tool", name,
		"success", !isErr,
		"duration_ms", duration.Milliseconds(),
		"round", round,
		"request_id", reqID,
	)

	return result, isErr
}

// sanitizeInput returns a string summary of tool input suitable for logging.
// It strips any field whose key contains sensitive substrings to avoid leaking credentials.
func sanitizeInput(raw json.RawMessage) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "<invalid json>"
	}
	for k := range m {
		lowerK := strings.ToLower(k)
		if strings.Contains(lowerK, "key") || strings.Contains(lowerK, "secret") || strings.Contains(lowerK, "password") || strings.Contains(lowerK, "token") {
			m[k] = "REDACTED"
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return "<failed to sanitize json>"
	}
	return string(out)
}

// dispatchTool executes a tool call and returns the JSON result string and whether it's an error.
func (a *Agent) dispatchTool(ctx context.Context, name string, rawInput json.RawMessage) (string, bool) {
	if result, isErr, handled := a.dispatchSonarr(ctx, name, rawInput); handled {
		return result, isErr
	}
	if result, isErr, handled := a.dispatchRadarr(ctx, name, rawInput); handled {
		return result, isErr
	}
	if result, isErr, handled := a.dispatchProwlarr(ctx, name, rawInput); handled {
		return result, isErr
	}
	if result, isErr, handled := a.dispatchOverseerr(ctx, name, rawInput); handled {
		return result, isErr
	}
	if result, isErr, handled := a.dispatchNotebook(ctx, name, rawInput); handled {
		return result, isErr
	}
	return jsonError("unknown tool: " + name), true
}

// findIndexer fetches the indexer list from Prowlarr and returns the indexer with the given ID.
// On failure, it returns a non-empty JSON error string.
func (a *Agent) findIndexer(ctx context.Context, id int) (*prowlarr.Indexer, string) {
	indexers, err := a.prowlarr.ListIndexers(ctx)
	if err != nil {
		return nil, jsonError(err.Error())
	}
	for i := range indexers {
		if indexers[i].ID == id {
			return &indexers[i], ""
		}
	}
	return nil, jsonError(fmt.Sprintf("indexer %d not found", id))
}

func jsonError(msg string) string {
	data, _ := json.Marshal(map[string]string{"error": msg})
	return string(data)
}
