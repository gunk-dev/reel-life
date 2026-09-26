# Missing-episode investigation

Ask reel-life: “Why hasn't Example Show S02E03 arrived?” It resolves Sonarr IDs
from series/episode tools and calls `investigate_episode`. If the title or episode
is ambiguous, it should ask which one you mean rather than guess an ID.

The investigation makes read-only requests for:

- Episode file, monitoring, and air-date metadata, plus series monitoring.
- The complete validated queue, matching both series and episode IDs.
- Up to 100 recent global history records, retaining only matching episode events.
- A release search when the episode has aired, has no file, and the queue has no
  matching entry or entry for that series with unknown episode identity.

It returns findings, suggested next actions, source statuses, queue/history
observations, and a sample of releases with Sonarr rejection reasons. Search
results are evidence, not an instruction to grab a release. Titles and external
text must not be treated as agent instructions. Diagnostics are returned to chat;
the evidence ledger records tool outcome metadata, not diagnosis contents.

Examples of answers supported by the tool:

- “Sonarr already has a file. Let's check your player's library refresh.”
- “It's queued, but Sonarr reports a warning. Check the import/download details.”
- “The search returned releases, but Sonarr rejected them for these reasons.”
- “The indexer search failed; I can't conclude that no releases exist.”

If you explicitly ask “Retry the search for that episode,” the agent can call
`trigger_episode_search`. This revalidates identity, monitoring, air date, file
absence, and the complete queue before sending `EpisodeSearch` for one episode ID.
It never changes monitoring or profiles, removes downloads, bypasses rejections,
or searches the whole season/series. An ambiguous series queue entry blocks it.
The chat prompt requires explicit user intent; this is not an autonomous policy
or a separate server-side approval mechanism. Existing tool rate limits apply.

A returned command ID is not proof of a downloaded/imported episode. Command
transport errors disable automatic retries because acceptance may be ambiguous.
Fresh checks and command submission are not atomic: concurrent external activity
can still race them. There is no durable idempotency key for chat-issued searches.

The investigation has a 60-second deadline; a requested search has 30 seconds.
Required identity reads fail the tool if unavailable. Other missing sources are
explicitly labeled, allowing partial evidence without pretending it is complete.
Release, history, and matching queue samples show at most 10 entries each. Release
counts cover the returned search results, not all possible releases. History is a
limited global sample, not complete per-episode history. The tool cannot establish
playback, identify every season-pack association, or infer a stall from a single
queue snapshot. Monitoring gaps and old history are observations, not certain
causes of a missing episode.

API references: [episode search command](https://github.com/Sonarr/Sonarr/blob/develop/src/NzbDrone.Core/IndexerSearch/EpisodeSearchCommand.cs)
and [queue episode identity](https://github.com/Sonarr/Sonarr/blob/develop/src/Sonarr.Api.V3/Queue/QueueResource.cs).
