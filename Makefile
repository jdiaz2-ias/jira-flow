GO ?= go
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || printf unknown)
GOVULNCHECK_VERSION := v1.7.0
GORELEASER_VERSION := v2.18.1
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: help fmt fmt-check lint test test-race test-foundation build build-all check release-check security
help:
	@printf '%s\n' 'make check: formato, vet, pruebas y dependencias' 'make build: bin/jflow' 'make build-all: cuatro plataformas sin CGO' 'make test-race: detector de carreras' 'make security: govulncheck fijado'

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

# F0 validates configuration/build readiness; publishing is a separate F6 task.
release-check: build-all
	@printf '%s\n' 'F0: builds verificados; empaquetado/publicación pendientes de F6.'

security:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) -tags=foundation ./...
