# OpenFGA Demo

A hands-on workshop for learning [OpenFGA](https://openfga.dev). OpenFGA is
Auth0's open-source version of Google Zanzibar. It does relationship-based
access control (ReBAC). This workshop has three authorization models. Each one
is a bit more advanced than the last. Together they teach you the language, how
to test, and a real AI-agent delegation pattern. A small Go HTTP server lets you
try the model through a tiny document-management API.

> **New to ReBAC? Want the theory behind each step?** See
> [concepts.md](concepts.md). It covers the RBAC/ReBAC/Zanzibar background, a
> deep look at every concept in the workshop, and the OpenFGA MCP server used to
> write the models. This README only covers setup and the demos.

---

## Prerequisites

Install these before the workshop begins.

| Tool | Required for | Install |
| --- | --- | --- |
| [OpenFGA CLI](https://openfga.dev/docs/getting-started/install-sdk) | Steps 1–3 (model authoring & tests) | `brew install openfga/tap/fga` |
| [Docker](https://docs.docker.com/get-docker/) + Compose | Step 4 (live server demo) | per platform |
| [Go 1.22+](https://go.dev/dl/) | [OPTIONAL] Step 4 (live server demo) | per platform |
| `curl`, `jq` | [OPTIONAL] Step 4 (HTTP examples) | per platform |

Verify your install:

```bash
fga version
docker --version
go version
```

---

## The Three Models

This repo has three authorization models. Each one builds on the one before.
Do them in order. Each model adds a new OpenFGA concept to the last one.

```
OpenFGADemo/
└── models/
    ├── basic/                       ← Step 1: the language
    │   └── authorization-model-basic.fga
    ├── mcp-guide/                   ← Step 2: can_* permissions
    │   ├── authorization-model-mcp-guide.fga
    │   ├── authorization-model.json
    │   ├── tests.fga.yaml
    │   └── README.md
    └── ai-agent/                    ← Step 3: AI-agent delegation
        ├── authorization-model-ai-agent.fga
        ├── tests.fga.yaml
        └── README.md
```

| Folder | What it teaches | Time |
| --- | --- | --- |
| [models/basic/](models/basic/) | DSL syntax, types, base relations, role implication, parent inheritance, wildcards | ~10 min |
| [models/mcp-guide/](models/mcp-guide/) | Why you should expose `can_*` permissions instead of role names | ~15 min |
| [models/ai-agent/](models/ai-agent/) | Bounded delegation to AI agents using intersection (`and`) | ~20 min |

---

## Step 1 — Read the Basic Model

Open [models/basic/authorization-model-basic.fga](models/basic/authorization-model-basic.fga)
and read it top to bottom.

```fga
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

Four ideas are at work here: **role implication**, **`X from Y`** (folder
permissions that flow down), the **`organization#member`** userset reference,
and the **`user:*`** wildcard. They are all explained in
[concepts.md → Core Concepts](concepts.md#core-concepts).

---

## Step 2 — Add Permission Relations and Run Your First Tests

The basic model uses role names (`editor`, `viewer`) directly. This ties your
app's API to today's roles. The fix: add `can_*` permission relations. Then your
app code asks about *intent*, not *roles*. See
[concepts.md → Permission relations](concepts.md#permission-relations-can_).

Read [models/mcp-guide/](models/mcp-guide/). Look at the four new lines added to
each type:

```fga
define can_view:   viewer
define can_edit:   editor
define can_delete: editor
define can_share:  owner
```

**Run the tests:**

```bash
cd models/mcp-guide
fga model test --tests tests.fga.yaml
```

Expected output: **5 tests passed, 0 failed**.

The test file [tests.fga.yaml](models/mcp-guide/tests.fga.yaml) shows:

1. The owner of a top folder gets `can_view/edit/delete` on a grand-child doc — but **not** `can_share` (sharing needs direct ownership).
2. An org member gets `can_view` through `organization#member` → folder viewer → document viewer (TupleToUserset across three types).
3. A direct editor on a parent folder passes `can_edit` and `can_delete` down to child documents.
4. The wildcard `(user:*, viewer, document:public-memo)` gives `can_view` to anyone, but not `can_edit`.
5. `list_objects` returns every document a user can view.

**Try it yourself:** in `tests.fga.yaml`, add a tuple that gives user:dave
the `editor` role on `folder:product`. Then add an assertion that he
`can_edit` `document:roadmap`. Run the tests again.

> The folder is named `mcp-guide` because this model was written with help from
> the [`openfga-mcp`](https://github.com/openfga) MCP server. That server shares
> the official rule: "Always define permissions in the authorization models."
> The full story — what the server is, plus a worked example — is in
> [concepts.md → The OpenFGA MCP Demo](concepts.md#the-openfga-mcp-demo).

---

## Step 3 — Bounded Delegation to AI Agents

The most advanced model is in [models/ai-agent/](models/ai-agent/). It answers a
question you face the moment you give an AI agent credentials: **how do I let an
agent edit some of my files, but not all of them?**

The pattern uses an intersection (`and`):

```fga
type agent
  relations
    define principal: [user]

type folder
  relations
    define owner: [user]
    define parent: [folder]
    define editor: [user, agent] or owner or editor from parent
    define viewer: [user, user:*, agent] or editor or viewer from parent

    define edit_authorized:   [user:*, agent] or edit_authorized from parent
    define delete_authorized: [user:*, agent] or delete_authorized from parent

    define can_view:   viewer
    define can_edit:   editor and edit_authorized
    define can_delete: editor and delete_authorized
    define can_share:  owner
```

The full explanation is in
[concepts.md → Intersections and bounded delegation](concepts.md#intersections-and-and-bounded-delegation).
It covers why the intersection limits the agent, why `user:*` lets humans pass
but not agents, and how subtree scoping works.

**Run the tests:**

```bash
cd models/ai-agent
fga model test --tests tests.fga.yaml
```

Expected: **5 tests passed, 0 failed**.

Cases worth tracing by hand:

- `agent:scribe` is `editor` and `edit_authorized` on `folder:projects`, so it `can_edit` `file:report`. But it has **no** `delete_authorized`, so `can_delete` is false.
- `agent:janitor` has `delete_authorized` on `folder:projects` but **not** `folder:root`. So it can delete `file:report` but not `file:secret` (which sits directly under root).
- `list_objects(agent:scribe, file, can_edit)` returns only `file:report`. The intersection is checked during the listing too, so the agent can't even *find* files outside its grant.

---

## Step 4 — Run the Live HTTP Demo (optional)

The `cmd/`, `internal/`, and `scripts/` directories hold a Go HTTP server. It
runs Check / Write / Delete / ListObjects / Expand against a real OpenFGA
instance. This brings the model to life, beyond `fga model test`.

### 4a. Start the infrastructure

```bash
make up
```

Starts MariaDB and the OpenFGA server. The visual
[OpenFGA Playground](https://openfga.dev/docs/getting-started/setup-openfga/playground)
is now at <http://localhost:3000/playground>. (The bare root
`http://localhost:3000` returns a 404. The Playground lives under the
`/playground` path.)

### 4b. Run the interactive CLI walkthrough

```bash
make cli
```

Walks through Check, Write, Delete, ListObjects, and Expand, with printed notes.
Press Enter to go to the next step.

### 4c. Seed and serve

```bash
# Terminal 1 — seed demo data and start the HTTP server on :8000
make serve
```

`make serve` seeds the demo data and serves in the **same** process. The
document store is in memory and lives only inside one process. So seeding must
happen in the process that serves. If you run `make seed` on its own, it fills a
store that vanishes when that short process exits. The server would then have no
documents, and every lookup would return `404`.

```bash
# Terminal 2 — call the API
curl -s -H "X-User-Id: alice" http://localhost:8000/documents | jq .

# bob has no access to roadmap → 403
curl -s -w "\nHTTP %{http_code}\n" -H "X-User-Id: bob" http://localhost:8000/documents/roadmap

# charlie is editor on folder:product, so can view roadmap (cascade)
curl -s -H "X-User-Id: charlie" http://localhost:8000/documents/roadmap | jq .

# randomstranger can view public-memo via the wildcard
curl -s -H "X-User-Id: randomstranger" http://localhost:8000/documents/public-memo | jq .

# alice shares design-doc with bob as editor
curl -s -X POST -H "X-User-Id: alice" -H "Content-Type: application/json" \
  -d '{"user":"user:bob","relation":"editor"}' \
  http://localhost:8000/documents/design-doc/share | jq .
```

Or run the full automated walkthrough:

```bash
make demo
```

### 4d. Browser walkthrough (no Postman)

The [`web/`](web/) directory holds a browser version of the same 8-chapter
raw-API walkthrough. It needs no install. The Postman collection
([postman/](postman/)) and `make cli` run the same steps. It calls the OpenFGA
REST API on `:8080` straight from the browser. (OpenFGA returns
`Access-Control-Allow-Origin: *`, so you need no proxy.)

```bash
make up    # OpenFGA on :8080 (if not already running)
make web   # serves web/ on http://localhost:8090
```

Then open:

- <http://localhost:8090/> — **narrated walkthrough.** One card per chapter, with
  a **Run** button per step and a **Run all chapters** button. Chapter 1 grabs
  `store_id` / `model_id` for you (like the Postman test scripts). Every Check
  shows expected vs. actual ALLOWED/DENIED with a ✓/✗ badge.
- <http://localhost:8090/swagger.html> — **Swagger UI**, a raw-API reference
  built from [web/openfga-openapi.json](web/openfga-openapi.json). Use it to try
  single endpoints. It does not enforce order or grab IDs for you.

`/write` is not idempotent. So if you run the demo again on the same store, it
fails with HTTP 400 on tuples that already exist. To fix this, click **Create
Store** again for a fresh store, or use the Appendix → **Reset Chapter 6** step.
This is the same as in the Postman collection.

### 4e. Tear down

```bash
make down
```

Stops all containers and removes the volumes.

---

## Project Structure

```
OpenFGADemo/
├── cmd/
│   ├── cli/          # Interactive teaching walkthrough binary (make cli)
│   └── server/       # HTTP API server binary (seeds data, serves :8000)
├── internal/
│   ├── auth/         # X-User-Id middleware and Authorize helper
│   ├── fga/          # OpenFGA Go SDK wrapper (Check/Write/ListObjects/Expand) + store bootstrap
│   ├── httpapi/      # Chi HTTP handlers and error mapping
│   └── store/        # Thread-safe in-memory document/folder/org store
├── models/
│   ├── basic/        # Step 1: DSL syntax and base relations
│   ├── mcp-guide/    # Step 2: can_* permission relations + tests
│   │   └── mcp_guidance/   # Official "define permissions" guidance from the MCP server
│   └── ai-agent/     # Step 3: bounded AI-agent delegation + tests
├── web/              # Zero-install browser walkthrough + Swagger UI (make web)
│   ├── index.html             # Narrated 8-chapter raw-API walkthrough
│   ├── swagger.html           # Swagger UI rendered from the spec below
│   └── openfga-openapi.json   # OpenAPI spec for the OpenFGA REST API
├── postman/          # Postman collection mirroring the raw-API walkthrough
├── scripts/
│   └── demo.sh       # Automated curl walkthrough of the HTTP API
├── concepts.md                    # ReBAC theory + per-concept deep dives + MCP demo
├── docker-compose.yml             # MariaDB + OpenFGA + Playground
├── Makefile
└── go.mod
```

---

## Where to Next

- **Theory & concepts:** [concepts.md](concepts.md) — RBAC/ReBAC/Zanzibar
  background, a deep look at every concept above, and the OpenFGA MCP demo.
- [OpenFGA documentation](https://openfga.dev/docs) · [FGA modeling concepts](https://openfga.dev/docs/concepts) · [DSL reference](https://openfga.dev/docs/configuration-language)
