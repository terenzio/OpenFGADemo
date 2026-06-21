# Document Management with `can_*` Permissions

A document-management authorization model. It adds explicit `can_*` permission
relations on top of the roles from the basic model in [`../basic/`](../basic/).
It was written with help from the [`openfga-mcp`](https://github.com/openfga) MCP
server. That server's playbook states one rule the basic model breaks:

> It's a common practice to define specific permissions, that can't be
> directly assigned, using `can_<permission>` relations. […] **Always
> define permissions in the authorization models.**

This demo follows that rule. It also ships a test file you can run to see how the
new permissions behave.

---

## What's here

| File                                                                     | Purpose                                                                                                     |
| ------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| [`authorization-model-mcp-guide.fga`](authorization-model-mcp-guide.fga) | DSL model. Same types and base relations as the basic model, plus four `can_*` permissions per object type. |
| [`authorization-model.json`](authorization-model.json)                   | JSON form of the same model — what the OpenFGA HTTP API accepts. Generated from the `.fga` file.            |
| [`tests.fga.yaml`](tests.fga.yaml)                                       | Inline tuples + `check` and `list_objects` assertions. Run with `fga model test`.                           |

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

**Why this matters:** app code should check *intent* (`can_edit`), not *role*
(`editor`). With `can_edit` as its own relation, you can later let some viewers
edit too. You change one line in the model, not every call site.

| Permission   | Implied by | Rationale                                                                                                     |
| ------------ | ---------- | ------------------------------------------------------------------------------------------------------------- |
| `can_view`   | `viewer`   | Public reads. Includes the wildcard and org-member access.                                                    |
| `can_edit`   | `editor`   | Normal edit. Comes from `owner` via role implication, and from parent folders via `editor from parent`.        |
| `can_delete` | `editor`   | Delete is at edit level, not owner level — same as the MCP guide's example.                                    |
| `can_share`  | `owner`    | Sharing gives access to others. Only direct owners can do it — it does **not** flow down the parent chain.     |

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

The five test cases below cover every concept in the model: owner cascade, role
implication, parent inheritance, userset references, wildcards, and
`list_objects` listing.

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

Alice owns `folder:company`. Owner implies editor (role implication). That flows
down the parent chain to `folder:product`, and again to `document:roadmap`, via
`editor from parent`. So she gets view, edit, and delete.

`can_share` is **false**. The share check needs a direct `owner` tuple on the
document itself — ownership does not flow down for sharing. This is on purpose. A
share endpoint that fired for parent owners would let an org admin reshare
anyone's documents.

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
`(organization:acme#member, viewer, folder:product)` gives viewer to every acme
member, Eve included. That viewer status flows down to `document:roadmap` via
`viewer from parent`. She gets `can_view` but not `can_edit`, because nothing in
the model makes acme members editors.

### Test 3 — Direct editor on a parent folder

```yaml
user: user:charlie
object: document:roadmap
assertions:
  can_edit:   true
  can_delete: true
  can_share:  false
```

Charlie has a direct `editor` tuple on `folder:product`. `editor from parent`
passes that to `document:roadmap`. So both `can_edit` and `can_delete` (both
aliases for `editor`) succeed. `can_share` is owner-only, so it is still false.

### Test 4 — Wildcard for public read, no public write

```yaml
user: user:randomstranger
object: document:public-memo
assertions:
  can_view: true
  can_edit: false
```

The tuple `(user:*, viewer, document:public-memo)` makes the doc public. Any
`user:foo` counts as a viewer, so `can_view` succeeds. `can_edit` fails because
the editor relation does not list `user:*` in its directly-related types. Viewer
is public; editor is not.

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

The owner cascade gets her `document:roadmap`. The wildcard tuple gets her
`document:public-memo`. OpenFGA combines both paths while listing. So the app can
build a "documents you can see" list with one API call, instead of N `Check`
calls.

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

- [`../ai-agent/`](../ai-agent/) — next workshop step: AI agents as first-class principals, granted the same roles as users and bounded to a subtree.
- [`../../docs/model-explained.md`](../../docs/model-explained.md) — annotated walkthrough of the document-management model.
- [OpenFGA DSL reference](https://openfga.dev/docs/configuration-language)
- [`fga model test` documentation](https://openfga.dev/docs/getting-started/cli)
