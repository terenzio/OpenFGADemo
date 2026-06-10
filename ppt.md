# OpenFGA Workshop — PPT Slide Skeleton

> This document is a slide-by-slide breakdown of `presentation.md`, formatted as a PPT skeleton.
> Each slide includes: a **Slide title**, a **Content** description (what should appear on the slide),
> and **Speaker notes** (what the presenter says / context for the slide).
> Feed this into a slide-generation agent to produce the final deck.

---

## Slide 1 — Title

**Content:**
- Title: **OpenFGA Workshop**
- Subtitle: *Fine-Grained Authorization for Modern Applications*
- Tagline: A hands-on tour of relationship-based access control, from a single-line `Check` to bounded AI-agent delegation.
- Event: DevOps Conference
- Visual suggestion: OpenFGA / CNCF logo, clean title layout.

**Speaker notes:**
Welcome the audience. Set expectations: this is hands-on and practical. We start from the simplest possible authorization question and build up to bounding what AI agents are allowed to do. By the end you'll have run a real authorization engine locally.

---

## Slide 2 — Workshop Goals

**Content:**
- Heading: "By the end of this session you will:"
- Bullet list:
  - Understand **ReBAC** and how it compares to RBAC / ABAC
  - Read and write **OpenFGA DSL** models
  - Run **`fga model test`** — TDD for authorization
  - Stand up an **OpenFGA server** with Docker
  - Wire `Check` calls into a real Go HTTP service
  - Apply the pattern that bounds **AI-agent permissions**
- Footer: Workshop repo link — github.com/terenzio/OpenFGADemo

**Speaker notes:**
Walk through the six concrete outcomes. Emphasize that everything is reproducible from the repo. Reassure attendees they can follow along or just watch — code is provided.

---

## Slide 3 — The Problem

**Content:**
- Big quote: *"Can user X do action Y on resource Z?"*
- Sub-line: "Sounds simple. Until:"
- Bullet list of complications:
  - Resources are nested (folders, projects, teams)
  - Users belong to groups (organizations, departments)
  - Permissions cascade (folder owner → document editor)
  - Some objects are public (`user:*`)
  - Some actors are non-human (AI agents, services)
  - The product roadmap changes the rules every quarter

**Speaker notes:**
Frame the core authorization question. The one-liner looks trivial, but each bullet is where homegrown auth systems break down. The last bullet — changing rules — is what motivates a declarative model instead of scattered if-statements.

---

## Slide 4 — RBAC vs ABAC vs ReBAC

**Content:**
- Comparison table:

  | Approach | Decision based on... | Strength | Weakness |
  |----------|----------------------|----------|----------|
  | RBAC | Role assigned to user | Simple, auditable | Role explosion, no relationships |
  | ABAC | Attributes of subject + resource | Flexible | Hard to audit, policy sprawl |
  | ReBAC | Relationships between objects | Models real domains | Newer, less tooling — until now |

- Callout: ReBAC is the model behind Google Drive, GitHub, Slack — anywhere you say "Charlie shared this folder with the engineering team."

**Speaker notes:**
Contrast the three models. RBAC is familiar but explodes. ABAC is powerful but unauditable. ReBAC matches how people actually talk about access ("shared with"). Name-drop the big products to show this is proven at scale, not experimental.

---

## Slide 5 — What is OpenFGA?

**Content:**
- Heading: What is OpenFGA?
- Bullet list:
  - **Open-source ReBAC engine**, CNCF Sandbox project
  - Inspired by the **Google Zanzibar paper** (2019)
  - Started by **Auth0**, now community-driven
  - HTTP + gRPC API
  - Storage: Postgres, MySQL, MariaDB, in-memory
  - Performance: P99 sub-10ms `Check` at billions of tuples (Zanzibar lineage)

