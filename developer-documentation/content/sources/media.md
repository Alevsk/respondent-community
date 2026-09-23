---
title: "Cameras and Radio"
description: "Public camera snapshots and internet radio as ordinary declarative entities"
weight: 8
---

Public cameras and internet radio stations are ordinary Respondent entities:
declared in YAML, ingested by the same engine, and shown through the same map,
search and detail panels. Only the *catalog* travels through the pipeline. The
frames and the audio are fetched by the browser directly from the provider.

For the YAML fields themselves, see
[`display.media`](../display-config/#media) and
[playback notifications](../transports/#playback-notifications).

## What ships today

| Source files | Layer | Media | Catalog poll | Media refresh |
|---|---|---|---|---|
| `cctv_austin.yaml` | `cctv` | JPEG snapshot | 1 h | 30 s while viewed |
| `cctv_calgary.yaml` | `cctv` | JPEG snapshot | 1 h | 30 s while viewed |
| `cctv_caltrans_d1…d12.yaml` | `cctv` | JPEG snapshot | 1 h | 30 s while viewed |
| `cctv_tfl_london.yaml` | `cctv` | JPEG snapshot | 1 h | 30 s while viewed |
| `cctv_drivebc.yaml` | `cctv` | JPEG snapshot | 1 h | 30 s while viewed |
| `cctv_ontario511.yaml` | `cctv` | JPEG snapshot | 1 h | 30 s while viewed |
| `radio_browser_stations.yaml` | `radio_stations` | Native MP3 / AAC audio | 6 h | continuous stream |

Roughly 7,000 cameras across six providers — Austin, Calgary, Caltrans (twelve
California districts), Transport for London, DriveBC and Ontario 511 — all
feeding one `cctv` layer, the way fifteen news sources feed `news_articles`.

Caltrans gets one file per district because it publishes one catalog per
district and a source declares one URL. Every file's display block is identical;
a test asserts that every source feeding `cctv` declares the same contract apart
from its `allowed_origins`, which the registry unions. Without that union the
last file to load would decide the origins for all of them and every other
provider's cameras would fail admission in the browser.

Ontario 511 exposes several views per camera. This release shows the first
enabled one; showing them all needs a record-expansion capability the engine
does not have yet.

## Who owns what

Respondent is a directory and a viewer, not a host:

- **Camera frames and audio bytes never enter Respondent.** They do not reach
  SQLite, observation metadata, realtime messages or map textures. The browser
  loads them straight from the provider, so the Go binary carries no streaming
  bandwidth.
- **The catalog is the only thing ingested** — identity, position, status,
  media URL and attribution.
- **Third parties own availability.** A camera that goes dark, a station that
  moves its stream, a mirror that stops answering: all are the provider's, and
  Respondent reports them as unavailable or stale rather than substituting
  anything.

Each media entry declares an `attribution_key`, and the credit it resolves to is
shown beside the media. Keep it populated: it is how a viewer knows whose camera
they are looking at.

## Freshness, honestly labelled

- The time shown under a camera is **when the frame arrived**, not when the
  camera captured it. Nothing upstream tells us the capture time.
- A failed refresh leaves the last good frame on screen with a **stale** badge,
  rather than blanking the panel or implying the image is current.
- For radio, `provider_last_check` is the *directory's* health check, not ours.
- Historical exploration shows catalog metadata only. Moving the time range does
  not request current frames, and a radio session that is already playing stays
  clearly labelled **LIVE**.

## Resource use

One camera refreshes at a time across the whole application, and only while its
panel is expanded, on screen, in live mode, with its layer enabled and the
browser tab visible. A refresh is scheduled only after the previous one settles,
so requests never overlap, and failures back off. There is exactly one audio
element, and it plays only after an explicit gesture.

## Providers evaluated but not shipped

These were assessed against the current engine. Catalog ingestion fitting is not
the same as verified playback, and nothing is enabled until the media itself is
confirmed to work in a browser.

| Provider | Why it is not in this release |
|---|---|
| Transport for NSW | The catalog is healthy (217 cameras) but the image host is not: twelve sampled camera URLs all returned `text/html` — the provider's own "camera image temporarily unavailable" page — with HTTP 200. Catalog availability is not camera availability. |
| TxDOT | Images are base64 inside a JSON response. Needs a reusable media response decoder, not a provider branch in the player. |
| Fintraffic | The station catalog needs a `Digitraffic-User` header and carries nested presets; full parity needs child-record expansion that keeps station context. |
| Tarktee (Estonia) | Two DATEX/XML feeds needing a refreshable join between location and image records; lookup tables today are inline or local files. |
| Tallinn, Warendorf, other curated files | Static catalogs that need a first-class local/static source or an intentionally hosted catalog. |

Full-motion HLS video, RTSP conversion, video projection onto terrain and camera
calibration are out of scope for this release.

{{< callout type="note" title="Adding a provider" >}}
If a provider's catalog is reachable over an already supported transport and its
media is an HTTPS still image or a native MP3/AAC stream, onboarding it is a new
file in `sources.d/` — no Go and no React changes. Anything else (base64 image
payloads, HLS, feed joins, multi-view cameras) needs engine work first.
{{< /callout >}}
