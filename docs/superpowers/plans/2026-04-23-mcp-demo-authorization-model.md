# MCP-Assisted Doc-Mgmt Authorization Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Populate the empty `mcp_demo/` directory with an OpenFGA authorization model, a `.fga.yaml` test file, and a README — authored under guidance from the `openfga-mcp` MCP server — showing how MCP-assisted modeling extends the parent repo's hand-written model with explicit `can_*` permission relations.

**Architecture:** Three static artifacts, no build step. The `.fga` model file is a pure extension of `model/authorization-model.fga` (adds `can_view`, `can_edit`, `can_delete`, `can_share` on `folder` and `document`). The `.fga.yaml` file declares inline tuples mirroring the parent's seed data and runs assertions via `fga model test`. The README narrates the MCP workflow and diffs the two models.

**Tech Stack:** OpenFGA DSL, `fga` CLI (for validation/testing), YAML. No new language runtime; does not require Docker/MariaDB/OpenFGA server.

**Spec:** `docs/superpowers/specs/2026-04-23-mcp-demo-authorization-model-design.md`

**Working directory for all `fga` commands:** `/Users/terence/Documents/GitHub/OpenFGADemo/mcp_demo`

---

## Prerequisite: Verify `fga` CLI is installed

- [ ] **Step 0.1: Check whether `fga` is on PATH**

Run: `fga version`

Expected: a version string like `v0.x.y`. If the command is not found, install:

```bash
brew install openfga/tap/fga
```

Then re-run `fga version` to confirm.

---

## File Structure

Three files will be created in `/Users/terence/Documents/GitHub/OpenFGADemo/mcp_demo/`:

| File | Responsibility |
|---|---|
| `authorization-model.fga` | OpenFGA DSL model: `user`, `organization`, `folder`, `document` types, plus `can_*` permission relations on `folder` and `document`. |
| `tests.fga.yaml` | Inline tuple fixtures + `check` / `list_objects` assertions. Consumed by `fga model test`. |
| `README.md` | Narrative: what the demo is, which MCP context was queried, diff vs parent model, and how to run tests. |

---

## Task 1: Scaffold the model (parent-equivalent relations only)

Goal: produce a `.fga` file that validates and matches the parent repo's model byte-for-byte in semantics (before adding `can_*`). This establishes the floor the demo extends from.

**Files:**
- Create: `mcp_demo/authorization-model.fga`

- [ ] **Step 1.1: Create `mcp_demo/authorization-model.fga` with parent-equivalent content**

File content:

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

