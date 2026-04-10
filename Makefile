.DEFAULT_GOAL := help

.PHONY: help setup dev dev-local up down clean test \
        test-go test-ts test-e2e check \
        build build-backend build-frontend \
        daemon cli \
        db-up db-down migrate-up migrate-down sqlc \
        worktree-env setup-worktree dev-worktree \
        logs

# ---------------------------------------------------------------------------
# Variables
# ---------------------------------------------------------------------------

MAIN_ENV_FILE     ?= .env
WORKTREE_ENV_FILE ?= .env.worktree
ENV_FILE          ?= $(if $(wildcard $(MAIN_ENV_FILE)),$(MAIN_ENV_FILE),$(if $(wildcard $(WORKTREE_ENV_FILE)),$(WORKTREE_ENV_FILE),$(MAIN_ENV_FILE)))

ifneq ($(wildcard $(ENV_FILE)),)
include $(ENV_FILE)
endif

POSTGRES_DB       ?= multica
POSTGRES_USER     ?= multica
POSTGRES_PASSWORD ?= multica
POSTGRES_PORT     ?= 5432
PORT              ?= 8080
FRONTEND_PORT     ?= 3000
FRONTEND_ORIGIN   ?= http://localhost:$(FRONTEND_PORT)
MULTICA_APP_URL   ?= $(FRONTEND_ORIGIN)
DATABASE_URL      ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
NEXT_PUBLIC_API_URL ?= http://localhost:$(PORT)
NEXT_PUBLIC_WS_URL  ?= ws://localhost:$(PORT)/ws
GOOGLE_REDIRECT_URI ?= $(FRONTEND_ORIGIN)/auth/callback
MULTICA_SERVER_URL  ?= ws://localhost:$(PORT)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

export

MULTICA_ARGS ?= $(ARGS)

COMPOSE      := docker compose
DEV_COMPOSE  := $(COMPOSE) -f docker-compose.dev.yml
PROD_COMPOSE := $(COMPOSE) --env-file $(ENV_FILE) -f docker-compose.prod.yml

define REQUIRE_ENV
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Missing env file: $(ENV_FILE)"; \
		echo "Create .env from .env.example, or run 'make worktree-env'."; \
		exit 1; \
	fi
endef

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

help: ## Show available targets
	@echo "Usage: make <target>"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ---------------------------------------------------------------------------
# Lifecycle
# ---------------------------------------------------------------------------

setup: ## First-time setup: install deps, start DB, run migrations
	$(REQUIRE_ENV)
	@echo "==> Using env file: $(ENV_FILE)"
	@echo "==> Installing dependencies..."
	@pnpm install
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@echo "==> Running migrations..."
	@cd server && go run ./cmd/migrate up
	@echo ""
	@echo "✓ Setup complete! Run 'make dev' to start developing."

dev: ## Start local dev with hot-reload (Docker-only, no local toolchain needed)
	@$(DEV_COMPOSE) up --build; $(DEV_COMPOSE) down

dev-local: ## Start local dev with hot-reload (requires Go + Node)
	$(REQUIRE_ENV)
	@echo "Backend:  http://localhost:$(PORT)"
	@echo "Frontend: http://localhost:$(FRONTEND_PORT)"
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@trap 'kill 0' EXIT; \
		(cd server && go run ./cmd/server) & \
		pnpm dev:web & \
		wait

up: ## Start production-like services via Docker (no hot-reload)
	$(REQUIRE_ENV)
	@docker build -t multica-backend --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -f Dockerfile .
	@docker build -t multica-frontend -f apps/web/Dockerfile .
	@docker tag multica-backend ghcr.io/nullne/multica/backend:latest
	@docker tag multica-frontend ghcr.io/nullne/multica/frontend:latest
	@$(PROD_COMPOSE) up -d
	@echo ""
	@echo "✓ Production stack running at http://localhost:$${LISTEN_PORT:-80}"

