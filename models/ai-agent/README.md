# Agent Authorization Demo

An OpenFGA model for **AI agents acting on behalf of users**. Here, risky
operations on files (`can_edit`, `can_delete`) need an extra capability. Giving
the agent an `editor` role is not enough. Risky ops are denied by default. They
are allowed only when granted, and only within set limits. `can_share` is kept
for owners and can never be given to agents.

It sits next to [`../mcp-guide/`](../mcp-guide/), which shows the basic OpenFGA
permission-alias pattern (`can_edit: editor`). This demo adds an
intersection-based capability gate on top.

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

1. **Intersection (`and`).** Being an `editor` is not enough on its own. A
   separate capability relation must also be true. If either side is missing, the
   action is denied.
2. **Different default for users vs. agents.** The seed tuple
   `(user:*, edit_authorized, folder:root)` lets every human pass the gate
   everywhere. So humans are not affected by the new check. `user:*` does **not**
   match agents. So agents start with no capability and need an explicit grant
   per folder.
3. **Subtree scoping comes for free.** `or edit_authorized from parent` means one
   grant `(agent:bot, edit_authorized, folder:projects)` covers every folder and
   file below it — and only those. Move the file elsewhere, and it loses the
   capability.

The result: giving an agent `editor` on a folder lets it view content. But to
let it write or delete, you must add a separate grant for each capability.
Sharing stays with owners and can never be given to agents.

> The `agent.principal` relation records which user an agent acts for. It is for
> information only — useful for audit logs and app-side guardrails — and does not
> appear in any permission expression in this demo.

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

- **`agent:scribe`** — edit only, scoped to `folder:projects/`.
- **`agent:janitor`** — edit and delete, scoped to `folder:projects/`.

Neither agent has any capability on `folder:root` itself. So `file:secret` (which
sits directly under root) is off-limits to both.

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

Alice is `owner` of `folder:root`. That passes `editor` down to `file:report` via
`editor from parent`. The seeds `(user:*, edit_authorized, folder:root)` and
`(user:*, delete_authorized, folder:root)` cover the right side of each
intersection. `can_share` is false because the share check needs a direct `owner`
tuple on the file itself — sharing does not flow down.

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

`agent:scribe` is `editor` on `folder:projects` (which passes down to
`file:report`) and has `edit_authorized` on `folder:projects`. Both sides of
`editor and edit_authorized` are true → `can_edit` is true. But scribe has **no**
`delete_authorized` tuple anywhere, so `can_delete` fails the intersection.

This is the key case: giving an agent `editor` is not enough on its own. You must
add the second tuple yourself, one per capability.

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

`file:secret` sits directly under `folder:root`. `agent:scribe` has no `editor`
relation on root, and `user:*` does not match agents. So neither path applies.
Agents are denied by default in any scope they were not granted.

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

The same agent that can delete `file:report` cannot touch `file:secret`, even
though both files have the same human owner. The capability tuple is scoped to a
subtree, and OpenFGA enforces that scope on every Check. Move a file from
`folder:projects` to under `folder:root`, and the agent quietly loses access — no
app code change needed.

### Test 5 — `list_objects` enforces the intersection during enumeration

```yaml
list_objects:
  - user: agent:scribe
    type: file
    assertions:
      can_edit:   [file:report]
      can_delete: []
```

`list_objects(agent:scribe, file, can_edit)` returns only `file:report`. The
agent cannot even *find* files outside its grant. This matters for autonomy: an
LLM-driven agent that asks "which files can I edit?" gets back only the allowed
set, never `file:secret`.

---

## What's different vs `../mcp-guide/`

| Concept                 | `../mcp-guide/`                  | `ai-agent/` (this demo)                                          |
| ----------------------- | -------------------------------- | ---------------------------------------------------------------- |
| Principals              | `user`                           | `user` + `agent`                                                 |
| `can_edit` definition   | `editor`                         | `editor and edit_authorized`                                     |
| `can_delete` definition | `editor`                         | `editor and delete_authorized`                                   |
| `can_share` definition  | `owner`                          | `owner` (unchanged)                                              |
| Capability relations    | none                             | `edit_authorized`, `delete_authorized`                           |
| Default for non-owners  | role-based                       | role-based for users; agents denied by default on risky ops      |

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
