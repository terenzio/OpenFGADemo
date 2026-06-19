# Authoring OpenFGA Models

> Reference guidance retrieved from the **OpenFGA MCP Server** (`openfga-mcp`),
> context prompt **"Author authorization models with OpenFGA"**
> (`authorization-model.md`), via `get_context_for_query`. Reproduced here as a
> standalone reference for this workshop.

This guide provides a comprehensive overview of authoring OpenFGA authorization
models. It covers core concepts, the modeling language, relationship
definitions, and testing methodologies, drawing insights from the
`openfga/sample-stores` repository for practical examples.

## 1. Introduction to OpenFGA and Authorization Modeling

OpenFGA is an open-source authorization solution that empowers developers to
implement fine-grained access control within their applications through an
intuitive modeling language. It functions as a flexible authorization engine,
simplifying the process of defining application permissions.

Inspired by Google's Zanzibar paper, OpenFGA primarily champions
Relationship-Based Access Control (ReBAC), while also effectively addressing use
cases for Role-Based Access Control (RBAC) and Attribute-Based Access Control
(ABAC). Its "developer-first" philosophy is evident in its Domain Specific
Language (DSL) and supporting tools, which lower the barrier to entry for
developers.

The core purpose of an authorization model is to define a system's permission
structure, answering questions like, "Can user U perform an action A on object
O?". By externalizing authorization logic from application code, OpenFGA
provides a robust mechanism for managing complex access policies, especially in
large-scale systems. The modeling language is designed to be powerful for
engineers yet accessible to other team stakeholders, fostering collaborative
policy development.

## 2. OpenFGA Core Concepts: The Building Blocks of Your Model

- **Authorization Model:** A static blueprint that combines one or more type
  definitions to precisely define the permission structure of a system. It
  represents the *possible* relations between users and objects. Models are
  immutable; each modification creates a new version with a unique ID. Models
  aren't expected to change often — only when new product features or change in
  functionality is introduced. They're also generally expected to be backward
  compatible, but can break backward compatibility once the system has
  completely moved off the older relations in it.
- **Type:** A string that defines a class of objects sharing similar
  characteristics (e.g., user, document, folder, organization, team, repo).
- **Object:** A specific instance of a defined Type (e.g., `document:roadmap`,
  `user:anne`, `organization:acme`). An object's relationships are defined
  through relationship tuples and the authorization model.
- **User:** An entity that can be related to an object. A user can be a specific
  individual (e.g., `user:anne`), a wildcard representing everyone of a certain
  type (e.g., `user:*` which means all users), or a userset (e.g.,
  `team:product#member`), which denotes a group of users.
- **Relation:** A string defined within a type definition in the authorization
  model. It specifies the *possibility* of a relationship existing between an
  object of that type and a user. Relation names are arbitrary (e.g., owner,
  editor, viewer, member, admin), but must be defined on the object type in the
  model.
- **Relationship Tuple:** Dynamic data elements representing the *facts* about
  relationships between users and objects (e.g.,
  `{"user": "user:anne", "relation": "editor", "object": "document:new-roadmap"}`).
  Without these, authorization checks will fail, as the model only defines what
  is *possible*, not what *currently exists*.

The clear separation between the static authorization model (schema) and dynamic
relationship tuples (data) is a fundamental design principle. This enables
efficient permission evaluation and decouples core logic changes from specific
user permission modifications. The immutability of models supports robust
versioning, allowing for controlled rollouts and managing complex migrations.

| Concept | Description | Example |
| :---- | :---- | :---- |
| **Type** | A class of objects that share similar characteristics. | user, document, folder, organization |
| **Object** | A specific instance of a defined type, an entity in the system. | `user:anne`, `document:report_2023`, `folder:marketing_docs` |
| **User** | An entity that can be related to an object. Can be a specific object, a wildcard, or a userset. | `user:bob`, `user:*` (everyone), `team:product#member` |
| **Relation** | A string defined within a type definition that specifies the possibility of a relationship between an object of that type and a user. | owner, editor, viewer, member, admin |
| **Relationship Tuple** | A grouping of a user, a relation, and an object, representing a factual relationship in the system. | `{"user": "user:anne", "relation": "viewer", "object": "document:roadmap"}` |
| **Authorization Model** | A static definition combining type definitions to define the entire permission structure of a system. | `type document relations define viewer: [user]` |

## 3. The OpenFGA Modeling Language: DSL

OpenFGA's Configuration Language is fundamental to constructing a system's
authorization model, informing OpenFGA about object types and their
relationships. It describes all possible relations for an object of a given type
and the conditions under which one entity is related to that object. The
language is primarily expressed in DSL (Domain Specific Language).

### DSL: Developer-Friendly Syntax for Readability

The DSL provides syntactic sugar over the underlying JSON, designed for ease of
use and improved readability. It is the preferred syntax for developers using
the Playground, CLI, and IDE extensions (like Visual Studio Code), offering
features like syntax highlighting and validation. DSL models are compiled to
JSON before being sent to OpenFGA's API.

```dsl.openfga
model
  schema 1.1
type user
type document
  relations
    define viewer: [user] or editor
    define editor: [user]
```

## 4. Defining Relationships: Crafting Your Authorization Logic

### Direct Relationships: Explicit Access Grants

A direct relationship is established when a specific relationship tuple (e.g.,
user=X, relation=R, object=Y) is explicitly stored. The authorization model must
explicitly permit this through direct relationship type restrictions. These
restrictions define which types of users can be directly associated with an
object for a given relation, using formats like `[<type>]`, `[<type:*>]`, or
`[<type>#<relation>]`.

```
define owner: [user]
```

means only individual users can be directly assigned as owners. A tuple like
`{"user": "user:anne", "relation": "owner", "object": "document:1"}` is a direct
relationship.

### Concentric Relationships: Inheriting Permissions

Concentric relationships represent nested or implied relations, where one
relation automatically confers another (e.g., "all editors are viewers"). This
is implemented using the `or` keyword within a relation definition.

```
define viewer: [user] or editor
```

means a user is a viewer if directly assigned OR if they are an editor. If
`user:anne` is an editor of `document:new-roadmap`, she implicitly has viewer
access, reducing the number of required tuples.

### Indirect Relationships with 'X from Y': Scalable Hierarchical and Group-Based Access

The `X from Y` syntax is crucial for scalability, allowing a user to acquire a
relation (X) to an object through an intermediary object (Y) and a defined
relation on Y. This avoids individual tuple creation for every permission,
enabling higher-level abstraction. It is highly effective for hierarchical and
group-based access control.

```
define admin: [user] or repo_admin from organization
```

on a `repo` type means a user is an admin if directly assigned, or if they have
the `repo_admin` relation to an org that owns the repo. This simplifies
management; revoking access can be done by deleting a single tuple linking the
intermediary.

### Contextual Authorization with Conditions: Dynamic Permissions

Conditions introduce dynamic, contextual authorization. A condition is a
function using Google's Common Expression Language (CEL), with parameters and a
boolean expression.

```
condition less_than_hundred(x: int) {
  x < 100
}
```

Conditions are required to be defined at the end of the model (after the type
definitions), and are instantiated using conditional relationship tuples.

### Leveraging Usersets for Group-Based Access Control

A userset represents a set or collection of users, denoted by `object#relation`
(e.g., `company:xy#employee`). Usersets are fundamental for assigning permissions
to groups, reducing tuple count and providing flexibility for bulk access
management. They can be used in direct relationship type restrictions, such as:

```
define editor: [user, team#member]
```

OpenFGA computes implied relationships based on userset membership, and usersets
are integral to defining complex access rules involving union, intersection, or
exclusion of groups.

Note that specifying `team#member` means "all members from a specific team". It
does not mean that "you need to be a team member to be an editor". Only use it
when you need to assign a relation to a set of users from specific object.

| Pattern Name | Description | DSL Syntax Example | Explanation of Effect |
| :---- | :---- | :---- | :---- |
| **Direct Relationship** | Explicitly grants a user a relation to an object via a stored tuple, subject to type restrictions. | `define owner: [user]` | Only individual users can be directly assigned as owner. |
| **Concentric Relationship** | Defines that having one relation implies having another relation to the same object. | `define viewer: [user] or editor` | A user is a viewer if directly assigned OR if they are an editor. |
| **Indirect Relationship ('X from Y')** | A user gains a relation (X) to an object through another object (Y) and a specific relation on Y. | `define admin: [user] or repo_admin from owner` | A user is admin of a repo if directly assigned OR if they are repo_admin of an org that owns the repo. |
| **Conditional Relationship** | A relationship is permissible only if a specified condition, evaluated at runtime, is true. | `define admin: [user with non_expired_grant]` | A user is admin only if the non_expired_grant condition evaluates to true. |
| **Usersets** | Represents a collection of users (e.g., a group or a set of users related by a specific relation). | `define editor: [user, team#member]` | An editor can be a direct user OR any member of a specified team. |

## 5. Step-by-Step: Authoring Your First OpenFGA Model

The process is iterative, starting with critical features and systematically
translating authorization requirements into a structured model. The central
question is: "Why could user U perform an action A on object O?"

Recommended steps:

