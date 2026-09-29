.PHONY: run watch build test test-cover test-integration tidy clean docker-build up down logs deps-up

IMAGE := hospital-middleware

# Use the same settings as the app (DB_PORT etc.) when running tests from make.
-include .env
export

## Development

run:
	go run ./cmd/api

# Live reload with air; uses an installed air or falls back to go run.
watch:
	@if command -v air > /dev/null 2>&1; then air; else go run github.com/air-verse/air@latest; fi

# Builds every command in cmd/ into bin/.
build:
	go build -o bin/ ./cmd/...

test:
	go test ./...

# Prints per-package coverage.
test-cover:
	go test -cover ./...

# Runs the repository contracts against the real database too. Start it
# first with "make deps-up".
test-integration:
	INTEGRATION=1 go test -count=1 ./...

tidy:
	go mod tidy

clean:
	rm -rf bin tmp

## Docker

docker-build:
	docker build -t $(IMAGE) .

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

# Start only the services the app needs (db), for
# "make run" or "make watch" on the host.
deps-up:
	docker compose up -d --wait db
