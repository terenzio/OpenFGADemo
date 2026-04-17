# Architecture

## System Overview

```
┌─────────────────────────────────────────────────┐
│  Docker Compose                                 │
│  ┌──────────┐   ┌─────────────┐   ┌──────────┐ │
│  │ MariaDB  │◄──│  OpenFGA    │──►│Playground│ │
│  │  :3306   │   │  :8080      │   │  :3000   │ │
│  └──────────┘   └─────────────┘   └──────────┘ │
└─────────────────────────────────────────────────┘
          ▲
          │ HTTP :8080
          │
┌─────────────────┐        ┌───────────────────┐
│  cmd/server     │        │  cmd/cli           │
│  HTTP :8000     │        │  interactive REPL  │
└─────────────────┘        └───────────────────┘
     │        │
     ▼        ▼
internal/  internal/
httpapi    fga
     │        │
     ▼        ▼
internal/  internal/
store      auth
```

## Components

### Infrastructure (Docker Compose)

**MariaDB (:3306)**
Persistent storage backend for OpenFGA. Holds authorization tuples and model
definitions across restarts. The `openfga-migrate` service runs schema
migrations before the main server starts.

**OpenFGA Server (:8080)**
The authorization engine. Evaluates permission checks, stores relationship
tuples, and serves the gRPC and HTTP APIs. All permission logic lives here;
the Go application only calls out to it.

**OpenFGA Playground (:3000)**
A browser UI bundled with OpenFGA for interactively exploring and testing the
authorization model. Useful for visualizing the tuple graph and running ad-hoc
checks.

### Application Binaries

**cmd/server**
The demo HTTP API server. Bootstraps an OpenFGA store and model on startup,
optionally seeds demo data (`-seed -exit`), and listens on `:8000`. All routes
require an `X-User-Id` header; every data-mutating or data-reading operation
calls OpenFGA before returning a response.

**cmd/cli**
An interactive teaching walkthrough that steps through all five FGA concepts
(Check, Write, Delete, ListObjects, Expand) with explanations printed to the
terminal. Does not require the HTTP server to be running.

### Internal Packages

**internal/fga**
Wraps the OpenFGA Go SDK. Provides `Check`, `WriteTuples`, `DeleteTuples`,
`ListObjects`, and `Expand`. Also contains `Bootstrap` (uploads the DSL model)
and `EnsureStoreAndModel` (one-call setup used by both binaries).

**internal/store**
Thread-safe in-memory store for `Document`, `Folder`, and `Organization`
entities. Intentionally simple — all authorization decisions are delegated to
OpenFGA; this package only holds the application data.

**internal/auth**
Two helpers: `Middleware` extracts the `X-User-Id` header and injects it into
the request context; `Authorize` calls `fga.Check` and returns a typed
`ForbiddenError` or `UnauthorizedError` so handlers can remain terse.

**internal/httpapi**
Chi-based HTTP handlers for all routes. Each handler follows the same pattern:
authenticate (via middleware), authorize (via `auth.Authorize` or an inline
`fga.Check`), mutate data, write FGA tuples. Also defines the `FGAClient`
interface so handlers are testable with a mock.
