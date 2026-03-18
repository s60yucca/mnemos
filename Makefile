BINARY := mnemos
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"
GO := go

.PHONY: build test lint clean release install

build:
	$(GO) build $(LDFLAGS) -o bin/$(BINARY) ./cmd/mnemos

test:
	$(GO) test ./... -v -count=1

test-short:
	$(GO) test ./... -short -count=1

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ dist/

release:
	goreleaser release --clean

release-snapshot:
	goreleaser release --snapshot --clean

install:
	$(GO) install $(LDFLAGS) ./cmd/mnemos
