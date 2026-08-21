SHELL := /bin/sh

GATEWAY_ADDR ?= :3080
ADMIN_DIR := web/admin

.PHONY: help install test vet build build-admin gateway admin dev clean

help:
	@printf '%s\n' \
		'make install      Install Admin dependencies' \
		'make test         Run Go tests' \
		'make vet          Run go vet' \
		'make build        Build Gateway and mcp-connect binaries' \
		'make build-admin  Build the Admin frontend' \
		'make gateway      Start the MCP Gateway' \
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

admin:
	cd $(ADMIN_DIR) && pnpm dev

dev: install
	@set -eu; \
	trap 'kill 0 2>/dev/null || true' INT TERM EXIT; \
	go run ./cmd/mcp-gateway --addr $(GATEWAY_ADDR) & \
	GATEWAY_PID=$$!; \
	(cd $(ADMIN_DIR) && pnpm dev) & \
	ADMIN_PID=$$!; \
	wait $$GATEWAY_PID $$ADMIN_PID

clean:
	rm -rf $(ADMIN_DIR)/dist $(ADMIN_DIR)/.vite
