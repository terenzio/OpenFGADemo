# OpenFGA Concepts & Theory

The theoretical companion to the [README](README.md). The README gets you set
up and walks you through the demos; this file explains the *why* behind the
models — the relationship-based access control (ReBAC) concepts the workshop
teaches and the OpenFGA MCP server that helped author them.

Read the README first. Come back here whenever you want the deeper "what is
actually going on" explanation for a step.

---

## Background — RBAC, ReBAC, and Zanzibar

[OpenFGA](https://openfga.dev) is Auth0's open-source implementation of a
Google Zanzibar-style **relationship-based access control (ReBAC)** system.

Where classic **RBAC** asks *"what role does this user have?"*, ReBAC asks
*"what is the relationship between this user and this object?"* — and lets those
relationships compose: through roles that imply other roles, through hierarchies
(a folder's permissions flow to its documents), and through groups (membership
in an organization grants access to everything that org can see).

The model is expressed as relationship **tuples** of the form
`(user, relation, object)`, e.g. `(user:alice, owner, document:roadmap)`. An
authorization **model** (written in OpenFGA's DSL) defines which relations exist
and how they imply one another. The classic reference is the paper that started
it all: [Zanzibar: Google's Consistent, Global Authorization System](https://research.google/pubs/pub48190/).

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

## Core Concepts

These are the building blocks the three models use, in roughly the order the
workshop introduces them.

### Direct relations and role implication

A relation can be granted directly — `(user:alice, owner, folder:top)` — or
*implied* by another relation. In the [basic model](models/basic/authorization-model-basic.fga):

```fga
define editor: [user, organization#member] or owner or editor from parent
define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

`viewer` is `... or editor`, and `editor` is `... or owner`. So owners get
editor and viewer for free; editors get viewer for free. You write one tuple
(owner) and three roles light up.

> **Discussion prompt:** what would happen if you removed `or editor` from the
> `viewer` clause? Who loses access?

### TupleToUserset (`X from Y`)

`editor from parent` says: *"if you are an editor on this folder's parent, you
are also an editor here."* That single clause gives you cascading folder
permissions — a grant on a top folder flows down to every descendant folder and
document without writing a tuple per object.

### Userset references (`organization#member`)

`organization#member` is a *userset reference*. Granting
`(organization:acme#member, viewer, folder:x)` lets **every** member of acme
view that folder — without writing a tuple per person. Add a new employee to
acme and they inherit the access automatically.

### Wildcards (`user:*`)

A document with `(user:*, viewer, document:public-memo)` is publicly viewable —
the `user:*` wildcard matches any user. Note it matches *users*, not other
types, which becomes important in the AI-agent model below.

### Permission relations (`can_*`)

The basic model exposes role names (`editor`, `viewer`) directly, so application
code ends up calling `Check(user, "editor", doc)`. That couples the API surface
to today's role structure. The fix is to add `can_*` permission relations:

```fga
define can_view:   viewer
define can_edit:   editor
define can_delete: editor
define can_share:  owner
```

Now app code asks about *intent* (`can_edit`) instead of a *role* (`editor`).
If tomorrow you decide some viewers should also be able to edit, you change one
line in the model — not every call site. This is the central lesson of
[Step 2](README.md#step-2--add-permission-relations-and-run-your-first-tests):
**always define permissions in the authorization model.**

### Intersections (`and`) and bounded delegation

The [AI-agent model](models/ai-agent/authorization-model-ai-agent.fga) answers
the question that arises the moment you give an AI agent credentials: **how do I
let an agent edit my files without letting it edit all of them?**

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

The three ideas:

1. **Intersection (`and`).** Being an `editor` is no longer enough on its own. A
   separate `edit_authorized` capability must also evaluate true. Either side
   missing → denied.
2. **Different default for users vs. agents.** The seed tuple
   `(user:*, edit_authorized, folder:root)` makes every human pass the gate
   everywhere — humans behave exactly as in Step 2. `user:*` does **not** match
   agents, so agents start with no capability and need an explicit per-folder
   grant.
3. **Subtree scoping comes for free.** `or edit_authorized from parent` means a
   single grant `(agent:bot, edit_authorized, folder:projects)` covers every
   descendant — and only those descendants.

`can_share` is reserved for `owner` and is never delegable to agents at all.

The `agent.principal` relation records which user an agent acts on behalf of.
It's informational only — useful for audit logs and app-side guardrails — and
does not appear in any permission expression.

> **Discussion prompt:** suppose you wanted a one-shot capability that
> auto-expires after a single edit. The model can't express time, but where in
> the application stack would you enforce that? (Answer: write the tuple before
> the call, delete it after — OpenFGA gives you the gate, your code runs the
> timer.)

---

## The OpenFGA MCP Demo

Step 2's model folder is named `mcp-guide` because it was authored with help
from the [`openfga-mcp`](https://github.com/openfga) server. This section
explains what that server actually is and walks through using it to improve the
[basic model](models/basic/authorization-model-basic.fga).

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

## Further Reading

- [OpenFGA documentation](https://openfga.dev/docs)
- [FGA modeling concepts](https://openfga.dev/docs/concepts)
- [OpenFGA DSL reference](https://openfga.dev/docs/configuration-language)
- [`fga model test` documentation](https://openfga.dev/docs/getting-started/cli)
- [OpenFGA Go SDK](https://github.com/openfga/go-sdk)
- [Zanzibar: Google's Consistent, Global Authorization System](https://research.google/pubs/pub48190/) — the paper that inspired ReBAC
- [docs/model-explained.md](docs/model-explained.md) — annotated walkthrough of the document-management model
- [docs/architecture.md](docs/architecture.md) — component diagram for the Go server
