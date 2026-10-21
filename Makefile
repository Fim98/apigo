SHELL := /bin/sh
GO ?= go
MYGO ?= $(GO) tool mygo

.PHONY: help run dev build test test-race fmt fmt-check vet lint verify clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

run: ## Run the app with go run
	$(GO) run ./cmd/apigo

dev: ## Live-reload development app
	$(MYGO) dev

build: ## Package the app for this machine
	$(MYGO) build

test: ## Unit tests without cgo
	CGO_ENABLED=0 $(GO) test ./...

test-race: ## Unit tests with the race detector (cgo required by -race)
	CGO_ENABLED=1 $(GO) test -race ./...

fmt: ## Format Go and documentation
	$(GO) tool gofumpt -w .
	npx --no-install prettier --write "**/*.{md,yml,yaml,json,jsonc}"

fmt-check: ## Fail when formatting is stale
	@test -z "$$($(GO) tool gofumpt -l .)" || (echo "run 'make fmt'"; exit 1)
	npx --no-install prettier --check "**/*.{md,yml,yaml,json,jsonc}"

vet: ## Go vet plus MyGo's native UI lifetime checks
	$(GO) vet ./...
	$(MYGO) vet .

lint: fmt-check vet ## Formatting and vet checks

verify: ## Everything CI runs locally
	./scripts/verify.sh

clean: ## Remove build output
	rm -rf build dist .mygo coverage.out coverage.html
