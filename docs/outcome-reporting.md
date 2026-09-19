# Restricted outcome reporting

The Nix package installs both `reel-life` and `reel-life-report`. Operators with
direct ledger access can run the latter with `-events /path/to/events.jsonl`.
Its default detailed output includes tool operation names and is not the
restricted interface.

For access to aggregate outcomes only, configure:

```nix
services.reel-life = {
  evidencePath = "/var/lib/reel-life/events.jsonl";
  outcomeReportUsers = [ "patrick" ];
};
```

After deployment, the named user can run:

```sh
sudo -n /run/current-system/sw/bin/reel-life-outcomes
```

The sudo rule allows only the system-profile wrapper with no arguments. The wrapper
also rejects arguments and fixes the executable, ledger path, summary mode,
64 MiB input limit, and 30-second timeout (plus five seconds before forced kill).
The rule explicitly forbids environment overrides (`NOSETENV`), including for
wheel members who have broader sudo rights after password authentication.
Enabling the option does not change raw-ledger permissions or permit arbitrary
file reads. No HTTP endpoint or network listener is added.

The summary's fixed schema contains counters, source byte size, generation time,
and first/last parsed event timestamps. It never returns ledger-provided operation
names, IDs, titles, attributes, or error text. Missing or oversized ledgers fail
without producing a partial report. Malformed records are counted. The command
reads the file size observed at startup, so concurrent appends do not extend the
read; a partially appended final record can count as malformed.

Counts cover the retained ledger, not a rolling interval. Unique incident counts
retain the report's existing deduplication rules. Resolved and escalated categories
are independent and may overlap. Tool and detection/action failure counts are
per-event. Queue removal does not prove replacement download or playback success.
The `monitor` object counts poll starts/completions, Sonarr health request
successes/failures, and health-alert delivery successes/failures. It includes the
latest poll start/completion and health success/failure timestamps. A completion
is recorded after the remediation runner returns, including when a health request
fails; it does not mean Sonarr is healthy or that remediation succeeded. A health
success means the request succeeded, even if Sonarr reported issues. These alert
counters cover monitor health alerts only, not remediation notifications.

Compare poll timestamps with the configured monitor interval and report generation
time. A stale completion is a reason to investigate, not proof of an outage:
monitoring may be disabled, a poll may be blocked, or evidence writes may fail.
Old ledgers have zero monitor counters and absent monitor timestamps; they cannot
establish that monitoring ran. Evidence-disabled installations have no heartbeat.
The monitor records three events per ordinary poll (start, health, completion),
plus notification results when alerts are sent. Evidence errors are logged and do
not stop health monitoring; remediation retains its own mandatory evidence gates.

`tool_failure_kinds` contains only the allowlisted categories `decode`, `network`,
`auth`, `not_found`, `invalid_input`, `rate_limited`, and `unknown`. Missing or
unrecognized categories become `unknown`; no raw error strings are emitted.
Only recorded failed tool results are counted (local rate-limit rejections before
dispatch are currently not recorded). Historical missing categories cannot be
reconstructed. Detection/action failures retain their separate counters.

Do not truncate the ledger to satisfy the size limit: it also holds remediation
recovery state. Bounded compaction that preserves that state is follow-up work.

Run the integration test on a Linux Nix builder with KVM:

```sh
nix build .#checks.x86_64-linux.outcome-report
```

It verifies successful reporting by the named user, rejection of extra arguments
and other users, unchanged raw-ledger permissions, unchanged ledger contents,
and exclusion of a seeded private-text canary.
