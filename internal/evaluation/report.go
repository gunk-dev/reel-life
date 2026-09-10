// Package evaluation turns the local evidence ledger into improvement signals.
package evaluation

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/patflynn/reel-life/internal/events"
)

type Operation struct {
	Name      string `json:"name"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
}

type Report struct {
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
	scanner := bufio.NewScanner(input)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		var event events.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			report.MalformedLines++
			continue
		}
		report.Events++
		switch event.Type {
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
			report.RemediationsDetected++
		case "remediation.verified":
			if event.Outcome == "resolved" {
				report.RemediationsResolved++
			}
		case "remediation.verification":
			if event.Outcome == "escalated" {
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
