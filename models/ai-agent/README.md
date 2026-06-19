# Agent Authorization Demo

An OpenFGA model for **AI agents acting on behalf of users**, where
destructive operations on files (`can_edit`, `can_delete`) require a
capability the user did not grant simply by giving the agent an
`editor` role. Default-deny on destructive ops; bounded delegation
when granted; `can_share` reserved for owners and never delegable to
agents.

This sits alongside
[`../mcp-guide/`](../mcp-guide/),
which demonstrates the basic OpenFGA permission-alias pattern
(`can_edit: editor`). This demo extends that with an
intersection-based capability gate.

---

## The model

[`authorization-model-ai-agent.fga`](authorization-model-ai-agent.fga) — key relations:

```fga
type agent
  relations
    define principal: [user]    # informational: which user this agent acts for

type folder
  relations
    define editor: [user, agent] or owner or editor from parent

    define edit_authorized:   [user:*, agent] or edit_authorized from parent
    define delete_authorized: [user:*, agent] or delete_authorized from parent

    define can_view:   viewer
    define can_edit:   editor and edit_authorized
    define can_delete: editor and delete_authorized
    define can_share:  owner
```

(`type file` mirrors `folder` for the same `can_*` definitions.)

Three ideas combined:

1. **Intersection (`and`).** Being an `editor` is not enough on its
   own. A separate capability relation must also evaluate to true.
   If either side is missing, the action is denied.
2. **Different default for users vs. agents.** The seed tuple
   `(user:*, edit_authorized, folder:root)` makes every human pass
   the gate everywhere — humans are unaffected by the new check.
   `user:*` does **not** match agents, so agents start with no
   capability and need an explicit per-folder grant.
3. **Subtree scoping comes for free.** `or edit_authorized from parent`
   means a single grant `(agent:bot, edit_authorized, folder:projects)`
   covers every descendant — and only those descendants. Move the
   file somewhere else, lose the capability.

The result: granting an agent `editor` on a folder allows it to view
content, but a separate explicit grant per capability is required to
let it write or delete. Sharing is reserved for owners and is never
delegable to agents at all.

> The `agent.principal` relation records which user an agent acts
> for. It's informational only — useful for audit and
> application-side guardrails — and does not appear in any permission
> expression in this demo.

---

## The test scenario

[`tests.fga.yaml`](tests.fga.yaml) seeds:

```text
folder:root              ← user:alice (owner)
├── folder:projects      ← agent:scribe   (editor + edit_authorized)
│   │                    ← agent:janitor  (editor + edit_authorized + delete_authorized)
│   └── file:report
└── file:secret

agent:scribe.principal  = user:alice
agent:janitor.principal = user:alice

(user:*, edit_authorized,   folder:root)   ← humans always pass the gate
(user:*, delete_authorized, folder:root)
```

Two agents, both delegated by alice:

- **`agent:scribe`** — edit-only, scoped to `folder:projects/`.
- **`agent:janitor`** — edit + delete, scoped to `folder:projects/`.

Neither agent has any capability on `folder:root` itself, so
`file:secret` (which lives directly under root) is off-limits to both.

---

## Walking through the tests

Run them:

```bash
fga model test --tests tests.fga.yaml
```

Expected: **5 tests passed, 0 failed**.

### Test 1 — Alice (human owner) is unaffected by the gate

```yaml
user: user:alice
object: file:report
assertions:
  can_view:   true
  can_edit:   true
  can_delete: true
  can_share:  false   # share requires a direct owner tuple on file:report
```

Alice is `owner` of `folder:root`, which cascades `editor` down to
`file:report` via `editor from parent`. The seeds
`(user:*, edit_authorized, folder:root)` and
`(user:*, delete_authorized, folder:root)` satisfy the right side of
each intersection. `can_share` is false because the share check
requires a direct `owner` tuple on the file itself — sharing does not
cascade.

### Test 2 — agent:scribe can edit but not delete

```yaml
user: agent:scribe
object: file:report
assertions:
  can_view:   true
  can_edit:   true
  can_delete: false   # no delete_authorized for scribe
  can_share:  false
```

`agent:scribe` is `editor` on `folder:projects` (cascades to
`file:report`) and has `edit_authorized` on `folder:projects`. Both
sides of `editor and edit_authorized` are satisfied → `can_edit` is
true. But scribe has **no** `delete_authorized` tuple anywhere, so
`can_delete` fails the intersection.

This is the headline case: giving an agent `editor` is not enough by
itself. The second tuple must be added explicitly, per capability.

### Test 3 — agent:scribe can't even see file:secret

```yaml
user: agent:scribe
object: file:secret
assertions:
  can_view:   false
  can_edit:   false
  can_delete: false
  can_share:  false
```

`file:secret` lives directly under `folder:root`. `agent:scribe` has
no `editor` relation on root, and `user:*` does not match agents, so
neither path applies. Default-deny for agents in scope they were not
granted.

### Test 4 — agent:janitor's delete is bounded to the projects subtree

```yaml
- user: agent:janitor
  object: file:report
  assertions:
    can_edit:   true
    can_delete: true       # has delete_authorized on folder:projects
- user: agent:janitor
  object: file:secret
  assertions:
    can_edit:   false      # no editor on root
    can_delete: false      # no delete_authorized on root
```

The same agent that can delete `file:report` cannot touch
`file:secret`, even though both files share the same human owner. The
capability tuple is scoped to a subtree, and the OpenFGA evaluator
enforces that scope on every Check. Move a file from `folder:projects`
to under `folder:root` and the agent silently loses access — no app
code change required.

### Test 5 — `list_objects` enforces the intersection during enumeration

```yaml
list_objects:
  - user: agent:scribe
    type: file
    assertions:
      can_edit:   [file:report]
      can_delete: []
```

`list_objects(agent:scribe, file, can_edit)` returns only
`file:report` — the agent cannot even *discover* files outside its
grant. This matters for autonomy: an agent driven by an LLM that asks
"which files can I edit?" gets back only the bounded set, never
`file:secret`.

---

## What's different vs `../mcp-guide/`

| Concept                 | `../mcp-guide/`                  | `ai-agent/` (this demo)                                          |
| ----------------------- | -------------------------------- | ---------------------------------------------------------------- |
| Principals              | `user`                           | `user` + `agent`                                                 |
| `can_edit` definition   | `editor`                         | `editor and edit_authorized`                                     |
| `can_delete` definition | `editor`                         | `editor and delete_authorized`                                   |
| `can_share` definition  | `owner`                          | `owner` (unchanged)                                              |
| Capability relations    | none                             | `edit_authorized`, `delete_authorized`                           |
| Default for non-owners  | role-based                       | role-based for users; default-deny for agents on destructive ops |

---

## Try changing it

Add a one-shot, edit-only agent under root:

```yaml
# Add to tuples:
- user: agent:newbot
  relation: editor
  object: folder:root
- user: agent:newbot
  relation: edit_authorized
  object: folder:root

# Add to tests:
- name: agent:newbot can edit anywhere but never delete
  check:
    - user: agent:newbot
      object: file:report
      assertions:
        can_edit:   true
        can_delete: false
    - user: agent:newbot
      object: file:secret
      assertions:
        can_edit:   true
        can_delete: false
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
