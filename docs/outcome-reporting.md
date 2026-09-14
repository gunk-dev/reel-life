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
An old last-event timestamp can mean a quiet queue, not necessarily a stopped
monitor; the report is not a heartbeat.

Do not truncate the ledger to satisfy the size limit: it also holds remediation
recovery state. Bounded compaction that preserves that state is follow-up work.

Run the integration test on a Linux Nix builder with KVM:

```sh
nix build .#checks.x86_64-linux.outcome-report
```

It verifies successful reporting by the named user, rejection of extra arguments
and other users, unchanged raw-ledger permissions, unchanged ledger contents,
and exclusion of a seeded private-text canary.
