# MCP-Assisted OpenFGA Modeling Demo

This directory contains a document-management authorization model authored
with guidance from the [`openfga-mcp`](https://github.com/openfga) MCP server.
It sits beside the parent repo's hand-written model
([`model/authorization-model.fga`](../model/authorization-model.fga)) as a
comparison: **what does MCP-assisted modeling add that a human working from
scratch might miss?**

Short answer: explicit `can_*` permission relations.

---

## What's here

| File | Purpose |
|---|---|
| `authorization-model.fga` | DSL model. Same types and base relations as parent, plus four `can_*` permissions. |
| `tests.fga.yaml` | Inline tuples + `check` and `list_objects` assertions. Run with `fga model test`. |
| `README.md` | This file. |

---

## The MCP workflow

The `openfga-mcp` server exposes a single context prompt:

```
Available Context Prompts:

**Author authorization models with OpenFGA**
File: authorization-model.md
Patterns: authorization model, auth model, access control, rbac, ...
```

A single query pulled in the full authoring guide:

```
mcp__openfga-mcp__get_context_for_query(
  query="authorization model for a document management system"
)
```

The returned document is an end-to-end playbook: DSL syntax, relationship
patterns (direct / concentric / `X from Y`), testing via `.fga.yaml`, and —
most relevant to this demo — **section 6: "Adding permissions"**, which
states:

> It's a common practice to define specific permissions, that can't be
> directly assigned, using `can_<permission>` relations. [...] **Always define
> permissions in the authorization models.**

The parent repo's model predates this guidance and does not follow it.

---

## What's different from the parent model

```diff
  type folder
    relations
      define owner: [user]
      define parent: [folder]
      define editor: [user, organization#member] or owner or editor from parent
      define viewer: [user, user:*, organization#member] or editor or viewer from parent
+
+     define can_view:   viewer
+     define can_edit:   editor
+     define can_delete: editor
+     define can_share:  owner
```

(Identical block added to `type document`.)

**Why this matters:** application code should check *intent* (`can_edit`),
not *role* (`editor`). The parent repo's HTTP handlers call `Check(..., "editor", ...)` directly — if tomorrow you wanted viewers to also be able to
edit in some cases, you'd have to change every call site. With `can_edit` as
a first-class relation, you change one line in the model.

| Permission | Implied by | Rationale |
|---|---|---|
| `can_view` | `viewer` | Public reads, including wildcard and org-member inheritance. |
| `can_edit` | `editor` | Standard edit; inherits via role implication from `owner` and via `editor from parent`. |
| `can_delete` | `editor` | Deletion is edit-level, not owner-level — matches the MCP guide's illustrative example. |
| `can_share` | `owner` | Sharing grants access to others. Restricted to direct owners only (does **not** inherit via parent chain). This matches the parent repo's `/share` endpoint, which only makes sense for direct owners. |

---

## Running the tests

Prerequisite: the [OpenFGA CLI](https://openfga.dev/docs/getting-started/install-sdk).

```bash
brew install openfga/tap/fga    # macOS
# or see the OpenFGA docs for other platforms
```

Run the test suite:

```bash
cd mcp_demo
fga model test --tests tests.fga.yaml
```

Expected output: **5 tests passed, 0 failed**.

The test file contains five cases covering:

1. Owner of a grand-parent folder gets `can_view/edit/delete` on a grandchild
   doc via the parent chain — but **not** `can_share` (sharing requires
   direct ownership).
2. Organization member gets `can_view` via `organization#member` → folder
   viewer → document viewer inheritance (TupleToUserset across three type
   boundaries).
3. Direct editor on a parent folder gets `can_edit` and `can_delete`
   cascaded to child documents.
4. Wildcard viewer tuple grants `can_view` to any user, but not `can_edit`.
5. `list_objects` returns all documents a given user can view.

---

## Further reading

- Parent repo's annotated model walkthrough: [`docs/model-explained.md`](../docs/model-explained.md)
- Parent repo's HTTP server and CLI: [`README.md`](../README.md)
- [OpenFGA DSL reference](https://openfga.dev/docs/configuration-language)
- [`fga model test` documentation](https://openfga.dev/docs/getting-started/cli)
