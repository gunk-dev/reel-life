package evaluation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMonitorAndSafeFailureSummary(t *testing.T) {
	r, err := Build(strings.NewReader(`{"type":"monitor.poll.started","timestamp":"2026-09-19T10:00:00Z"}
{"type":"monitor.health","outcome":"succeeded","timestamp":"2026-09-19T10:00:01Z"}
{"type":"monitor.poll.completed","timestamp":"2026-09-19T10:00:02Z"}
{"type":"monitor.poll.completed","timestamp":"2026-09-18T10:00:02Z"}
{"type":"monitor.poll.started","timestamp":"2026-09-19T11:00:00Z"}
{"type":"monitor.health","outcome":"failed","timestamp":"2026-09-19T11:00:01Z"}
{"type":"monitor.notification","outcome":"failed"}
{"type":"monitor.notification","outcome":"succeeded"}
{"type":"tool.result","outcome":"failed","error_kind":"auth"}
{"type":"tool.result","outcome":"failed","error_kind":"PRIVATE_CANARY"}
{"type":"tool.result","outcome":"failed"}
{"type":"tool.result","outcome":"succeeded","error_kind":"network"}
`))
	if err != nil {
		t.Fatal(err)
	}
	s := r.Summary(100, time.Now())
	m := s.Monitor
	if m.PollsStarted != 2 || m.PollsCompleted != 2 || m.HealthSucceeded != 1 || m.HealthFailed != 1 || m.NotificationsFailed != 1 || m.NotificationsSucceeded != 1 {
		t.Fatalf("bad monitor counts: %+v", m)
	}
	if m.LastPollCompletedAt.Format(time.RFC3339) != "2026-09-19T10:00:02Z" || !m.LastPollStartedAt.After(*m.LastPollCompletedAt) {
		t.Fatal("bad poll timestamps")
	}
	if s.ToolFailureKinds["auth"] != 1 || s.ToolFailureKinds["unknown"] != 2 || len(s.ToolFailureKinds) != 2 {
		t.Fatalf("bad categories: %v", s.ToolFailureKinds)
	}
	data, _ := json.Marshal(s)
	if strings.Contains(string(data), "PRIVATE_CANARY") {
		t.Fatal("private failure kind exposed")
	}
	old, err := Build(strings.NewReader(`{"type":"tool.result","outcome":"failed"}`))
	if err != nil || old.Monitor.LastPollCompletedAt != nil || old.Monitor.PollsCompleted != 0 {
		t.Fatal("legacy evidence implies monitoring")
	}
}