1. **Pick the most important feature:** Focus on a high-priority use case.
2. **List the object types:** Identify all relevant entities (user, document,
   folder, organization).
3. **List relations for those types:** Determine relationships users or other
   objects can have (owner, editor, viewer, member).
4. **Define relations:** Translate into DSL — direct, concentric, indirect.
5. **Test the model:** Validate against expected behaviors using assertions.
6. **Iterate:** Refine based on testing and evolving requirements.

### Illustrative Example: Document Management Authorization Model

Requirements: documents created by users; sharing grants editor/viewer; creators
get delete/share/edit/view; editors edit and view; viewers only view; documents
belong to folders with inherited permissions.

#### Step 1: Identify Types
Core types: `user`, `document`, `folder`. An `organization` type can represent groups.

#### Step 2: Define Relations for organization
```dsl.openfga
type organization
  relations
     define member: [user] # Users can be members.
```

#### Step 3: Define Relations for document
```dsl.openfga
type document
  relations
    define organization: [organization] # A document belongs to an organization.
    define parent_folder: [folder] # A document can have a parent folder.

    define owner: [user]
    define editor: [user] or owner or editor from parent_folder
    define viewer: [user] or editor or viewer from parent_folder or member from organization
    define can_share: owner
    define can_delete: owner or editor
```

#### Step 4: Consider Folder Inheritance (Parent-Child Objects)
```dsl.openfga
type folder
  relations
    define parent_folder: [folder]
    define owner: [user]
    define editor: [user] or owner or editor from parent_folder
    define viewer: [user] or editor or viewer from parent_folder
```

## 6. Adding Permissions

It's a common practice to define specific permissions, that can't be directly
assigned, using `can_<permission>` relations:

```dsl.openfga
type folder
  relations
    define parent_folder: [folder]
    define owner: [user]
    define editor: [user] or owner or editor from parent_folder
    define viewer: [user] or editor or viewer from parent_folder

    define can_view: viewer
    define can_edit: editor
    define can_delete: editor
    define can_share: owner
```

**Always define permissions in the authorization models.**

## 7. Testing and Validating Your OpenFGA Models

**ALWAYS test the models you create. Run the `fga` CLI command directly, do not
create a script to call the CLI.**

### OpenFGA CLI

A cross-platform command-line tool for interacting with an OpenFGA server:

- Reading, writing, validating, and transforming authorization models.
- Running tests on an authorization model.
- Managing OpenFGA stores (create, list, retrieve, delete, import, export).

Run tests with: `fga model test --tests <filename>.fga.yaml`

Install options:
- macOS (Homebrew): `brew install openfga/tap/fga`
- Debian: `sudo apt install ./fga_<version>_linux_<arch>.deb`
- Fedora: `sudo dnf install ./fga_<version>_linux_<arch>.rpm`
- Alpine: `sudo apk add --allow-untrusted ./fga_<version>_linux_<arch>.apk`
- Windows (Scoop): `scoop install openfga`
- Docker: `docker pull openfga/cli; docker run -it openfga/cli`

### Automated Testing with `.fga.yaml`

Fields:
- `name` (optional): descriptive name.
- `model` or `model_file`: inline model or reference to `.fga`/`.json`/`.mod`.
- `tuples` / `tuple_file` / `tuple_files` (optional): inline or external tuples,
  applied to all tests.

Inline model + conditional tuples example:

```yaml
name: Model Tests # optional
model: |
  model
  schema 1.1
  type user
  type organization
    relations
    define member : [user]
    define admin : [user with non_expired_grant]
  condition non_expired_grant(current_time: timestamp, grant_time: timestamp, grant_duration: duration) {
    current_time < grant_time + grant_duration
  }
tuples:
  - user: user:anne
    relation: member
    object: organization:acme
  - user: user:peter
    relation: admin
    object: organization:acme
    condition:
      name: non_expired_grant
      context:
        grant_time : "2024-02-01T00:00:00Z"
        grant_duration : 1h
```

Three test kinds:

- **Check Tests:** verify whether user U has relation R with object O.
  ```yaml
  tests:
    - name: Test
      check:
        - user: user:peter
          object: organization:acme
          context:
            current_time : "2024-02-01T00:10:00Z"
          assertions:
            member: false
            admin: true
  ```
- **List Objects Tests:** which objects a user has a relation with.
  ```yaml
  list_objects:
    - user: user:anne
      type: organization
      assertions:
        member:
          - organization:acme
        admin:
    - user: user:peter
      type: organization
      context:
        current_time : "2024-02-01T00:10:00Z"
      assertions:
        member:
        admin:
          - organization:acme
  ```
