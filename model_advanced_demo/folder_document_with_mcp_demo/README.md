# Document Management with `can_*` Permissions

A document-management authorization model that adds explicit `can_*`
permission relations on top of the role definitions from the basic
model in [`../../model_basic_demo/`](../../model_basic_demo/). Authored
with guidance from the [`openfga-mcp`](https://github.com/openfga) MCP
server, whose authoring playbook calls out one rule the basic model
violates:

> It's a common practice to define specific permissions, that can't be
> directly assigned, using `can_<permission>` relations. […] **Always
> define permissions in the authorization models.**

This demo applies that rule and ships a runnable test file showing how
the new permissions behave.

---

## What's here

| File                                                   | Purpose                                                                                                     |
| ------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| [`authorization-model.fga`](authorization-model.fga)   | DSL model. Same types and base relations as the basic model, plus four `can_*` permissions per object type. |
| [`authorization-model.json`](authorization-model.json) | JSON form of the same model — what the OpenFGA HTTP API accepts. Generated from the `.fga` file.            |
| [`tests.fga.yaml`](tests.fga.yaml)                     | Inline tuples + `check` and `list_objects` assertions. Run with `fga model test`.                           |

---

## What changed from the basic model

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

**Why this matters:** application code should check *intent*
(`can_edit`), not *role* (`editor`). With `can_edit` as a first-class
relation, you can later decide that some viewers should also be able to
edit by changing one line in the model — instead of every call site.

| Permission   | Implied by | Rationale                                                                                                     |
| ------------ | ---------- | ------------------------------------------------------------------------------------------------------------- |
| `can_view`   | `viewer`   | Public reads, including wildcard and org-member inheritance.                                                  |
| `can_edit`   | `editor`   | Standard edit; inherits from `owner` via role implication and from ancestor folders via `editor from parent`. |
| `can_delete` | `editor`   | Deletion is edit-level, not owner-level — matches the MCP guide's example.                                    |
| `can_share`  | `owner`    | Sharing grants access to others. Restricted to direct owners only — does **not** inherit via parent chain.    |

---

## The test scenario

[`tests.fga.yaml`](tests.fga.yaml) seeds a small tenant:

```text
organization:acme
  └── user:eve (member)

folder:company             ← user:alice (owner)
└── folder:product         ← user:charlie (editor)
    │                      ← organization:acme#member (viewer)
    └── document:roadmap

document:public-memo       ← user:* (viewer)
```

The five test cases below exercise every concept in the model: owner
cascade, role implication, parent inheritance, userset references,
wildcards, and `list_objects` enumeration.

---

## Walking through the tests

Run them:

```bash
fga model test --tests tests.fga.yaml
```

Expected: **5 tests passed, 0 failed**.

### Test 1 — Owner cascade, with `can_share` opt-out

```yaml
user: user:alice
object: document:roadmap
assertions:
  can_view:   true
  can_edit:   true
  can_delete: true
  can_share:  false
```

Alice owns `folder:company`. Owner implies editor (role implication),
which cascades down the parent chain to `folder:product` and again to
`document:roadmap` via `editor from parent`. That gives her view,
edit, and delete.

`can_share` is **false**: the share check requires a direct `owner`
tuple on the document itself — ownership does not cascade for sharing
purposes. This is intentional. A share endpoint that auto-fired for
ancestor owners would let an org admin reshare anyone's documents.

### Test 2 — Org member inheritance via TupleToUserset

```yaml
user: user:eve
object: document:roadmap
assertions:
  can_view:   true
  can_edit:   false
  can_share:  false
```

Eve is a `member` of `organization:acme`. The tuple
`(organization:acme#member, viewer, folder:product)` grants viewer to
every acme member — Eve included. That viewer status cascades to
`document:roadmap` via `viewer from parent`. She gets `can_view` but
not `can_edit`, because nothing in the model makes acme members
editors.

### Test 3 — Direct editor on a parent folder

```yaml
user: user:charlie
object: document:roadmap
assertions:
  can_edit:   true
  can_delete: true
  can_share:  false
```

Charlie has a direct `editor` tuple on `folder:product`. `editor from
parent` cascades that to `document:roadmap`, so both `can_edit` and
`can_delete` (both aliases for `editor`) succeed. `can_share` is
owner-only, so still false.

### Test 4 — Wildcard for public read, no public write

```yaml
user: user:randomstranger
object: document:public-memo
assertions:
  can_view: true
  can_edit: false
```

The tuple `(user:*, viewer, document:public-memo)` makes the doc
public — any `user:foo` evaluates as a viewer, so `can_view` succeeds.
`can_edit` fails because the editor relation does not list `user:*` in
its directly-related types: viewer is public, editor is not.

### Test 5 — `list_objects` enumerates Alice's reachable documents

```yaml
list_objects:
  - user: user:alice
    type: document
    assertions:
      can_view:
        - document:roadmap
        - document:public-memo
```

The owner cascade gets her `document:roadmap`; the wildcard tuple gets
her `document:public-memo`. The OpenFGA evaluator combines both paths
during enumeration, so the application can render a "documents you can
see" list with a single API call instead of N `Check`s.

---

## Try changing it

Add the following to `tests.fga.yaml` and re-run:

```yaml
# Add to tuples:
- user: user:dave
  relation: editor
  object: folder:product

# Add to tests:
- name: dave (direct editor on product) cascades to child doc
  check:
    - user: user:dave
      object: document:roadmap
      assertions:
        can_edit:   true
        can_delete: true
        can_share:  false
```

You should see **6 tests passed**.

---

## How to run

Install the OpenFGA CLI:

```bash
brew install openfga/tap/fga    # macOS
# or see the OpenFGA docs for other platforms
```

Run the test suite from this directory:

```bash
fga model test --tests tests.fga.yaml
```

Expected output: **5 tests passed, 0 failed**.

---

## Further reading

- [`../agent_auth_demo/`](../agent_auth_demo/) — next workshop step: bounded delegation to AI agents using `editor and edit_authorized` intersections.
- [`../../docs/model-explained.md`](../../docs/model-explained.md) — annotated walkthrough of the document-management model.
- [OpenFGA DSL reference](https://openfga.dev/docs/configuration-language)
- [`fga model test` documentation](https://openfga.dev/docs/getting-started/cli)
