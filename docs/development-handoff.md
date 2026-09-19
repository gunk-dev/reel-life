# Development handoff

Last updated: 2026-09-19.

## Direction and approval boundary

The agreed priority is autonomous problem remediation with measurable recovery,
trustworthy diagnostics, and a short evidence-to-fix cycle. The agent may prepare
draft PRs. Promotion requires the owner's `i approve` comment for the current
PR head, plus all required checks; see [mission and autonomy](mission-and-autonomy.md).

## Landed work

Verified against GitHub on 2026-09-13:

- PR #51: remediation foundation, redacted event ledger, outcome report,
  deterministic Sonarr simulator, and owner approval gate.
- PR #52: notify cosmo when reel-life updates merge.
- PR #53: redact Telegram tokens from HTTP errors.
- PR #54: frozen remediation evaluations, CI gate, and unique incident counts.
- PR #55: durable attempt reservations, restart recovery, and pre-action evidence gates.
- PR #56: complete, bounded Sonarr queue verification (18 evaluation scenarios).

Reel-life was active on laddie during a read-only check on 2026-09-13. A production
evidence snapshot was not obtained because sudo required authentication. The
GitHub CI and cosmo notification runs for the PR #54 merge both succeeded; this
does not by itself verify which binary laddie is running.

## Current increment

PR #57 and cosmo #859/#864 are merged. On September 19, laddie was running
reel-life and the restricted report worked: 25 events, 22 successful tool calls,
3 failed calls, no malformed records, and no remediation incidents. The latest
event was September 17; the old report cannot establish monitor activity.

PR #58 is merged. Its cosmo update is #880; all host checks passed, but the
September 19 initial deployment probe still returned the old report.

Branch: `feat/reconcile-interrupted-remediation` adds read-only reconciliation of
pending attempts after restarts and failed verification. Recorded executions can
be verified from a complete queue read; ambiguous plans only gain an observation.
Repeat mutation remains blocked. Awaiting review and deployment.

## Next increments

1. After the reporting changes deploy, collect the restricted production report
   and choose the next failure to address. Keep credentials and raw user text
   out of fixtures. The report is lifetime evidence, not a monitor heartbeat.
2. Track remediation notification delivery separately from the media action's
   outcome; reconciliation notifications currently remain best effort.
3. Design bounded ledger retention/compaction that preserves attempt state and
   incident identity across queue-ID reuse. Multi-process coordination remains
   outside the current single-owner deployment model.
4. Turn observed failures into reviewed evaluation cases and focused draft PRs.
   Expand remediation policies only after these reliability gaps are covered.

The first evaluation suite does not establish production recovery rates or
complete the self-improvement loop. Each next increment needs its own validation
and review.
