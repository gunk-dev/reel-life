// Package evaluation turns the local evidence ledger into improvement signals.
package evaluation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/patflynn/reel-life/internal/events"
)

type Operation struct {
	Name      string `json:"name"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
}

type Report struct {
	FirstEventAt          *time.Time  `json:"first_event_at,omitempty"`
	LastEventAt           *time.Time  `json:"last_event_at,omitempty"`
	DetectionFailures     int         `json:"detection_failures"`
	ActionFailures        int         `json:"action_failures"`
	Events                int         `json:"events"`
	MalformedLines        int         `json:"malformed_lines"`
	ToolOperations        []Operation `json:"tool_operations"`
	RemediationsDetected  int         `json:"remediations_detected"`
	RemediationsResolved  int         `json:"remediations_resolved"`
	RemediationsEscalated int         `json:"remediations_escalated"`
}

// Build reads a JSONL evidence ledger. Malformed lines are counted instead of
// aborting the report so one damaged record cannot hide all later evidence.
func Build(input io.Reader) (Report, error) {
	var report Report
	operations := make(map[string]*Operation)
	// Polls and replayed ledger entries are observations of the same incident.
	// Keep each outcome category independent: an escalated incident can later
	// resolve, and both facts should remain visible in the report.
	seen := make(map[[3]string]bool)
	countIncident := func(event events.Event) bool {
		key := event.CorrelationID
		if key == "" {
			key, _ = event.Attributes["incident_key"].(string)
		}
		if key == "" {
			// Legacy events without identity cannot safely be deduplicated.
			return true
		}
		identity := [3]string{event.Operation, key, event.Type}
		if seen[identity] {
			return false
		}
		seen[identity] = true
		return true
	}
	scanner := bufio.NewScanner(input)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		var event events.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || event.Type == "" {
			report.MalformedLines++
			continue
		}
		report.Events++
		if !event.Timestamp.IsZero() {
			ts := event.Timestamp.UTC()
			if report.FirstEventAt == nil || ts.Before(*report.FirstEventAt) {
				report.FirstEventAt = &ts
			}
			if report.LastEventAt == nil || ts.After(*report.LastEventAt) {
				report.LastEventAt = &ts
			}
		}
		switch event.Type {
		case "remediation.detection":
			if event.Outcome == "failed" {
				report.DetectionFailures++
			}
		case "remediation.action":
			if event.Outcome == "failed" {
				report.ActionFailures++
			}
		case "tool.result":
			op := operations[event.Operation]
			if op == nil {
				op = &Operation{Name: event.Operation}
				operations[event.Operation] = op
			}
			switch event.Outcome {
			case "succeeded":
				op.Succeeded++
			case "failed":
				op.Failed++
			}
		case "remediation.detected":
			if countIncident(event) {
				report.RemediationsDetected++
			}
		case "remediation.verified":
			if event.Outcome == "resolved" && countIncident(event) {
				report.RemediationsResolved++
			}
		case "remediation.verification":
			if event.Outcome == "escalated" && countIncident(event) {
				report.RemediationsEscalated++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Report{}, fmt.Errorf("read evidence ledger: %w", err)
	}
	for _, operation := range operations {
		report.ToolOperations = append(report.ToolOperations, *operation)
	}
	sort.Slice(report.ToolOperations, func(i, j int) bool {
		leftFailures := report.ToolOperations[i].Failed
		rightFailures := report.ToolOperations[j].Failed
		if leftFailures != rightFailures {
			return leftFailures > rightFailures
		}
		return report.ToolOperations[i].Name < report.ToolOperations[j].Name
	})
	return report, nil
}
