package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/patflynn/reel-life/internal/chat"
	"github.com/patflynn/reel-life/internal/sonarr"
)

// Monitor periodically polls Sonarr for health issues and sends alerts via chat.
type Monitor struct {
	sonarr   sonarr.Client
	notifier chat.Notifier
	interval time.Duration
	logger   *slog.Logger

	// Track previously seen issues to avoid duplicate alerts.
	lastIssues map[string]bool
	remediator interface{ RunOnce(context.Context) }
}

func New(sonarrClient sonarr.Client, notifier chat.Notifier, interval time.Duration, logger *slog.Logger) *Monitor {
	return &Monitor{
		sonarr:     sonarrClient,
		notifier:   notifier,
		interval:   interval,
		logger:     logger,
		lastIssues: make(map[string]bool),
	}
}

// SetRemediator runs deterministic recovery policies after each health check.
func (m *Monitor) SetRemediator(remediator interface{ RunOnce(context.Context) }) {
	m.remediator = remediator
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (m *Monitor) Run(ctx context.Context) error {
	m.logger.Info("monitor started", "interval", m.interval)

	// Run immediately on start, then on interval.
	m.check(ctx)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("monitor stopped")
			return ctx.Err()
		case <-ticker.C:
			m.check(ctx)
		}
	}
}

func (m *Monitor) check(ctx context.Context) {
	defer func() {
		if m.remediator != nil {
			m.remediator.RunOnce(ctx)
		}
	}()
	checks, err := m.sonarr.Health(ctx)
	if err != nil {
		m.logger.Error("health check failed", "error", err)
		return
	}

	m.logger.Debug("health check complete", "issues", len(checks))

	currentIssues := make(map[string]bool)
	var newIssues []sonarr.HealthCheck

	for _, check := range checks {
		if check.Type == "ok" {
			continue
		}
		key := issueKey(check)
		currentIssues[key] = true
		if !m.lastIssues[key] {
			newIssues = append(newIssues, check)
		}
	}

	// Check for resolved issues
	for key := range m.lastIssues {
		if !currentIssues[key] {
			m.logger.Info("issue resolved", "key", key)
		}
	}
	m.lastIssues = currentIssues

	if len(newIssues) > 0 {
		m.alert(ctx, newIssues)
	}
}

func (m *Monitor) alert(ctx context.Context, issues []sonarr.HealthCheck) {
	var b strings.Builder
	b.WriteString("⚠️ Sonarr Health Alert\n\n")
	for _, issue := range issues {
		fmt.Fprintf(&b, "• [%s] %s: %s\n", strings.ToUpper(issue.Type), issue.Source, issue.Message)
	}

	msg := b.String()
	m.logger.Warn("sending health alert", "issues", len(issues))

	if err := m.notifier.SendAdmin(ctx, msg, "sonarr-health"); err != nil {
		m.logger.Error("failed to send alert", "error", err)
	}
}

func issueKey(check sonarr.HealthCheck) string {
	return check.Source + ":" + check.Message
}
