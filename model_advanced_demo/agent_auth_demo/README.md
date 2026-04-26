# Agent Authorization Demo

An OpenFGA model for **AI agents acting on behalf of users**, where
destructive operations on files (`can_edit`, `can_delete`) require a
capability the user did not grant simply by giving the agent an `editor`
role. Default-deny on destructive ops; bounded delegation when granted;
`can_share` reserved for owners and never delegable to agents.

This sits alongside [`../mcp_demo/`](../mcp_demo/), which demonstrates the
basic OpenFGA permission-alias pattern (`can_edit: editor`). This demo
extends that with an intersection-based capability gate.

---

## The pattern

The key relations on `folder` and `file`:

```fga
define editor: [user, agent] or owner or editor from parent

define edit_authorized:   [user:*, agent] or edit_authorized from parent
define delete_authorized: [user:*, agent] or delete_authorized from parent

define can_edit:   editor and edit_authorized
define can_delete: editor and delete_authorized
define can_share:  owner
```

Three ideas combined:

1. **Intersection (`and`).** Being an `editor` is not enough on its own.
   A separate capability relation must also evaluate to true. If either
   side is missing, the action is denied.
2. **Different default for users vs. agents.** The seed tuple
   `(user:*, edit_authorized, folder:root)` makes every human pass the
   gate everywhere — humans are unaffected by the new check.
   `user:*` does **not** match agents, so agents start with no
   capability and need an explicit per-folder grant.
3. **Subtree scoping comes for free.** `or edit_authorized from parent`
   means a single grant `(agent:bot, edit_authorized, folder:projects)`
   covers every descendant — and only those descendants. Move the file
   somewhere else, lose the capability.

> Note: the model also declares `agent.principal: [user]` to record which
> user an agent acts for. It is informational only — useful for audit and
> application-side guardrails — and does not appear in any permission
> expression in this demo.

The result: granting an agent `editor` on a folder allows it to view
content, but a separate explicit grant per capability is required to
let it write or delete. Sharing is reserved for owners and is never
delegable to agents at all.

---

## What's different vs `mcp_demo/`

| Concept                | `mcp_demo/`                | `agent_auth_demo/`                       |
|------------------------|----------------------------|------------------------------------------|
| Principals             | `user`                     | `user` + `agent`                         |
| `can_edit` definition  | `editor`                   | `editor and edit_authorized`             |
| `can_delete` definition| `editor`                   | `editor and delete_authorized`           |
| `can_share` definition | `owner`                    | `owner` (unchanged)                      |
| Capability relations   | none                       | `edit_authorized`, `delete_authorized`   |
| Default for non-owners | role-based                 | role-based for users; default-deny for agents on destructive ops |

---

## How to run

Install the OpenFGA CLI if you don't already have it:

```bash
brew install openfga/tap/fga
```

Then from this directory:

```bash
cd agent_auth_demo
fga model test --tests tests.fga.yaml
```

Expected: 5/5 tests pass, exit code 0.
