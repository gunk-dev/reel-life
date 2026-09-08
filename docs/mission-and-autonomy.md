# Mission, outcomes, and autonomy

## Mission

reel-life keeps a home media system healthy and useful with the least possible
operator effort. It detects problems, explains them plainly, safely remediates
the problems it understands, verifies the result, and learns from outcomes and
operator feedback.

Media discovery and library management support that mission, but autonomous
problem remediation is the first product priority.

## Product outcomes

Changes are evaluated against outcomes rather than feature count:

1. **Recovery:** known incidents are resolved without operator diagnosis.
2. **Trust:** every action is attributable, bounded, and accurately reported.
3. **Speed:** incidents are detected and resolved before they affect playback.
4. **Signal quality:** alerts are actionable and rarely noisy.
5. **Learning:** repeated incidents lead to measurable improvements.

Initial targets:

- zero unconfirmed destructive actions;
- zero duplicate mutations caused by retries or duplicate messages;
- at least 80% precision for actionable alerts;
- at least 70% of eligible incidents resolved automatically;
- every remediation records detection, plan, action, and verification events;
- every behavior change must match or improve the frozen evaluation suite.

## Runtime autonomy policy

Remediation policies are allowlisted in code. A policy may run automatically
only when its action is reversible or repeat-safe, its preconditions are
machine-verifiable, its attempts are bounded, and a postcondition can prove
whether it worked.

Each remediation follows this lifecycle:

```text
detect -> correlate -> plan -> act -> verify -> resolved | escalated
```

The runtime must:

- record a redacted event at every transition;
- use a stable incident key and idempotency key;
- apply per-policy cooldown and attempt limits;
- stop when verification fails or the state becomes ambiguous;
- notify the operator of the action and its verified outcome;
- never infer success merely because an API call returned without error.

Automatic policies must not delete media, remove a series or movie, approve or
decline a request, delete an indexer, weaken authentication, change credentials,
or expand their own permissions. Those actions require an explicit user action.

## Improvement and release boundary

Production events, sanitized conversations, tool calls, feedback, and observed
outcomes may be retained locally for evaluation. Secrets and credentials must
never be stored, and free-form user text is excluded by default unless it has
been explicitly sanitized.

The improvement loop may analyze evidence, create evaluation cases, recommend
changes, and open draft pull requests. It may not merge or deploy a pull request
until the repository owner posts a comment whose normalized content is exactly:

```text
i approve
```

Approval is scoped to the current pull-request head commit. A new commit makes
the approval stale and requires a new approval comment. Approval does not waive
CI, evaluation, security, canary, or rollback gates.

The `reel-life/owner-approval` commit status implements this boundary. Configure
it as a required branch-protection check. The approving GitHub login defaults
to `patflynn` and can be changed with the `REEL_LIFE_APPROVER` repository
variable. Run `make report` (optionally with
`EVIDENCE_PATH=/path/to/events.jsonl`) to summarize current outcome evidence.

## Production testing

The production media stack currently has no users and may be used for bounded
integration validation. Tests must still default to read-only operations. A
deterministic simulated stack is the required CI environment and the first place
where mutating and failure-injection scenarios run.
