# Agent Authorization Demo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a self-contained OpenFGA demo at `agent_auth_demo/` showing how AI agents acting on behalf of users get default-deny on destructive file ops via intersection-based authorization, with humans transparently authorized via `user:*`.

**Architecture:** Three files (`authorization-model.fga`, `tests.fga.yaml`, `README.md`). The model defines four types (`user`, `agent`, `folder`, `file`) with capability-gated permissions: `can_edit: editor and edit_authorized`, `can_delete: editor and delete_authorized`. A single `(user:*, edit_authorized, folder:root)` tuple authorizes every human; per-agent grants are bounded to a folder subtree via `or edit_authorized from parent` cascade. Tests are written iteratively — one new check per task, run after each addition to verify the model behaves as specified.

**Tech Stack:** OpenFGA DSL (`.fga`), OpenFGA YAML test format (`.fga.yaml`), `fga` CLI (`brew install openfga/tap/fga`).

**Spec:** `docs/superpowers/specs/2026-04-25-agent-auth-demo-design.md`

---

## Prerequisites for the implementer

- The `fga` CLI must be installed. Verify with `fga version`. If missing: `brew install openfga/tap/fga`.
- All commands in this plan are run from the repo root (`/Users/terence/Documents/GitHub/OpenFGADemo`) unless stated otherwise.
- The directory `agent_auth_demo/` does not exist yet at plan-start; Task 1 creates it.
- TDD note: in OpenFGA, the model file IS the implementation. Tests live in `tests.fga.yaml`. Each task adds one test, runs `fga model test`, and commits only when green. There is no "implementation step" separate from "the model" — the model is written once in Task 1 and validated by every subsequent test.
- The model in Task 1 is complete and final. Subsequent tasks only add tuples/tests; they do not modify the model.

---

## File Structure

| Path | Created in | Responsibility |
|---|---|---|
| `agent_auth_demo/authorization-model.fga` | Task 1 | The OpenFGA model: 4 types, intersection-based permissions. |
| `agent_auth_demo/tests.fga.yaml` | Task 2 (created); Tasks 3–6 (extend tests) | Inline tuples + 5 test cases. |
| `agent_auth_demo/README.md` | Task 7 | What the demo is, the pattern, comparison with `mcp_demo/`, run command. |

---

### Task 1: Create the authorization model

**Files:**
- Create: `agent_auth_demo/authorization-model.fga`

- [ ] **Step 1: Create the demo directory**

Run from repo root:
```bash
mkdir -p agent_auth_demo
```
Expected: command succeeds; `ls agent_auth_demo/` shows an empty directory.

- [ ] **Step 2: Write the model file**

Create `agent_auth_demo/authorization-model.fga` with exactly this content:

```fga
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
    define parent: [folder]
    define owner: [user]
    define editor: [user, agent] or owner or editor from parent
    define viewer: [user, user:*, agent] or editor or viewer from parent

    define edit_authorized:   [user:*, agent] or edit_authorized from parent
    define delete_authorized: [user:*, agent] or delete_authorized from parent

    define can_view:   viewer
    define can_edit:   editor and edit_authorized
    define can_delete: editor and delete_authorized
    define can_share:  owner
```

- [ ] **Step 3: Validate the model typechecks**

Run from repo root:
```bash
fga model validate --file agent_auth_demo/authorization-model.fga
```
Expected: exits 0 and prints `{ "is_valid": true }` (or equivalent success output). Any non-zero exit means a syntax error — re-read the model carefully.

- [ ] **Step 4: Commit**

```bash
git add agent_auth_demo/authorization-model.fga
git commit -m "feat(agent_auth_demo): add authorization model"
```

---

### Task 2: Tests file skeleton + Test 1 (alice has everything)

**Files:**
- Create: `agent_auth_demo/tests.fga.yaml`

- [ ] **Step 1: Write the failing test (skeleton + Test 1)**

Create `agent_auth_demo/tests.fga.yaml` with exactly this content:

