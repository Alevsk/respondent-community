# Declarative CCTV and radio integration

Status: approved by the user on 2026-09-22; implementation in progress.

## Objective and first release

Make public cameras and internet radio stations normal Respondent entities: declared in YAML, ingested through the existing engine, visible through the existing map/search/detail flows, and playable inside the application. Preserve the single Go binary, SQLite storage, viewport filtering, and existing React/MUI architecture.

The proposed first release includes Austin and Calgary camera snapshots, Radio Browser stations with native HTTPS MP3/AAC playback, a reusable media display contract, and reusable HTTP endpoint discovery. Camera snapshots refresh while viewed; they are not full-motion video. Radio continues while the user explores the globe and has an explicit stop control.

Full-motion HLS video, RTSP conversion, video projection onto terrain, camera calibration, and the remaining provider packs are subsequent increments. These require capabilities beyond the verified first sources.

## Verified current architecture

The current path is:

```text
sources.d YAML
  -> schema validation and CEL compilation
  -> transport -> parser -> record filter and mapping
  -> domain Entity + Observation
  -> feeder -> SQLite / in-process caches / realtime updates
  -> existing layer, search and entity APIs -> globe and entity panels

display YAML
  -> DisplaySpecToDomain -> DynamicSourceRegistry
  -> LayerService -> layers.proto -> GetLayers
  -> frontend layer store -> icons and text field renderers
```

Evidence and implications:

- `internal/ingest/declarative/schema.go` and `loader.go` define and validate source contracts. `adapter_mapping.go` maps one record to one entity/observation pair. Metadata values are strings.
- `display.go` provides the shared display conversion. The lightweight display registration path deliberately works without ingestion credentials and must support any new display configuration too.
- `internal/domain/dynamic_registry.go` stores display settings per **layer**, with last-write-wins semantics. Sources with different media policies need distinct layer types. The initial layers will be `cctv_austin`, `cctv_calgary`, and `radio_stations`.
- `api/proto/layers.proto` owns the API display contract. Generated Go/OpenAPI files must be regenerated, not edited manually. Frontend model types mirror that contract.
- `fieldRenderers.ts` resolves and formats string values. A player has lifecycle and controls that do not fit this scalar formatter interface.
- Desktop and mobile entity panels share `OverviewTab`. Both initial bootstrap and later layer queries hydrate display configuration; both paths must retain the media contract.
- `CameraFeed`, `Calibration`, and `internal/app/cctv` exist, but `cmd/community/serve.go` does not register a CCTV API service. `CCTVPanel.tsx` contains placeholder controls. These are not an operating declarative camera pipeline.
- ADS-B, AIS, FIRMS, and USGS definitions already exist. APRS and radiosondes are different from internet radio playback. No camera catalog or Radio Browser source YAML was found.

## Provider assessment

“Fits” below describes catalog ingestion, not guaranteed live media availability. Source-code inspection was performed against the local `gods-eye-view` checkout.

| Provider | Existing engine fit | Additional requirement / first-release decision |
| --- | --- | --- |
| Austin | HTTP JSON + CEL; use Socrata resource objects instead of positional `rows.json` | Include. Filter `TURNED_ON`; map stable `camera_id`, `location.coordinates`, and `screenshot_address`. |
| Calgary | HTTP JSON + CEL | Include. Stable identity from the camera URL; normalize the specifically verified official host to HTTPS. `quadrant` describes the address, not the camera heading. |
| NSW | GeoJSON + CEL | Catalog fits. A sampled image returned HTML despite HTTP 200; leave out of the initial enabled sources until browser media verification succeeds. |
| Caltrans | JSON + nested CEL fields | Catalog fits per district. Add provider packs after first-release validation; verify stable identifiers and images. |
| TfL JamCams | JSON + CEL selection from additional properties | Catalog fits; verify image endpoints and carry provider attribution before onboarding. |
| Ontario 511 | JSON catalog | Multiple views per camera need an explicit view model or generic record expansion for full parity. |
| DriveBC | JSON + CEL | Catalog fits; preserve image attribution. Playback remains to be verified. |
| Fintraffic | GeoJSON stations with nested presets | Full parity needs generic child-record expansion retaining station context. Do not silently discard all but one view. |
| TxDOT | JSON catalogs per district | Images are base64 inside a JSON response. Requires a reusable media response decoder and controlled delivery, not a provider branch in the player. |
| Tarktee | Two DATEX/XML feeds | Requires a refreshable join between location and image records. Existing lookup tables are inline/local files, not dynamic feed joins. |
| Tallinn / Warendorf / other curated files | Static catalogs | Need a first-class local/static source or an intentionally hosted catalog. Some media URLs also need separate delivery compatibility work. |
| Radio Browser | HTTP JSON + CEL mapping | Include with generic endpoint discovery/failover and playback notification. The native stream bytes go directly to the browser. |

