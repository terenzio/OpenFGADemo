# Design: MCP-Assisted Authorization Model Demo

**Date:** 2026-04-23
**Location:** `mcp_demo/`

## Purpose

Demonstrate authoring an OpenFGA authorization model for a document management
system **with guidance from the `openfga-mcp` MCP server**. The artifact is
self-contained inside `mcp_demo/` and sits alongside the parent repo's
hand-written model as a comparison: what does MCP-assisted modeling add that a
human working from scratch might miss?

The demo intentionally targets the **same scenario** as the parent
(`model/authorization-model.fga`) so the comparison is direct, and extends it
in the specific way the MCP guide recommends: explicit `can_*` permission
relations, which the parent model does not have.

## Non-goals

- No Go code, HTTP server, Docker setup, or seed binary. Those already exist
  in the parent repo.
- No new scenarios (comments, drafts, teams). Same four types as parent.
- No conditions, custom roles, or modules. The MCP guide flags all three as
  opt-in ("only use if explicitly asked"), and they add complexity without
  advancing the demo's point.

## Artifacts

Three files in `mcp_demo/`:

```
mcp_demo/
├── authorization-model.fga   # DSL model: parent model + can_* permissions
├── tests.fga.yaml            # inline tuples + check / list_objects assertions
└── README.md                 # MCP-workflow narrative, diff vs parent, how to run
```

No build step, no dependencies beyond the `fga` CLI.

## The Model

Identical type and base-relation structure to the parent. The only additions
are four `can_*` permission relations on `folder` and `document`:

```
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

    define can_view:   viewer
    define can_edit:   editor
    define can_delete: editor
    define can_share:  owner

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

### Why these four permissions

| Relation | Maps to | Justification |
|---|---|---|
| `can_view` | `viewer` | Trivial alias, but forces application code to check intent ("can I view?") not role ("am I a viewer?") — the point MCP guide section 6 makes. |
| `can_edit` | `editor` | Same as above. |
| `can_delete` | `editor` | Editors can delete content they can edit. Matches MCP guide's illustrative example. |
| `can_share` | `owner` | Sharing grants access to others — a privileged action. Matches the parent repo's `/share` endpoint, which only owners should be able to call. |

### Backward compatibility vs parent

Purely additive. All existing relations (`owner`, `editor`, `viewer`,
`parent`, `member`) keep the same definitions, so any tuples written against
the parent model remain valid. TupleToUserset (`editor from parent`), userset
references (`organization#member`), and wildcards (`user:*`) all keep
working unchanged.

## The Tests

`tests.fga.yaml` mirrors the parent repo's seed tuples so the test narrative
connects to `docs/model-explained.md`'s worked examples. Five test cases
exercise every major concept:

1. **Owner has all four permissions** — smoke test for the role-implication
   chain `owner → editor → viewer`.
2. **Org member inherits viewer via folder parent** — TupleToUserset across
   type boundaries (the "Eve sees roadmap" example from parent's
   `model-explained.md`).
3. **Direct editor on parent folder cascades to child document** —
   `editor from parent`.
4. **Wildcard grants public view but not edit** — `user:*`.
5. **`list_objects` for all documents alice can view** — demonstrates the
   `list_objects` test form the MCP guide describes.

See the full file content in the implementation plan.

### Tuple set

Reuses the parent's seed data (visible at runtime today via `make seed`):

- `user:alice` owns `folder:company`
- `folder:product` parents `folder:company`; `document:roadmap` parents
  `folder:product`
- `user:charlie` is a direct editor of `folder:product`
- `organization:acme#member` is a viewer of `folder:product`
- `user:eve` is a member of `organization:acme`
- `user:*` is a viewer of `document:public-memo`

## The README

Four sections:

1. **What this is** — one paragraph framing the demo as "author an OpenFGA
   doc-mgmt model with MCP guidance, in contrast to the parent repo's
   hand-written model."
2. **MCP context used** — names the `authorization-model.md` context prompt
   returned by `openfga-mcp`, and the single query that fetched it
   (`"authorization model for a document management system"`).
3. **What changed vs parent** — a short diff table: parent lacks `can_*`, MCP
   guide section 6 says *"Always define permissions in the authorization
   models,"* we added four. Links back to `model/authorization-model.fga`.
4. **How to run** — `brew install openfga/tap/fga` then `cd mcp_demo &&
   fga model test --tests tests.fga.yaml`.

No separate "architecture" or "model explained" section — parent's
`docs/model-explained.md` already covers the underlying ReBAC concepts, and
the README links to it.

## Testing methodology

The `.fga.yaml` file itself is the test suite. Running `fga model test
--tests tests.fga.yaml` is the verification step — it typechecks the model,
loads the tuples, and evaluates every `check` and `list_objects` assertion.
If any fails, the CLI exits non-zero.

"Done" for this project means:

- `fga model test --tests tests.fga.yaml` exits 0
- All five test cases pass
- README renders cleanly and commands in it work as written

## Open questions

None. Scope is fixed; no external dependencies or integrations beyond the
`fga` CLI.
