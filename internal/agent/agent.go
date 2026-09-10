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
	"github.com/patflynn/reel-life/internal/events"
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
- The right pattern is: "I'll add these now" → call tools → "X added successfully (per add_movie result), Y failed because <error>, Z still pending".

Tool errors:
- Tool results are JSON. When the result has "success": false, treat it as a real failure — do NOT pretend the call succeeded or fabricate output the tool would have produced.
- Read "error_kind" and "error" to understand what failed. If "retryable" is true you may try the call once more; otherwise report the failure plainly to the user and stop attempting the same operation.
- If a tool fails repeatedly across a conversation, surface that pattern to the user rather than silently moving on.`

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
	events    events.Recorder
}

// SetEventRecorder enables durable outcome evidence. It is optional so tests
// and embedded callers do not need persistent storage.
func (a *Agent) SetEventRecorder(recorder events.Recorder) {
	a.events = recorder
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
				tr := a.executeToolWithAudit(ctx, v.Name, v.Input, round, reqID)
				toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, tr.Marshal(), !tr.Success))
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
func (a *Agent) executeToolWithAudit(ctx context.Context, name string, rawInput json.RawMessage, round int, reqID string) ToolResult {
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
			return errorResult("rate_limited", err.Error(), true)
		}
	}

	start := time.Now()
	result := a.dispatchTool(ctx, name, rawInput)
	duration := time.Since(start)
	if a.events != nil {
		outcome := "succeeded"
		if !result.Success {
			outcome = "failed"
		}
		if err := a.events.Record(ctx, events.Event{
			Type:          "tool.result",
			CorrelationID: reqID,
			Component:     "agent",
			Operation:     name,
			Outcome:       outcome,
			DurationMS:    duration.Milliseconds(),
			ErrorKind:     result.ErrorKind,
			Attributes: map[string]any{
				"round":       round,
				"mutative":    IsMutative(name),
				"destructive": IsDestructive(name),
			},
		}); err != nil {
			a.logger.Error("failed to record tool evidence", "error", err, "request_id", reqID)
		}
	}

	// Audit log: result
	a.logger.Info("tool result",
		"tool", name,
		"success", result.Success,
		"error_kind", result.ErrorKind,
		"duration_ms", duration.Milliseconds(),
		"round", round,
		"request_id", reqID,
	)

	return result
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

// dispatchTool executes a tool call and returns a structured ToolResult.
// Errors are first-class: classification and retryability are encoded into
// the result so the agent loop can surface them to the model.
func (a *Agent) dispatchTool(ctx context.Context, name string, rawInput json.RawMessage) ToolResult {
	if result, handled := a.dispatchSonarr(ctx, name, rawInput); handled {
		return result
	}
	if result, handled := a.dispatchRadarr(ctx, name, rawInput); handled {
		return result
	}
	if result, handled := a.dispatchProwlarr(ctx, name, rawInput); handled {
		return result
	}
	if result, handled := a.dispatchOverseerr(ctx, name, rawInput); handled {
		return result
	}
	if result, handled := a.dispatchNotebook(ctx, name, rawInput); handled {
		return result
	}
	return errorResult("invalid_input", "unknown tool: "+name, false)
}

// findIndexer fetches the indexer list from Prowlarr and returns the indexer with the given ID.
// On failure, it returns a non-nil ToolResult describing the error.
func (a *Agent) findIndexer(ctx context.Context, id int) (*prowlarr.Indexer, *ToolResult) {
	indexers, err := a.prowlarr.ListIndexers(ctx)
	if err != nil {
		r := errorResultFromErr(err)
		return nil, &r
	}
	for i := range indexers {
		if indexers[i].ID == id {
			return &indexers[i], nil
		}
	}
	r := errorResult("not_found", fmt.Sprintf("indexer %d not found", id), false)
	return nil, &r
}

// marshalResult JSON-encodes a tool's success payload, returning a wrapped
// ToolResult with classified marshal errors on failure.
func marshalResult(result any) ToolResult {
	data, err := json.Marshal(result)
	if err != nil {
		return errorResult("decode", "failed to marshal result: "+err.Error(), false)
	}
	return successResult(string(data))
}
