---
title: "Recording Modes"
description: "Control how entities are persisted — append, upsert, or deduplicate"
weight: 5
---

The recording mode controls how observations are persisted when the feeder processes new data. Choose the mode based on how your source data changes over time.

```yaml
recording:
  mode: upsert
```

{{< field name="mode" type="string" required="true" >}}
Recording strategy. Allowed values: `append`, `upsert`, `dedupe`.
{{< /field >}}

## Append

Every poll inserts a new observation row, building a time-series history for each entity.

```yaml
recording:
  mode: append
```

Use `append` when:
- Entity positions change frequently (satellite tracks, ship voyages, flight paths)
- You need historical trajectory data
- You want to replay entity movement over time

Each poll creates a new row even if nothing changed. The observation history grows with every poll cycle.

**Example use cases:** satellites, aircraft, ships, weather balloons, GPS trackers.

## Upsert

Each poll inserts a new observation or updates an existing row on a conflict against the composite key `(entity_id, ts)` (`ON CONFLICT (entity_id, ts) DO UPDATE`). When an entity's observation timestamp stays the same across polls, the existing row is overwritten in place. When the observation timestamp advances between polls, the upsert inserts a new row -- so for an entity whose timestamp changes every poll, upsert behaves like append for that entity.

```yaml
recording:
  mode: upsert
```

Use `upsert` when:
- You only care about the current state, not historical positions
- Entities are relatively static (their position rarely changes)
- The observation timestamp is stable across polls so updates land on the same row
- You want to minimize storage

New data overwrites the previous observation only when the `(entity_id, ts)` pair matches; a changed timestamp produces an additional row rather than replacing the prior one.

**Example use cases:** earthquakes, weather stations, sensor readings, infrastructure status.

## Dedupe

Inserts a new observation only when the `content_hash` changes from the last recorded value. Requires `observation.content_hash` to be set.

```yaml
recording:
  mode: dedupe

# content_hash must be defined in observation:
observation:
  content_hash: >
    record.link
```

Use `dedupe` when:
- The source re-publishes the same content across polls
- You want to record changes but avoid duplicates
- Data arrives as a feed with overlapping entries (news articles, alerts)

The content hash is compared against the last recorded hash for each entity. If they match, no new row is written.

**Example use cases:** news feeds, alert bulletins, status pages, event logs.

{{< callout type="warning" title="content_hash is required for dedupe mode" >}}
When `mode: dedupe` is set, you must define `observation.content_hash` with a CEL expression that produces a unique string for each distinct observation state. Without it, deduplication cannot function.
{{< /callout >}}

---

## Cache TTL

The `cache` section controls how long entities remain visible on the globe after the last successful fetch.

```yaml
cache:
  ttl: "3600s"
```

{{< field name="cache.ttl" type="duration" required="true" >}}
Time-to-live for the hot cache (Valkey in production, MemCache in community edition). Entities disappear from the globe after this duration without a refresh.
{{< /field >}}

{{< callout type="tip" title="Set TTL higher than the polling interval" >}}
Set `cache.ttl` higher than `transport.interval` so entities survive a few missed polls. For a source polling every 60 seconds, a TTL of 120-300 seconds gives a reasonable buffer.
{{< /callout >}}

---

## Choosing a recording mode

| Scenario | Mode | Reason |
|----------|------|--------|
| Satellite positions every 30s | `append` | Need trajectory history for trail rendering |
| Ship AIS positions | `append` | Track vessel routes over time |
| Earthquake catalog | `upsert` | Only latest magnitude/depth matters per event |
| Weather station readings | `upsert` | Current conditions replace previous readings |
| RSS news articles | `dedupe` | Same article appears in multiple polls |
| Alert bulletins | `dedupe` | Record changes but ignore re-published identical alerts |
| Fire hotspots | `upsert` | Latest detection replaces previous observation |
| Radiosonde flights | `append` | Altitude changes over time as balloon ascends |

{{< callout type="info" title="Storage impact" >}}
`append` mode grows linearly with each poll cycle. For sources with many entities and frequent polling, this can accumulate significant data. Use `upsert` or `dedupe` when historical tracking is not needed to reduce storage requirements.
{{< /callout >}}