```yaml
name: Agent Authorization Demo Tests
model_file: ./authorization-model.fga

tuples:
  # Folder/file hierarchy
  - user: folder:root
    relation: parent
    object: folder:projects
  - user: folder:projects
    relation: parent
    object: file:report
  - user: folder:root
    relation: parent
    object: file:secret

  # Human ownership: alice owns root (cascades down)
  - user: user:alice
    relation: owner
    object: folder:root

  # Agent → user delegation pointers (informational)
  - user: user:alice
    relation: principal
    object: agent:scribe
  - user: user:alice
    relation: principal
    object: agent:janitor

  # Agents granted editor role
  - user: agent:scribe
    relation: editor
    object: folder:projects
  - user: agent:janitor
    relation: editor
    object: folder:root

  # Global gate: humans always authorized everywhere
  - user: user:*
    relation: edit_authorized
    object: folder:root
  - user: user:*
    relation: delete_authorized
    object: folder:root

  # Bounded agent grants
  - user: agent:scribe
    relation: edit_authorized
    object: folder:projects
  - user: agent:janitor
    relation: edit_authorized
    object: folder:projects
  - user: agent:janitor
    relation: delete_authorized
    object: folder:projects

tests:
  - name: Alice (owner of root) can view, edit, delete file:report via cascade — but not share (share requires direct owner tuple)
    check:
      - user: user:alice
        object: file:report
        assertions:
          can_view: true
          can_edit: true
          can_delete: true
          can_share: false
```

- [ ] **Step 2: Run the test**

Run from repo root:
```bash
fga model test --tests agent_auth_demo/tests.fga.yaml
```
Expected: exits 0; output reports 1/1 test passed. If the test fails, re-check the tuples for typos against the spec — the model from Task 1 is correct as-is.

- [ ] **Step 3: Commit**

```bash
git add agent_auth_demo/tests.fga.yaml
git commit -m "test(agent_auth_demo): add tuples and owner-cascade test"
```

---

### Task 3: Test 2 — agent:scribe can edit but cannot delete file:report

**Files:**
- Modify: `agent_auth_demo/tests.fga.yaml` (append to `tests:` list)

This test demonstrates the intersection actually denies. `agent:scribe` is `editor` of `folder:projects` (cascades to `file:report`) AND has `edit_authorized` on `folder:projects` (also cascades). But scribe has no `delete_authorized` tuple anywhere — so `can_edit` passes, `can_delete` fails.

- [ ] **Step 1: Append the new test case**

In `agent_auth_demo/tests.fga.yaml`, add the following entry to the end of the `tests:` list (preserve all earlier content; just append):

```yaml
  - name: agent:scribe is editor + edit_authorized via projects → can_edit, but no delete_authorized → cannot delete
    check:
      - user: agent:scribe
        object: file:report
        assertions:
          can_view: true
          can_edit: true
          can_delete: false
          can_share: false
```

- [ ] **Step 2: Run the tests**

```bash
fga model test --tests agent_auth_demo/tests.fga.yaml
```
Expected: exits 0; output reports 2/2 tests passed.

- [ ] **Step 3: Commit**

```bash
git add agent_auth_demo/tests.fga.yaml
git commit -m "test(agent_auth_demo): verify intersection denies delete for scribe"
```

---

### Task 4: Test 3 — agent:scribe cannot edit file:secret (scope is real)

**Files:**
- Modify: `agent_auth_demo/tests.fga.yaml` (append to `tests:` list)

`file:secret` is parented by `folder:root`, not `folder:projects`. `agent:scribe` is `editor` only on `folder:projects` and the `editor from parent` cascade flows down (children inherit from their parent), so being editor on `folder:projects` does not make scribe editor on `folder:root`. Scribe also has no `edit_authorized` tuple on `folder:root`. Both branches of the intersection fail.

- [ ] **Step 1: Append the new test case**

In `agent_auth_demo/tests.fga.yaml`, append to `tests:`:

```yaml
  - name: agent:scribe has no editor or edit_authorized on root → cannot edit file:secret
    check:
      - user: agent:scribe
        object: file:secret
        assertions:
          can_view: false
          can_edit: false
          can_delete: false
          can_share: false
```

- [ ] **Step 2: Run the tests**

