.PHONY: test test-backend test-media test-frontend build up down logs fmt vet

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
