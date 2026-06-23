# rocway Makefile

GO ?= go
WIRE ?= $(shell go env GOPATH)/bin/wire
SWAG ?= $(shell go env GOPATH)/bin/swag
BIN := bin

.PHONY: help tidy wire build run test vet fmt lint clean cli docker swagger install-hooks

help:
	@echo "make tidy          - go mod tidy"
	@echo "make wire         - regenerate wire_gen.go"
	@echo "make swagger      - generate swagger docs"
	@echo "make build        - build rocway and rocway-cli to bin/"
	@echo "make run          - run rocway locally"
	@echo "make cli          - build rocway-cli only"
	@echo "make test         - go test ./..."
	@echo "make vet          - go vet ./..."
	@echo "make fmt          - gofmt -w ."
	@echo "make docker       - build docker image"
	@echo "make install-hooks - install git hooks to .git/hooks/"

tidy:
	$(GO) mod tidy

wire:
	@if [ ! -x "$(WIRE)" ]; then $(GO) install github.com/google/wire/cmd/wire@latest; fi
	$(WIRE) ./internal/wire

swagger:
	@if [ ! -x "$(SWAG)" ]; then $(GO) install github.com/swaggo/swag/cmd/swag@latest; fi
	mkdir -p api/docs
	$(SWAG) init -g cmd/rocway/main.go -o api/docs

build: wire
	mkdir -p $(BIN)
	$(GO) build -o $(BIN)/rocway ./cmd/rocway
	$(GO) build -o $(BIN)/rocway-cli ./cmd/rocway-cli

cli: wire
	mkdir -p $(BIN)
	$(GO) build -o $(BIN)/rocway-cli ./cmd/rocway-cli

run: build
	./$(BIN)/rocway

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint: vet fmt

clean:
	rm -rf $(BIN)

docker:
	docker build -f build/package/Dockerfile -t rocway:latest .

install-hooks:
	@echo "Installing git hooks..."
	@chmod +x githooks/pre-commit githooks/commit-msg
	@ln -sf ../../githooks/pre-commit .git/hooks/pre-commit
	@ln -sf ../../githooks/commit-msg .git/hooks/commit-msg
	@echo "Git hooks installed successfully!"
