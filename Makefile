GO ?= go

.PHONY: build test fmt vet

build:
	$(GO) build ./cmd/...

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

