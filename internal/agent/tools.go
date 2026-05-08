package agent

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

// ToolResult is the structured outcome of a single tool dispatch.
// Errors are first-class — surfaced to the agent (not just logged) so the
// model can decide whether to retry, report the failure, or escalate.
//
// Success results carry the original JSON payload in Content.
// Failure results populate ErrorKind, Error, and Retryable.
type ToolResult struct {
	Success   bool   `json:"success"`
	Content   string `json:"content,omitempty"`
	ErrorKind string `json:"error_kind,omitempty"`
	Error     string `json:"error,omitempty"`
	Retryable bool   `json:"retryable,omitempty"`
}

// Marshal returns the wire form sent to the model as a tool result.
// Successful results pass Content through untouched. Failures are encoded
// as a JSON object with success=false and the structured error fields,
// so the model can read and react to error_kind / retryable.
func (r ToolResult) Marshal() string {
	if r.Success {
		return r.Content
	}
	payload := map[string]any{
		"success":    false,
		"error_kind": r.ErrorKind,
		"error":      r.Error,
		"retryable":  r.Retryable,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return `{"success":false,"error_kind":"unknown","error":"failed to encode tool error"}`
	}
	return string(data)
}

// successResult wraps a JSON payload string as a successful tool outcome.
func successResult(content string) ToolResult {
	return ToolResult{Success: true, Content: content}
}

// errorResult builds a failure ToolResult with explicit kind and retryability.
func errorResult(kind, msg string, retryable bool) ToolResult {
	return ToolResult{Success: false, ErrorKind: kind, Error: msg, Retryable: retryable}
}

// errorResultFromErr classifies an arbitrary error and returns a failure ToolResult.
func errorResultFromErr(err error) ToolResult {
	kind, retryable := classifyError(err)
	return ToolResult{
		Success:   false,
		ErrorKind: kind,
		Error:     err.Error(),
		Retryable: retryable,
	}
}

// inputDecodeError builds a ToolResult for failures unmarshalling a tool's input.
func inputDecodeError(err error) ToolResult {
	return errorResult("decode", "invalid input: "+err.Error(), false)
}

// notConfiguredError builds a ToolResult for tools whose backing service is unconfigured.
func notConfiguredError(service string) ToolResult {
	return errorResult("invalid_input", service+" integration is not configured", false)
}

var apiErrorStatusRegex = regexp.MustCompile(`API error (\d{3})`)

// classifyError maps an error from a downstream client into one of the
// ErrorKind values, plus a hint about whether a retry might succeed.
//
// The taxonomy is intentionally small and pragmatic:
//   - decode:        JSON marshal/unmarshal failures
//   - network:       transport-level or 5xx server errors (retryable)
//   - auth:          HTTP 401/403
//   - not_found:     HTTP 404
//   - invalid_input: other 4xx (client must fix the call)
//   - rate_limited:  HTTP 429 (retryable after a delay)
//   - unknown:       anything we couldn't classify
func classifyError(err error) (kind string, retryable bool) {
	if err == nil {
		return "", false
	}
	msg := err.Error()

	if m := apiErrorStatusRegex.FindStringSubmatch(msg); len(m) == 2 {
		status, _ := strconv.Atoi(m[1])
		switch {
		case status == 401 || status == 403:
			return "auth", false
		case status == 404:
			return "not_found", false
		case status == 429:
			return "rate_limited", true
		case status >= 400 && status < 500:
			return "invalid_input", false
		case status >= 500 && status < 600:
			return "network", true
		}
	}

	if strings.Contains(msg, "decode response") ||
		strings.Contains(msg, "marshal ") ||
		strings.Contains(msg, "unmarshal ") ||
		strings.Contains(msg, "invalid character") {
		return "decode", false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "network", true
	}
	if errors.Is(err, context.Canceled) {
		return "network", false
	}
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "connection reset") {
		return "network", true
	}

	return "unknown", false
}

func generateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

// toolDef pairs an Anthropic tool definition with rate-limiting flags.
type toolDef struct {
	Param       anthropic.ToolParam
	Mutative    bool // state-changing but safe/additive
	Destructive bool // removes or deletes data (implicitly also mutative)
}

// mutativeTools is the set of tools that change state but are additive/safe.
var mutativeTools = map[string]bool{
	"add_series":                     true,
	"add_movie":                      true,
	"approve_request":                true,
	"decline_request":                true,
	"retry_request":                  true,
	"notebook_write":                 true,
	"notebook_delete":                true,
	"update_series_monitoring":       true,
	"update_episode_monitoring":      true,
	"monitor_season_episodes":        true,
	"trigger_series_search":          true,
	"trigger_movie_search":           true,
	"update_movie_monitoring":        true,
	"grab_release":                   true,
	"grab_movie_release":             true,
	"update_series_profile":          true,
	"update_movie_profile":           true,
	"update_movie_language_profile":  true,
	"update_series_language_profile": true,
	"enable_indexer":                 true,
	"update_indexer_priority":        true,
}

// destructiveTools is the set of tools that remove or delete data.
// Destructive tools are implicitly also mutative.
var destructiveTools = map[string]bool{
	"remove_failed":              true,
	"remove_failed_movie":        true,
	"delete_series":              true,
	"delete_movie":               true,
	"remove_blocklist_item":      true,
	"remove_movie_blocklist_item": true,
	"delete_request":             true,
	"delete_indexer":             true,
}

// IsMutative reports whether the named tool changes state.
// Destructive tools are implicitly mutative.
func IsMutative(name string) bool {
	return mutativeTools[name] || destructiveTools[name]
}

// IsDestructive reports whether the named tool removes or deletes data.
func IsDestructive(name string) bool {
	return destructiveTools[name]
}

func allToolDefs() []toolDef {
	defs := sonarrToolDefs()
	defs = append(defs, radarrToolDefs()...)
	defs = append(defs, prowlarrToolDefs()...)
	defs = append(defs, overseerrToolDefs()...)
	defs = append(defs, notebookToolDefs()...)
	return defs
}

func toolDefinitions() []anthropic.ToolUnionParam {
	defs := allToolDefs()
	result := make([]anthropic.ToolUnionParam, len(defs))
	for i, d := range defs {
		p := d.Param
		result[i] = anthropic.ToolUnionParam{OfTool: &p}
	}
	return result
}
