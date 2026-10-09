SHELL := /bin/bash
export PATH := /usr/local/go/bin:$(HOME)/.nvm/versions/node/v24.19.0/bin:$(HOME)/go/bin:$(PATH)

.PHONY: all dev test lint migrate gen eval simulate build deploy smoke-test clean help

all: build test

## Help
help:
	@echo "Triage Monorepo Commands:"
	@echo "  make dev        - Start local environment with docker-compose"
	@echo "  make test       - Run all Go and Web test suites"
	@echo "  make lint       - Run linters and typecheckers"
	@echo "  make migrate    - Run database migrations (Phase 2+)"
	@echo "  make gen        - Generate OpenAPI contracts, sqlc types, and SvelteKit types"
	@echo "  make eval       - Run evaluation CLI benchmark"
	@echo "  make simulate   - Trigger fault-injection scenario simulator"
	@echo "  make build      - Build Go binaries and SvelteKit web application"
	@echo "  make deploy     - Deploy stack with zero-downtime rolling update"
	@echo "  make smoke-test - Run automated post-deploy smoke tests"
	@echo "  make clean      - Clean build artifacts"

## Development
dev:
	docker compose up --build

## Testing
test: test-api test-web

test-api:
	@echo "==> Running Go test suite..."
	cd apps/api && go test -v -race -cover ./...

test-web:
	@echo "==> Running Web unit tests..."
	cd apps/web && npm run test:unit -- --run

## Linting
lint: lint-api lint-web

lint-api:
	@echo "==> Running Go vet..."
	cd apps/api && go vet ./...

lint-web:
	@echo "==> Running Web typecheck..."
	cd apps/web && npm run check

## Database Migration & Seeding
migrate:
	@echo "==> Running database migrations..."
	cd apps/api && go run ./cmd/seed

seed:
	@echo "==> Seeding database with demo tenants and scenario data..."
	cd apps/api && go run ./cmd/seed

## Code Generation
gen:
	@echo "==> Generating sqlc database models and SvelteKit contracts..."
	cd apps/api && sqlc generate
	cd apps/web && npm run prepare

## Evaluation Benchmarks
eval:
	@echo "==> Running Evaluation harness..."
	cd apps/api && go run ./cmd/eval --provider=mock

## Fault-Injection Simulation
simulate:
	@echo "==> Running Fault-injection simulator..."
	cd apps/api && go run ./cmd/simulate --scenario=rabbitmq-backlog

## Build
build: build-api build-web

build-api:
	@echo "==> Building Go binaries..."
	mkdir -p apps/api/bin
	cd apps/api && go build -o bin/api ./cmd/api
	cd apps/api && go build -o bin/worker ./cmd/worker
	cd apps/api && go build -o bin/eval ./cmd/eval
	cd apps/api && go build -o bin/simulate ./cmd/simulate
	cd apps/api && go build -o bin/mcp ./cmd/mcp
	cd apps/api && go build -o bin/triage ./cmd/triage
	cd apps/api && go build -o bin/seed ./cmd/seed

build-web:
	@echo "==> Building SvelteKit production bundle..."
	cd apps/web && npm run build

## Continuous Deployment
deploy:
	@echo "==> Deploying stack with zero downtime..."
	./deploy/scripts/deploy.sh $${IMAGE_TAG:-latest}

smoke-test:
	@echo "==> Running deployment smoke tests..."
	./deploy/scripts/smoke-test.sh $${TARGET_URL:-http://localhost}

## Clean
clean:
	rm -rf apps/api/bin apps/web/build apps/web/.svelte-kit
