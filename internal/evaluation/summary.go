package evaluation

import "time"

// Summary is the public outcome report. Its fixed fields contain only counts
// and parsed timestamps, plus allowlisted failure categories: no names, IDs,
// titles, attributes, or error messages
// from the ledger can become output fields or values.
type Summary struct {
	ReconciliationAbsent  int            `json:"reconciliation_absent"`
	ReconciliationPresent int            `json:"reconciliation_present"`
	Monitor               MonitorHealth  `json:"monitor"`
	ToolFailureKinds      map[string]int `json:"tool_failure_kinds"`
	GeneratedAt           time.Time      `json:"generated_at"`
	SourceBytes           int64          `json:"source_bytes"`
	FirstEventAt          *time.Time     `json:"first_event_at,omitempty"`
	LastEventAt           *time.Time     `json:"last_event_at,omitempty"`
	Events                int            `json:"events"`
	MalformedLines        int            `json:"malformed_lines"`
	ToolSucceeded         int            `json:"tool_succeeded"`
	ToolFailed            int            `json:"tool_failed"`
	DetectionFailures     int            `json:"detection_failures"`
	ActionFailures        int            `json:"action_failures"`
	RemediationsDetected  int            `json:"remediations_detected"`
	RemediationsResolved  int            `json:"remediations_resolved"`
	RemediationsEscalated int            `json:"remediations_escalated"`
}

func (r Report) Summary(sourceBytes int64, now time.Time) Summary {
	s := Summary{
		ReconciliationAbsent: r.ReconciliationAbsent, ReconciliationPresent: r.ReconciliationPresent,
		Monitor: r.Monitor, ToolFailureKinds: make(map[string]int),
		GeneratedAt: now.UTC(), SourceBytes: sourceBytes,
		FirstEventAt: r.FirstEventAt, LastEventAt: r.LastEventAt,
		Events: r.Events, MalformedLines: r.MalformedLines,
		DetectionFailures: r.DetectionFailures, ActionFailures: r.ActionFailures,
		RemediationsDetected: r.RemediationsDetected, RemediationsResolved: r.RemediationsResolved,
		RemediationsEscalated: r.RemediationsEscalated,
	}
	for kind, count := range r.ToolFailureKinds {
		s.ToolFailureKinds[safeFailureKind(kind)] += count
	}
	for _, op := range r.ToolOperations {
		s.ToolSucceeded += op.Succeeded
		s.ToolFailed += op.Failed
	}
	return s
}

// MonitorHealth describes recorded observations, not inferred service health.
type MonitorHealth struct {
	PollsStarted           int        `json:"polls_started"`
	PollsCompleted         int        `json:"polls_completed"`
	LastPollStartedAt      *time.Time `json:"last_poll_started_at,omitempty"`
	LastPollCompletedAt    *time.Time `json:"last_poll_completed_at,omitempty"`
	HealthSucceeded        int        `json:"health_succeeded"`
	HealthFailed           int        `json:"health_failed"`
	LastHealthSucceededAt  *time.Time `json:"last_health_succeeded_at,omitempty"`
	LastHealthFailedAt     *time.Time `json:"last_health_failed_at,omitempty"`
	NotificationsSucceeded int        `json:"notifications_succeeded"`
	NotificationsFailed    int        `json:"notifications_failed"`
}

func latest(dst **time.Time, timestamp time.Time) {
	if timestamp.IsZero() {
		return
	}
	ts := timestamp.UTC()
	if *dst == nil || ts.After(**dst) {
		*dst = &ts
	}
}

func safeFailureKind(kind string) string {
	switch kind {
	case "decode", "network", "auth", "not_found", "invalid_input", "rate_limited":
		return kind
	default:
		return "unknown"
	}
}
