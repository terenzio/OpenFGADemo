.PHONY: up down cli serve seed demo test lint

up:
	docker compose up -d
	@echo "Waiting for OpenFGA healthcheck..."
	@until curl -sf http://localhost:8080/healthz > /dev/null 2>&1; do sleep 1; done
	@echo "OpenFGA is ready. Playground: http://localhost:3000"

down:
	docker compose down -v

cli:
	go run ./cmd/cli

serve:
	go run ./cmd/server

seed:
	go run ./cmd/server -seed -exit

demo: up
	@sleep 2
	./scripts/demo.sh

test:
	go test ./...

lint:
	go vet ./...
