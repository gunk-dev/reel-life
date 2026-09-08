// Package remediation implements allowlisted, deterministic recovery policies.
package remediation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/patflynn/reel-life/internal/chat"
	"github.com/patflynn/reel-life/internal/events"
	"github.com/patflynn/reel-life/internal/sonarr"
)

const failedDownloadPolicy = "sonarr.failed-download"

type sonarrQueueClient interface {
	Queue(context.Context) (*sonarr.QueuePage, error)
	RemoveFailed(context.Context, int, bool) error
}

// Runner detects, acts on, and verifies allowlisted incidents.
type Runner struct {
	sonarr      sonarrQueueClient
	notifier    chat.Notifier
	events      events.Recorder
	logger      *slog.Logger
	maxAttempts int
	cooldown    time.Duration

	mu       sync.Mutex
	attempts map[string]int
	lastTry  map[string]time.Time
}

func New(sonarrClient sonarrQueueClient, notifier chat.Notifier, recorder events.Recorder, logger *slog.Logger, maxAttempts int, cooldown time.Duration) *Runner {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	return &Runner{
		sonarr: sonarrClient, notifier: notifier, events: recorder, logger: logger,
		maxAttempts: maxAttempts, cooldown: cooldown,
		attempts: make(map[string]int), lastTry: make(map[string]time.Time),
	}
}

// RunOnce evaluates the current queue and remediates eligible failed downloads.
func (r *Runner) RunOnce(ctx context.Context) {
	queue, err := r.sonarr.Queue(ctx)
	if err != nil {
		r.record(ctx, events.Event{Type: "remediation.detection", Component: "remediation", Operation: failedDownloadPolicy, Outcome: "failed", ErrorKind: "dependency"})
		r.logger.Error("remediation queue check failed", "error", err)
		return
	}
	for _, item := range queue.Records {
		if isFailed(item) {
			r.remediateFailedDownload(ctx, item)
		}
	}
}

func isFailed(item sonarr.QueueItem) bool {
	status := strings.ToLower(item.Status)
	trackedStatus := strings.ToLower(item.TrackedDownloadStatus)
	state := strings.ToLower(item.TrackedDownloadState)
	return status == "failed" || trackedStatus == "failed" || state == "downloadfailed"
}

func (r *Runner) remediateFailedDownload(ctx context.Context, item sonarr.QueueItem) {
	incidentKey := fmt.Sprintf("sonarr.queue.%d", item.ID)
	attrs := map[string]any{"incident_key": incidentKey, "queue_id": item.ID, "series_id": item.SeriesID}
	r.record(ctx, events.Event{Type: "remediation.detected", CorrelationID: incidentKey, Component: "remediation", Operation: failedDownloadPolicy, Outcome: "eligible", Attributes: attrs})

	if !r.allowAttempt(incidentKey) {
		return
	}
	r.record(ctx, events.Event{Type: "remediation.planned", CorrelationID: incidentKey, Component: "remediation", Operation: failedDownloadPolicy, Outcome: "approved_by_policy", Attributes: attrs})

	if err := r.sonarr.RemoveFailed(ctx, item.ID, true); err != nil {
		r.record(ctx, events.Event{Type: "remediation.action", CorrelationID: incidentKey, Component: "remediation", Operation: failedDownloadPolicy, Outcome: "failed", ErrorKind: "dependency", Attributes: attrs})
		r.escalate(ctx, incidentKey, fmt.Sprintf("I couldn't remove and blocklist failed Sonarr download %q (queue %d): %v", item.Title, item.ID, err), attrs)
		return
	}
	r.record(ctx, events.Event{Type: "remediation.action", CorrelationID: incidentKey, Component: "remediation", Operation: failedDownloadPolicy, Outcome: "executed", Attributes: attrs})

	queue, err := r.sonarr.Queue(ctx)
	if err != nil {
		r.escalate(ctx, incidentKey, fmt.Sprintf("I removed failed Sonarr download %q (queue %d), but could not verify the result: %v", item.Title, item.ID, err), attrs)
		return
	}
	for _, remaining := range queue.Records {
		if remaining.ID == item.ID {
			r.escalate(ctx, incidentKey, fmt.Sprintf("I attempted to remove failed Sonarr download %q (queue %d), but it is still present.", item.Title, item.ID), attrs)
			return
		}
	}

	r.record(ctx, events.Event{Type: "remediation.verified", CorrelationID: incidentKey, Component: "remediation", Operation: failedDownloadPolicy, Outcome: "resolved", Attributes: attrs})
	r.clearAttempt(incidentKey)
	msg := fmt.Sprintf("✅ Automatically resolved failed Sonarr download %q (queue %d): removed it, added the release to the blocklist, and verified the queue entry is gone.", item.Title, item.ID)
	if err := r.notifier.SendAdmin(ctx, msg, incidentKey); err != nil {
		r.logger.Error("failed to send remediation result", "error", err, "incident_key", incidentKey)
	}
}

func (r *Runner) allowAttempt(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.attempts[key] >= r.maxAttempts {
		return false
	}
	if last := r.lastTry[key]; !last.IsZero() && time.Since(last) < r.cooldown {
		return false
	}
	r.attempts[key]++
	r.lastTry[key] = time.Now()
	return true
}

func (r *Runner) clearAttempt(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, key)
	delete(r.lastTry, key)
}

func (r *Runner) escalate(ctx context.Context, key, message string, attrs map[string]any) {
	r.record(ctx, events.Event{Type: "remediation.verification", CorrelationID: key, Component: "remediation", Operation: failedDownloadPolicy, Outcome: "escalated", Attributes: attrs})
	if err := r.notifier.SendAdmin(ctx, "⚠️ "+message, key); err != nil {
		r.logger.Error("failed to send remediation escalation", "error", err, "incident_key", key)
	}
}

func (r *Runner) record(ctx context.Context, event events.Event) {
	if r.events == nil {
		return
	}
	if err := r.events.Record(ctx, event); err != nil {
		r.logger.Error("failed to record remediation evidence", "error", err, "operation", event.Operation)
	}
}
