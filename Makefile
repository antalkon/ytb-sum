BINARY  := ytb-sum
PKG     := github.com/antalkon/ytb-sum
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/internal/cli.Version=$(VERSION)

.PHONY: build install test lint fmt snapshot clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/ytb-sum

install:
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/ytb-sum

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist
