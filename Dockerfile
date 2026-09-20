# syntax=docker/dockerfile:1

# tonistiigi/xx provides cross-compilation helpers (xx-go, xx-apk, xx-verify): we
# build on the NATIVE build platform and cross-compile to each target arch. This
# avoids QEMU, whose emulated gcc/cc1 segfaults on CGO builds — and this project
# needs CGO (pg_query_go, the NL-query validator).
FROM --platform=$BUILDPLATFORM tonistiigi/xx:1.6.1 AS xx

# ── Frontend ────────────────────────────────────────────────────────────────
# Static assets are architecture-independent — build once on the native platform.
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.25-alpine AS frontend
RUN apk add --no-cache nodejs npm
WORKDIR /app/frontend
COPY frontend/ ./
RUN --mount=type=cache,target=/root/.npm \
    npm install && npm --prefix apps/earth run build

# ── Go build ────────────────────────────────────────────────────────────────
# Runs on $BUILDPLATFORM (no emulation); xx supplies the target-arch C toolchain.
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.25-alpine AS builder
COPY --from=xx / /
RUN apk add --no-cache clang lld
ARG TARGETPLATFORM
# Target-arch cross C toolchain + libc (CGO).
RUN xx-apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=frontend /app/frontend/apps/earth/dist/ embedfs/frontend/apps/earth/dist/
# Cache the module and build caches across builds; xx-go cross-compiles with CGO,
# then xx-verify asserts the output is the right arch and statically linked.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 xx-go build -trimpath \
        -ldflags="-s -w -linkmode external -extldflags '-static'" \
        -o /community ./cmd/community && \
    xx-verify --static /community

# ── Export (for extracting binaries) ─────────────────────────────────────────
FROM scratch AS export
COPY --from=builder /community /


# ── Runtime ─────────────────────────────────────────────────────────────────
FROM docker.io/library/alpine:3.20
RUN apk --no-cache add ca-certificates && \
    adduser -D -u 10001 community && \
    mkdir -p /data && \
    chown -R 10001:10001 /data
COPY --from=builder /community /usr/local/bin/community

USER 10001:10001
EXPOSE 8090 9091

HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
  CMD wget -q --spider http://127.0.0.1:8090/healthz || exit 1

ENTRYPOINT ["community"]
# Default subcommand so `docker run <image>` serves out of the box. The config PATH
# is deployment-specific and belongs in the deployment (compose `command:` or a
# --config flag), NOT baked into a reusable image — e.g.
#   command: ["serve", "--config", "/etc/respondent/respondent.community.yaml"]
CMD ["serve"]

LABEL org.opencontainers.image.title="respondent-community" \
      org.opencontainers.image.description="Respondent Community Edition — real-time geospatial OSINT map" \
      org.opencontainers.image.source="https://github.com/alevsk/respondent-community" \
      org.opencontainers.image.licenses="MIT"

