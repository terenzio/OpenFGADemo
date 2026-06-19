# OpenFGA Demo

A hands-on workshop for learning [OpenFGA](https://openfga.dev) — Auth0's
open-source implementation of Google Zanzibar-style relationship-based access
control (ReBAC). Three progressively richer authorization models walk you
through the language, the testing workflow, and a real-world AI-agent
delegation pattern. A companion Go HTTP server lets you exercise the model
through a tiny document-management API.

---

## What You'll Learn

By the end of the workshop you will have written, tested, and reasoned about:

- **Direct relations** — `(user:alice, owner, document:roadmap)`
- **Role implication** — owners are also editors, editors are also viewers
- **TupleToUserset** (`X from Y`) — permissions that flow down a folder hierarchy
- **Userset references** — `organization:acme#member` grants access to a whole group
- **Wildcards** — `user:*` makes an object public to anyone
- **Permission relations** — `can_view` / `can_edit` / `can_share` as the API surface for app code
- **Intersections** (`and`) — capability gates that bound what an AI agent can do
- **The five core API operations** — `Check`, `Write`, `Delete`, `ListObjects`, `Expand`

---

## Prerequisites

Install these before the workshop begins.

| Tool | Required for | Install |
| --- | --- | --- |
| [OpenFGA CLI](https://openfga.dev/docs/getting-started/install-sdk) | Steps 1–3 (model authoring & tests) | `brew install openfga/tap/fga` |
| [Docker](https://docs.docker.com/get-docker/) + Compose | Step 4 (live server demo) | per platform |
| [Go 1.22+](https://go.dev/dl/) | Step 4 (live server demo) | per platform |
| `curl`, `jq` | Step 4 (HTTP examples) | per platform |

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

**What to notice:**

1. **Role implication.** `viewer` is `... or editor`, and `editor` is `... or owner`. Owners get editor and viewer for free; editors get viewer for free.
2. **`X from Y`.** `editor from parent` says: "if you are an editor on this folder's parent, you are also editor here." That single clause gives you cascading folder permissions.
3. **`organization#member`.** A *userset reference*. Granting `organization:acme#member` viewer access on a folder lets every member of acme view it — without writing a tuple per person.
4. **`user:*` wildcard.** A document with `(user:*, viewer, document:public-memo)` is publicly viewable.

**Discussion prompt:** what would happen if you removed `or editor` from the
`viewer` clause? Who loses access?

---

## Step 2 — Add Permission Relations and Run Your First Tests

The basic model exposes role names (`editor`, `viewer`) directly. Application
code ends up calling `Check(user, "editor", doc)`, which couples the API
surface to today's role structure. The fix is to add `can_*` permission
relations.

Read [models/mcp-guide/](models/mcp-guide/)
— specifically the four added lines on each type:

```fga
define can_view:   viewer
define can_edit:   editor
define can_delete: editor
define can_share:  owner
```

Now app code asks `can_edit` (intent), not `editor` (role). If tomorrow you
decide some viewers should also be able to edit, you change one line in the
model — not every call site.

**Run the tests:**

```bash
cd models/mcp-guide
fga model test --tests tests.fga.yaml
```

Expected output: **5 tests passed, 0 failed**.

The test file [tests.fga.yaml](models/mcp-guide/tests.fga.yaml)
demonstrates:

1. Owner of a top folder gets `can_view/edit/delete` cascaded to a grand-child doc — but **not** `can_share` (sharing requires direct ownership).
2. Org member inherits `can_view` via `organization#member` → folder viewer → document viewer (TupleToUserset across three type boundaries).
3. Direct editor on a parent folder cascades `can_edit` and `can_delete` to child documents.
4. Wildcard `(user:*, viewer, document:public-memo)` grants `can_view` to anyone, but not `can_edit`.
5. `list_objects` returns every document a given user can view.

**Try it yourself:** add a tuple in `tests.fga.yaml` that grants user:dave
the `editor` role on `folder:product`, then add an assertion that he
`can_edit` `document:roadmap`. Re-run the tests.

The `mcp-guide` folder name comes from the fact that this model was authored
with help from the [`openfga-mcp`](https://github.com/openfga) MCP server,
which surfaces the official "Always define permissions in the authorization
models" guidance. See [its README](models/mcp-guide/README.md)
for the full story.

---

## Step 3 — Bounded Delegation to AI Agents

The most advanced model is in
[models/ai-agent/](models/ai-agent/).
It answers a question that comes up the moment you give an AI agent
credentials: **how do I let an agent edit my files without letting it edit
all of them?**

The pattern uses an intersection:

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

**The three ideas:**

1. **Intersection (`and`).** Being an `editor` is no longer enough on its own. A separate `edit_authorized` capability must also evaluate true. Either side missing → denied.
2. **Different default for users vs. agents.** The seed tuple `(user:*, edit_authorized, folder:root)` makes every human pass the gate everywhere — humans behave exactly as in step 2. `user:*` does **not** match agents, so agents start with no capability and need an explicit per-folder grant.
3. **Subtree scoping comes for free.** `or edit_authorized from parent` means a single grant `(agent:bot, edit_authorized, folder:projects)` covers every descendant — and only those descendants.

`can_share` is reserved for `owner` and is never delegable to agents at all.

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

**Discussion prompt:** suppose you wanted a one-shot capability that
auto-expires after a single edit. The model can't express time, but where
in the application stack would you enforce that? (Answer: write the tuple
before the call, delete it after — OpenFGA gives you the gate, your code
runs the timer.)

The `agent.principal` relation records which user an agent acts on behalf
of. It's informational only — useful for audit logs and app-side
guardrails — and does not appear in any permission expression.

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

### 4d. Tear down

```bash
make down
```

Stops all containers and removes volumes.

---

## OpenFGA MCP Demo

Step 2's model folder is named `mcp-guide` because it was
authored with help from the [`openfga-mcp`](https://github.com/openfga) server.
This section explains what that server actually is and walks through using it to
improve the [basic model](models/basic/authorization-model-basic.fga).

### What the server is (and isn't)

`openfga-mcp` is a [Model Context Protocol](https://modelcontextprotocol.io)
server that gives an AI assistant the *official OpenFGA modeling playbook* on
demand. It is a **context/prompt provider**, not a live connection to a running
OpenFGA store:

- It **does** inject best-practice authoring guidance — the same advice the
  OpenFGA team publishes about DSL syntax, relationship patterns, `can_*`
  permissions, `.fga.yaml` testing, and custom roles — directly into the
  assistant's context.
- It **does not** read your tuples, run `Check`, or talk to the server you boot
  with `make up`. For that you still use the `fga` CLI or the Go SDK (Step 4).

Think of it as a retrieval layer that primes the assistant with the right
guidance *before* it helps you write or review a model.

### The two tools

The server exposes exactly two tools:

| Tool | What it does |
| --- | --- |
| `list_available_contexts` | Lists every context prompt the server can supply, with the keyword patterns that trigger each one. |
| `get_context_for_query` | Takes a natural-language query and returns the most relevant context prompt in full. |

Calling `list_available_contexts` against this server returns a single context:

```
Available Context Prompts:

**Author authorization models with OpenFGA**
File: authorization-model.md
Patterns: authorization model, auth model, rbac, abac, permission, role based,
          tuple, zanzibar, rebac, fine grained access control, fga,
          permission check, can user, access check, ...
```

So any query about access models, permissions, tuples, or `can_*` checks will
match and pull back the full authoring guide.

### Wiring it up

MCP servers are registered in your client's MCP config. For Claude Code, add it
to `.mcp.json` (or your user-level `settings.json`):

```json
{
  "mcpServers": {
    "openfga-mcp": {
      "command": "npx",
      "args": ["-y", "@openfga/mcp"]
    }
  }
}
```

> Check the [`openfga-mcp` project](https://github.com/openfga) for the exact
> package/command — the point of this demo is the *behavior* (two tools, context
> on demand), which is what the examples below show.

### Worked example — upgrading the basic model

The [basic model](models/basic/authorization-model-basic.fga) exposes role names
(`editor`, `viewer`) but **no permission relations**. That's exactly the gap the
MCP guidance tells you to close. Here is the round trip.

**1. Ask the server for guidance.** A call to `get_context_for_query` with a
query like *"How do I expose permissions instead of role names in OpenFGA?"*
matches the `permission` / `can user` patterns and returns the authoring guide,
which states:

> It's a common practice to define specific permissions, that can't be directly
> assigned, using `can_<permission>` relations … **Always define permissions in
> the authorization models.**

**2. Apply it to the basic model.** The basic `document` type looks like this:

```fga
type document
  relations
    define parent: [folder]
    define owner: [user]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

Following the guidance, you add a `can_*` surface so application code asks about
*intent* (`can_edit`) instead of *roles* (`editor`):

```fga
type document
  relations
    define parent: [folder]
    define owner: [user]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent

    define can_view:   viewer
    define can_edit:   editor
    define can_delete: editor
    define can_share:  owner
```

That is precisely the four-line change you see materialized in
[Step 2's model](models/mcp-guide/authorization-model-mcp-guide.fga).

**3. Ask the server how to test it.** A follow-up query like *"How do I write
tests for an OpenFGA model?"* matches the same context and returns the
`.fga.yaml` recipe — inline `model`, `tuples`, and `tests` with `check`,
`list_objects`, and `list_users` assertions — which is the shape of
[Step 2's `tests.fga.yaml`](models/mcp-guide/tests.fga.yaml):

```yaml
model_file: ./authorization-model-mcp-guide.fga
tuples:
  - user: user:alice
    relation: owner
    object: folder:top
tests:
  - name: owner cascade
    check:
      - user: user:alice
        object: document:roadmap
        assertions:
          can_edit: true
          can_share: false   # sharing requires direct ownership
```

```bash
fga model test --tests tests.fga.yaml
```

**The takeaway:** the MCP server didn't change your model — it handed the
assistant the official guidance, and the assistant applied it. Every decision is
still verifiable with the `fga` CLI, so you get authoritative best practices
*and* reproducible tests.

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
├── models/
│   ├── basic/                                 # Step 1
│   ├── mcp-guide/                             # Step 2
│   └── ai-agent/                              # Step 3
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
- [OpenFGA DSL reference](https://openfga.dev/docs/configuration-language)
- [`fga model test` documentation](https://openfga.dev/docs/getting-started/cli)
- [OpenFGA Go SDK](https://github.com/openfga/go-sdk)
- [Zanzibar: Google's Consistent, Global Authorization System](https://research.google/pubs/pub48190/) — the paper that inspired ReBAC
- [docs/model-explained.md](docs/model-explained.md) — annotated walkthrough of the document-management model
- [docs/architecture.md](docs/architecture.md) — component diagram for the Go server
