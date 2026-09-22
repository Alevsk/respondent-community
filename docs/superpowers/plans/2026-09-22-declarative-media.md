# Declarative Media Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development for independent implementation and review. Steps below track execution in the current session.

**Goal:** Deliver the approved Austin/Calgary snapshots and Radio Browser native audio through declarative sources and existing entity UI.

**Architecture:** Catalog records remain entities and observations. Optional typed media configuration flows through the existing display contract; browser media sessions remain outside ingestion. HTTP SRV discovery and bounded playback notifications are reusable supporting services wired through domain ports.

**Tech Stack:** Go, CEL, YAML, protobuf/gRPC-gateway, SQLite, React, TypeScript, Zustand, MUI, Vitest, Playwright.

## Agreed interfaces

`display.media` entries have `id`, `kind`, `label`, `url_key`, optional `attribution_key`, `allowed_origins`, and `playback_action`. Snapshot entries carry `snapshot.refresh_interval` (5s–1h) and optional `cache_bust_param`. Audio entries have an empty audio option in protobuf. The API uses camelCase and a protobuf oneof for snapshot/audio options.

```ts
type MediaConfig = {
  id: string; label: string; urlKey: string; attributionKey?: string;
  allowedOrigins?: string[]; playbackAction?: string;
} & (
  | { kind: 'snapshot'; snapshot: { refreshIntervalSeconds: number; cacheBustParam?: string } }
  | { kind: 'audio'; audio?: Record<string, never> }
);
// LayerDisplayConfig adds media?: MediaConfig[].
// POST /v1/media/playback receives { entityId, mediaId }, returns { reported: boolean }.
```

HTTP discovery is optional `transport.discovery` with `srv_name`, `allowed_suffix`, `port: 443`, and `cache_ttl`. The declared HTTPS URL supplies the path/query. Top-level `media_actions` entries have `name`, `method: GET`, and CEL `path` with `metadata` (merged string map) in scope. Action definitions never reach the browser.

## Task 1 — Contracts and validation (root)

Files: `internal/ingest/declarative/media.go`, `media_test.go`, `schema.go`, `loader.go`, `display.go`; `internal/domain/media.go`, `layer.go`; `api/proto/layers.proto`, `media.proto`; `internal/transport/grpc/proto_convert.go`; generated API output. Frontend model is owned by the frontend task.

- [ ] Write load tests using valid source fixtures plus media YAML; assert unknown kinds, duplicate IDs, missing metadata keys, wrong variant settings, and invalid origins are rejected in full and lightweight loading. Run `rtk go test ./internal/ingest/declarative -run Media -count=1` and observe failure.
- [ ] Implement shared validation and optional domain display mapping. Add a protobuf media oneof and dedicated MediaService request/response without changing existing methods.
- [ ] Run `rtk make proto`, refresh generated frontend OpenAPI using the repository generation mechanism, add wire-conversion assertions, and rerun the targeted tests.

## Task 2 — Browser media lifecycle (frontend implementer)

Files: `frontend/packages/core/src/models/layer.ts`; new `frontend/apps/earth/src/features/media/` modules/tests; `features/entity/tabs/OverviewTab.tsx`; `features/globe/icons/`; app/layout integration where needed; deterministic `e2e/tests/media.spec.ts`.

- [ ] Write failing tests for URL admission, opt-in cache-busting, no requests for inactive entities, stale-image behavior, switching while pending, cleanup, single audio ownership, and explicit playback notification.
- [ ] Implement a typed snapshot/audio resolver and provider-independent media section using the agreed interface. Keep media URLs out of ordinary overview rows and retain raw metadata.
- [ ] Implement one visible snapshot session with image completion scheduling, timeout, bounded backoff, visibility/history/layer gating and controls. One app-owned native audio element backs a persistent accessible player; native playback starts only after a user gesture. Stop on layer disable, keep playing after detail close.
- [ ] Add camera icon via the existing shape registry. Integrate responsive layout and deterministic browser tests. Run scoped Vitest, typecheck and lint. Record test results and touched files for review.

## Task 3 — HTTP discovery and notifications (root)

Files: new `internal/ingest/declarative/discovery.go`, `discovery_test.go`, `media_actions.go`, `media_actions_test.go`; `transport.go`, `transport_factory.go`, `adapter.go`, `adapter_pagination.go`, loader/compiled-source modules as needed; `internal/app/media/service.go`, tests; `internal/transport/grpc/media_grpc_server.go`, tests; `cmd/community/serve.go`.

- [ ] Add failing injected-resolver/HTTP tests for valid SRV candidates, cache expiry, invalid suffix/port, failures/cancellation, response limits, mirror rotation and atomic paginated refresh. No public-network dependency in tests.
- [ ] Implement resolver-backed origin selection while keeping configured path/query and fixed-URL behavior. Restart failed discovered pagination at page zero on a new mirror and retain previous entities on failure. Log ceiling truncation.
- [ ] Add failing app/transport/action tests: resolve stored entity and declared media ID, merge metadata consistently, reject caller URL injection and unsafe relative paths, return only notification status, and bound HTTP concurrency/rate/time/response size.
- [ ] Compile metadata path expressions at source load; execute declared GET actions through the discovered transport. Wire the service via domain interfaces and protobuf gateway at the composition root. Run Go package tests and race tests.

## Task 4 — Source definitions and documentation (after contract stabilization)

Files: `sources.d/cctv_austin.yaml`, `cctv_calgary.yaml`, `radio_browser_stations.yaml`; `internal/ingest/declarative/media_sources_test.go`, `testdata/media/`; source template and developer source/display/transport docs.

- [ ] Write fixture-driven tests that load source YAML and run actual parsing/mapping. Include missing/null/zero coordinates, inactive cameras, unsupported codecs/HLS, URL updates and content hashes.
- [ ] Add YAML declarations: separate regional camera layers, hourly catalogs, 30s snapshots, 24h cache; six-hour radio catalogs with 1,000-row offset pages, 50-page cap, HTTPS/native-codec filters, discovered mirrors and declared playback action.
- [ ] Run `rtk go test ./internal/ingest/declarative -run 'Media|ValidateSourceDefinitions' -count=1`. Document configuration, supported media, resource ownership, attribution, freshness and known excluded providers.

## Task 5 — Review and integrated verification (root and reviewers)

- [ ] Review against each approved design requirement, then request independent code-quality review and resolve material findings.
- [ ] Run `rtk make all-format`, inspect unrelated changes, run `rtk make all-lint`, `rtk make all-test`, and `rtk make e2e`.
- [ ] Verify actual camera images and a compatible public radio stream in a browser, separately from fixture-based CI tests. Compare bundle size; inspect active-session request counts and cleanup during navigation.
- [ ] Commit scoped work on `codex/declarative-media`; integrate the verified result into the user's working checkout without touching `test-retention.yaml`. Report verified results and remaining upstream limitations.

## Coverage check

All approved contract, ingestion, media UI, source, lifecycle, discovery, notification, documentation and verification requirements map to the five tasks above. Full-motion video and additional providers remain outside this approved release. The frontend and root own disjoint implementation files; shared interfaces are fixed above and changes are communicated before editing.
