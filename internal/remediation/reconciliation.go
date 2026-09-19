package remediation

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/patflynn/reel-life/internal/events"
	"github.com/patflynn/reel-life/internal/sonarr"
)

type reconciliation struct {
	executed    bool
	observation string
}

// reconcile only observes previously reserved attempts. It never reauthorizes
// mutations, and absence alone cannot establish that an interrupted action ran.
// The caller holds mu and supplies a successful complete queue read.
func (r *Runner) reconcile(ctx context.Context, queue *sonarr.QueuePage) {
	present := make(map[int]bool, len(queue.Records))
	for _, item := range queue.Records {
		present[item.ID] = true
	}
	keys := make([]string, 0, len(r.pending))
	for key := range r.pending {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if ctx.Err() != nil {
			return
		}
		id, err := strconv.Atoi(strings.TrimPrefix(key, "sonarr.queue."))
		// Unknown legacy identities stay blocked and require operator investigation.
		if err != nil || id <= 0 || key != fmt.Sprintf("sonarr.queue.%d", id) {
			continue
		}
		state := r.pending[key]
		outcome := "queue_absent"
		if present[id] {
			outcome = "queue_present"
		}
		if state.observation != outcome {
			if !r.record(ctx, events.Event{Type: "remediation.reconciled", CorrelationID: key, Component: "remediation", Operation: failedDownloadPolicy, Outcome: outcome}) {
				continue
			}
			state.observation = outcome
			r.pending[key] = state
		}
		if present[id] {
			continue
		}
		message := "An interrupted Sonarr remediation was checked again: the queue entry is now absent. The action outcome is unknown; no action was repeated."
		if state.executed {
			if !r.record(ctx, events.Event{Type: "remediation.verified", CorrelationID: key, Component: "remediation", Operation: failedDownloadPolicy, Outcome: "resolved", Attributes: map[string]any{"reconciled": true}}) {
				continue
			}
			message = "A previously executed Sonarr remediation is now verified: the queue entry is absent. No action was repeated. Replacement download and playback have not been verified."
		}
		delete(r.pending, key)
		if err := r.notifier.SendAdmin(ctx, message, key); err != nil {
			r.logger.Error("failed to send reconciliation result", "error", err)
		}
	}
}