Live HTTP probes on 2026-09-22:

- Austin resource catalog: HTTP 200. A sampled `TURNED_ON` camera returned `image/jpeg` with JPEG magic bytes.
- Calgary resource catalog: HTTP 200. The sampled camera's HTTPS URL returned `image/jpeg` with JPEG magic bytes.
- NSW catalog: HTTP 200, 217 features. The sampled image URL returned `text/html`, not JPEG. Catalog availability must not be reported as successful camera playback.
- Radio Browser station search: HTTP 200 with geolocation, resolved stream URLs, codec and health fields. DNS SRV discovery returned an HTTPS mirror.
- These are bounded network probes, not browser playback tests or guarantees about every feed.

## Approaches considered

1. **Dedicated `display.media` contract — recommended.** Reuse entities, observations, layers and YAML mappings; add typed media presentation and player lifecycle. This gives additional providers a YAML-only path when their protocol is already supported.
2. **Add image/audio values to `field_renderers`.** Fewer initial schema changes, but changes a string-formatting abstraction into stateful UI and complicates every consumer of resolved fields. It also leaves multi-field attribution, refresh and playback behavior implicit.
3. **Port separate CCTV/radio catalogs and services from Gods Eye.** Reuses reference code but duplicates ingestion, identity, caching and selection paths. It does not meet the requested architecture.

## Proposed display contract

Add an optional `display.media` list to the current source schema, domain display config, protobuf display config, and TypeScript model. Existing source definitions retain their behavior. Use typed media variants in protobuf and TypeScript; reject unsupported kinds at source load time.

Example proposed YAML fragment (not supported by the current implementation):

```yaml
display:
  # Existing icon, trail, style and field_renderers remain here.
  media:
    - id: camera
      kind: snapshot
      label: Camera feed
      url_key: snapshot_url
      attribution_key: attribution
      allowed_origins:
        - https://cctv.austinmobility.io
      snapshot:
        refresh_interval: 30s
        cache_bust_param: _respondent_frame
```

The audio variant uses `kind: audio`, `url_key: stream_url`, and an optional `playback_action` reference. It has no snapshot refresh fields. The first release supports one media entry per entity in the UI; the list preserves a stable slot identifier for later multi-view capabilities.

Validation rules:

- Media IDs are unique, stable source-name-style identifiers. Labels and URL metadata keys are required. Referenced keys must exist in the source's declared entity or observation metadata.
- Only the `snapshot` variant accepts refresh configuration. Refresh intervals are bounded to 5 seconds through 1 hour. The initial camera definitions use 30 seconds.
- A cache-busting query parameter is opt-in, declared by the source, and added using URL parsing. Signed URLs must not be modified implicitly.
- Accept absolute HTTPS media URLs without embedded credentials. Reject executable/local URL schemes and literal loopback/private hosts. Camera definitions declare exact allowed origins; Radio Browser station hosts vary.
- Browser-side URL validation is not a DNS or redirect firewall. This release performs direct third-party media loading; any future server relay requires separate outbound-request controls.
- Use the same media validation from full source loading and lightweight display registration. Invalid declarations produce source-specific diagnostics and do not become playable configuration.

Keep provider identity, URL derivation, metadata labels and attribution in YAML. The UI selects a component by the declared media kind, never by a provider name, layer-name prefix, or URL suffix. Omitted media configuration renders the current overview unchanged.

## Radio Browser transport and playback notification

The official API guidance requests mirror discovery, retry through another mirror, a descriptive User-Agent, stable station UUIDs, and a station-click request when the user starts playback. A single hardcoded mirror does not provide this behavior.

