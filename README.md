# OpenFGA Demo

A hands-on Go application that teaches the five core OpenFGA API operations —
Check, Write, Delete, ListObjects, and Expand — through a realistic
document-management scenario. You get a working HTTP server, an interactive CLI
walkthrough, a seeded dataset of users with overlapping permissions, and an
authorization model that exercises every major ReBAC concept: direct relations,
role implication, TupleToUserset (inherited permissions), organization
membership, and wildcards.

---

## What You'll Learn

- **Check** — ask "can user:alice view document:roadmap?" and get a boolean
- **Write** — assert a relationship tuple such as `(user:bob, editor, document:design-doc)`
- **Delete** — revoke a tuple to instantly remove access
- **ListObjects** — ask "which documents can user:eve view?" without scanning every object
- **Expand** — inspect the full userset tree for a relation on a single object
- **TupleToUserset** — permissions that flow down a folder hierarchy via `editor from parent`
- **Userset references** — grant access to all members of an organization at once
- **Wildcards** — `user:*` makes a document public to anyone

---

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose
- `curl`
- `jq`

---

## Quick Start

### 1. Start the infrastructure

```bash
make up
```

Starts MariaDB and the OpenFGA server (with the visual Playground at
http://localhost:3000). Waits until OpenFGA is healthy before returning.

### 2. Run the interactive CLI walkthrough

```bash
make cli
```

Steps through Check, Write, Delete, ListObjects, and Expand with printed
explanations. Press Enter to advance through each step. No server needed.

### 3. Seed data, start the server, and call the API

```bash
# In terminal 1 — seed demo data then exit
make seed

# In terminal 1 — start the HTTP server on :8000
make serve

# In terminal 2 — list alice's documents
curl -s -H "X-User-Id: alice" http://localhost:8000/documents | jq .

# bob has no access to roadmap
curl -s -w "\nHTTP %{http_code}\n" -H "X-User-Id: bob" http://localhost:8000/documents/roadmap

# charlie is editor on folder:product, so can view roadmap
curl -s -H "X-User-Id: charlie" http://localhost:8000/documents/roadmap | jq .

# randomstranger can view public-memo (wildcard)
curl -s -H "X-User-Id: randomstranger" http://localhost:8000/documents/public-memo | jq .

# share design-doc with bob as editor
curl -s -X POST -H "X-User-Id: alice" -H "Content-Type: application/json" \
  -d '{"user":"user:bob","relation":"editor"}' \
  http://localhost:8000/documents/design-doc/share | jq .
```

Or run the full automated walkthrough:

```bash
make demo
```

### 4. Tear down

```bash
make down
```

Stops all containers and removes volumes.

---

## Project Structure

```
OpenFGADemo/
├── cmd/
│   ├── cli/          # Interactive teaching walkthrough binary
│   └── server/       # HTTP API server binary (seeds data, serves :8000)
├── docs/
│   ├── architecture.md       # System diagram and component descriptions
│   └── model-explained.md    # Annotated walkthrough of the FGA model
├── internal/
│   ├── auth/         # X-User-Id middleware and Authorize helper
│   ├── fga/          # OpenFGA Go SDK wrapper (Check, Write, ListObjects, Expand)
│   ├── httpapi/      # Chi HTTP handlers
│   └── store/        # Thread-safe in-memory document/folder/org store
├── model/
│   └── authorization-model.fga   # ReBAC model (DSL format)
├── scripts/
│   └── demo.sh       # Automated curl walkthrough of the HTTP API
├── docker-compose.yml            # MariaDB + OpenFGA + Playground
├── Makefile
└── go.mod
```

---

## Further Reading

- [OpenFGA documentation](https://openfga.dev/docs)
- [FGA modeling concepts](https://openfga.dev/docs/concepts)
- [OpenFGA Go SDK](https://github.com/openfga/go-sdk)
- [Zanzibar: Google's Consistent, Global Authorization System](https://research.google/pubs/pub48190/) — the paper that inspired ReBAC
- [Authorization model reference](docs/model-explained.md) — this repo's annotated model walkthrough
- [Architecture overview](docs/architecture.md) — component diagram and descriptions
