package evaluation

import (
	"strings"
	"testing"
)

func TestBuildAggregatesEvidenceAndToleratesMalformedLines(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		`{"type":"tool.result","operation":"get_queue","outcome":"succeeded"}`,
		`not-json`,
		`{"type":"tool.result","operation":"get_queue","outcome":"failed"}`,
		`{"type":"remediation.detected","operation":"sonarr.failed-download","outcome":"eligible"}`,
		`{"type":"remediation.verified","operation":"sonarr.failed-download","outcome":"resolved"}`,
	}, "\n"))
	report, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Events != 4 || report.MalformedLines != 1 {
		t.Fatalf("unexpected event counts: %+v", report)
	}
	if report.RemediationsDetected != 1 || report.RemediationsResolved != 1 {
		t.Fatalf("unexpected remediation counts: %+v", report)
	}
	if len(report.ToolOperations) != 1 || report.ToolOperations[0].Succeeded != 1 || report.ToolOperations[0].Failed != 1 {
		t.Fatalf("unexpected operations: %+v", report.ToolOperations)
	}
}
