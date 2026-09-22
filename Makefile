GO ?= go
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
GOVULNCHECK_VERSION := v1.7.0
GORELEASER_VERSION := v2.18.1
GORELEASER ?= .tools/goreleaser
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: help fmt fmt-check lint test test-race test-foundation build build-all check release-tool release-assets release-check security
help:
	@printf '%s\n' 'make check: formato, vet, pruebas y dependencias' 'make build: bin/jflow' 'make build-all: cuatro plataformas sin CGO' 'make test-race: detector de carreras' 'make security: govulncheck fijado'
	@printf '%s\n' 'make release-check: paquetes candidatos, checksums y smoke nativo; no publica' 'make release-assets: ayuda, completions y avisos de dependencias'

fmt:
	$(GO) fmt ./...
	gofmt -w internal/foundation

fmt-check:
	@test -z "$$(gofmt -l cmd internal tests)" || { gofmt -l cmd internal tests; exit 1; }

lint:
	$(GO) vet ./...
	$(GO) vet -tags=foundation ./internal/foundation

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-foundation:
	$(GO) test -tags=foundation ./internal/foundation

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/jflow ./cmd/jflow

build-all:
	@set -eu; for target_os in linux darwin; do \
		for target_arch in amd64 arm64; do \
			printf '%s\n' "Building $$target_os/$$target_arch"; \
			CGO_ENABLED=0 GOOS=$$target_os GOARCH=$$target_arch $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/$$target_os-$$target_arch/jflow ./cmd/jflow; \
			CGO_ENABLED=0 GOOS=$$target_os GOARCH=$$target_arch $(GO) build -tags=foundation ./internal/foundation; \
		done; \
	done

check: fmt-check lint test test-foundation

release-assets: build
	python3 scripts/release-assets.py ./bin/jflow

release-tool:
	sh scripts/install-goreleaser.sh $(GORELEASER_VERSION)

.tools/goreleaser:
	$(MAKE) release-tool

# Development packages only: never publishes or creates a tag.
release-check: $(GORELEASER)
	$(GORELEASER) check
	$(GORELEASER) release --snapshot --clean
	python3 scripts/verify-packages.py

security:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) -tags=foundation ./...
