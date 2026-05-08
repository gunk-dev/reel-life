package agent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/patflynn/reel-life/internal/notebook"
)

func (a *Agent) dispatchNotebook(ctx context.Context, name string, rawInput json.RawMessage) (ToolResult, bool) {
	if a.notebook == nil {
		switch name {
		case "notebook_write", "notebook_read", "notebook_list", "notebook_delete":
			return notConfiguredError("Notebook"), true
		}
	}

	var result any
	var err error

	switch name {
	case "notebook_write":
		var input notebookWriteInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		noteType := notebook.NoteType(input.Type)
		if noteType != notebook.Pinned && noteType != notebook.Reference {
			return errorResult("invalid_input", "type must be 'pinned' or 'reference'", false), true
		}
		// Check for duplicate titles to avoid creating redundant notes.
		if input.ID == "" {
			existing, searchErr := a.notebook.Search(ctx, input.Title)
			if searchErr == nil {
				for _, n := range existing {
					if strings.EqualFold(n.Title, input.Title) {
						data, _ := json.Marshal(map[string]string{
							"warning":     "a note with a similar title already exists",
							"existing_id": n.ID,
							"title":       n.Title,
						})
						return successResult(string(data)), true
					}
				}
			}
		}
		err = a.notebook.Write(ctx, notebook.Note{
			ID:      input.ID,
			Type:    noteType,
			Title:   input.Title,
			Content: input.Content,
		})
		if err == nil {
			result = map[string]string{"status": "saved"}
		}
	case "notebook_read":
		var input notebookReadInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		result, err = a.notebook.Read(ctx, input.ID)
	case "notebook_list":
		var input notebookListInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		summaries, listErr := a.notebook.List(ctx)
		if listErr != nil {
			err = listErr
		} else if input.Type != "" {
			filtered := make([]notebook.NoteSummary, 0)
			for _, s := range summaries {
				if string(s.Type) == input.Type {
					filtered = append(filtered, s)
				}
			}
			result = filtered
		} else {
			result = summaries
		}
	case "notebook_delete":
		var input notebookDeleteInput
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return inputDecodeError(err), true
		}
		err = a.notebook.Delete(ctx, input.ID)
		if err == nil {
			result = map[string]string{"status": "deleted"}
		}
	default:
		return ToolResult{}, false
	}

	if err != nil {
		a.logger.Warn("tool error", "tool", name, "error", err)
		return errorResultFromErr(err), true
	}
	return marshalResult(result), true
}