- **List Users Tests:** which users have access to an object. Specify `object`,
  a `user_filter` (by type or userset), `context`, and `assertions`. The `users`
  field supports `<type>:<id>`, `<type>:<id>#<relation>`, and `<type>:*`.
  ```yaml
  list_users:
    - object: organization:acme
      user_filter:
        - type: user
      context:
        current_time : "2024-02-02T00:10:00Z"
      assertions:
        member:
          users:
            - user:anne
        admin:
          users:
  ```

The `.fga.yaml` format actively promotes a test-driven development methodology
for authorization logic.

## 8. Modeling Custom Roles

Many applications require end-users to define their own custom roles, in addition
to pre-defined roles. **Only consider this if you are asked to implement custom
roles.**

### Simple User-Defined Roles

```dsl.openfga
model
  schema 1.1

type user

type role
  relations
    define assignee: [user]

type organization
  relations
    define admin: [user]  # static role

    # permissions can be assigned to custom roles or static roles
    define can_create_projectexport: [role#assignee] or admin
    define can_edit_project: [role#assignee] or admin
```

**Setup:** define role permissions via tuples, then assign users to the role.

```yaml
- user: role:acme-project-admin#assignee
  relation: can_create_project
  object: organization:acme
- user: role:acme-project-admin#assignee
  relation: can_edit_project
  object: organization:acme
- user: user:anne
  relation: assignee
  object: role:acme-project-admin
```

**Adding new permissions:** existing roles don't automatically receive them.
Create additional tuples to grant a new permission to an existing role. You do
not need to add these tuples when adding the new permission — end-users add it to
their custom roles when appropriate.

### Custom Roles with Role Assignments

Use when roles attach to different object instances with different members per
instance. **DO NOT use this approach for the top-level type (e.g.,
'organization').**

Step 1 — define the role and its permissions:
```dsl.openfga
type role
  relations
    define can_view_project: [user:*]
    define can_edit_project: [user:*]
```
```yaml
- user: user:*
  relation: can_view_project
  object: role:project-admin
- user: user:*
  relation: can_edit_project
  object: role:project-admin
```

Step 2 — assign users to the role on an entity:
```dsl.openfga
type role_assignment
  relations
    define assignee: [user]
    define role: [role]

    define can_view_project: assignee and can_view_project from role
    define can_edit_project: assignee and can_edit_project from role
```

Step 3 — connect to your objects (combine static + custom roles; prefer static
roles when known in advance):
```dsl.openfga
type organization
  relations
    define admin: [user]

type project
  relations
    define organization: [organization]
    define role_assignment: [role_assignment]

    define can_edit_project: can_edit_project from role_assignment or admin from organization
    define can_view_project: can_view_project from role_assignment or admin from organization
```

Setting up role assignments:
```yaml
- user: user:anne
  relation: assignee
  object: role_assignment:project-admin-openfga
- user: role:project-admin
  relation: role
  object: role_assignment:project-admin-openfga
- user: role_assignment:project-admin-openfga
  relation: role_assignment
  object: project:openfga
- user: organization:acme
  relation: organization
  object: project:openfga
```

| Pattern | Use Case | Pros | Cons |
|---------|----------|------|------|
| **Global Custom Roles** | Organization-wide roles with consistent permissions | Simple, efficient | Less flexible for per-resource customization |
| **Role Assignments** | Resource-specific roles with different members per resource | Highly flexible | More complex, potential performance impact |

**Migration strategies:** additive approach → gradual migration → backwards
compatibility during transition.

## 9. Simplify Models

After generating model and tests, remove from the model all types and relations
that are not referenced in the model or the tests.

## 10. Naming Users

When naming users, use proper naming conventions that reflect their roles and
responsibilities within the organization (prefixes/suffixes such as `admin_`,
`member_`, or `guest_`).

## 11. Creating Modules

Modular models allow splitting your authorization model across multiple files.
Use them when you define models with features for multiple products or modules
that have several related types each.

Define an `fga.mod` that lists all modules:

```yaml
schema: '1.2'
contents:
  - core.fga
  - module-1.fga
  - module-2.fga
```

`core.fga` holds types shared across modules (e.g., organization, group, roles).
Each module has types specific to its functionality:

```
module issue-tracker

extend type organization
  relations
    define can_create_project: admin

type project
  relations
    define organization: [organization]
    define viewer: member from organization
```

Test files point to the `fga.mod`:

```
name: Document Management System Authorization Model Tests
model_file: ./fga.mod
```

Notes:
- You can `extend` types defined in other modules and add relations to them.
- All `.fga` files start with `module <module name>` (even `core`). They do not
  start with a schema declaration — that's in `fga.mod`.

**IMPORTANT: Only use modules if you are EXPLICITLY ASKED to create modules.**
