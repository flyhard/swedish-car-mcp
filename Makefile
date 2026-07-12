.PHONY: build test lint tidy clean install-launchers

GO ?= go
BIN_DIR := bin
PREFIX ?= $(HOME)/.local

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/aviloo-mcp ./cmd/aviloo-mcp
	$(GO) build -o $(BIN_DIR)/bilmarknad-mcp ./cmd/bilmarknad-mcp

test:
	$(GO) test -race -cover ./...

lint:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)

install-launchers:
	SWEDISH_CAR_MCP_PREFIX="$(PREFIX)" ./scripts/install.sh --prefix "$(PREFIX)"
