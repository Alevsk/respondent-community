.PHONY: help \
	run build run-frontend run-frontend-build \
	docker-build docker-push \
	compose-up compose-down compose-restart compose-logs \
	test test-live lint fmt frontend-format frontend-lint frontend-typecheck frontend-test \
	all-format all-lint all-test ci-fmt ci-format \
	e2e e2e-frontend-build e2e-earth-test \
	perf-smoke \
	clean \
	docs-dev docs-build docs-clean \
	docs-up docs-down docs-dev-up docs-dev-down docs-image docs-buildx-setup docs-push

BIN := $(HOME)/go/bin

CONTAINER_RUNTIME ?= $(shell which docker 2>/dev/null || which podman 2>/dev/null)
COMPOSE_RUNTIME ?= $(shell if docker compose version >/dev/null 2>&1; then echo "docker compose"; elif which podman-compose 2>/dev/null; then echo "podman-compose"; fi)
IMAGE ?= alevsk/respondent-community

# Target architectures for container images. Multi-arch builds go through
# `docker buildx`; the Dockerfile cross-compiles each arch via tonistiigi/xx
# (no QEMU). Override for a faster single-arch local build, e.g.
#   make docker-build PLATFORMS=linux/arm64
PLATFORMS ?= linux/amd64,linux/arm64

ENV_FILE       := .env

# Static linking only works on Linux (musl). On macOS we build a dynamic binary.
LDFLAGS_LINUX   := -s -w -linkmode external -extldflags '-static'
LDFLAGS_DARWIN  := -s -w
LDFLAGS         := $(if $(filter $(shell go env GOOS),linux),$(LDFLAGS_LINUX),$(LDFLAGS_DARWIN))

EARTH_DIST := frontend/apps/earth/dist/index.html

# ─── Documentation image variables ──────────────────────────────────────────
REGISTRY          ?= docker.io/alevsk
DOCS_IMAGE        ?= $(REGISTRY)/respondent-community-docs
DOCS_TAG          ?= latest
BUILDX_BUILDER    ?= respondent-community-builder
BUILDX_PLATFORMS  ?= linux/amd64,linux/arm64

DOCS_DIR          := developer-documentation
DOCS_COMPOSE      := $(DOCS_DIR)/compose.yaml
DOCS_COMPOSE_DEV  := $(DOCS_DIR)/compose.dev.yaml
DOCS_DOCKERFILE   := $(DOCS_DIR)/Dockerfile

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ═══════════════════════════════════════════════════════════════════════════════
# CODE GENERATION
# ═══════════════════════════════════════════════════════════════════════════════

proto: ## Generate protobuf code with Buf
	$(BIN)/buf generate
	$(BIN)/sqlc generate

generate: proto sqlc ## Generate all code (proto + SQL)

run: run-frontend-build build ## Build frontend, embed, and run the community server
	set -a; [ -f $(ENV_FILE) ] && . ./$(ENV_FILE); set +a; \
	./bin/community serve --config respondent.yaml

run-frontend: ## Start Vite dev server for earth app (proxies to backend)
	cd frontend && set -a; [ -f ../$(ENV_FILE) ] && . ../$(ENV_FILE); set +a; \
	npx vite --host -w @respondent/earth

run-frontend-build: ## Install deps and build the earth frontend for production
	cd frontend && npm ci
	set -a; [ -f $(ENV_FILE) ] && . ./$(ENV_FILE); set +a; \
	cd frontend/apps/earth && npx vite build

# ═══════════════════════════════════════════════════════════════════════════════
# BUILD (native Go — runs on host, for local development)
# ═══════════════════════════════════════════════════════════════════════════════


