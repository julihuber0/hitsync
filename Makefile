.PHONY: test test-backend test-media test-frontend build up down logs fmt vet \
	dev-up dev-down dev-backend dev-media dev-frontend

test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-media:
	cd media && go build ./... && go vet ./...

test-frontend:
	cd frontend && npm run test

fmt:
	cd backend && gofmt -l .
	cd media && gofmt -l .

vet:
	cd backend && go vet ./...
	cd media && go vet ./...

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

# --- Local development (see docs/local-development.md) ---

dev-up:
	docker compose -f docker-compose.dev.yml up -d postgres

dev-down:
	docker compose -f docker-compose.dev.yml down

dev-backend:
	cd backend && set -a && . ../.env.dev && set +a && go run ./cmd/server

dev-media:
	cd media && set -a && . ../.env.dev && set +a && go run ./cmd/media

dev-frontend:
	cd frontend && npm run dev
