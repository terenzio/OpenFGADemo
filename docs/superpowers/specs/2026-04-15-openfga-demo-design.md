# OpenFGA ReBAC Demo — Design

**Date:** 2026-04-15
**Status:** Approved (brainstorming phase)

## Purpose

Build a teaching-oriented demo program that shows how to use [OpenFGA](https://openfga.dev) as a Relationship-Based Access Control (ReBAC) authorization backend. The demo models a Google Drive-style permission system and is delivered as two Go binaries: a scripted CLI walkthrough (to teach the concepts) and a minimal HTTP API (to show real integration into a service).

## Goals

- Make ReBAC concepts concrete: direct relations, role implication, parent inheritance, userset expansion (groups), and wildcards.
- Show how to drive OpenFGA from Go: writing models, writing tuples, `Check`, `ListObjects`, `Expand`.
- Provide a realistic integration example: an HTTP service that authorizes every request through OpenFGA.
- Be runnable end-to-end in one command (`make up && make cli`).

## Non-goals

- Production-grade auth (no JWT, no real login — `X-User-Id` header only).
- Persistent application data (documents live in an in-memory map).
- Exhaustive test coverage; tests exist to illustrate patterns, not to guarantee behavior.
- Performance tuning, multi-region concerns, or full observability.

## Architecture

```
┌─────────────────────────────────────────────────┐
│  Docker Compose                                 │
│  ┌──────────┐   ┌─────────────┐   ┌──────────┐  │
│  │ MariaDB  │◄──│  OpenFGA    │──►│Playground│  │
│  │  :3306   │   │  :8080      │   │  :3000   │  │
│  └──────────┘   └─────────────┘   └──────────┘  │
└─────────────────────────────────────────────────┘
              ▲                  ▲
              │                  │
       ┌──────┴──────┐    ┌──────┴──────┐
       │  cmd/cli    │    │ cmd/server  │
       │ (teaching   │    │ (REST API   │
       │  walkthrough)│   │  using FGA) │
       └─────────────┘    └─────────────┘
              │                  │
              └── internal/fga ──┘
```

- **MariaDB** — OpenFGA's datastore. OpenFGA's `mysql` datastore engine is wire-compatible with MariaDB.
- **OpenFGA server** — exposes gRPC + HTTP on `:8080`. Auto-migrates on startup.
- **OpenFGA Playground** — a web UI on `:3000` for interactively exploring the model, tuples, and queries. Huge teaching aid.
- **Go binaries** — both import `internal/fga` which wraps the OpenFGA SDK with project-specific helpers (`WriteModelFromFile`, `Check`, `ListObjects`, `WriteTuple`, `DeleteTuple`).

## The Authorization Model

Single source of truth: `models/basic/authorization-model-basic.fga`. Both binaries read and upload it at startup.

```dsl
model
  schema 1.1

type user

type organization
  relations
    define member: [user]

type folder
  relations
    define owner: [user]
    define parent: [folder]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent

type document
  relations
    define parent: [folder]
    define owner: [user]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

Concepts demonstrated:

| Concept | Where | Teaches |
|---|---|---|
| Direct relation | `owner: [user]` | Alice owns doc1 |
| Role implication | `viewer … or editor` | Editors are viewers; owners are editors |
| Parent inheritance (TupleToUserset) | `viewer from parent` | Permissions flow folder → subfolder → document |
| Userset (group expansion) | `organization#member` | "Everyone in Marketing can view" |
| Wildcard / public sharing | `user:*` | Public documents |

## Component: CLI teaching walkthrough (`cmd/cli`)

One binary, no arguments required (`go run ./cmd/cli`). Runs ordered chapters, each of which:

1. Prints a heading.
2. Writes a few tuples (narrated).
3. Runs `Check` / `ListObjects` queries.
4. Prints the result with a hand-authored "Because …" explanation.

Flags: `-pause` (pause between chapters for live demo), `-auto` (no pauses, default).

Chapters:

1. **Bootstrap** — create store, write model, print IDs so viewer can open the Playground.
2. **Direct permissions** — `alice` owns `document:roadmap`; Check alice (allowed via owner→editor→viewer implication) and `bob` (denied).
3. **Folder inheritance** — place document under `folder:product`; grant `charlie` editor on folder; Check charlie on document.
4. **Nested folders** — `folder:product` has parent `folder:company`; grant `diana` viewer on `folder:company`; Check diana on document (two-hop).
5. **Groups via usersets** — create `organization:acme` with members `eve`, `frank`; grant `organization:acme#member` viewer on folder; Check members; add `grace` to org; re-check without new folder tuple.
6. **Public sharing** — grant `user:*` viewer on a document; Check arbitrary user.
7. **Reverse queries** — `ListObjects(user=diana, relation=viewer, type=document)`.
8. **Expand** — print full resolution tree for one relation to make the graph visible.

Sample output format:

```
── Chapter 3: Folder inheritance ──────────────
Writing tuples:
  + document:roadmap#parent@folder:product
  + folder:product#editor@user:charlie

Q: Can charlie edit document:roadmap?
   Check(user:charlie, editor, document:roadmap) → ALLOWED
   Because: charlie is editor on folder:product,
            and document:roadmap's parent is folder:product,
            and 'editor from parent' grants it.
```

## Component: HTTP API server (`cmd/server`)

Minimal but slightly richer than bare-bones: folders, orgs, docs, middleware for authorization, typed errors.

### Layout

```
cmd/server/main.go            # wires config, store, fga client, router, graceful shutdown
internal/fga/client.go        # OpenFGA wrapper
internal/fga/bootstrap.go     # ensure store + write model from .fga file
internal/store/store.go       # in-memory docs + folders + orgs (map + RWMutex)
internal/auth/middleware.go   # X-User-Id → ctx
internal/auth/authorize.go    # Authorize(ctx, relation, type, id) helper
internal/httpapi/handlers.go  # route handlers
internal/httpapi/errors.go    # typed errors → HTTP status
```

### Endpoints

All require `X-User-Id` header. 401 if missing, 403 if FGA denies.

| Method | Path | Check | Behavior |
|---|---|---|---|
| `POST` | `/organizations/:id/members` | `owner` on org | add member tuple |
| `POST` | `/folders` | — | create folder; creator becomes owner |
| `GET`  | `/folders` | `ListObjects(viewer, folder)` | list folders caller can see |
| `POST` | `/documents` | `editor` on parent folder | create doc in folder |
| `GET`  | `/documents` | `ListObjects(viewer, document)` | list docs caller can see |
| `GET`  | `/documents/:id` | `viewer` on doc | read |
| `PUT`  | `/documents/:id` | `editor` on doc | update |
| `POST` | `/documents/:id/share` | `owner` on doc | grant relation |
| `DELETE` | `/documents/:id/share` | `owner` on doc | revoke relation |

### Authorization pattern

Two forms shown for teaching:

1. **Explicit form** (one handler) — inline `Check` so readers see the shape.
2. **Helper form** (everywhere else) — `auth.Authorize(ctx, relation, objType, id)` returns a typed error that the error handler maps to 403.

### Write-path consistency

When creating a document: insert into store, then write `owner@user:<creator>` and `parent@folder:<folderID>` tuples. If the FGA write fails, roll back the store insert. Document prominently: "a real system would use an outbox or saga; we take the simple path for clarity."

### Dependencies

- Router: `github.com/go-chi/chi/v5`
- Logging: stdlib `log/slog`
- OpenFGA: `github.com/openfga/go-sdk`
- Config: env vars with defaults (`FGA_API_URL`, `FGA_STORE_ID`, `SERVER_ADDR`)

### Seed

`go run ./cmd/server -seed -exit` populates store + FGA with the CLI's cast of characters, enabling immediate `curl` exploration.

## Repository layout

```
OpenFGADemo/
├── README.md                        # pitch, diagram, quickstart, curl flows
├── docker-compose.yml               # mariadb + openfga + openfga-playground
├── go.mod / go.sum
├── Makefile                         # up, down, cli, serve, seed, demo, test
├── model/
│   └── authorization-model-basic.fga
├── cmd/
│   ├── cli/main.go
│   └── server/main.go
├── internal/
│   ├── fga/{client.go, bootstrap.go, client_test.go}
│   ├── store/{store.go, store_test.go}
│   ├── auth/{middleware.go, authorize.go}
│   └── httpapi/{handlers.go, errors.go, handlers_test.go}
├── scripts/
│   └── demo.sh                      # curl flow exercising the HTTP API
└── docs/
    ├── model-explained.md
    └── architecture.md
```

## Testing strategy

Pragmatic; this is a teaching demo, not a production service.

- **`internal/store`** — plain unit tests.
- **`internal/fga`** — test wrapper against `httptest` stub mimicking OpenFGA's Check/Write responses. Fast; no Docker in CI.
- **`internal/httpapi`** — handler tests with `httptest.NewRecorder` and a fake `FGAClient` interface. Verify correct relation checked on correct object, 403 on deny, 200 on allow, write-path rollback on FGA failure.
- **`cmd/cli`** — one smoke test that runs against real OpenFGA, gated by `FGA_INTEGRATION=1` (skipped in default CI).
- **No end-to-end server integration test** — `scripts/demo.sh` is the manual substitute.

## Makefile

```
make up      # docker compose up -d; wait for /healthz
make down    # docker compose down -v
make cli     # go run ./cmd/cli
make serve   # go run ./cmd/server
make seed    # go run ./cmd/server -seed -exit
make demo    # scripts/demo.sh
make test    # go test ./...
```

## Versions

- Go 1.22+
- `github.com/openfga/go-sdk` latest
- `github.com/go-chi/chi/v5` latest
- OpenFGA server: latest stable image
- MariaDB: 11.x

## Open questions / explicit trade-offs

- **In-memory store vs SQLite:** chose in-memory for simplicity. The FGA integration point is the same either way.
- **Single model vs per-binary model:** chose a single `.fga` file loaded by both binaries. Keeps the demo coherent.
- **`chi` vs stdlib router:** `chi` is idiomatic enough in the Go ecosystem that readers will recognize the pattern; stdlib `http.ServeMux` would work but is less expressive for path params.
