package agent

import (
	"context"
	"encoding/json"
)

func (a *Agent) dispatchOverseerr(ctx context.Context, name string, rawInput json.RawMessage) (ToolResult, bool) {
	if a.overseerr == nil {
		switch name {
		case "list_requests", "approve_request", "decline_request", "get_request_detail",
			"delete_request", "retry_request", "get_request_count", "search_media":
			return notConfiguredError("Overseerr"), true
		}
	}

	var result any
	var err error

	switch name {
	case "list_requests":
		var input listRequestsInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		take := input.Take
		if take == 0 {
			take = 20
		}
		result, err = a.overseerr.ListRequests(ctx, input.Filter, take, input.Skip)
	case "approve_request":
		var input approveRequestInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.overseerr.ApproveRequest(ctx, input.ID)
		if err == nil {
			result = map[string]string{"status": "approved"}
		}
	case "decline_request":
		var input declineRequestInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.overseerr.DeclineRequest(ctx, input.ID)
		if err == nil {
			result = map[string]string{"status": "declined"}
		}
	case "get_request_detail":
		var input getRequestDetailInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.overseerr.GetRequest(ctx, input.ID)
	case "delete_request":
		var input deleteRequestInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.overseerr.DeleteRequest(ctx, input.ID)
		if err == nil {
			result = map[string]string{"status": "deleted"}
		}
	case "retry_request":
		var input retryRequestInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.overseerr.RetryRequest(ctx, input.ID)
	case "get_request_count":
		result, err = a.overseerr.GetRequestCount(ctx)
	case "search_media":
		var input searchMediaInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		page := input.Page
		if page == 0 {
			page = 1
		}
		result, err = a.overseerr.SearchMedia(ctx, input.Query, page)
	default:
		return ToolResult{}, false
	}

	if err != nil {
		a.logger.Warn("tool error", "tool", name, "error", err)
		return errorResultFromErr(err), true
	}
	return marshalResult(result), true
}
