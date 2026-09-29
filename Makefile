GO ?= go
GO_TOOLCHAIN ?= go1.25.0
GO_CACHE ?= /tmp/xlsx-viewer-go-cache

.PHONY: audit build dev go-build go-test test verify web-build web-install web-test

web-install:
	pnpm install --frozen-lockfile

web-test:
	pnpm --dir web run check:oss
	pnpm --dir web run typecheck

web-build:
	pnpm --dir web run build

go-test:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./...

go-build:
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) build -o bin/xlsx-viewer ./cmd/xlsx-viewer

test: web-test go-test

verify: web-build
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test ./...
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) test -race ./...
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) vet ./...
	python3 test_server.py

build: web-build go-build

audit:
	pnpm audit --prod

dev:
	@test -n "$(FILE)" || (echo "Usage: make dev FILE=/absolute/path/to/book.xlsx" && exit 2)
	GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE) $(GO) run ./cmd/xlsx-viewer -file "$(FILE)"
