---
marp: true
title: OpenFGA Workshop — Fine-Grained Authorization for Modern Apps
theme: default
paginate: true
---

# OpenFGA Workshop

## Fine-Grained Authorization for Modern Applications

A hands-on tour of relationship-based access control,
from a single-line `Check` to bounded AI-agent delegation.

DevOps Conference

---

## Workshop Goals

By the end of this session you will:

- Understand **ReBAC** and how it compares to RBAC / ABAC
- Read and write **OpenFGA DSL** models
- Run **`fga model test`** — TDD for authorization
- Stand up an **OpenFGA server** with Docker
- Wire `Check` calls into a real Go HTTP service
- Apply the pattern that bounds **AI-agent permissions**

Workshop repo: [github.com/terenzio/OpenFGADemo](.)

---

## The Problem

> "Can user X do action Y on resource Z?"

Sounds simple. Until:

- Resources are nested (folders, projects, teams)
- Users belong to groups (organizations, departments)
- Permissions cascade (folder owner → document editor)
- Some objects are public (`user:*`)
- Some actors are non-human (AI agents, services)
- The product roadmap changes the rules every quarter

---

## RBAC vs ABAC vs ReBAC

| Approach | Decision based on...                | Strength            | Weakness                           |
| -------- | ----------------------------------- | ------------------- | ---------------------------------- |
| RBAC     | Role assigned to user               | Simple, auditable   | Role explosion, no relationships   |
| ABAC     | Attributes of subject + resource    | Flexible            | Hard to audit, policy sprawl       |
| ReBAC    | Relationships between objects       | Models real domains | Newer, less tooling — until now    |

ReBAC is the model behind Google Drive, GitHub, Slack — anywhere you
say "Charlie shared this folder with the engineering team."

---

## What is OpenFGA?

