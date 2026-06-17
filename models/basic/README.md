# Basic OpenFGA Model

The smallest authorization model that exercises the OpenFGA DSL's main
ideas — direct relations, role implication, parent inheritance, userset
references, and wildcards — for a document-management system.

This model is **role-only**. Application code calls
`Check(user, "editor", doc)` directly. Step 2 of the workshop
([`../model_advanced_demo/folder_document_with_mcp_demo/`](../model_advanced_demo/folder_document_with_mcp_demo/))
shows why you should expose `can_*` permission relations on top of these
roles instead — and ships a runnable test file you can use against this
exact role structure.

---

## The model

[`authorization-model.fga`](authorization-model.fga):

```fga
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

## The five DSL ideas this model uses

### 1. Direct relations

`define owner: [user]` says "the `owner` relation can be directly
assigned between a `user` and a `folder` / `document`."

```
Tuple:  user:alice  →  owner  →  folder:company
Check(user:alice, owner, folder:company)  →  true
```

### 2. Role implication

Each relation can `or` another relation defined on the same object:

```fga
define editor: [user, organization#member] or owner or editor from parent
define viewer: [user, user:*, organization#member] or editor or viewer from parent
```

Owners get editor for free; editors get viewer for free.

```
Tuple:  user:alice  →  owner   →  folder:company
Check(user:alice, editor, folder:company)  →  true   (via "or owner")
Check(user:alice, viewer, folder:company)  →  true   (via "or editor" → "or owner")
```

### 3. Parent inheritance — `X from Y` (TupleToUserset)

`editor from parent` reads as "if you are an editor on this object's
parent, you are also editor here." That single clause cascades editor
status down a folder hierarchy with no extra tuples.

```
Tuples:
  user:alice       owner   folder:company
  folder:company   parent  folder:product
  folder:product   parent  document:roadmap

Check(user:alice, editor, document:roadmap)  →  true
  via owner-on-folder:company
  → "or owner" makes alice editor of folder:company
  → "editor from parent" cascades that to folder:product
  → "editor from parent" cascades again to document:roadmap
```

### 4. Userset references — `organization#member`

You can grant a relation not just to a single user but to *every member
of an organization*, by listing `organization:acme#member` as the
target.

```
Tuples:
  user:eve                  member  organization:acme
  organization:acme#member  viewer  folder:product

Check(user:eve, viewer, folder:product)  →  true
```

Add or remove members of `organization:acme` and access changes
automatically — no per-user tuples to manage.

### 5. Wildcards — `user:*`

`user:*` in a relation's directly-related types list means "anyone."
Grant it once and the object is public.

```
Tuple:  user:*  →  viewer  →  document:public-memo

Check(user:randomstranger, viewer, document:public-memo)  →  true
```

Wildcards work for `viewer` here but **not** `editor`, because
`editor`'s directly-related types are `[user, organization#member]` —
no `user:*`. To make a doc publicly editable you would have to add
`user:*` to the editor type list.

---

## Worked example — putting it all together

A tenant containing one company tree and one public document:

```
organization:acme
  └── user:eve (member)

folder:company             ← user:alice (owner)
└── folder:product         ← user:charlie (editor)
    │                      ← organization:acme#member (viewer)
    └── document:roadmap

document:public-memo       ← user:* (viewer)
```

Tuples:

```
user:alice                owner    folder:company
folder:company            parent   folder:product
folder:product            parent   document:roadmap
user:charlie              editor   folder:product
organization:acme#member  viewer   folder:product
user:eve                  member   organization:acme
user:*                    viewer   document:public-memo
```

Expected `Check` results on `document:roadmap` and `document:public-memo`:

| User | Object | Relation | Result | Why |
|---|---|---|---|---|
| user:alice          | document:roadmap     | viewer | true  | owner-of-company → editor cascade → viewer cascade |
| user:alice          | document:roadmap     | editor | true  | owner-of-company → editor cascade |
| user:charlie        | document:roadmap     | editor | true  | direct editor on parent folder:product |
| user:charlie        | document:roadmap     | viewer | true  | "or editor" |
| user:eve            | document:roadmap     | viewer | true  | acme member → folder:product viewer → cascade |
| user:eve            | document:roadmap     | editor | false | acme members are viewers, not editors |
| user:bob            | document:roadmap     | viewer | false | no path |
| user:randomstranger | document:public-memo | viewer | true  | `user:*` wildcard |
| user:randomstranger | document:public-memo | editor | false | wildcard not in editor's type list |

These cases match the actual assertions in
[`../model_advanced_demo/folder_document_with_mcp_demo/tests.fga.yaml`](../model_advanced_demo/folder_document_with_mcp_demo/tests.fga.yaml),
just with role names (`editor`, `viewer`) instead of the `can_*` aliases
that step 2 adds.

---

## Why no test file in this folder

This model is the teaching baseline. Step 2 of the workshop adds
`can_view` / `can_edit` / `can_delete` / `can_share` permission
relations on top — and that is the level at which application code
should be calling `Check`. The test file at
[`../model_advanced_demo/folder_document_with_mcp_demo/tests.fga.yaml`](../model_advanced_demo/folder_document_with_mcp_demo/tests.fga.yaml)
exercises exactly the same tuples and concepts shown above, against the
augmented model.

If you want to test the bare model in this folder, copy that test file
here, point `model_file:` at this folder's `authorization-model.fga`,
and replace each `can_*` assertion with its underlying role name
(`can_view` → `viewer`, `can_edit` / `can_delete` → `editor`,
`can_share` → `owner`).

---

## Try it live

With the OpenFGA server running (see [root README](../README.md), step 4):

```bash
fga model write --file authorization-model.fga
```

Then issue Check / Write / ListObjects calls via the
[Playground](http://localhost:3000) or the `fga` CLI.

---

## Next step

[`../model_advanced_demo/folder_document_with_mcp_demo/`](../model_advanced_demo/folder_document_with_mcp_demo/)
adds `can_*` permission relations and a runnable test file built on top
of this exact role structure.