type document
  relations
    define parent: [folder]
    define owner: [user]
    define editor: [user, organization#member] or owner or editor from parent
    define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

- [ ] **Step 1.2: Validate the DSL**

Run (from repo root):

```bash
fga model validate --file mcp_demo/authorization-model.fga
```

Expected: the CLI prints `{"is_valid":true}` (or equivalent "model is valid" output) and exits 0. If the CLI complains about syntax, compare the file to `model/authorization-model.fga` — they should be identical except this one lives under `mcp_demo/`.

- [ ] **Step 1.3: Commit**

```bash
git add mcp_demo/authorization-model.fga
git commit -m "feat(mcp_demo): scaffold parent-equivalent authorization model"
```

---

## Task 2: Add `can_*` permission relations

Goal: extend the model with the four `can_*` permissions recommended by the MCP guide (section 6, "Adding permissions"). This is the *one semantic change* vs parent.

**Files:**
- Modify: `mcp_demo/authorization-model.fga`

- [ ] **Step 2.1: Add `can_*` relations to `folder`**

Replace the `folder` block with:

```
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
```

- [ ] **Step 2.2: Add `can_*` relations to `document`**

Replace the `document` block with:

```
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

- [ ] **Step 2.3: Validate the updated DSL**

Run:

```bash
fga model validate --file mcp_demo/authorization-model.fga
```

Expected: `{"is_valid":true}` / exits 0. If invalid, the most likely issue is indentation — `can_*` lines must be indented the same as `define editor`/`viewer`.

- [ ] **Step 2.4: Commit**

```bash
git add mcp_demo/authorization-model.fga
git commit -m "feat(mcp_demo): add can_view/can_edit/can_delete/can_share permissions"
```

---

## Task 3: Create test file with tuples + first test (TDD smoke)

Goal: Establish the `.fga.yaml` test harness with seed tuples and one passing smoke test that exercises all four new `can_*` permissions for an owner.

**Files:**
- Create: `mcp_demo/tests.fga.yaml`

- [ ] **Step 3.1: Write the test file**

Create `mcp_demo/tests.fga.yaml` with this exact content.

Note on the expected assertions below: alice is owner of `folder:company`, which is the top of the parent chain to `document:roadmap`. She gets `can_view/can_edit/can_delete` via the role-implication + `editor from parent` chain. But `can_share: owner` depends on a **direct** `(user, owner, object)` tuple, and alice has no such tuple on `document:roadmap` — so `can_share` is false.

```yaml
name: Document Management Authorization Tests
model_file: ./authorization-model.fga

tuples:
  # Organization membership
  - user: user:eve
    relation: member
    object: organization:acme

  # Folder & document hierarchy
  - user: folder:company
    relation: parent
    object: folder:product
  - user: folder:product
    relation: parent
    object: document:roadmap

  # Ownership & direct assignments
  - user: user:alice
    relation: owner
    object: folder:company
  - user: user:charlie
    relation: editor
    object: folder:product
  - user: organization:acme#member
    relation: viewer
    object: folder:product

  # Wildcard public document
  - user: user:*
    relation: viewer
    object: document:public-memo

tests:
  - name: Owner of top folder gets can_view/edit/delete via parent chain, but not can_share (share requires direct owner tuple)
    check:
      - user: user:alice
        object: document:roadmap
        assertions:
          can_view: true
          can_edit: true
          can_delete: true
          can_share: false
```

- [ ] **Step 3.2: Run the test file**

Run from repo root:

```bash
cd mcp_demo && fga model test --tests tests.fga.yaml
```

Expected: **1 test passed, 0 failed**, exit 0. If this fails, the most likely causes are:
- YAML indentation (all tuples must align under `tuples:`, all tests under `tests:`).
- `can_share: false` mis-specified as `true` — re-read the semantic note above.

- [ ] **Step 3.3: Commit**

```bash
git add mcp_demo/tests.fga.yaml
git commit -m "test(mcp_demo): add seed tuples and owner smoke test"
```

---

## Task 4: Add remaining four test cases

Goal: cover TupleToUserset via org membership, direct editor cascade, wildcard, and `list_objects`. Append each as a separate test entry; run the full suite after each add.

**Files:**
- Modify: `mcp_demo/tests.fga.yaml`

- [ ] **Step 4.1: Add the org-member test**

Append this entry to the `tests:` list (after the existing first test):

```yaml
  - name: Org member inherits viewer via folder parent (TupleToUserset across types)
    check:
      - user: user:eve
        object: document:roadmap
        assertions:
          can_view: true
          can_edit: false
          can_share: false
```

Rationale: eve is member of `organization:acme`; `organization:acme#member` is a direct viewer of `folder:product`; `document:roadmap` inherits that viewer via `viewer from parent`. She is not an editor or owner, so `can_edit` and `can_share` are false.

- [ ] **Step 4.2: Run tests, confirm 2 pass**

Run:

```bash
cd mcp_demo && fga model test --tests tests.fga.yaml
```

Expected: **2 tests passed, 0 failed**, exit 0.

- [ ] **Step 4.3: Add the direct-editor-on-parent-folder test**

Append:

```yaml
  - name: Direct editor on parent folder cascades can_edit and can_delete to child doc
    check:
      - user: user:charlie
        object: document:roadmap
        assertions:
          can_edit: true
          can_delete: true
          can_share: false
```

Rationale: charlie is `(user:charlie, editor, folder:product)`; `document:roadmap.editor` is derived via `editor from parent`. `can_delete = editor` → true. `can_share = owner` → false (charlie isn't an owner anywhere in the chain).

- [ ] **Step 4.4: Run, confirm 3 pass**

```bash
cd mcp_demo && fga model test --tests tests.fga.yaml
```

Expected: **3 tests passed, 0 failed**, exit 0.

- [ ] **Step 4.5: Add the wildcard test**

Append:

```yaml
  - name: Wildcard grants can_view publicly but not can_edit
    check:
      - user: user:randomstranger
        object: document:public-memo
        assertions:
          can_view: true
          can_edit: false
```

Rationale: `(user:*, viewer, document:public-memo)` matches any user type (including randomstranger) for `viewer`, hence `can_view`. But `editor` doesn't have `user:*` in its type list, so `can_edit` is false.

- [ ] **Step 4.6: Run, confirm 4 pass**

```bash
cd mcp_demo && fga model test --tests tests.fga.yaml
```

Expected: **4 tests passed, 0 failed**, exit 0.

- [ ] **Step 4.7: Add the `list_objects` test**

Append:

```yaml
  - name: list_objects — which documents can alice view
    list_objects:
      - user: user:alice
        type: document
        assertions:
          can_view:
            - document:roadmap
            - document:public-memo
```

Rationale: alice inherits viewer on `document:roadmap` via the folder chain, and anyone gets viewer on `document:public-memo` via the wildcard. No other documents exist in the tuple set.

- [ ] **Step 4.8: Run full suite, confirm 5 pass**

```bash
cd mcp_demo && fga model test --tests tests.fga.yaml
```

Expected: **5 tests passed, 0 failed**, exit 0.

If `list_objects` fails with an ordering mismatch, note that `fga` compares the list as a set, not as an ordered sequence — any ordering error indicates a real missing/extra object.

- [ ] **Step 4.9: Commit**

```bash
git add mcp_demo/tests.fga.yaml
git commit -m "test(mcp_demo): add org-member, editor-cascade, wildcard, list_objects cases"
```

---

## Task 5: Write the README

Goal: produce `mcp_demo/README.md` narrating the MCP workflow, the diff vs the parent model, and how to run the tests.

**Files:**
- Create: `mcp_demo/README.md`

- [ ] **Step 5.1: Create `mcp_demo/README.md`**

File content:

```markdown
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
```

- [ ] **Step 5.2: Sanity-check the README**

Verify:
- All links (`../model/authorization-model.fga`, `../docs/model-explained.md`, `../README.md`) resolve to existing files.
- The `fga model test` command in the README matches what Task 4 verified.
- The "5 tests passed" claim matches reality: re-run once more from repo root:

```bash
cd mcp_demo && fga model test --tests tests.fga.yaml
```

Expected: **5 tests passed, 0 failed**, exit 0.

- [ ] **Step 5.3: Commit**

```bash
git add mcp_demo/README.md
git commit -m "docs(mcp_demo): add README with MCP workflow narrative and usage"
```

---

## Task 6: Final verification

- [ ] **Step 6.1: Confirm the directory is complete**

Run:

```bash
ls -1 mcp_demo/
```

Expected output (order may vary):

```
README.md
authorization-model.fga
tests.fga.yaml
```

- [ ] **Step 6.2: Run the full test suite one last time**

```bash
cd mcp_demo && fga model test --tests tests.fga.yaml
```

Expected: **5 tests passed, 0 failed**, exit 0.

- [ ] **Step 6.3: Confirm git status is clean**

Run:

```bash
git status
```

Expected: either clean working tree, or only the pre-existing `docker-compose.yml` modification that was present before this work started. Nothing under `mcp_demo/` should be untracked or unstaged.

---

## Self-review notes (for the executor)

- Task 1 validates the model **before** adding `can_*` to catch any DSL mistakes independently from the additions in Task 2. If Task 2's validation fails, you know the `can_*` block is to blame.
- Task 3's one non-obvious assertion is `can_share: false` for alice on `document:roadmap`. The note in Step 3.1 explains why (direct-owner-only). If you're tempted to "fix" this to `true`, re-read that note first.
- The README's "5 tests passed" count must match the test file. If any test is removed or added during execution, update the README accordingly.