Extend `http_poll` with optional DNS SRV endpoint discovery. The declaration supplies the SRV name, HTTPS scheme, allowed DNS target suffix, and bounded discovery cache lifetime. Resolve targets through an injected resolver, respect SRV priorities/weights, validate returned hosts/ports, and use bounded retries. Existing fixed-URL sources keep their current transport behavior. Radio uses `_api._tcp.radio-browser.info`, HTTPS, port 443, and targets under `api.radio-browser.info`.

Discovery replaces only the origin of the declared request URL; the configured path/query, headers, response limits and rate budget remain authoritative. One catalog refresh uses one selected mirror. If that mirror fails partway through pagination, discard the partial refresh and restart from page zero on the next mirror within the same overall budget. Never mix page offsets from different directory snapshots or publish a failed refresh as a successful empty catalog.

Use offset pagination with 1,000 rows per page, a 50-page ceiling, explicit deterministic ordering supported by the API, and empty-page termination. Poll every six hours; apply a per-host rate budget of one request/second. Keep the previous catalog on refresh failure. Use a 24-hour entity cache TTL. Surface a reached pagination ceiling in logs rather than describing a truncated catalog as complete.

For playback notification, add a narrowly scoped declarative media action:

- A trusted source definition declares a named GET action, relative to the same discovered origin, with a CEL path derived from stored entity metadata. Radio declares `/json/url/` plus its validated station UUID.
- A protobuf-defined `ReportMediaPlayback` operation accepts entity ID and media ID, resolves the registered action on the server, and invokes an injected action executor. It accepts no caller-provided URL, headers, or CEL expression.
- The app service depends on domain ports; HTTP execution and discovery stay in adapters/supporting packages, wired at the composition root.
- Validate the path as relative and reject authority changes, traversal and unexpected query construction. Bound response size, time, concurrency and request rate. Return only notification status, never an upstream stream URL.
- Notify once for each user-initiated successful playback start. Notification failure is observable but does not stop audio or cause repeated notification retries. Catalog polls, selection, rendering and automatic reconnects never report plays.

Map `stationuuid` to identity, require finite in-range non-null coordinates, a successful directory health check, HTTPS resolved URL and a supported MP3/AAC codec, and exclude HLS for this release. Preserve `countrycode`, languages, bitrate, tags and last provider check time. Zero latitude or longitude is valid; missing coordinates must not be coerced to zero.

## Media UI and lifecycle

Add a shared media section near the top of the existing overview, above location/details. Use the current MUI theme, typography and panel shell. Camera markers use a camera shape added through the icon registry; radio reuses the existing radio shape. Both are selected by YAML.

Camera behavior:

- Show an aspect-ratio-preserving image, provider attribution, refresh/pause control and expand control inside the existing desktop/mobile panel.
- One active camera refresh session per application. Selecting or explicitly activating another camera releases the previous session; additional pinned panels offer a view action.
- Start refresh only when the media section is visible. Suspend on collapsed panels, non-overview tabs, hidden browser documents, historical exploration or layer disable. Dispose timers and in-flight work when switching or closing.
- Schedule the next refresh after the current load settles; requests never overlap. Discard late results from a previous camera or URL. Bound loading time and back off after errors.
- Distinguish loading, current image, paused, stale and unavailable. A displayed receipt time is labeled as the last successful load, not the camera capture time. Show a stale indicator over the last good frame after a failed refresh.

Radio behavior:

- One app-owned audio element; Zustand contains serializable session state and controls, not the element. Media events drive the displayed state.
- Explicit Play starts playback; neither layer visibility nor entity selection starts audio. Use native media with no speculative preload. Handle rejected play promises and unsupported codecs visibly.
- A compact persistent player carries station name, attribution, play/pause, volume and stop while the user navigates the globe. It uses the existing layout system and respects mobile safe areas.
- Starting another station stops and releases the previous source. Closing the detail panel keeps the player available; Stop, disabling its layer, or leaving the application releases it. A reload does not automatically restart playback.
- Playback errors expose a retry action; there is no unbounded reconnect loop. Volume preference may persist, while playback state and stream URLs do not persist across reloads.

