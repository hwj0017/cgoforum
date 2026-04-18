.PHONY: infra-up infra-down backend-build backend-run backend-test frontend-install frontend-dev frontend-build

infra-up:
	docker compose up -d

infra-down:
	docker compose down

backend-build:
	cd backend && go build -o bin/server ./cmd/server

backend-run:
	cd backend && go run ./cmd/server

backend-test:
	cd backend && go test ./... -count=1

frontend-install:
	cd frontend && npm install

frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build
