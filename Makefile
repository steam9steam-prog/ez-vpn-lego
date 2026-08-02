GO ?= go

.PHONY: build test test-race fmt vet generate generate-check

build:
	$(GO) build ./cmd/...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

generate:
	sqlc generate

generate-check:
	sqlc diff
