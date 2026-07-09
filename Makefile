VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

.PHONY: build build-zzmux test test-race test-integration test-agent-surfaces vuln lint fmt hooks install install-zzmux clean keys-gen

build:
	go build $(LDFLAGS) -o zmux ./cmd/zmux/

# Edge build: identical binary, named zzmux. Lets you test changes via
# zzmux without overwriting the live ~/.local/bin/zmux you're running.
build-zzmux:
	go build $(LDFLAGS) -o zzmux ./cmd/zmux/

keys-gen:
	go run ./cmd/zmux keys gen

test:
	go test ./...

# Race-enabled unit run — mirrors the CI gate. Slower; keep `test` for fast loops.
test-race:
	go test -race ./...

# Integration tests exec the built ./zmux binary, so build it first.
test-integration: build
	go test -tags integration ./tests/...

test-agent-surfaces:
	go test ./internal/setup ./internal/cli ./internal/tabs
	cd pi-zmux && npm run typecheck && npm test
	./qa lint
	node skills/zmux/test/doctor.mjs

# Vulnerability scan. govulncheck@latest needs Go >= 1.25; the go toolchain
# directive auto-fetches it if the local Go is older.
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Mirrors CI. `golangci-lint run` includes the gofumpt formatting check.
# go vet kept explicit as a guard against linter config drift.
lint:
	go vet ./...
	golangci-lint run

# Auto-format with gofumpt (via golangci-lint v2 formatters).
fmt:
	golangci-lint fmt

# Install the versioned git hooks (points core.hooksPath at scripts/hooks).
# Run once per clone. The pre-push hook gates master pushes on `make lint` +
# test-race (feature/wip pushes skip; GATE_ALL=1 to force).
hooks:
	git config core.hooksPath scripts/hooks
	@echo "git hooks installed (core.hooksPath=scripts/hooks); bypass once with: git push --no-verify"

install: build
	rm -f ~/.local/bin/zmux 2>/dev/null || true
	cp zmux ~/.local/bin/zmux

install-zzmux: build-zzmux
	rm -f ~/.local/bin/zzmux 2>/dev/null || true
	cp zzmux ~/.local/bin/zzmux

clean:
	rm -f zmux zzmux
