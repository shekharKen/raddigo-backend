BINARY_NAME := raddigo
CMD_PATH    := ./cmd/server
BIN_DIR     := bin
COMPOSE     := docker compose

.DEFAULT_GOAL := help
.PHONY: help all build run dev test vet fmt tidy clean \
        up down logs ps db-shell

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | awk -F': ' '{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## all: Run vet, tests and build
all: vet test build

## build: Compile the server binary into ./bin
build:
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

## run: Run the server
run:
	go run $(CMD_PATH)

## dev: Run the server with live reload (Air)
dev:
	air

## test: Run tests with race detector and coverage
test:
	go test ./... -race -cover

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format all Go files
fmt:
	go fmt ./...

## tidy: Tidy go.mod / go.sum
tidy:
	go mod tidy

## clean: Remove build artifacts
clean:
	rm -rf $(BIN_DIR) coverage.out

## up: Build and start all services (app + db) in the background
up:
	$(COMPOSE) up -d --build

## down: Stop and remove all services
down:
	$(COMPOSE) down

## logs: Tail logs from all services
logs:
	$(COMPOSE) logs -f

## ps: List running services
ps:
	$(COMPOSE) ps

## db-shell: Open a psql shell in the db service
db-shell:
	$(COMPOSE) exec db psql -U postgres -d raddigo

