# Development handoff

Last updated: 2026-09-26.

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

PRs #58 and #59 are merged and their reporting features are deployed. On September
26, laddie reported 1,527 completed polls, no health-request failures, one delivered
health alert, and no remediation incidents. Tool failures were one invalid-input
and two not-found results. Production recovery rates remain unproven.

Branch: `feat/investigate-missing-episode` adds read-only episode investigation and
an explicitly requested, freshly checked single-episode search. See
[episode investigation](episode-investigation.md). The next capability increment
is awaiting review and deployment; no production retry was performed in testing.

## Next increments

1. After episode investigation deploys, exercise it through chat on a real missing
   episode. Use sanitized diagnosis cases to improve explanations and verification;
   do not infer recovery from an accepted search command.
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