- **Open-source ReBAC engine**, CNCF Sandbox project
- Inspired by the [Google Zanzibar paper](https://research.google/pubs/pub48190/) (2019)
- Started by **Auth0**, now community-driven
- HTTP + gRPC API
- Storage: Postgres, MySQL, MariaDB, in-memory
- Performance: P99 sub-10ms `Check` at billions of tuples (Zanzibar lineage)

---

## Core Concepts

A **model** declares types and relations:

```fga
type document
  relations
    define editor: [user]
    define viewer: [user] or editor
```

A live system stores **tuples**: `(user, relation, object)`.

```
user:alice  →  editor  →  document:roadmap
```

`Check(user:alice, viewer, document:roadmap)` → **true**
(via `or editor`)

---

## The Five Core API Operations

| Call             | Question                                                  |
| ---------------- | --------------------------------------------------------- |
| **Check**        | Can this user do this thing on this object?               |
| **Write**        | Add a tuple (grant access)                                |
| **Delete**       | Remove a tuple (revoke access)                            |
| **ListObjects**  | Which objects can this user act on?                       |
| **Expand**       | Show the full subtree behind a relation (debugging)       |

App code lives mostly in `Check` and `ListObjects`. Tuple CRUD happens
in admin / share flows.

---

## Workshop Roadmap

Three progressive models, each adds one concept:

1. **Basic** — DSL fundamentals: roles, cascade, wildcards, groups.
2. **`can_*` permissions** — separate intent from implementation.
3. **AI-agent delegation** — bounded capabilities via intersection.

```
OpenFGADemo/
├── model_basic_demo/                     ← Step 1
└── model_advanced_demo/
    ├── folder_document_with_mcp_demo/    ← Step 2
    └── agent_auth_demo/                  ← Step 3
```

---

# Step 1 — The Basic Model

[`model_basic_demo/authorization-model.fga`](model_basic_demo/authorization-model.fga)

```fga
type folder
  relations
    define owner: [user]
    define parent: [folder]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

Five DSL ideas in one type definition.

---

## Idea 1 — Direct Relations

```
user:alice  →  owner  →  folder:company
```

`define owner: [user]` declares which subject *types* may hold this
relation.

One tuple per grant. The base unit of OpenFGA storage.

---

## Idea 2 — Role Implication

```fga
define viewer: [...] or editor
define editor: [...] or owner
```

- Owners get editor for free.
- Editors get viewer for free.

One write satisfies three Checks.

---

## Idea 3 — Parent Inheritance (`X from Y`)

```fga
define editor: [...] or editor from parent
```

```
user:alice       owner   folder:company
folder:company   parent  folder:product
folder:product   parent  document:roadmap
```

`Check(user:alice, editor, document:roadmap)` → **true**

Two parent hops, zero extra grant tuples.

---

## Idea 4 — Userset References

Grant a relation to *every member of a group*:

```
user:eve                   member   organization:acme
organization:acme#member   viewer   folder:product
```

Add or remove acme members → folder access updates automatically.
**No per-user tuples to manage.**

---

## Idea 5 — Wildcards (`user:*`)

```
user:*  →  viewer  →  document:public-memo
```

Anyone is now a viewer.

Wildcards work only if `user:*` appears in the relation's
directly-related-types list. In our model, `viewer` is public —
but `editor` is **not**.

---

# Step 2 — Add `can_*` Permissions

[`model_advanced_demo/folder_document_with_mcp_demo/`](model_advanced_demo/folder_document_with_mcp_demo/)

The basic model exposes role names. App code calls:

```go
Check(user, "editor", doc)   // couples API to today's role names
```

Step 2 adds permission relations:

```fga
define can_view:   viewer
define can_edit:   editor
define can_delete: editor
define can_share:  owner
```

App code now calls `Check(user, "can_edit", doc)` — **intent**, not role.

---

## Why It Matters Operationally

- New requirement: "let viewers edit on weekends." Change one line in
  the model — not 20 call sites.
- Auditing: log says `can_edit` denied — clear what the user *tried*
  to do.
- Versioning: model changes don't break callers.

> `can_*` is the **API surface**.
> Roles are the **implementation**.

---

## TDD for Authorization

```bash
cd model_advanced_demo/folder_document_with_mcp_demo
fga model test --tests tests.fga.yaml
```

→ **5 tests passed, 0 failed**

Tests run inline tuples through a sandboxed evaluator — no server,
no DB, no Docker.

CI-friendly. Fail the build on a model regression. Treat your auth
model like code.

---

## Step 2 — Test Walkthrough

| # | Scenario                                | Expected                         |
| - | --------------------------------------- | -------------------------------- |
| 1 | Owner cascade to grandchild doc         | view/edit/delete yes, share no   |
| 2 | Org member via TupleToUserset           | view yes, edit no                |
| 3 | Direct editor on parent folder cascades | edit/delete yes                  |
| 4 | `user:*` wildcard                       | view yes, edit no                |
| 5 | `list_objects` enumeration              | returns every reachable doc      |

Key insight: **`can_share` does not cascade.** Sharing is owner-only
by design — an org admin should not auto-reshare a report writer's
draft.

---

# Step 3 — The AI-Agent Problem

You give an AI coding agent your credentials.

It can now:

- View every file you can view (you wanted this)
- Edit every file you can edit (you didn't want **all** of them)
- Delete every file you can delete (you really didn't want this)
- Share files with strangers (you absolutely didn't want this)

**Need:** bounded delegation, per capability, per scope.

---

## Step 3 — The Intersection Pattern

[`model_advanced_demo/agent_auth_demo/authorization-model.fga`](model_advanced_demo/agent_auth_demo/authorization-model.fga)

```fga
type agent
  relations
    define principal: [user]    # informational: which user this agent acts for

type folder
  relations
    define editor: [user, agent] or owner or editor from parent

    define edit_authorized:   [user:*, agent] or edit_authorized from parent
    define delete_authorized: [user:*, agent] or delete_authorized from parent

    define can_edit:   editor and edit_authorized
    define can_delete: editor and delete_authorized
    define can_share:  owner    # never delegable to agents
```

---

## Three Ideas, One Model

1. **Intersection (`and`)** — both relations must evaluate true. Either
   side missing → denied.
2. **Different default for users vs. agents.** `(user:*, edit_authorized, folder:root)`
   makes humans default-pass everywhere. `user:*` does **not** match
   agents → agents are default-deny until explicitly granted.
3. **Subtree scoping for free.** `or edit_authorized from parent`
   means one grant per subtree covers every descendant — and only
   those descendants.

---

## Step 3 — Concrete Scenario

```
folder:root              ← user:alice (owner)
├── folder:projects      ← agent:scribe   (editor + edit_authorized)
│   │                    ← agent:janitor  (editor + edit + delete_authorized)
│   └── file:report
└── file:secret
```

| Actor         | file:report          | file:secret          |
| ------------- | -------------------- | -------------------- |
| user:alice    | view / edit / delete | view / edit / delete |
| agent:scribe  | view + edit (no del) | nothing              |
| agent:janitor | view + edit + delete | nothing              |
| anyone        | share: owner-only    | share: owner-only    |

---

## `list_objects` Bounds Discovery

```yaml
list_objects:
  - user: agent:scribe
    type: file
    assertions:
      can_edit:   [file:report]
      can_delete: []
```

`list_objects(agent:scribe, file, can_edit)` returns **only**
`file:report`.

The agent cannot even *discover* files outside its grant — critical
when the agent is an LLM that auto-enumerates available actions.

---

# Step 4 — Run the Live Demo

The repo ships a Go HTTP server that exercises Check / Write / Delete /
ListObjects / Expand against a real OpenFGA instance.

```bash
make up      # MariaDB + OpenFGA + Playground (localhost:3000)
make seed    # Apply model, write tuples, set demo state
make serve   # Go HTTP server on :8000
```

Or all at once:

```bash
make demo    # automated walkthrough with curl
```

---

## Live Demo — `curl` Examples

```bash
# alice — owner of folder:company, sees everything via cascade
curl -H "X-User-Id: alice" :8000/documents | jq

# bob — no grant, gets 403
curl -H "X-User-Id: bob" :8000/documents/roadmap

# charlie — direct editor on folder:product, can view roadmap
curl -H "X-User-Id: charlie" :8000/documents/roadmap | jq

# randomstranger — public-memo via user:* wildcard
curl -H "X-User-Id: randomstranger" :8000/documents/public-memo | jq

# alice grants bob editor on design-doc
curl -X POST -H "X-User-Id: alice" \
  -d '{"user":"user:bob","relation":"editor"}' \
  :8000/documents/design-doc/share
```

---

## Architecture at a Glance

```
+-----------+     +----------------+     +-------------+
|  Client   | --> |  Go HTTP API   | --> |  OpenFGA    |
|  (curl)   |     |  (chi router)  |     |  Server     |
+-----------+     +--------+-------+     +------+------+
                           |                    |
                           v                    v
                  +----------------+     +-------------+
                  | In-memory      |     |  MariaDB    |
                  | document store |     |  (tuples)   |
                  +----------------+     +-------------+
```

`X-User-Id` header → middleware → `Check(user, can_*, doc)` → handler

---

## Try the Playground

OpenFGA ships a visual debugger:

<http://localhost:3000>

- Author models with live syntax checking
- Visualize the relation graph
- Run `Check` and `Expand` interactively
- See which tuples contributed to a decision

Great for explaining auth decisions to non-engineering stakeholders.

---

# DevOps Considerations

---

## Storage and Deployment

- **Storage**: Postgres / MySQL / MariaDB in production. SQLite for dev.
- **Binary**: ~30 MB, single static binary. Stateless. Horizontal scaling.
- **Container**: official image at `openfga/openfga`. We use it via
  [`docker-compose.yml`](docker-compose.yml).
- **Backup**: tuples are your data — back up the DB, not the binary.

---

## Versioning Models

Models are **immutable**. Every `fga model write` creates a new model
ID. You never mutate; you roll forward.

Migration pattern:

1. Write the new model — get back a new model ID.
2. **Dual-check** during cutover: app calls `Check` against both old
   and new model IDs, alerts on divergence.
3. Once divergence is zero for N days, point app at new model ID.
4. Old model ID stays around forever — auditable.

---

## CI / CD Integration

```yaml
# .github/workflows/test.yml
- name: Validate authorization model
  run: |
    cd model_advanced_demo/folder_document_with_mcp_demo
    fga model test --tests tests.fga.yaml
```

`fga model test` is a hermetic, sub-second check. Run it on every PR.

A regression in your auth model is a security incident — catch it
before merge.

---

## Observability

- Built-in **OpenTelemetry** traces and metrics
- Log every `Check` decision: allow / deny + the tuple resolution
  path (use `Expand` to capture the "why")
- Track P99 latency on `Check` — it sits in the request hot path
- Alert on tuple count growth — runaway grants indicate a bug or
  a missing revoke

---

## When Not to Use OpenFGA

- **Trivial apps** with one role and 10 users — overkill.
- **Pure attribute-based** decisions (`if request.region == us-east`).
  ReBAC is about relationships; tools like OPA fit better.
- **Time-of-day rules** in the engine. ReBAC has no time concept —
  enforce in your app, write/delete tuples around capability windows.

OpenFGA shines when **relationships drive authorization**: documents,
folders, teams, projects, agents.

---

# Recap

1. **Tuples** describe relationships. **Models** describe what those
   relationships mean.
2. **`can_*` relations** decouple intent from role.
3. **Intersection (`and`)** bounds AI-agent power without
   re-architecting your role model.
4. **`fga model test`** is TDD for authorization. Run it in CI.
5. **`make up && make demo`** runs the full live stack locally.

---

## Further Reading

- [openfga.dev](https://openfga.dev) — official docs and tutorials
- [OpenFGA DSL reference](https://openfga.dev/docs/configuration-language)
- [Zanzibar paper](https://research.google/pubs/pub48190/) — the foundation
- [OpenFGA Go SDK](https://github.com/openfga/go-sdk)
- [`docs/model-explained.md`](docs/model-explained.md) — annotated walkthrough in this repo
- [`docs/architecture.md`](docs/architecture.md) — Go server architecture

---

# Q & A

Workshop repo:

```bash
git clone <this-repo>
cd OpenFGADemo
make up && make demo
```

Questions, war stories, edge cases — let's hear them.

Thank you.
