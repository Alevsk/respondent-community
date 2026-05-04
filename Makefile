# ═══════════════════════════════════════════════════════════════════════════════
# Respondent — Community Edition
# ═══════════════════════════════════════════════════════════════════════════════
# This repository distributes runtime configs (sources.d, analysis.d), the
# developer documentation site, and the docker-compose manifest for the
# `respondent-community` image. The application image itself is built and
# published from the upstream engine repo — there are NO build targets for it
# here.
#
# Targets in this Makefile cover:
#   • Running / stopping the application stack (docker compose)
#   • Hugo developer documentation: dev server, local build, container build
#   • Multi-arch build & push of the documentation image
#
# Usage:
#   make help                        # list all targets
#   make up                          # start the application stack
#   make docs-dev                    # serve the docs locally with live reload
#   make docs-push                   # multi-arch build & push docs image
# ═══════════════════════════════════════════════════════════════════════════════

CONTAINER_RUNTIME ?= docker
REGISTRY          ?= docker.io/alevsk
DOCS_IMAGE        ?= $(REGISTRY)/respondent-community-docs
DOCS_TAG          ?= latest
BUILDX_BUILDER    ?= respondent-community-builder
BUILDX_PLATFORMS  ?= linux/amd64,linux/arm64

DOCS_DIR          := developer-documentation
DOCS_COMPOSE      := $(DOCS_DIR)/compose.yaml
DOCS_COMPOSE_DEV  := $(DOCS_DIR)/compose.dev.yaml
DOCS_DOCKERFILE   := $(DOCS_DIR)/Dockerfile

.DEFAULT_GOAL := help

# ═══════════════════════════════════════════════════════════════════════════════
# HELP
# ═══════════════════════════════════════════════════════════════════════════════

.PHONY: help
help: ## List available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Available targets:\n\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ═══════════════════════════════════════════════════════════════════════════════
# APPLICATION STACK — `respondent-community` image (pulled, not built here)
# ═══════════════════════════════════════════════════════════════════════════════

.PHONY: up down restart logs ps pull
up: ## Start the application stack (docker compose up -d)
	docker compose up -d

down: ## Stop the application stack
	docker compose down

restart: ## Restart the application stack
	docker compose restart

logs: ## Tail application logs
	docker compose logs -f

ps: ## Show stack status
	docker compose ps

pull: ## Pull the latest application image
	docker compose pull

# ═══════════════════════════════════════════════════════════════════════════════
# DOCS — local Hugo (no Docker)
# ═══════════════════════════════════════════════════════════════════════════════

.PHONY: docs-dev docs-build docs-clean
docs-dev: ## Run Hugo dev server with live reload (requires hugo installed locally)
	$(MAKE) -C $(DOCS_DIR) serve

docs-build: ## Build the docs site locally (writes $(DOCS_DIR)/public/)
	$(MAKE) -C $(DOCS_DIR) build

docs-clean: ## Remove generated Hugo output
	$(MAKE) -C $(DOCS_DIR) clean

# ═══════════════════════════════════════════════════════════════════════════════
# DOCS — Docker compose (containerized preview)
# ═══════════════════════════════════════════════════════════════════════════════

.PHONY: docs-up docs-down docs-dev-up docs-dev-down docs-image
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

.PHONY: docs-buildx-setup docs-push
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
