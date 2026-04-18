# cgoforum

Monorepo layout after refactor:

- backend: Go API service and jobs
- frontend: Vite + React web client
- docker-compose.yaml: infrastructure dependencies

## Quick Start

1. Start dependencies

	make infra-up

2. Start backend

	make backend-run

3. Start frontend

	make frontend-install
	make frontend-dev

Optional: run backend + frontend in compose profile

	docker compose --profile app up -d --build

Frontend default URL: http://127.0.0.1:5173
Backend default URL: http://127.0.0.1:8080
