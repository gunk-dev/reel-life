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

func TestBuildCountsUniqueIncidentsPerPolicyAndOutcome(t *testing.T) {
	input := strings.NewReader(strings.Join([]string{
		`{"type":"remediation.detected","operation":"sonarr.failed-download","correlation_id":"queue.1"}`,
		`{"type":"remediation.detected","operation":"sonarr.failed-download","correlation_id":"queue.1"}`,
		`{"type":"remediation.detected","operation":"radarr.failed-download","correlation_id":"queue.1"}`,
		`{"type":"remediation.detected","operation":"sonarr.failed-download","attributes":{"incident_key":"queue.2"}}`,
		`{"type":"remediation.detected","operation":"sonarr.failed-download","correlation_id":"queue.2"}`,
		`{"type":"remediation.verification","operation":"sonarr.failed-download","correlation_id":"queue.1","outcome":"pending"}`,
		`{"type":"remediation.verification","operation":"sonarr.failed-download","correlation_id":"queue.1","outcome":"escalated"}`,
		`{"type":"remediation.verification","operation":"sonarr.failed-download","correlation_id":"queue.1","outcome":"escalated"}`,
		`{"type":"remediation.verified","operation":"sonarr.failed-download","correlation_id":"queue.1","outcome":"resolved"}`,
		`{"type":"remediation.verified","operation":"sonarr.failed-download","correlation_id":"queue.1","outcome":"resolved"}`,
	}, "\n"))
	report, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Events != 10 || report.RemediationsDetected != 3 || report.RemediationsResolved != 1 || report.RemediationsEscalated != 1 {
		t.Fatalf("unexpected counts: %+v", report)
	}
}

func TestBuildPreservesCountsWithoutIncidentIdentity(t *testing.T) {
	input := strings.NewReader(strings.Repeat(`{"type":"remediation.detected","operation":"sonarr.failed-download"}`+"\n", 2))
	report, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.RemediationsDetected != 2 {
		t.Fatalf("legacy count = %d, want 2", report.RemediationsDetected)
	}
}
