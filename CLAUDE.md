# Claude Context — respondent-community

This file orients an agent picking up work in this repository. Read it first.

## What This Repo Is

Respondent **Community Edition** — a real-time geospatial intelligence dashboard. A single static Go binary embeds a CesiumJS 3D-globe frontend and serves the HTTP API + WebSocket from one origin. Zero external dependencies: no Postgres, no NATS, no Redis — everything runs in-process backed by a SQLite file.

The repo contains application source code (Go backend, React frontend, Protobuf API), runtime configuration (`respondent.yaml`, `.env.example`), Docker Compose manifest, declarative YAML source/analysis definitions (`sources.d/`, `analysis.d/`), a Hugo documentation site (`developer-documentation/`), and internal specs (`docs/`).

## Backend Architecture (Hexagonal / Ports & Adapters)

The Go backend follows strict hexagonal architecture with dependency inversion.

**Layer dependency rule** (inner never imports outer):
```
domain/  →  app/  →  transport/ | infra/
(inner)     (mid)     (outer)
```

| Layer | Directory | Role | Depends on |
|-------|-----------|------|------------|
| Domain | `internal/domain/` | Pure types, interfaces (`repository.go`, `services.go`), errors. Zero infra imports. | Nothing |
| App | `internal/app/<service>/` | Use-case orchestration. Receives interfaces via constructor injection. | `domain/` only |
| Transport | `internal/transport/grpc/` | Thin gRPC-gateway handlers: validate → call app service → map errors. | `app/`, `domain/` |
| Infra | `internal/infra/<adapter>/` | Concrete implementations: `sqlite`, `inproc`, `memcache`, `ingest`. | `domain/` interfaces |
| Supporting | `internal/{realtime,ingest,llm,geocoder,ai,ratelimit,orbital,config,logging}/` | Cross-cutting services wired at composition root. | `domain/` + stdlib |

**Key patterns:**
- App services receive interfaces, not concrete types — dependency inversion is non-negotiable.
- SQLite with hand-written SQL in `internal/infra/sqlite/*.go`. No sqlc, no `internal/repo/queries/`.
- Proto is the contract — all API paths/types flow from `api/proto/`.
- gRPC-gateway auto-exposes RPCs as REST under `/v1/*`.

## Proto → Code Generation Pipeline

When you modify any `.proto` file in `api/proto/`, regenerate:

```bash
make proto && go build ./...
```

`make proto` → `gen/go/` (Go protobuf + gRPC stubs + gateway handlers) + `gen/openapi/` (OpenAPI v2 spec). **Never** hand-edit anything under `gen/` or `frontend/packages/core/src/api/generated/`.

The frontend REST client is hand-written. The OpenAPI spec is the source of truth for response shapes; mirror them as TypeScript types in `frontend/packages/core/src/api/types.ts`.

## Frontend

React 18 + TypeScript + Vite, **TanStack Query** for server state, **Zustand** for UI/client state, **CesiumJS** 3D globe, **MUI** v5. Do not introduce Redux/RTK Query. Single app: `frontend/apps/earth` — embedded into the Go binary at build time. Shared library code in `frontend/packages/core` (`@respondent/core`). Same-origin API — the client uses relative `/v1/*` URLs; Vite dev proxies `/v1` and `/ws` to `localhost:8090`. Proto `int64` fields arrive as JSON strings — normalize at the API boundary via helpers in `types.ts`.

## Makefile Quick Reference

| Target | What it does |
|--------|-------------|
| `make build` | Build `bin/community` (embeds the earth frontend dist) |
| `make run` | Build frontend + binary, then `./bin/community serve` |
| `make run-frontend` | Vite dev server for `apps/earth` (proxies `/v1`,`/ws` → :8090) |
| `make proto` | Regenerate Go gRPC/gateway stubs + OpenAPI spec via Buf |
| `make all-format` | Format Go + frontend |
| `make all-lint` | Lint Go + lint & typecheck frontend |
| `make all-test` | Run Go tests + Vitest |
| `make e2e` | Boot isolated server (:8091) → Playwright suite → teardown |
| `make compose-up` | Run the stack via `compose.yaml` |
| `make docker-build` | Cross-build multi-arch container image |
| `make docs-dev` | Hugo dev server with live reload |
| `make clean` | Remove `bin/` and embedded frontend dist |

## Constraints

- **Image registry**: `docker.io/alevsk/...`. Do not introduce internal registries.
- **Image names**: only `respondent-community` (app) and `respondent-community-docs` (docs). Do not reference upstream-only images (`respondent-feeder`, `respondent-analyzer`, etc.).
- **No proprietary references**: this is the open-source distribution. No `Platform Edition`, `Enterprise Edition`, or PostGIS-specific guidance.
- **Database path quirk**: `respondent.yaml` has `database.path: "data/respondent.db"` (relative). It is overridden at runtime by `RESPONDENT_DATABASE_PATH=/data/respondent.db` in `compose.yaml`. Do not "fix" the YAML to an absolute path.
- **Demo video embed**: `README.md` references the demo via a `github.com/user-attachments/assets/` URL. Do not "correct" it to a relative path.

## Validation Pipeline

Before claiming work is done, run:
```bash
make all-format && make all-lint && make all-test && make e2e
```
