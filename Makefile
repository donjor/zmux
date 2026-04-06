VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: build test test-integration lint install clean

build:
	go build $(LDFLAGS) -o zmux ./cmd/zmux/

test:
	go test ./...

test-integration:
	go test -tags integration ./tests/...

lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

install: build
	rm -f ~/.local/bin/zmux 2>/dev/null || true
	cp zmux ~/.local/bin/zmux

clean:
	rm -f zmux
