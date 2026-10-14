VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/reilandeubank/grower/internal/buildinfo.Version=$(VERSION)

.PHONY: gen test lint build dev dev-api dev-web e2e docker clean web-install

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
	@command -v golangci-lint >/dev/null && golangci-lint run || echo "golangci-lint not installed; skipped (CI runs it)"
	npm --prefix web run lint
	npm --prefix web run typecheck

build:
	npm --prefix web run build
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/grower ./cmd/grower

# Local development: API on :8080, Vite on :5173 proxying /api to it.
# Create a user first: go run ./cmd/grower user create --username you
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
