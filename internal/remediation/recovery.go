package remediation

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/patflynn/reel-life/internal/chat"
	"github.com/patflynn/reel-life/internal/events"
)

// NewPersistent restores attempt reservations from the same ledger used for
// future evidence writes. Call before starting the monitor or other writers.
// One process must own the ledger and automatic remediation for a Sonarr stack.
func NewPersistent(client sonarrQueueClient, notifier chat.Notifier, recorder *events.FileRecorder, logger *slog.Logger, maxAttempts int, cooldown time.Duration) (*Runner, error) {
	if recorder == nil {
		return nil, fmt.Errorf("persistent remediation requires a file recorder")
	}
	r := New(client, notifier, recorder, logger, maxAttempts, cooldown)
	if err := recorder.Replay(r.restore); err != nil {
		return nil, fmt.Errorf("restore remediation attempts: %w", err)
	}
	return r, nil
}

func (r *Runner) restore(event events.Event) error {
	switch event.Type {
	case "remediation.planned", "remediation.action", "remediation.verified", "remediation.verification":
	default:
		return nil
	}
	if event.Operation == "" {
		return fmt.Errorf("remediation event is missing policy identity")
	}
	if event.Operation != failedDownloadPolicy {
		return nil
	}
	key := event.CorrelationID
	if key == "" || event.Timestamp.IsZero() {
		return fmt.Errorf("remediation event is missing incident identity or timestamp")
	}
	if event.Type == "remediation.planned" {
		if event.Outcome != "approved_by_policy" {
			return fmt.Errorf("unsupported remediation plan outcome")
		}
		r.attempts[key]++
		if event.Timestamp.After(r.lastTry[key]) {
			r.lastTry[key] = event.Timestamp
		}
		r.blocked[key] = true
		return nil
	}
	if r.attempts[key] == 0 {
		return fmt.Errorf("remediation outcome has no preceding attempt reservation")
	}
	switch event.Type {
	case "remediation.action":
		switch event.Outcome {
		case "failed":
			r.blocked[key] = false
		case "executed":
			r.blocked[key] = true
		default:
			return fmt.Errorf("unsupported remediation action outcome")
		}
	case "remediation.verified":
		if event.Outcome != "resolved" {
			return fmt.Errorf("unsupported remediation verification outcome")
		}
		// Retain completed incident reservations too: stale observations must
		// never cause a previously verified mutation to run again.
		r.blocked[key] = true
	case "remediation.verification":
		if event.Outcome != "escalated" {
			return fmt.Errorf("unsupported remediation escalation outcome")
		}
		// An escalation after an action failure preserves retry eligibility;
		// one after an executed action preserves the uncertainty block.
	}
	return nil
}
