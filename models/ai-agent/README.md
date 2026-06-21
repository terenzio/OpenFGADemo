# Agent Authorization Demo

An OpenFGA model for **AI agents acting on behalf of users**. Agents are
**first-class principals** alongside users: they get access by being granted a
role (`viewer`, `editor`, or `owner`), and they are **denied by default**
anywhere they hold no role. Roles cascade down the folder hierarchy, so a single
grant bounds an agent to one subtree. `can_share` stays with owners and is never
given to agents.

It sits next to [`../mcp-guide/`](../mcp-guide/), which shows the same
permission-alias pattern (`can_edit: editor`) for human users. This demo adds the
`agent` type so agents can be granted those same roles and reasoned about
explicitly.

---

## The model

[`authorization-model-ai-agent.fga`](authorization-model-ai-agent.fga) — key relations:

```fga
type agent
  relations
    define principal: [user]    # informational: which user this agent acts for

type folder
  relations
    define owner: [user]
    define parent: [folder]
    define editor: [user, agent] or owner or editor from parent
    define viewer: [user, user:*, agent] or editor or viewer from parent

    define can_view:   viewer
    define can_edit:   editor
    define can_delete: editor
    define can_share:  owner
```

(`type file` mirrors `folder` for the same `can_*` definitions.)

Two ideas combined:

1. **Agents are principals, granted by role.** Both `editor` and `viewer` accept
   `[..., agent]`, so an agent can be assigned a role exactly like a user. No
   role on an object (or its ancestors) means no access — agents are denied by
   default.
2. **Subtree scoping comes for free.** `editor from parent` / `viewer from parent`
   means one grant `(agent:bot, editor, folder:projects)` covers every folder and
   file below it — and only those. Move a file elsewhere and the agent loses
   access, no app-code change needed.

The result: granting an agent `viewer` lets it read; granting `editor` lets it
read, write, and delete within that subtree. Sharing stays with owners and can
never be given to agents.

> The `agent.principal` relation records which user an agent acts for. It is for
> information only — useful for audit logs and app-side guardrails — and does not
> appear in any permission expression in this demo.

---

## The test scenario

[`tests.fga.yaml`](tests.fga.yaml) seeds:

```text
folder:root              ← user:alice  (owner)
│                        ← agent:reader (viewer — read-only, whole tree)
├── folder:projects      ← agent:scribe (editor — read/write/delete here)
│   └── file:report
└── file:secret

agent:scribe.principal = user:alice
agent:reader.principal = user:alice
```

Two agents, both delegated by alice:

- **`agent:scribe`** — `editor` scoped to `folder:projects/` (read, write, delete
  there; nothing on `folder:root` itself).
- **`agent:reader`** — `viewer` on `folder:root` (read everything under root,
  never write or delete).

Because `agent:scribe` has no role on `folder:root`, `file:secret` (which sits
directly under root) is off-limits to it.

---

## Walking through the tests

Run them:

```bash
fga model test --tests tests.fga.yaml
```

Expected: **5 tests passed, 0 failed**.

### Test 1 — Alice (human owner) keeps full access via cascade

```yaml
user: user:alice
object: file:report
assertions:
  can_view:   true
  can_edit:   true
  can_delete: true
  can_share:  false   # share requires a direct owner tuple on file:report
```

Alice is `owner` of `folder:root`, which passes `editor` down to `file:report`
via `editor from parent`. `can_share` is false because the share check needs a
direct `owner` tuple on the file itself — sharing does not flow down.

### Test 2 — agent:scribe can read, edit and delete in its subtree

```yaml
user: agent:scribe
object: file:report
assertions:
  can_view:   true
  can_edit:   true
  can_delete: true
  can_share:  false
```

`agent:scribe` is `editor` on `folder:projects`, which passes down to
`file:report`. The editor role carries view, edit, and delete. Sharing still
requires `owner`, so `can_share` is false.

### Test 3 — agent:scribe is denied by default outside its grant

```yaml
user: agent:scribe
object: file:secret
assertions:
  can_view:   false
  can_edit:   false
  can_delete: false
  can_share:  false
```

`file:secret` sits directly under `folder:root`. `agent:scribe` has no role on
root, and `user:*` (the viewer wildcard) does not match agents. So no path
applies — agents are denied by default in any scope they were not granted.

### Test 4 — agent:reader can read everything but never write

```yaml
- user: agent:reader
  object: file:report
  assertions:
    can_view:   true
    can_edit:   false
    can_delete: false
- user: agent:reader
  object: file:secret
  assertions:
    can_view:   true
    can_edit:   false
    can_delete: false
```

`agent:reader` is `viewer` on `folder:root`, so it can read both files via
`viewer from parent`. But `viewer` is not `editor`, so `can_edit` and
`can_delete` are false everywhere. This is the read-only-agent pattern: useful
for summarizers or search agents that should never mutate data.

### Test 5 — `list_objects` reflects each agent's role

```yaml
list_objects:
  - user: agent:scribe
    type: file
    assertions:
      can_edit: [file:report]
      can_view: [file:report]
  - user: agent:reader
    type: file
    assertions:
      can_edit: []
      can_view: [file:report, file:secret]
```

`list_objects(agent:scribe, file, can_edit)` returns only `file:report` — the
agent cannot even *find* files outside its grant. `agent:reader` can edit nothing
but can view both files. This matters for autonomy: an LLM-driven agent that asks
"which files can I edit?" gets back only the allowed set.

---

## What's different vs `../mcp-guide/`

| Concept                 | `../mcp-guide/`  | `ai-agent/` (this demo)                            |
| ----------------------- | ---------------- | -------------------------------------------------- |
| Principals              | `user`           | `user` + `agent`                                   |
| `can_edit` definition   | `editor`         | `editor` (same alias, now grantable to agents)     |
| `can_delete` definition | `editor`         | `editor`                                           |
| `can_share` definition  | `owner`          | `owner` (never granted to agents)                  |
| Role inheritance        | `from parent`    | `from parent` (bounds an agent to a subtree)       |
| Default for non-owners  | role-based       | role-based; agents denied by default where no role |

---

## Try changing it

Add a read-only agent under `folder:projects` only:

```yaml
# Add to tuples:
- user: agent:newbot
  relation: viewer
  object: folder:projects

# Add to tests:
- name: agent:newbot can read file:report but not file:secret, and never edits
  check:
    - user: agent:newbot
      object: file:report
      assertions:
        can_view:   true
        can_edit:   false
    - user: agent:newbot
      object: file:secret
      assertions:
        can_view:   false
        can_edit:   false
```

You should see **6 tests passed**.

---

## How to run

Install the OpenFGA CLI if you don't already have it:

```bash
brew install openfga/tap/fga
```

Then from this directory:

```bash
fga model test --tests tests.fga.yaml
```

Expected: 5/5 tests pass, exit code 0.