```bash
fga model test --tests agent_auth_demo/tests.fga.yaml
```
Expected: exits 0; output reports 3/3 tests passed.

- [ ] **Step 3: Commit**

```bash
git add agent_auth_demo/tests.fga.yaml
git commit -m "test(agent_auth_demo): verify scribe scope is bounded to projects"
```

---

### Task 5: Test 4 — agent:janitor delete is bounded to projects subtree

**Files:**
- Modify: `agent_auth_demo/tests.fga.yaml` (append to `tests:` list)

Two checks in one test. `agent:janitor` is `editor folder:root` (so editor on every file) but only has `delete_authorized` on `folder:projects`. So:
- `file:report` (under `folder:projects`): editor ✓, delete_authorized ✓ → can_delete true
- `file:secret` (under `folder:root` directly): editor ✓, delete_authorized ✗ (the user:* tuple is for users, not agents) → can_delete false

- [ ] **Step 1: Append the new test case**

In `agent_auth_demo/tests.fga.yaml`, append to `tests:`:

```yaml
  - name: agent:janitor delete_authorized is bounded to projects/ subtree
    check:
      - user: agent:janitor
        object: file:report
        assertions:
          can_edit: true
          can_delete: true
      - user: agent:janitor
        object: file:secret
        assertions:
          can_edit: false
          can_delete: false
```

- [ ] **Step 2: Run the tests**

```bash
fga model test --tests agent_auth_demo/tests.fga.yaml
```
Expected: exits 0; output reports 4/4 tests passed.

- [ ] **Step 3: Commit**

```bash
git add agent_auth_demo/tests.fga.yaml
git commit -m "test(agent_auth_demo): verify janitor delete bounded by subtree grant"
```

---

### Task 6: Test 5 — list_objects honors the intersection

**Files:**
- Modify: `agent_auth_demo/tests.fga.yaml` (append to `tests:` list)

`agent:scribe` is `editor` on every file (cascaded from `folder:projects` → `file:report` is in the projects subtree; but scribe is NOT editor on `file:secret` because secret is under root, not under projects). So scribe's `editor`-set for files is `{file:report}`. Scribe's `edit_authorized` on files is also `{file:report}`. Intersection: `{file:report}`.

- [ ] **Step 1: Append the new test case**

In `agent_auth_demo/tests.fga.yaml`, append to `tests:`:

```yaml
  - name: list_objects(agent:scribe, file, can_edit) returns only file:report (intersection enforced during enumeration)
    list_objects:
      - user: agent:scribe
        type: file
        assertions:
          can_edit:
            - file:report
          can_delete: []
```

- [ ] **Step 2: Run the tests**

```bash
fga model test --tests agent_auth_demo/tests.fga.yaml
```
Expected: exits 0; output reports 5/5 tests passed (4 check tests + 1 list_objects test).

- [ ] **Step 3: Commit**

```bash
git add agent_auth_demo/tests.fga.yaml
git commit -m "test(agent_auth_demo): verify list_objects honors intersection"
```

---

### Task 7: README

**Files:**
- Create: `agent_auth_demo/README.md`

- [ ] **Step 1: Write the README**

Create `agent_auth_demo/README.md` with exactly this content:

````markdown
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

Two ideas combined:

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
````

- [ ] **Step 2: Verify the run command in the README still works**

```bash
cd agent_auth_demo && fga model test --tests tests.fga.yaml && cd ..
```
Expected: exits 0; 5/5 tests pass.

- [ ] **Step 3: Commit**

```bash
git add agent_auth_demo/README.md
git commit -m "docs(agent_auth_demo): add README explaining the agent pattern"
```

---

## Final verification

After all tasks complete, run from repo root:

```bash
fga model validate --file agent_auth_demo/authorization-model.fga && \
fga model test --tests agent_auth_demo/tests.fga.yaml
```

Expected: both commands exit 0; test output reports all 5 tests passing.

The repo should now contain:

```
agent_auth_demo/
├── authorization-model.fga
├── tests.fga.yaml
└── README.md
```

And `git log --oneline` should show 7 new commits, one per task.
