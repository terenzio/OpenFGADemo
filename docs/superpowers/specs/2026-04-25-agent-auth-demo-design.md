# Design: Agent Authorization Demo

**Date:** 2026-04-25
**Location:** `agent_auth_demo/`

## Purpose

Demonstrate an OpenFGA authorization model for **AI agents acting on behalf of
users**, where destructive file operations require explicit authorization the
user did not necessarily grant by giving the agent a role.

The pattern: agents are first-class principals (just like users), but
destructive permissions (`can_edit`, `can_delete`) use **intersection** — being
an `editor` is not enough on its own. A separate capability relation
(`edit_authorized`, `delete_authorized`) must also be true. Humans get this
capability via a single global tuple (`user:*`); agents must be granted it
explicitly per-folder, with cascade down the subtree.

The result: agents have **default-deny** on destructive ops and **bounded
delegation** when granted — scoped to the folder subtree where the
authorization tuple lives. Sharing is reserved for owners only and never
delegable to agents.

## Non-goals

- No Go code, HTTP server, Docker, or seed binary. Pure model + tests + README.
- No conditions, custom roles, modules, or time-bounded grants.
- No richness from the parent/`mcp_demo/` demos: no organizations, no
  organization memberships, no comments/drafts/teams. Four types only.
- No "agent owns a file" or agent-to-agent delegation. Agents are leaves.

## The Model

```
model
  schema 1.1

type user

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

type file
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

### Why these choices

| Decision | Rationale |
|---|---|
| `agent` is a separate type, not a flag on `user` | Agents have a different trust profile and observability story. Modeling them as a distinct type lets policy diverge cleanly. |
| `agent` has `principal: [user]` | Records *which* user the agent acts for. Informational at the model level — useful for audit and for application-side guardrails. Not used in any permission expression. |
| `agent` appears directly in `editor`/`viewer` type lists | OpenFGA usersets resolve to users, so to make agents addressable as principals at all, they must be listed in the direct type sets. |
| `edit_authorized`/`delete_authorized` accept `[user:*, agent]` | One tuple `(user:*, edit_authorized, folder:root)` lets every human pass the gate at every depth. Agents need explicit per-folder grants. |
| Cascade via `or edit_authorized from parent` | Per-agent grants become subtree-scoped automatically. Granting `(agent:bot, edit_authorized, folder:projects)` covers everything under it. |
| `can_edit: editor and edit_authorized` | Intersection is what makes this default-deny for agents. The role alone is not enough; the capability gate must also pass. |
| `can_share: owner` (no intersection) | Sharing redistributes access. Agents must never be able to do this — even if a user added the agent as `editor`. |

### Comparison to `mcp_demo/`

| Concept | `mcp_demo/` | `agent_auth_demo/` |
|---|---|---|
| Principals | `user` only | `user` + `agent` |
| `can_edit` | `editor` (unconditional alias) | `editor and edit_authorized` (intersection) |
| `can_delete` | `editor` | `editor and delete_authorized` |
| `can_share` | `owner` | `owner` |
| Capability relations | none | `edit_authorized`, `delete_authorized` |
| Default posture for non-owners | role-based | role-based for users; default-deny for agents on destructive ops |

## Tuple set

```yaml
# Folder hierarchy
- folder:projects parent folder:root
- file:report     parent folder:projects
- file:secret     parent folder:root

# Human ownership: alice owns root, cascades down
- user:alice owner folder:root

# Agent delegation pointers (informational)
- user:alice principal agent:scribe
- user:alice principal agent:janitor

# Agents granted editor role
- agent:scribe  editor folder:projects
- agent:janitor editor folder:root

# Global gate: humans always authorized everywhere
- user:* edit_authorized   folder:root
- user:* delete_authorized folder:root

# Bounded agent grants
- agent:scribe  edit_authorized   folder:projects
- agent:janitor edit_authorized   folder:projects
- agent:janitor delete_authorized folder:projects
```

Note: `agent:scribe` is intentionally **not** granted `delete_authorized`
anywhere — to demonstrate that being `editor` does not imply `can_delete`.

## Test cases

Five tests, each exercising a distinct branch of the model:

1. **Alice can view, edit, delete, share `file:report`** — owner cascade plus
   the global `user:*` capability gates. Confirms humans are not penalized by
   the new intersection.

2. **`agent:scribe` can edit `file:report` but cannot delete it** —
   `editor` (cascaded from `folder:projects`) ✓; `edit_authorized` (cascaded
   from `folder:projects`) ✓; `delete_authorized` ✗. Demonstrates the
   intersection actually denies.

3. **`agent:scribe` cannot edit `file:secret`** — `file:secret` is parented
   by `folder:root`, not `folder:projects`. Scribe is neither `editor` there
   nor has `edit_authorized` there. Demonstrates scope is real.

4. **`agent:janitor` can delete `file:report` but cannot delete
   `file:secret`** — janitor's `delete_authorized` grant is on
   `folder:projects`, not `folder:root`. Even though janitor is `editor` on
   `folder:root`, they cannot delete files outside the `projects` subtree.
   Demonstrates bounded delegation.

5. **`list_objects` for `agent:scribe`, type=file, relation=`can_edit`** →
   `[file:report]`. Confirms enumeration honors the intersection: scribe is
   *editor* of more objects (via the `folder:projects` cascade), but only
   `file:report` also passes the `edit_authorized` gate.

## Artifacts

```
agent_auth_demo/
├── authorization-model.fga   # the model above
├── tests.fga.yaml            # the tuples + 5 tests above
└── README.md                 # below
```

### README outline

Four sections:

1. **What this is** — one paragraph framing the demo as "OpenFGA model for
   AI agents acting on behalf of users, with default-deny on destructive ops
   and bounded delegation via intersection." Contrasts briefly with
   `mcp_demo/`.
2. **The pattern** — explain the
   `can_edit: editor and edit_authorized` intersection, why it produces
   default-deny for agents while staying transparent for humans (the
   `user:*` global grant), and how `or edit_authorized from parent` makes
   per-folder grants scope automatically.
3. **What's different vs `mcp_demo/`** — the comparison table from the
   "Comparison" section above.
4. **How to run** — `brew install openfga/tap/fga` (if needed) then
   `cd agent_auth_demo && fga model test --tests tests.fga.yaml`.

## Testing methodology

The `tests.fga.yaml` file is the test suite. Running
`fga model test --tests tests.fga.yaml` typechecks the model, loads the
tuples, and evaluates every `check` and `list_objects` assertion. Non-zero
exit means failure.

**Done** means:

- `fga model test --tests tests.fga.yaml` exits 0
- All 5 test cases pass
- README renders cleanly and the run command works as written

## Open questions

None. Scope is fixed; no external dependencies beyond the `fga` CLI.
