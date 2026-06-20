.PHONY: up down cli serve seed demo web test lint

# Boot the infrastructure: MariaDB + OpenFGA (and Playground) via Docker, then
# block until OpenFGA answers its health endpoint. No Go required.
up:
	docker compose up -d
	@echo "Waiting for OpenFGA healthcheck..."
	@until curl -sf http://localhost:8080/healthz > /dev/null 2>&1; do sleep 1; done
	@echo "OpenFGA is ready. Playground: http://localhost:3000/playground"

# Stop all containers and remove their volumes (wipes the OpenFGA datastore).
down:
	docker compose down -v

# Run the narrated 8-chapter CLI walkthrough (Check/Write/ListObjects/Expand).
cli:
	go run ./cmd/cli

# Seed demo data and serve the document-management HTTP API on :8000 (same
# process, so the in-memory store stays populated).
serve:
	go run ./cmd/server -seed

# Seed demo data only, then exit (no server). Mainly for scripted setups.
seed:
	go run ./cmd/server -seed -exit

# Bring up infra, then run the automated curl walkthrough of the HTTP API.
demo: up
	@sleep 2
	./scripts/demo.sh

# Serve the zero-install browser demo (narrated walkthrough + Swagger UI) on
# :8090. Needs python3 only; run 'make up' first so OpenFGA is on :8080.
web:
	@echo "Browser demo:   http://localhost:8090/"
	@echo "Swagger UI:      http://localhost:8090/swagger.html"
	@echo "(Run 'make up' first so OpenFGA is listening on :8080. Ctrl-C to stop.)"
	cd web && python3 -m http.server 8090

# Run the Go test suite.
test:
	go test ./...

# Static analysis / vet over all Go packages.
lint:
	go vet ./...
