# Development handoff

Last updated: 2026-09-13.

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

Reel-life was active on laddie during a read-only check on 2026-09-13. A production
evidence snapshot was not obtained because sudo required authentication. The
GitHub CI and cosmo notification runs for the PR #54 merge both succeeded; this
does not by itself verify which binary laddie is running.

## Current increment

Branch: `fix/complete-queue-verification`.

Reads the complete Sonarr queue with bounded pagination. Multi-page queues need
two scans with matching membership; missing fields, inconsistent pages, page
errors, and read limits return errors instead of partial data. This protects
both detection and post-action verification. Adds six frozen scenarios (18
total) plus client tests for malformed data, bounds, churn, and cancellation.
No new deployment setting is required. These reads are not atomic snapshots;
the existing queue filters remain in place, and Radarr is unchanged.

## Next increments

1. Take a read-only production evidence snapshot and use the report to choose
   the next failure to address. Keep credentials and raw user text out of fixtures.
2. Reconcile interrupted incidents through read-only checks, and track
   notification delivery separately from the media action's outcome.
3. Design bounded ledger retention/compaction that preserves attempt state and
   incident identity across queue-ID reuse. Multi-process coordination remains
   outside the current single-owner deployment model.
4. Turn observed failures into reviewed evaluation cases and focused draft PRs.
   Expand remediation policies only after these reliability gaps are covered.

The first evaluation suite does not establish production recovery rates or
complete the self-improvement loop. Each next increment needs its own validation
and review.
