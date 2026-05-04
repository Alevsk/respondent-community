.PHONY: serve build clean new-page lint test

PORT ?= 1313

serve: ## Run Hugo dev server with drafts
	hugo server --buildDrafts --port $(PORT) --bind 0.0.0.0

build: ## Build production site (minified)
	hugo --minify

clean: ## Remove generated output
	rm -rf public/ resources/_gen/

new-page: ## Create a new page (usage: make new-page SECTION=sources NAME=transports)
	hugo new $(SECTION)/$(NAME).md

lint: ## Validate templates and shortcodes
	hugo --minify
	@echo "Build succeeded - no broken shortcodes or template errors"

test: lint ## Alias for lint
