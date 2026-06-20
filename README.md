# OpenFGA Demo

A hands-on workshop for learning [OpenFGA](https://openfga.dev) — Auth0's
open-source implementation of Google Zanzibar-style relationship-based access
control (ReBAC). Three progressively richer authorization models walk you
through the language, the testing workflow, and a real-world AI-agent
delegation pattern. A companion Go HTTP server lets you exercise the model
through a tiny document-management API.

> **New to ReBAC, or want the theory behind each step?** See
> [concepts.md](concepts.md) — it covers the RBAC/ReBAC/Zanzibar background, a
> deep dive on every concept the workshop teaches, and the OpenFGA MCP server
> used to author the models. This README stays focused on getting set up and
> walking through the demos.

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

This repo contains three authorization models that build on each other. Work
through them in order — each one introduces a new OpenFGA concept on top of
the previous.

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

The four ideas at work here — **role implication**, **`X from Y`** (cascading
folder permissions), the **`organization#member`** userset reference, and the
**`user:*`** wildcard — are explained in
[concepts.md → Core Concepts](concepts.md#core-concepts).

---

## Step 2 — Add Permission Relations and Run Your First Tests

The basic model exposes role names (`editor`, `viewer`) directly, which couples
your application's API surface to today's role structure. The fix is to add
`can_*` permission relations so app code asks about *intent*, not *roles* — see
[concepts.md → Permission relations](concepts.md#permission-relations-can_).

Read [models/mcp-guide/](models/mcp-guide/) — specifically the four added lines
on each type:

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

The test file [tests.fga.yaml](models/mcp-guide/tests.fga.yaml) demonstrates:

1. Owner of a top folder gets `can_view/edit/delete` cascaded to a grand-child doc — but **not** `can_share` (sharing requires direct ownership).
2. Org member inherits `can_view` via `organization#member` → folder viewer → document viewer (TupleToUserset across three type boundaries).
3. Direct editor on a parent folder cascades `can_edit` and `can_delete` to child documents.
4. Wildcard `(user:*, viewer, document:public-memo)` grants `can_view` to anyone, but not `can_edit`.
5. `list_objects` returns every document a given user can view.

**Try it yourself:** add a tuple in `tests.fga.yaml` that grants user:dave
the `editor` role on `folder:product`, then add an assertion that he
`can_edit` `document:roadmap`. Re-run the tests.

> The `mcp-guide` folder name comes from the fact that this model was authored
> with help from the [`openfga-mcp`](https://github.com/openfga) MCP server,
> which surfaces the official "Always define permissions in the authorization
> models" guidance. The full story — what the server is and a worked example of
> using it — is in
> [concepts.md → The OpenFGA MCP Demo](concepts.md#the-openfga-mcp-demo).

---

## Step 3 — Bounded Delegation to AI Agents

The most advanced model is in [models/ai-agent/](models/ai-agent/). It answers a
question that comes up the moment you give an AI agent credentials: **how do I
let an agent edit my files without letting it edit all of them?**

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

The full explanation — why the intersection bounds the agent, why `user:*`
gives humans a free pass but agents none, and how subtree scoping works — is in
[concepts.md → Intersections and bounded delegation](concepts.md#intersections-and-and-bounded-delegation).

**Run the tests:**

```bash
cd models/ai-agent
fga model test --tests tests.fga.yaml
```

Expected: **5 tests passed, 0 failed**.

The cases worth tracing through by hand:

- `agent:scribe` is `editor` and `edit_authorized` on `folder:projects`, so it `can_edit` `file:report` — but it has **no** `delete_authorized`, so `can_delete` is false.
- `agent:janitor` has `delete_authorized` on `folder:projects` but **not** `folder:root`, so it can delete `file:report` but not `file:secret` (which lives directly under root).
- `list_objects(agent:scribe, file, can_edit)` returns only `file:report` — the intersection is enforced during enumeration, so the agent can't even *discover* files outside its grant.

---

## Step 4 — Run the Live HTTP Demo (optional)

The `cmd/`, `internal/`, and `scripts/` directories contain a Go HTTP server
that exercises Check / Write / Delete / ListObjects / Expand against a real
OpenFGA instance. This brings the model to life beyond `fga model test`.

### 4a. Start the infrastructure

```bash
make up
```

Boots MariaDB and the OpenFGA server. The visual
[OpenFGA Playground](https://openfga.dev/docs/getting-started/setup-openfga/playground)
is now at <http://localhost:3000/playground> (the bare root `http://localhost:3000`
returns a 404 — the Playground is served under the `/playground` path).

### 4b. Run the interactive CLI walkthrough

```bash
make cli
```

Steps through Check, Write, Delete, ListObjects, and Expand with printed
explanations. Press Enter to advance through each step.

### 4c. Seed and serve

```bash
# Terminal 1 — seed demo data and start the HTTP server on :8000
make serve
```

`make serve` seeds the demo data and serves in the **same** process. The
document store is in-memory and per-process, so seeding must happen in the
process that serves — running `make seed` separately would populate a store
that disappears when that short-lived process exits, leaving the server with
no documents (and every lookup returning `404`).

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

The [`web/`](web/) directory holds a zero-install browser version of the same
8-chapter raw-API walkthrough that the Postman collection
([postman/](postman/)) and `make cli` drive. It calls the OpenFGA REST API on
`:8080` directly from the browser (OpenFGA returns `Access-Control-Allow-Origin: *`,
so no proxy is needed).

```bash
make up    # OpenFGA on :8080 (if not already running)
make web   # serves web/ on http://localhost:8090
```

Then open:

- <http://localhost:8090/> — **narrated walkthrough.** One card per chapter with a
  per-step **Run** button and a **Run all chapters** button. Chapter 1 captures
  `store_id` / `model_id` automatically (like the Postman test scripts); every
  Check shows expected-vs-actual ALLOWED/DENIED with a ✓/✗ badge.
- <http://localhost:8090/swagger.html> — **Swagger UI**, a raw-API reference
  rendered from [web/openfga-openapi.json](web/openfga-openapi.json). Use it to
  poke individual endpoints; it does not enforce order or auto-capture IDs.

Because `/write` is non-idempotent, re-running the demo on the same store fails
with HTTP 400 on already-written tuples — click **Create Store** again for a
fresh store, or use the Appendix → **Reset Chapter 6** step, exactly as in the
Postman collection.

### 4e. Tear down

```bash
make down
```

Stops all containers and removes volumes.

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
  background, a deep dive on every concept above, and the OpenFGA MCP demo.
- [OpenFGA documentation](https://openfga.dev/docs) · [FGA modeling concepts](https://openfga.dev/docs/concepts) · [DSL reference](https://openfga.dev/docs/configuration-language)
