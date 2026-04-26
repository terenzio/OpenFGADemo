# Authorization Model Explained

This document walks through `model_basic_demo/authorization-model.fga` line by line,
explaining every concept with concrete examples from the demo data.

## The Full Model

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

---

## Types and Relations

### `type user`

A bare type with no relations. Every principal in the system is a `user`.
Tuples reference them as `user:alice`, `user:bob`, etc.

### `type organization`

```
define member: [user]
```

**Direct relation** — only literal `user` objects may be assigned as members.
A tuple `(user:eve, member, organization:acme)` makes eve a member of acme.

### `type folder` and `type document`

Both types share the same four-relation pattern, so they are explained together.

#### `owner: [user]`

Direct relation. Only a `user` can be an owner. Ownership is the root of all
permission — `owner` implies `editor` (see below).

#### `parent: [folder]`

A structural relation that points one folder (or document) at its containing
folder. This is not a permission itself; it is used by `editor from parent` and
`viewer from parent` to propagate permissions up the hierarchy.

Seed data example:
- `(folder:company, parent, folder:product)` — product lives inside company
- `(folder:product, parent, document:roadmap)` — roadmap lives inside product

#### `editor: [user, organization#member] or owner or editor from parent`

Three ways to become an editor — this is an **OR** union:

1. **Direct assignment** — a tuple `(user:charlie, editor, folder:product)`
   makes charlie a direct editor of that folder.
2. **Role implication** — `or owner` means any owner is automatically an editor.
   No extra tuple is needed; the model computes it.
3. **TupleToUserset** — `editor from parent` follows the `parent` pointer and
   asks "is this user an editor of the parent object?" This lets permissions
   cascade down the folder hierarchy.

`organization#member` in the type list means an organization's *member userset*
can be assigned as an editor. A tuple like
`(organization:acme#member, editor, folder:product)` would give every acme
member editor access to that folder.

#### `viewer: [user, user:*, organization#member] or editor or viewer from parent`

Builds on `editor` with two additions:

1. **Wildcard** — `user:*` matches *any* authenticated user. A tuple
   `(user:*, viewer, document:public-memo)` makes public-memo readable by
   everyone, including completely unknown users like `randomstranger`.
2. **Role implication** — `or editor` means every editor is automatically a
   viewer. Again, no extra tuple required.

---

## Worked Example: How Eve Sees `document:roadmap`

Eve is a member of organization:acme. The question is: can `user:eve` view
`document:roadmap`?

OpenFGA evaluates this by expanding the `viewer` relation on `document:roadmap`:

```
viewer on document:roadmap
  = direct viewer tuples            → none for eve
  OR editor on document:roadmap
      = direct editor tuples        → none for eve
      OR owner on document:roadmap  → user:alice (not eve)
      OR editor from parent
          → parent = folder:product
          → editor on folder:product
              = direct editors      → user:charlie (not eve)
              OR owner              → user:alice (not eve)
              OR editor from parent
                  → parent = folder:company
                  → editor on folder:company
                      = owner       → user:alice (not eve)
                      → (no match)
  OR viewer from parent
      → parent = folder:product
      → viewer on folder:product
          = direct viewer tuples
              → (organization:acme#member, viewer, folder:product) ✓
                  eve is a member of organization:acme → MATCH
```

Result: **allowed**. Eve can view `document:roadmap` because:
- `folder:product` grants viewer access to `organization:acme#member`
- `document:roadmap` inherits that via `viewer from parent`
- Eve is a member of `organization:acme`

This chain — organization membership → folder viewer → document viewer via
parent — demonstrates **TupleToUserset** traversal across three type boundaries.

---

## Concept Summary

| Concept | Where Used | What It Does |
|---|---|---|
| Direct relation | `member`, `owner` | Explicitly assigned by a tuple |
| Role implication | `or owner`, `or editor` | Higher role auto-grants lower role |
| TupleToUserset | `editor from parent` | Follows a pointer relation to another object |
| Userset reference | `organization#member` | Grants access to all members of a group |
| Wildcard | `user:*` | Grants access to every user |
