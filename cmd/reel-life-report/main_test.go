package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patflynn/reel-life/internal/evaluation"
)

func TestSummaryExcludesLedgerTextAndCountsOutcomes(t *testing.T) {
	const private = "PRIVATE_CANARY_DO_NOT_PUBLISH"
	ledger := strings.Join([]string{
		`{"type":"tool.result","timestamp":"2026-09-14T01:00:00Z","operation":"` + private + `","outcome":"failed","attributes":{"token":"` + private + `"}}`,
		`{"type":"tool.result","timestamp":"2026-09-14T02:00:00Z","operation":"get_queue","outcome":"succeeded"}`,
		`{"type":"remediation.detected","operation":"sonarr.failed-download","correlation_id":"` + private + `","outcome":"eligible"}`,
		`{"type":"remediation.detected","operation":"sonarr.failed-download","correlation_id":"` + private + `","outcome":"eligible"}`,
		`{"type":"remediation.verified","operation":"sonarr.failed-download","correlation_id":"` + private + `","outcome":"resolved"}`,
		`{"type":"remediation.verification","operation":"sonarr.failed-download","correlation_id":"` + private + `","outcome":"escalated"}`,
		`{"type":"remediation.detection","outcome":"failed","error_kind":"` + private + `"}`,
		`{"type":"remediation.action","outcome":"failed"}`,
		private,
		`null`,
	}, "\n") + "\n"
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(ledger), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"-summary", "-events", path}, &output); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{private, "get_queue", "sonarr.failed-download", path, "attributes", "correlation_id", "operation"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("summary leaked %q", forbidden)
		}
	}
	var got evaluation.Summary
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Events != 8 || got.MalformedLines != 2 || got.ToolSucceeded != 1 || got.ToolFailed != 1 || got.DetectionFailures != 1 || got.ActionFailures != 1 || got.RemediationsDetected != 1 || got.RemediationsResolved != 1 || got.RemediationsEscalated != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
	if got.SourceBytes != int64(len(ledger)) || got.GeneratedAt.IsZero() || got.FirstEventAt == nil || got.LastEventAt == nil || got.FirstEventAt.Hour() != 1 || got.LastEventAt.Hour() != 2 {
		t.Fatalf("missing observation metadata: %+v", got)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != ledger {
		t.Fatal("report changed the ledger")
	}
}

func TestReportRejectsOversizeMissingAndNonRegularSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-summary", "-events", path, "-max-bytes", "1"},
		{"-summary", "-events", path + ".missing"},
		{"-summary", "-events", filepath.Dir(path)},
		{"-summary", "-events", path, "unexpected"},
	} {
		var output bytes.Buffer
		if err := run(args, &output); err == nil || output.Len() != 0 {
			t.Fatalf("accepted invalid request %v", args)
		}
	}
}