**Speaker notes:**
Establish credibility: CNCF backing, Zanzibar heritage (the system that powers Google's own authorization), and real performance numbers. Mention it's API-driven so it fits any stack.

---

## Slide 6 — Core Concepts

**Content:**
- A **model** declares types and relations. Code block:
  ```fga
  type document
    relations
      define editor: [user]
      define viewer: [user] or editor
  ```
- A live system stores **tuples**: `(user, relation, object)`
  ```
  user:alice  →  editor  →  document:roadmap
  ```
- Result: `Check(user:alice, viewer, document:roadmap)` → **true** (via `or editor`)

**Speaker notes:**
Introduce the two halves of the system: the *model* (static schema of relations) and *tuples* (the live facts). Walk the example: alice is an editor; viewer is defined as "user or editor"; therefore alice is a viewer without any explicit viewer tuple. This is the core mental model for the whole workshop.

---

## Slide 7 — The Five Core API Operations

**Content:**
- Table:

  | Call | Question |
  |------|----------|
  | **Check** | Can this user do this thing on this object? |
  | **Write** | Add a tuple (grant access) |
  | **Delete** | Remove a tuple (revoke access) |
  | **ListObjects** | Which objects can this user act on? |
  | **Expand** | Show the full subtree behind a relation (debugging) |

- Callout: App code lives mostly in `Check` and `ListObjects`. Tuple CRUD happens in admin / share flows.

**Speaker notes:**
The entire API surface is five calls. Point out the split: read-side (Check, ListObjects, Expand) vs write-side (Write, Delete). Most application request paths only ever call Check.

---

## Slide 8 — Workshop Roadmap

**Content:**
- Heading: Three progressive models, each adds one concept.
- Numbered list:
  1. **Basic** — DSL fundamentals: roles, cascade, wildcards, groups.
  2. **`can_*` permissions** — separate intent from implementation.
  3. **AI-agent delegation** — bounded capabilities via intersection.
- Directory tree:
  ```
  OpenFGADemo/
  ├── model_basic_demo/                     ← Step 1
  └── model_advanced_demo/
      ├── folder_document_with_mcp_demo/    ← Step 2
      └── agent_auth_demo/                  ← Step 3
  ```

**Speaker notes:**
Give the audience the map of the workshop. Each step builds on the previous and adds exactly one new idea. Show how the directory layout mirrors the three steps so they can navigate the repo.

---

---

## Slide 9 — Step 1: The Basic Model (Section Divider + Model)

**Content:**
- Section header: **Step 1 — The Basic Model**
- File reference: `model_basic_demo/authorization-model.fga`
- Code block:
  ```fga
  type folder
    relations
      define owner: [user]
      define parent: [folder]
      define editor: [user, organization#member] or owner or editor from parent
      define viewer: [user, user:*, organization#member] or editor or viewer from parent
  ```
- Tagline: Five DSL ideas in one type definition.

**Speaker notes:**
This is the anchor slide for Step 1. Don't explain it all at once — tell the audience that the next five slides each unpack one idea hidden in this single type definition. Read the relations aloud once to set up the deep-dives.

---

## Slide 10 — Idea 1: Direct Relations

**Content:**
- Tuple example:
  ```
  user:alice  →  owner  →  folder:company
  ```
- Explanation: `define owner: [user]` declares which subject *types* may hold this relation.
- Key point: One tuple per grant. The base unit of OpenFGA storage.

**Speaker notes:**
The simplest case: a direct grant. The `[user]` in the definition is a type restriction — only users can be owners here. Each grant is one tuple; this is the atom everything else is built from.

---

## Slide 11 — Idea 2: Role Implication

**Content:**
- Code block:
  ```fga
  define viewer: [...] or editor
  define editor: [...] or owner
  ```
- Bullets:
  - Owners get editor for free.
  - Editors get viewer for free.
- Punchline: One write satisfies three Checks.

**Speaker notes:**
Role implication chains relations so higher roles automatically satisfy lower ones. Granting owner means the user passes editor and viewer checks too — no extra tuples. This is how you model "owner > editor > viewer" hierarchies cleanly.

---

## Slide 12 — Idea 3: Parent Inheritance (`X from Y`)

**Content:**
- Code: `define editor: [...] or editor from parent`
- Tuple chain:
  ```
  user:alice       owner   folder:company
  folder:company   parent  folder:product
  folder:product   parent  document:roadmap
  ```
- Result: `Check(user:alice, editor, document:roadmap)` → **true**
- Punchline: Two parent hops, zero extra grant tuples.

**Speaker notes:**
This is the cascade/inheritance idea. `editor from parent` means "you're an editor here if you're an editor of my parent." Walk the chain: alice owns company → company is parent of product → product is parent of roadmap. Alice edits roadmap purely through structure, no document-level grant.

---

## Slide 13 — Idea 4: Userset References

**Content:**
- Heading: Grant a relation to *every member of a group*.
- Tuple example:
  ```
  user:eve                   member   organization:acme
  organization:acme#member   viewer   folder:product
  ```
- Punchline: Add or remove acme members → folder access updates automatically. **No per-user tuples to manage.**

**Speaker notes:**
Usersets let you grant access to a group rather than individuals. The `organization:acme#member` syntax means "all members of acme." Managing the org membership automatically manages the folder access. This is how team-based sharing scales.

---

## Slide 14 — Idea 5: Wildcards (`user:*`)

**Content:**
- Tuple example:
  ```
  user:*  →  viewer  →  document:public-memo
  ```
- Result: Anyone is now a viewer.
- Caveat: Wildcards work only if `user:*` appears in the relation's directly-related-types list. In our model, `viewer` is public — but `editor` is **not**.

**Speaker notes:**
Wildcards model public access. `user:*` means "every user." Crucially, it only works where explicitly allowed in the model — viewer permits it, editor does not. This is the safety mechanism: you can't accidentally make something publicly editable.

---

## Slide 15 — Step 2: Add `can_*` Permissions (Section Divider)

**Content:**
- Section header: **Step 2 — Add `can_*` Permissions**
- File reference: `model_advanced_demo/folder_document_with_mcp_demo/`
- Problem statement: The basic model exposes role names. App code calls:
  ```go
  Check(user, "editor", doc)   // couples API to today's role names
  ```
- Solution — add permission relations:
  ```fga
  define can_view:   viewer
  define can_edit:   editor
  define can_delete: editor
  define can_share:  owner
  ```
- Punchline: App code now calls `Check(user, "can_edit", doc)` — **intent**, not role.

**Speaker notes:**
Introduce the indirection layer. The problem: if app code checks "editor" directly, the API is coupled to role names. The fix: define `can_*` permission relations that map to roles. App code expresses *what the user is trying to do*, not *which role grants it*.

---

## Slide 16 — Why It Matters Operationally

**Content:**
- Bullets:
  - New requirement: "let viewers edit on weekends." Change one line in the model — not 20 call sites.
  - Auditing: log says `can_edit` denied — clear what the user *tried* to do.
  - Versioning: model changes don't break callers.
- Pull-quote:
  > `can_*` is the **API surface**. Roles are the **implementation**.

**Speaker notes:**
Sell the operational payoff. Decoupling means policy changes happen in the model, not across the codebase. Audit logs become meaningful (intent, not implementation detail). And you can evolve roles without breaking API consumers. The quote is the takeaway.

---

## Slide 17 — TDD for Authorization

**Content:**
- Command:
  ```bash
  cd model_advanced_demo/folder_document_with_mcp_demo
  fga model test --tests tests.fga.yaml
  ```
- Result: **5 tests passed, 0 failed**
- Explanation: Tests run inline tuples through a sandboxed evaluator — no server, no DB, no Docker.
- Punchline: CI-friendly. Fail the build on a model regression. Treat your auth model like code.

**Speaker notes:**
Authorization models are testable in isolation. `fga model test` runs scenarios against a sandboxed evaluator in under a second — no infrastructure. This makes auth a first-class part of CI. A broken model fails the build like any other test.

---

## Slide 18 — Step 2: Test Walkthrough

**Content:**
- Table:

  | # | Scenario | Expected |
  |---|----------|----------|
  | 1 | Owner cascade to grandchild doc | view/edit/delete yes, share no |
  | 2 | Org member via TupleToUserset | view yes, edit no |
  | 3 | Direct editor on parent folder cascades | edit/delete yes |
  | 4 | `user:*` wildcard | view yes, edit no |
  | 5 | `list_objects` enumeration | returns every reachable doc |

- Key insight: **`can_share` does not cascade.** Sharing is owner-only by design — an org admin should not auto-reshare a report writer's draft.

**Speaker notes:**
Walk the five test scenarios — each exercises one DSL idea from Step 1 through the `can_*` layer. Spend time on the key insight: deliberately *not* cascading share is a security design choice. Inheritance is a tool, not a default to apply everywhere.

---

## Slide 19 — Step 3: The AI-Agent Problem (Section Divider)

**Content:**
- Section header: **Step 3 — The AI-Agent Problem**
- Scenario: You give an AI coding agent your credentials. It can now:
  - View every file you can view (you wanted this)
  - Edit every file you can edit (you didn't want **all** of them)
  - Delete every file you can delete (you really didn't want this)
  - Share files with strangers (you absolutely didn't want this)
- Need: **bounded delegation, per capability, per scope.**

**Speaker notes:**
This is the climax of the workshop and the most timely topic. Handing an agent your credentials grants it your full blast radius. The escalating bullet list (view → edit → delete → share) makes the danger visceral. We need delegation that's scoped per capability and per resource subtree.

---

## Slide 20 — Step 3: The Intersection Pattern

**Content:**
- File reference: `model_advanced_demo/agent_auth_demo/authorization-model.fga`
- Code block:
  ```fga
  type agent
    relations
      define principal: [user]    # informational: which user this agent acts for

  type folder
    relations
      define editor: [user, agent] or owner or editor from parent

      define edit_authorized:   [user:*, agent] or edit_authorized from parent
      define delete_authorized: [user:*, agent] or delete_authorized from parent

      define can_edit:   editor and edit_authorized
      define can_delete: editor and delete_authorized
      define can_share:  owner    # never delegable to agents
  ```

**Speaker notes:**
This is the anchor model for Step 3. Don't explain all at once — the next slide breaks it into three ideas. Highlight the `and` in `can_edit` and `can_delete`: that intersection is the whole trick. Note `can_share` deliberately has no agent path.

---

## Slide 21 — Three Ideas, One Model

**Content:**
- Numbered list:
  1. **Intersection (`and`)** — both relations must evaluate true. Either side missing → denied.
  2. **Different default for users vs. agents.** `(user:*, edit_authorized, folder:root)` makes humans default-pass everywhere. `user:*` does **not** match agents → agents are default-deny until explicitly granted.
  3. **Subtree scoping for free.** `or edit_authorized from parent` means one grant per subtree covers every descendant — and only those descendants.

**Speaker notes:**
Unpack the three mechanics. (1) Intersection is fail-closed: missing either side denies. (2) The asymmetry — `user:*` matches humans but not agents — gives humans a permissive default while agents start denied. (3) Inheritance scopes a single agent grant to a whole subtree, no more, no less. Together these give bounded, scoped delegation.

---

## Slide 22 — Step 3: Concrete Scenario

**Content:**
- Folder tree:
  ```
  folder:root              ← user:alice (owner)
  ├── folder:projects      ← agent:scribe   (editor + edit_authorized)
  │   │                    ← agent:janitor  (editor + edit + delete_authorized)
  │   └── file:report
  └── file:secret
  ```
- Outcome table:

  | Actor | file:report | file:secret |
  |-------|-------------|-------------|
  | user:alice | view / edit / delete | view / edit / delete |
  | agent:scribe | view + edit (no del) | nothing |
  | agent:janitor | view + edit + delete | nothing |
  | anyone | share: owner-only | share: owner-only |

**Speaker notes:**
Make it concrete. Alice owns root and can do everything everywhere. Scribe is granted on `projects` only, so it can edit `report` but cannot even see `secret` (different subtree). Janitor additionally has delete. Nobody but the owner can share. This table is the payoff of the whole pattern — read it row by row.

---

## Slide 23 — `list_objects` Bounds Discovery

**Content:**
- Test snippet:
  ```yaml
  list_objects:
    - user: agent:scribe
      type: file
      assertions:
        can_edit:   [file:report]
        can_delete: []
  ```
- Result: `list_objects(agent:scribe, file, can_edit)` returns **only** `file:report`.
- Punchline: The agent cannot even *discover* files outside its grant — critical when the agent is an LLM that auto-enumerates available actions.

**Speaker notes:**
Bounding isn't only about denying actions — it's about not revealing existence. ListObjects respects the same model, so an agent enumerating "what can I edit?" sees only its grant. This matters for LLM agents that probe their environment; you don't leak the file tree.

---

## Slide 24 — Step 4: Run the Live Demo (Section Divider)

**Content:**
- Section header: **Step 4 — Run the Live Demo**
- Description: The repo ships a Go HTTP server that exercises Check / Write / Delete / ListObjects / Expand against a real OpenFGA instance.
- Commands:
  ```bash
  make up      # MariaDB + OpenFGA + Playground (localhost:3000)
  make seed    # Apply model, write tuples, set demo state
  make serve   # Go HTTP server on :8000
  ```
- Or all at once:
  ```bash
  make demo    # automated walkthrough with curl
  ```

**Speaker notes:**
Transition from theory to running code. Show that the full stack is three make commands (or one). If doing this live, run `make demo` now. Emphasize this is a real OpenFGA server with a real database, not a mock.

---

## Slide 25 — Live Demo: `curl` Examples

**Content:**
- Code block of curl examples:
  ```bash
  # alice — owner of folder:company, sees everything via cascade
  curl -H "X-User-Id: alice" :8000/documents | jq

  # bob — no grant, gets 403
  curl -H "X-User-Id: bob" :8000/documents/roadmap

  # charlie — direct editor on folder:product, can view roadmap
  curl -H "X-User-Id: charlie" :8000/documents/roadmap | jq

  # randomstranger — public-memo via user:* wildcard
  curl -H "X-User-Id: randomstranger" :8000/documents/public-memo | jq

  # alice grants bob editor on design-doc
  curl -X POST -H "X-User-Id: alice" \
    -d '{"user":"user:bob","relation":"editor"}' \
    :8000/documents/design-doc/share
  ```

**Speaker notes:**
Each curl maps to a concept from earlier: alice = cascade, bob = denial (403), charlie = direct grant, randomstranger = wildcard, and the final POST = live Write (sharing). Run them in order if presenting live so the audience sees each idea in action against the running server.

---

## Slide 26 — Architecture at a Glance

**Content:**
- Architecture diagram:
  ```
  +-----------+     +----------------+     +-------------+
  |  Client   | --> |  Go HTTP API   | --> |  OpenFGA    |
  |  (curl)   |     |  (chi router)  |     |  Server     |
  +-----------+     +--------+-------+     +------+------+
                             |                    |
                             v                    v
                    +----------------+     +-------------+
                    | In-memory      |     |  MariaDB    |
                    | document store |     |  (tuples)   |
                    +----------------+     +-------------+
  ```
- Flow caption: `X-User-Id` header → middleware → `Check(user, can_*, doc)` → handler

**Speaker notes:**
Show how the pieces fit. The Go API is a thin layer: middleware extracts the user, calls Check against OpenFGA, then the handler reads from the document store. OpenFGA persists tuples in MariaDB. The auth decision is cleanly separated from business data.

---

## Slide 27 — Try the Playground

**Content:**
- Heading: OpenFGA ships a visual debugger.
- URL: http://localhost:3000
- Bullets:
  - Author models with live syntax checking
  - Visualize the relation graph
  - Run `Check` and `Expand` interactively
  - See which tuples contributed to a decision
- Callout: Great for explaining auth decisions to non-engineering stakeholders.

**Speaker notes:**
The Playground lowers the barrier. It's a visual way to author models, inspect the relation graph, and debug why a Check returned what it did. Highlight the stakeholder angle — you can show a PM exactly why someone was denied.

---

## Slide 28 — DevOps Considerations (Section Divider)

**Content:**
- Section header: **DevOps Considerations**
- (Divider slide — sets up the operational section that follows.)

**Speaker notes:**
Pivot to the operational side. The audience is a DevOps crowd — this section answers "how do I actually run, version, and observe this in production?"

---

## Slide 29 — Storage and Deployment

**Content:**
- Bullets:
  - **Storage**: Postgres / MySQL / MariaDB in production. SQLite for dev.
  - **Binary**: ~30 MB, single static binary. Stateless. Horizontal scaling.
  - **Container**: official image at `openfga/openfga`. We use it via `docker-compose.yml`.
  - **Backup**: tuples are your data — back up the DB, not the binary.

**Speaker notes:**
Operational facts. OpenFGA is a small stateless binary, so it scales horizontally trivially — all state lives in the database. The thing you protect and back up is the tuple store. Standard SQL backends mean no exotic infra.

---

## Slide 30 — Versioning Models. //REMOVE?

**Content:**
- Statement: Models are **immutable**. Every `fga model write` creates a new model ID. You never mutate; you roll forward.
- Migration pattern (numbered):
  1. Write the new model — get back a new model ID.
  2. **Dual-check** during cutover: app calls `Check` against both old and new model IDs, alerts on divergence.
  3. Once divergence is zero for N days, point app at new model ID.
  4. Old model ID stays around forever — auditable.

**Speaker notes:**
Models are versioned by immutability — each write is a new ID, so rollback is just pointing at the old ID. The safe migration is dual-checking: run both models in parallel, compare results, cut over only when they agree. This is a familiar blue/green pattern applied to authorization.

---

## Slide 31 — CI / CD Integration. //REMOVE?? HELM??

**Content:**
- GitHub Actions snippet:
  ```yaml
  # .github/workflows/test.yml
  - name: Validate authorization model
    run: |
      cd model_advanced_demo/folder_document_with_mcp_demo
      fga model test --tests tests.fga.yaml
  ```
- Notes:
  - `fga model test` is a hermetic, sub-second check. Run it on every PR.
  - A regression in your auth model is a security incident — catch it before merge.

**Speaker notes:**
Bring the TDD idea from Step 2 into the pipeline. Because the test is hermetic and fast, it belongs on every PR. Frame the stakes: an auth regression is a security incident, so this gate is non-negotiable.

---

## Slide 32 — Observability

**Content:**
- Bullets:
  - Built-in **OpenTelemetry** traces and metrics
  - Log every `Check` decision: allow / deny + the tuple resolution path (use `Expand` to capture the "why")
  - Track P99 latency on `Check` — it sits in the request hot path
  - Alert on tuple count growth — runaway grants indicate a bug or a missing revoke

**Speaker notes:**
Observability essentials. OTel is built in. Log decisions *with the reasoning path* so audits answer "why." Watch Check latency since it's in every request. And watch tuple growth — an unexpected climb usually means grants aren't being revoked.

---

## Slide 33 — When Not to Use OpenFGA

**Content:**
- Bullets (anti-patterns):
  - **Trivial apps** with one role and 10 users — overkill.
  - **Pure attribute-based** decisions (`if request.region == us-east`). ReBAC is about relationships; tools like OPA fit better.
  - **Time-of-day rules** in the engine. ReBAC has no time concept — enforce in your app, write/delete tuples around capability windows.
- Closing line: OpenFGA shines when **relationships drive authorization**: documents, folders, teams, projects, agents.

**Speaker notes:**
Build trust by being honest about the boundaries. Don't reach for ReBAC when a single role suffices, when decisions are pure attributes (use OPA), or when you need time logic (the engine has no clock). The closing line restates the sweet spot.

---

## Slide 34 — Recap

**Content:**
- Numbered takeaways:
  1. **Tuples** describe relationships. **Models** describe what those relationships mean.
  2. **`can_*` relations** decouple intent from role.
  3. **Intersection (`and`)** bounds AI-agent power without re-architecting your role model.
  4. **`fga model test`** is TDD for authorization. Run it in CI.
  5. **`make up && make demo`** runs the full live stack locally.

**Speaker notes:**
The five things to remember. Re-emphasize point 3 — the intersection pattern for agents is the most novel and timely idea. Point 5 invites them to try it themselves right after the talk.

---

## Slide 35 — Further Reading

**Content:**
- Link list:
  - openfga.dev — official docs and tutorials
  - OpenFGA DSL reference — openfga.dev/docs/configuration-language
  - Zanzibar paper — the foundation
  - OpenFGA Go SDK — github.com/openfga/go-sdk
  - `docs/model-explained.md` — annotated walkthrough in this repo
  - `docs/architecture.md` — Go server architecture

**Speaker notes:**
Point people to next steps: official docs to go deeper, the Zanzibar paper for the theory, the Go SDK to build, and the in-repo docs for an annotated tour of exactly what we showed.

---

## Slide 36 — Q & A / Thank You

**Content:**
- Heading: **Q & A**
- Workshop repo quickstart:
  ```bash
  git clone <this-repo>
  cd OpenFGADemo
  make up && make demo
  ```
- Invitation: Questions, war stories, edge cases — let's hear them.
- Closing: **Thank you.**

**Speaker notes:**
Open the floor. Repeat the quickstart so anyone can reproduce the demo. Invite real-world questions and edge cases — that's where the best discussion happens. Thank the audience.
