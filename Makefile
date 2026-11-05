VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/reilandeubank/grower/internal/buildinfo.Version=$(VERSION)
# The one golangci-lint version: make lint runs it, and CI reads it (make golangci-lint-version).
# Bump by hand: Dependabot can't see it.
GOLANGCI_LINT_VERSION := v2.15.0

.PHONY: gen test lint golangci-lint-version build dev dev-api dev-web e2e docker clean web-install

web-install:
	npm --prefix web ci

gen:
	go generate ./...
	npm --prefix web run gen

test:
	go test ./...
	npm --prefix web test

lint:
	go vet ./...
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run
	npm --prefix web run lint
	npm --prefix web run typecheck

golangci-lint-version:
	@echo $(GOLANGCI_LINT_VERSION)

build:
	npm --prefix web run build
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/grower ./cmd/grower

# Local development: API on :8080, Vite on :5173 proxying /api to it.
# On first run, create your account on the setup page (or: go run ./cmd/grower user create --username you).
DEV_ENV := GROWER_DATA_DIR=./data GROWER_BASE_URL=http://localhost:5173 GROWER_LOG_FORMAT=text
dev-api:
	$(DEV_ENV) go run ./cmd/grower serve

dev-web:
	npm --prefix web run dev

dev:
	$(MAKE) -j2 dev-api dev-web

e2e: build
	npm --prefix web run e2e

docker:
	docker build --build-arg VERSION=$(VERSION) -t grower:dev .

clean:
	rm -rf bin web/dist/assets web/dist/*.html web/dist/*.js web/dist/*.webmanifest