down: ## Stop all services (dev and prod)
	@-$(DEV_COMPOSE) down 2>/dev/null
	@-$(PROD_COMPOSE) down 2>/dev/null
	@-lsof -ti:$(PORT) | xargs kill -9 2>/dev/null
	@-lsof -ti:$(FRONTEND_PORT) | xargs kill -9 2>/dev/null
	@echo "✓ All services stopped."

clean: ## Stop services and destroy ALL local state (volumes, caches, build artifacts)
	@-$(DEV_COMPOSE) down -v 2>/dev/null
	@-$(PROD_COMPOSE) down -v 2>/dev/null
	@-$(COMPOSE) down -v 2>/dev/null
	@-lsof -ti:$(PORT) | xargs kill -9 2>/dev/null
	@-lsof -ti:$(FRONTEND_PORT) | xargs kill -9 2>/dev/null
	@rm -rf server/bin server/tmp
	@rm -rf apps/web/.next apps/web/node_modules/.cache
	@echo "✓ All state destroyed. Run 'make setup && make dev' for a fresh start."

logs: ## Stream production service logs
	@$(PROD_COMPOSE) logs -f

# ---------------------------------------------------------------------------
# Testing
# ---------------------------------------------------------------------------

test: ## Run full test suite (typecheck + TS + Go + E2E)
	$(REQUIRE_ENV)
	@ENV_FILE="$(ENV_FILE)" bash scripts/check.sh

test-go: ## Run Go tests only
	$(REQUIRE_ENV)
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@cd server && go test ./...

test-ts: ## Run TypeScript tests only (Vitest)
	@pnpm test

test-e2e: ## Run E2E tests only (requires backend + frontend running)
	@pnpm exec playwright test

check: ## Run typecheck only (no tests)
	@pnpm typecheck

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

build: build-backend build-frontend ## Build all services

build-backend: ## Build Go server and CLI binaries to server/bin/
	@cd server && go build -o bin/server ./cmd/server
	@cd server && go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" -o bin/multica ./cmd/multica

build-frontend: ## Build frontend Docker image
	@docker build -t multica-frontend -f apps/web/Dockerfile .

# ---------------------------------------------------------------------------
# CLI & Daemon
# ---------------------------------------------------------------------------

daemon: ## Start local agent daemon
	@$(MAKE) --no-print-directory _multica MULTICA_ARGS="daemon"

cli: ## Run multica CLI (usage: make cli ARGS="...")
	@$(MAKE) --no-print-directory _multica MULTICA_ARGS="$(MULTICA_ARGS)"

_multica:
	@cd server && go run ./cmd/multica $(MULTICA_ARGS)

# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

db-up: ## Start PostgreSQL container
	@$(COMPOSE) up -d postgres

db-down: ## Stop PostgreSQL container
	@$(COMPOSE) down

migrate-up: ## Run database migrations
	$(REQUIRE_ENV)
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@cd server && go run ./cmd/migrate up

migrate-down: ## Rollback database migrations
	$(REQUIRE_ENV)
	@bash scripts/ensure-postgres.sh "$(ENV_FILE)"
	@cd server && go run ./cmd/migrate down

sqlc: ## Regenerate sqlc Go code from SQL queries
	@cd server && sqlc generate

# ---------------------------------------------------------------------------
# Worktree (multi-branch parallel development)
# ---------------------------------------------------------------------------

worktree-env: ## Generate .env.worktree with unique DB/ports for this worktree
	@bash scripts/init-worktree-env.sh .env.worktree

setup-worktree: ## Setup worktree: generate env, install deps, DB, migrations
	@echo "==> Generating $(WORKTREE_ENV_FILE) with unique ports..."
	@FORCE=1 bash scripts/init-worktree-env.sh $(WORKTREE_ENV_FILE)
	@$(MAKE) --no-print-directory setup ENV_FILE=$(WORKTREE_ENV_FILE)

dev-worktree: ## Start dev using worktree config
	@$(MAKE) --no-print-directory dev-local ENV_FILE=$(WORKTREE_ENV_FILE)
