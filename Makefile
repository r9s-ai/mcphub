SHELL := /bin/sh

GATEWAY_ADDR ?= :3080
ADMIN_DIR := web/admin
COMPOSE_FILE := deploy/docker-compose.dev.yml
DATABASE_URL ?= postgres://mcphub:mcphub@127.0.0.1:15432/mcphub?sslmode=disable
REDIS_URL ?= redis://127.0.0.1:16379/0

.PHONY: help install test vet build build-admin gateway connect-daemon admin dev infra-up infra-down infra-logs migrate clean

help:
	@printf '%s\n' \
		'make install      Install Admin dependencies' \
		'make test         Run Go tests' \
		'make vet          Run go vet' \
		'make build        Build Gateway and mcp-connect binaries' \
		'make build-admin  Build the Admin frontend' \
		'make gateway      Start the MCP Gateway' \
		'make connect-daemon Start the mcp-connect daemon' \
		'make infra-up       Start PostgreSQL and Redis' \
		'make infra-down     Stop PostgreSQL and Redis' \
		'make migrate        Run PostgreSQL migrations' \
		'make admin        Start the Admin frontend' \
		'make dev          Start Gateway and Admin together'

install:
	cd $(ADMIN_DIR) && pnpm install --ignore-scripts

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/mcp-gateway ./cmd/mcp-connect

build-admin:
	cd $(ADMIN_DIR) && pnpm build

gateway:
	go run ./cmd/mcp-gateway --addr $(GATEWAY_ADDR)

infra-up:
	docker compose -f $(COMPOSE_FILE) up -d

infra-down:
	docker compose -f $(COMPOSE_FILE) down

infra-logs:
	docker compose -f $(COMPOSE_FILE) logs -f postgres redis

migrate:
	MCP_STORAGE=postgres DATABASE_URL=$(DATABASE_URL) go run ./cmd/mcp-gateway --migrate-only

connect-daemon:
	go run ./cmd/mcp-connect daemon

admin:
	cd $(ADMIN_DIR) && pnpm dev

dev: install infra-up
	@set -eu; \
	trap 'kill 0 2>/dev/null || true' INT TERM EXIT; \
	MCP_STORAGE=postgres DATABASE_URL=$(DATABASE_URL) REDIS_URL=$(REDIS_URL) go run ./cmd/mcp-gateway --addr $(GATEWAY_ADDR) & \
	GATEWAY_PID=$$!; \
	(cd $(ADMIN_DIR) && pnpm dev) & \
	ADMIN_PID=$$!; \
	wait $$GATEWAY_PID $$ADMIN_PID

clean:
	rm -rf $(ADMIN_DIR)/dist $(ADMIN_DIR)/.vite
