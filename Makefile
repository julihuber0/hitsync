.PHONY: test test-backend test-frontend build up down logs fmt vet \
	dev-up dev-down dev-backend dev-frontend

test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && npm run test

fmt:
	cd backend && gofmt -l .

vet:
	cd backend && go vet ./...

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

dev-frontend:
	cd frontend && npm run dev
