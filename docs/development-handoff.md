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

The earlier session reported evidence collection and five-minute remediation
running on laddie. That is historical context, not a fresh production check.

## Current increment

Branch: `feat/remediation-evaluation-gate`.

Adds nine frozen remediation scenarios, `make eval`, a visible CI evaluation
job, and incident deduplication in the evidence report. Repeated polling of one
failed queue item now counts as one incident. The suite also runs under the
existing required `test` check.

## Next increments

1. Take a read-only production evidence snapshot and use the report to choose
   the next failure to address. Keep credentials and raw user text out of fixtures.
2. Persist remediation attempts across restarts; test crash recovery and
   concurrent polling so retries cannot duplicate mutations.
3. Verify absence across paginated queues; stop when verification is incomplete.
4. Fail closed when durable pre-action evidence cannot be recorded, and track
   notification delivery separately from the media action's outcome.
5. Turn observed failures into reviewed evaluation cases and focused draft PRs.
   Expand remediation policies only after these reliability gaps are covered.

The first evaluation suite does not establish production recovery rates or
complete the self-improvement loop. Each next increment needs its own validation
and review.
