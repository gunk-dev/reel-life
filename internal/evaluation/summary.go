package evaluation

import "time"

// Summary is the public outcome report. Its fixed fields contain only counts
// and parsed timestamps: no names, IDs, titles, attributes, or error messages
// from the ledger can become output fields or values.
type Summary struct {
	GeneratedAt           time.Time  `json:"generated_at"`
	SourceBytes           int64      `json:"source_bytes"`
	FirstEventAt          *time.Time `json:"first_event_at,omitempty"`
	LastEventAt           *time.Time `json:"last_event_at,omitempty"`
	Events                int        `json:"events"`
	MalformedLines        int        `json:"malformed_lines"`
	ToolSucceeded         int        `json:"tool_succeeded"`
	ToolFailed            int        `json:"tool_failed"`
	DetectionFailures     int        `json:"detection_failures"`
	ActionFailures        int        `json:"action_failures"`
	RemediationsDetected  int        `json:"remediations_detected"`
	RemediationsResolved  int        `json:"remediations_resolved"`
	RemediationsEscalated int        `json:"remediations_escalated"`
}

func (r Report) Summary(sourceBytes int64, now time.Time) Summary {
	s := Summary{
		GeneratedAt: now.UTC(), SourceBytes: sourceBytes,
		FirstEventAt: r.FirstEventAt, LastEventAt: r.LastEventAt,
		Events: r.Events, MalformedLines: r.MalformedLines,
		DetectionFailures: r.DetectionFailures, ActionFailures: r.ActionFailures,
		RemediationsDetected: r.RemediationsDetected, RemediationsResolved: r.RemediationsResolved,
		RemediationsEscalated: r.RemediationsEscalated,
	}
	for _, op := range r.ToolOperations {
		s.ToolSucceeded += op.Succeeded
		s.ToolFailed += op.Failed
	}
	return s
}