Historical observations describe catalog metadata, not archived media. The media section does not automatically request current camera images during historical exploration. An already running radio session remains clearly identified as live.

## Performance and data integrity

Camera frames and audio bytes do not enter SQLite, observation metadata, realtime messages, or map textures. Only catalog data follows the fusion pipeline. Browser-native image/audio loading avoids a continuous bandwidth burden on the Go server.

Use `filtering: viewport`, stationary icons without interpolation, stable provider IDs, and content-based deduplication. Content hashes must include changes that affect identity presentation or playback: name, position, URL, relevant status and attribution. Poll camera catalogs hourly with a 24-hour cache TTL; image refresh has its own independent interval.

Normalize catalog metadata in CEL and resolve media fields from the same entity-plus-latest-observation precedence as the existing overview. Keep raw metadata available in the metadata tab; avoid repeating the media URL as an ordinary overview row. Do not interpret an arbitrary URL in unrelated entity metadata as permission to mount a player.

An upstream camera failure is reported as unavailable/stale. Gods Eye's Street View and synthetic-frame substitutions are not camera observations and must not be labeled as such in Respondent.

## Implementation sequence after design approval

1. Add the optional display contract, shared validation, domain/protobuf conversions and frontend types; regenerate API artifacts. Test backward compatibility and both registration paths.
2. Implement endpoint discovery and the bounded playback-notification use case through injected ports. Cover pagination/failover and preserve the fixed-URL path.
3. Add reusable snapshot/audio components, the single-session lifecycle, persistent player and overview integration for desktop/mobile.
4. Add Austin, Calgary and Radio Browser YAML definitions plus captured catalog fixtures. Compile and run actual mappings against fixtures, not only YAML unmarshalling.
5. Update `sources.d/TEMPLATE.yaml`, source/display/transport documentation, API documentation and operational guidance. Run browser verification and the repository checks.

## Acceptance and verification

- Existing source definitions compile and retain their display behavior; a layer with no media declaration never loads media.
- A new compatible provider can be onboarded with YAML alone, without a provider condition in Go or React.
- Catalog fixtures verify identity, coordinates, status filters, metadata types, URL policy, changed stream URLs and dedupe behavior. Snapshot refresh never generates observations.
- Resolver/action tests cover SRV failures and expiration, invalid target hosts/ports, bounded retries, 429 handling, canceled requests, mid-pagination mirror failure, unknown entity/media IDs and arbitrary-target rejection.
- Component tests cover switching while a load/play promise is pending, hidden/collapsed panels, cleanup, cache-busting with existing query parameters, errors/stale frames, unsupported playback and one audio session across multiple pinned panels.
- Playwright uses deterministic image/audio fixtures for desktop/mobile controls, panel navigation, focus/accessibility and request counts. Add separate manual live playback checks; CI does not depend on public stream availability.
- Verify zero media requests for inactive entities, at most one active snapshot session, one audio element, and no media traffic over the Respondent WebSocket. Record bundle impact and verify that long playback does not grow retained resources.
- Regenerate contracts with `rtk make proto`; run `rtk make all-format`, `rtk make all-lint`, `rtk make all-test`, and `rtk make e2e` before reporting implementation complete. Inspect formatting changes for unrelated edits.

Baseline checks run during assessment:

- Targeted declarative source validation/display tests: 96 passing tests/subtests.
- Existing field renderer, overview and mobile entity panel tests: 62 passing tests across three files.
- The full repository validation pipeline has not been rerun for this design-only change.
- The pre-existing untracked `test-retention.yaml` was not modified.

## External references

- [Radio Browser discovery and client guidance](https://api.radio-browser.info/).
- [Radio Browser station/search/playback API contract](https://docs.radio-browser.info/).
- [Transport for NSW camera dataset](https://data.nsw.gov.au/data/dataset/2-live-traffic-cameras).
- Austin catalog: `https://data.austintexas.gov/resource/b4k4-adkb.json`.
- Calgary catalog: `https://data.calgary.ca/resource/k7p9-kppz.json`.
- Reference implementation: local `gods-eye-view/server/providers/cctv/`, `server/providers/radio/`, and `src/sources/radioBrowser.js`.
