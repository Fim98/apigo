# apigo

An offline-first, Bruno-compatible API client in Go with a native UI. This
glossary fixes the words the project uses so specs, issues, ADRs and test names
stay in one language; each term points at the document that owns the detail.

## Language

### Collections and files

**Collection**:
A directory tree of requests, folders, environments and docs, loaded from
`bruno.json` plus `.bru` files, or from OpenCollection YAML.
_Avoid_: workspace, project, workspace folder

**Request file**:
A single `.bru` file (or OpenCollection item `.yml`) describing one HTTP, gRPC
or WebSocket request.
_Avoid_: endpoint, API, call

**Folder**:
A node inside a collection that groups requests and may carry inherited
headers, auth and variables.
_Avoid_: group, directory

**Environment**:
A named set of variables under `environments/` that a request resolves against.
_Avoid_: profile, env (as a noun for the whole set)

### Format grammar

**Block**:
A named `name { … }` region in a `.bru` file, such as `meta`, `headers` or
`body:json`.
_Avoid_: section, clause

**Disabled pair**:
A `~key: value` entry that is preserved in the file but excluded from the built
request.
_Avoid_: commented-out param, muted key

**Annotation**:
An `@`-prefixed line inside a block that carries metadata such as a description.
_Avoid_: comment, tag

**Text block**:
A `'''`-delimited value used when a line would otherwise break the grammar.
_Avoid_: multiline string, heredoc

**Canonical print**:
The one deterministic serialization of a parsed file: fixed block order,
two-space indent, LF endings, trailing newline.
_Avoid_: format, prettify

**Semantic round-trip**:
The invariant that parse → print → parse yields an equal model.
_Avoid_: lossless save

**Byte stability**:
The invariant that an unmodified file prints identically after the first
canonical save.
_Avoid_: idempotent write

### Compatibility and parity

**Capability matrix**:
The contract table in [CAPABILITY_MATRIX.md](docs/CAPABILITY_MATRIX.md): one row
per Bruno behavior, each classified as parity, divergent, extension or dropped.
_Avoid_: parity checklist, feature list

**Divergent**:
A behavior where apigo deliberately differs from Bruno, documented in the
relevant spec's divergences section.
_Avoid_: bug, deviation

**Extension**:
A capability apigo adds that Bruno has no counterpart for.
_Avoid_: nice-to-have, extra

**Dropped**:
A Bruno capability apigo deliberately does not implement.
_Avoid_: out of scope, missing

**OpenCollection**:
Bruno 2.x's YAML collection format, read and written alongside `.bru`.
_Avoid_: new format, YAML format

### Runtime

**Runner**:
The sequential executor that walks a collection or a selected subset, honouring
delay and tags, and skipping WebSocket and gRPC requests.
_Avoid_: test runner, batch

**Assertion**:
A `res.*` expression inside an `assert` block, evaluated after the response
arrives.
_Avoid_: check, expectation

**Script**:
User JavaScript in the goja sandbox, run before the request or after the
response, with no filesystem or network access.
_Avoid_: hook, plugin

**Variable precedence**:
The fixed resolution order: global environment < collection < environment <
folder < request < OAuth2 credentials < runtime < prompt.
_Avoid_: variable lookup, scope chain

**Secret variable**:
A `vars:secret` name whose value lives in the OS keychain and never in a file,
log or export.
_Avoid_: password, credential (use for the auth scheme, not the variable)

### Planning

**Milestone**:
A roadmap stage (`M0`–`M12`) with exit criteria in
[ROADMAP.md](docs/ROADMAP.md).
_Avoid_: phase, sprint

**Backlog task**:
A row in [BACKLOG.md](docs/BACKLOG.md) with an `M<number>-<nn>` id, dependencies
and acceptance criteria; it defines scope, not work in progress.
_Avoid_: ticket, issue

**Issue**:
A GitHub issue: the execution unit created by the engineering skills, carrying a
`Backlog:` id, discussion and closure. "Ticket" is only used when quoting the
skill docs.
_Avoid_: task, work item

**Handoff**:
[docs/HANDOFF.md](docs/HANDOFF.md), the only file that states where the project
is right now.
_Avoid_: status, standup

## Relationships

- A **Collection** contains many **Request files** and **Folders**
- A **Folder** inherits from its **Collection**; a **Request file** inherits from
  both
- A **Request file** is a sequence of **Blocks**; a **Block** holds **Disabled
  pairs**, **Annotations** or **Text blocks**
- A **Backlog task** is implemented by zero or more **Issues**
- An **Issue** may require an architectural decision, recorded as an ADR in
  [docs/decisions/](docs/decisions/)

## Flagged ambiguities

- "workspace" used to mean both the IDE shell and a collection. Resolved: the UI
  is the _workbench_ (ADR-0008); a directory of requests is a **Collection**.
- "task" used to mean both a backlog row and an issue. Resolved: a **Backlog
  task** has an `M`-id; an **Issue** has a number.