build: ## Build the community binary (with embedded frontend)
	@mkdir -p embedfs/frontend/apps/earth/dist
	@if [ ! -f $(EARTH_DIST) ]; then \
		echo "WARNING: frontend/apps/earth/dist not found, embedding placeholder"; \
		echo '<!DOCTYPE html><html><body>Run: cd frontend/apps/earth && npx vite build</body></html>' > embedfs/frontend/apps/earth/dist/index.html; \
	else \
		cp -r frontend/apps/earth/dist/* embedfs/frontend/apps/earth/dist/; \
	fi
	CGO_ENABLED=1 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/community ./cmd/community

build-binaries: ## Build multi-arch binaries (x86-64, arm64) using Docker and extract to bin/release/
	$(CONTAINER_RUNTIME) buildx build --platform $(PLATFORMS) --target export --output type=local,dest=bin/release .

docker-build: ## Cross-build the multi-arch image ($(PLATFORMS)) and load it into the local image store
	$(CONTAINER_RUNTIME) buildx build --platform $(PLATFORMS) -t $(IMAGE):latest  .

docker-push: ## Cross-build the multi-arch image ($(PLATFORMS)) and push the manifest list to the registry
	$(CONTAINER_RUNTIME) buildx build --platform $(PLATFORMS) -t $(IMAGE):latest --push .

compose-up: ## Start community server via compose
	$(COMPOSE_RUNTIME) -f compose.yaml up -d

compose-down: ## Stop compose stack
	$(COMPOSE_RUNTIME) -f compose.yaml down

compose-restart: ## Restart compose stack
	$(COMPOSE_RUNTIME) -f compose.yaml restart

compose-logs: ## Tail compose logs
	$(COMPOSE_RUNTIME) -f compose.yaml logs -f

# ═══════════════════════════════════════════════════════════════════════════════
# QUALITY: format / lint / test
# ═══════════════════════════════════════════════════════════════════════════════

fmt: ## Format Go code (excludes gen/ + vendor/)
	@PKGS=$$(go list ./... | grep -v '/gen/' | grep -v '/vendor/'); \
	go fmt $$PKGS
	@FILES=$$(find . -type f -name '*.go' -not -path './gen/*' -not -path './vendor/*' -not -path './node_modules/*'); \
	goimports -l -w $$FILES

lint: ## Lint Go (excludes generated gen/)
	golangci-lint run ./...

test: ## Run Go tests with race + coverage (excludes generated gen/)
	@PKGS=$$(go list ./... | grep -v '/gen/'); \
	go test -race -cover $$PKGS

test-live: ## Run opt-in live-network Go tests (build tag "e2e"; needs real creds in .env)
	go test -tags e2e $$(go list ./... | grep -v '/gen/')

frontend-format: ## Format frontend (Prettier --write)
	cd frontend && npm run format

frontend-lint: ## Lint frontend (ESLint)
	cd frontend && npm run lint

frontend-typecheck: ## Type-check the earth app (tsc --noEmit)
	cd frontend/apps/earth && npm run typecheck

frontend-test: ## Run frontend tests (Vitest, single run)
	cd frontend && npm run test -- --run

all-format: fmt frontend-format ## Format everything (Go + frontend)

all-lint: lint frontend-lint frontend-typecheck ## Lint + typecheck everything (Go + frontend)

all-test: test frontend-test ## Test everything (Go + frontend)

ci-fmt: ## Check Go formatting (fails if unformatted)
	@UNFORMATTED=$$(gofmt -l . 2>/dev/null | grep -v '^vendor/' | grep -v '^gen/'); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "Go files not formatted:"; echo "$$UNFORMATTED"; exit 1; \
	fi

ci-format: ## Check frontend formatting (Prettier --check)
	cd frontend && npm run format:check

clean: ## Remove build artifacts
	rm -rf bin/ embedfs/frontend/apps/earth/dist

# ═══════════════════════════════════════════════════════════════════════════════
# E2E — Earth frontend (Playwright)
# ═══════════════════════════════════════════════════════════════════════════════

E2E_DIR      := .e2e
E2E_DB       := $(E2E_DIR)/respondent-e2e.db
E2E_PORT     := 8091
E2E_BASE_URL := http://localhost:$(E2E_PORT)

# Build the frontend with VITE_API_URL="" so API calls use relative paths
# (same origin as the embedded Go server). Without this, the production bundle
# hardcodes http://localhost:8090 and breaks when the server runs on any other port.
e2e-frontend-build: ## Build earth frontend for e2e: relative API URLs (VITE_API_URL="")
	cd frontend && npm ci
	set -a; [ -f $(ENV_FILE) ] && . ./$(ENV_FILE); set +a; \
	cd frontend/apps/earth && VITE_API_URL="" npx vite build

e2e-earth-test: ## Run earth Playwright specs against E2E_BASE_URL (default: http://localhost:8090)
	cd frontend/apps/earth && npm run e2e

e2e: e2e-frontend-build build ## Boot isolated community server (port 8091, .e2e/ DB) → earth e2e → teardown
	@rm -rf $(E2E_DIR) && mkdir -p $(E2E_DIR); \
	set -e; \
	set -a; [ -f $(ENV_FILE) ] && . ./$(ENV_FILE); set +a; \
	RESPONDENT_DATABASE_PATH=$(E2E_DB) RESPONDENT_SERVER_PORT=$(E2E_PORT) \
	  RESPONDENT_INGEST_ENABLED=false \
	  ./bin/community serve --config respondent.yaml & SRV=$$!; \
	trap 'kill $$SRV 2>/dev/null || true; rm -rf $(E2E_DIR)' EXIT INT TERM; \
	E2E_BASE_URL=$(E2E_BASE_URL) $(MAKE) e2e-earth-test

# ═══════════════════════════════════════════════════════════════════════════════
# PERF-SMOKE — Report-only FPS baseline (local vs prod)
# ═══════════════════════════════════════════════════════════════════════════════

perf-smoke: ## Report-only FPS comparison: local vs prod (all layers on). NOT a CI gate.
	@echo "── LOCAL (http://localhost:8090) ──"
	node tools/perf-smoke.mjs http://localhost:8090 || true
	@echo "── PROD (https://respondent.alevsk.dev) ──"
	node tools/perf-smoke.mjs https://respondent.alevsk.dev || true

# ═══════════════════════════════════════════════════════════════════════════════
# DOCS — local Hugo (no Docker)
# ═══════════════════════════════════════════════════════════════════════════════

docs-dev: ## Run Hugo dev server with live reload (requires hugo installed locally)
	$(MAKE) -C $(DOCS_DIR) serve

docs-build: ## Build the docs site locally (writes $(DOCS_DIR)/public/)
	$(MAKE) -C $(DOCS_DIR) build

docs-clean: ## Remove generated Hugo output
	$(MAKE) -C $(DOCS_DIR) clean

# ═══════════════════════════════════════════════════════════════════════════════
# DOCS — Docker compose (containerized preview)
# ═══════════════════════════════════════════════════════════════════════════════

docs-up: ## Start docs (prod, nginx — pulls $(DOCS_IMAGE):$(DOCS_TAG))
	docker compose -f $(DOCS_COMPOSE) up -d

docs-down: ## Stop docs (prod)
	docker compose -f $(DOCS_COMPOSE) down

docs-dev-up: ## Start docs in dev mode (Hugo live reload, port 1313)
	docker compose -f $(DOCS_COMPOSE_DEV) up -d

docs-dev-down: ## Stop docs (dev)
	docker compose -f $(DOCS_COMPOSE_DEV) down

docs-image: ## Build the docs image for the local architecture only
	$(CONTAINER_RUNTIME) build -t $(DOCS_IMAGE):$(DOCS_TAG) -f $(DOCS_DOCKERFILE) $(DOCS_DIR)

# ═══════════════════════════════════════════════════════════════════════════════
# DOCS — multi-arch build & push (linux/amd64 + linux/arm64)
# ═══════════════════════════════════════════════════════════════════════════════
# One-time setup:
#   docker buildx create --name $(BUILDX_BUILDER) --use
#   docker login $(REGISTRY)

docs-buildx-setup: ## Create or refresh the buildx builder used for multi-arch pushes
	@$(CONTAINER_RUNTIME) buildx inspect $(BUILDX_BUILDER) >/dev/null 2>&1 \
		|| $(CONTAINER_RUNTIME) buildx create --name $(BUILDX_BUILDER) --use
	@$(CONTAINER_RUNTIME) buildx inspect --bootstrap $(BUILDX_BUILDER) >/dev/null

docs-push: docs-buildx-setup ## Build & push docs image (multi-arch, requires `docker login`)
	$(CONTAINER_RUNTIME) buildx build --builder $(BUILDX_BUILDER) \
		--platform $(BUILDX_PLATFORMS) \
		-t $(DOCS_IMAGE):$(DOCS_TAG) \
		-f $(DOCS_DOCKERFILE) \
		--push $(DOCS_DIR)

.DEFAULT_GOAL := help
