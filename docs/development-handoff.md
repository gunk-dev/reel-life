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

Reel-life was active on laddie during a read-only check on 2026-09-13. A production
evidence snapshot was not obtained because sudo required authentication. The
GitHub CI and cosmo notification runs for the PR #54 merge both succeeded; this
does not by itself verify which binary laddie is running.

## Current increment

Branch: `fix/durable-remediation-attempts`.

Restores attempt reservations and cooldowns from the existing evidence ledger.
Interrupted, unverified, and completed actions remain blocked from automatic
re-execution. Serializes overlapping polls, requires saved pre-action evidence,
and rejects unsafe recovery history. Extends the frozen evaluation suite to
twelve scenarios and adds crash-boundary, concurrency, and failed-write tests.
No new deployment setting is required, but the complete evidence ledger must
be retained because it now holds recovery state.

## Next increments

1. Take a read-only production evidence snapshot and use the report to choose
   the next failure to address. Keep credentials and raw user text out of fixtures.
2. Verify absence across paginated queues; stop when verification is incomplete.
3. Reconcile interrupted incidents through read-only checks, and track
   notification delivery separately from the media action's outcome.
4. Design bounded ledger retention/compaction that preserves attempt state and
   incident identity across queue-ID reuse. Multi-process coordination remains
   outside the current single-owner deployment model.
5. Turn observed failures into reviewed evaluation cases and focused draft PRs.
   Expand remediation policies only after these reliability gaps are covered.

The first evaluation suite does not establish production recovery rates or
complete the self-improvement loop. Each next increment needs its own validation
and review.
