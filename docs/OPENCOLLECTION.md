# OpenCollection YAML (Bruno 2.x)

Status: **verified for reading**. Bruno 2.0 ships two collection formats:

- `.bru` + `bruno.json` (the format described in [BRU_FORMAT.md](BRU_FORMAT.md)).
- OpenCollection YAML (`opencollection.yml` + `folder.yml` + item `.yml` files),
  the default for new collections in Bruno 2.x.

apigo must edit both. M1-13 (reader) is implemented in
`internal/opencollection` (YAML document layer) and `internal/collection/yaml.go`
(schema mapping); M1-14 adds the writer, M8 the migration. §8 records the shapes
verified against Bruno fixtures, which corrected several guesses in §3.

## 1. Format detection

A directory is an OpenCollection when it contains `opencollection.yml` (or
`opencollection.yaml`). Otherwise, `bruno.json` + `.bru` files mean the classic
format. When both config files exist, **OpenCollection wins** (Bruno behavior);
show a warning and let the user choose, but default to YAML. New collections in
Bruno 2.x default to YAML (`DEFAULT_COLLECTION_FORMAT = "yml"`), so both formats
must be first-class.

Do not detect by file extension alone: item files are `.yml`, but not every
`.yml` is a request.

## 2. Collection file

```yaml
opencollection: '1.0.0'

info:
  name: my-collection
  version: '2.5'

request: # collection-level request defaults
  headers:
    - name: content-type
      value: application/json
  auth:
    type: basic
    username: john
    password: '{{password}}'
  variables:
    - name: baseUrl
      value: https://api.example.com
  scripts:
    - type: pre-request
      code: |-
        console.log('collection pre');
  actions:
    - type: set-variable
      phase: after-response
      selector: { expression: res.body.token, method: jsonq }
      variable: { name: token, scope: runtime }

docs:
  content: Shared defaults
  type: text/markdown

config:
  proxy:
    inherit: true
    config:
      protocol: http
      hostname: ''
      port: ''
      auth:
        username: ''
        password: ''
      bypassProxy: ''

bundled: false

extensions:
  bruno:
    ignore:
      - node_modules
      - .git
    presets:
      request:
        type: graphql
```

- `opencollection` is the format version (`1.0.0` in the target snapshot).
- `info.name` is the display name.
- `request` holds the defaults every item inherits: `headers`, `auth`,
  `variables`, `scripts`, `actions`.
- `config` mirrors `bruno.json` (proxy, certificates, scripts, security).
- `extensions.bruno` holds Bruno-specific settings (ignore list, presets).
- Unknown keys must be preserved.

## 3. Item files

Each request is a YAML file. The `info.type` selects the request shape:

```yaml
info:
  name: Get user
  type: http
  seq: 1
  tags: [users]

http:
  method: GET
  url: https://api.example.com/users/:id
  headers:
    - name: Accept
      value: application/json
  params:
    - name: id
      value: '42'
      type: path
  body:
    type: json
    data: |
      {}
  auth:
    type: bearer
    token: '{{token}}'

runtime:
  variables:
    - name: id
      value:
        type: number
        data: '42'
  scripts:
    - type: pre-request
      code: |
        bru.setVar('x', 1);
  assertions:
    - expression: res.status
      operator: eq
      value: 200
  actions:
    - type: set-variable
      name: userId
      expression: res.body.id

settings:
  encodeUrl: true
  timeout: 30000
  followRedirects: true
  maxRedirects: 5
  forwardAuthorizationHeader: true

examples:
  - name: Success
    request:
      url: https://api.example.com/users/42
      method: GET
    response:
      status: 200
      statusText: OK
      headers:
        - name: Content-Type
          value: application/json
      body:
        type: json
        content: '{"id":42}'
```

Known request types: `http`, `graphql`, `grpc`, `websocket`, `script`, `app`,
`folder` (folders are separate files). The exact body/auth/assert/action shapes
must be taken from `packages/bruno-filestore/src/formats/yml/` and its specs, not
guessed.

## 4. Folders and environments

- Folders have their own `folder.yml` with `info.type: folder`, and put
  headers/auth/variables/scripts under a `request` block, exactly like the
  collection root.
- Environments live under `environments/<name>.yml`:
  `name`, optional `extends` (string or list), `color`, `variables` and
  `externalSecrets`. A secret is `{name, secret: true}` with no value on disk.
- `mock` configuration and `script`/`app` items exist in the format; preserve
  them even before the UI supports them.

## 5. Mapping to the apigo model

| OpenCollection                            | apigo model                                 |
| ----------------------------------------- | ------------------------------------------- |
| `info`                                    | `Meta` (name, type, seq, tags, description) |
| `http` / `graphql` / `grpc` / `websocket` | `Request` protocol fields                   |
| `runtime.variables`                       | request/folder variables (typed)            |
| `runtime.scripts`                         | pre/post scripts (typed by phase)           |
| `runtime.assertions`                      | assertions                                  |
| `runtime.actions`                         | post-response variable actions              |
| `settings`                                | request settings                            |
| `examples`                                | examples                                    |
| `config` / `extensions.bruno`             | collection config + raw extras              |

The model is format-neutral; both readers produce the same `collection` types.
The printer for each format is separate.

## 6. Round-trip and conversion policy

- Read: preserve unknown keys and ordering where the YAML library allows it
  (use `yaml.Node`, not `map[string]any`).
- Write: canonical YAML, 2-space indent, LF, quoted strings only where needed.
- Convert `.bru` → YAML and YAML → `.bru` through the model, with a warning list
  for fields that cannot be represented.
- Never silently drop scripts, assertions, examples or unknown extensions.
- `bruno.json` ↔ `config` mapping is lossy in both directions; record unmapped
  keys in `Extra`.

## 7. Open questions for M1-13

All answered in §8:

1. ~~Which format does Bruno 2.0 create by default?~~ filestore defaults to
   `yml`; existing `.bru` collections keep their format. Both are first-class.
2. ~~Exact `body`, `auth`, `assertions`, `actions` and `scripts` YAML shapes?~~
   See §8.1–§8.4.
3. ~~`extensions.bruno` vs top-level `config`?~~ `config` is the format's own
   block; `extensions.bruno` only holds Bruno-only settings (ignore list,
   presets). Both are preserved as raw nodes.
4. ~~Environment `extends` syntax and secret representation?~~ §8.5.
5. ~~Folder file name?~~ `folder.yml` (or `.yaml`); the root file is
   `opencollection.yml`. There is no `collection.yml` in the 2.0.0 snapshot this
   document was verified against, despite earlier notes.

## 8. Shapes verified against Bruno fixtures (M1-13)

Verified by reading the fixtures vendored in
`internal/collection/testdata/bruno-yaml/` and the 789-file Bruno checkout under
`~/Documents/bruno`.

### 8.1 Item envelope

```yaml
info: { name, type, seq, tags, description }
http: { method, url, headers, params, body, auth } # or graphql/grpc/websocket
runtime: { variables, scripts, assertions, actions }
settings: { encodeUrl, timeout, followRedirects, maxRedirects, forwardAuthorizationHeader }
docs: { content, type }
examples: [...]
```

- `info.type` is `http`, `graphql`, `grpc`, `websocket`, `folder`, `script`,
  `app`. apigo maps the first four onto `RequestType` (`websocket` becomes `ws`)
  and describes `info.description` as `Meta.Description`. `script`/`app` items
  stay on disk but are not shown as requests; the loader skips them with a
  warning instead of inventing an empty GET.
- The protocol block is named after the type (`graphql:`, `grpc:`,
  `websocket:`), and carries the same field names as `http:`.

### 8.2 Body

```yaml
body:
  type: json # json | text | xml | sparql | form-urlencoded | multipart-form | file
  data: |- # raw content for json/text/xml/sparql
    {"a": 1}
```

- Raw bodies use **`data`**, not `content` (earlier drafts had this wrong).
- GraphQL bodies omit `type` and use `query` + `variables` instead; the reader
  treats a body with `query` as GraphQL.
- `form-urlencoded` and `multipart-form` bodies put a list of
  `{name, value, disabled, description}` entries in `data`; multipart file
  entries add `files: [path]` and `contentType`.
- `file` bodies are a list of `{path, disabled, contentType}`.

### 8.3 Auth

```yaml
auth:
  type: bearer
  token: '{{token}}'
```

- `type` is the mode; every other **scalar** field is a mode parameter, in source
  order. `type: none` and `type: inherit` are written as bare scalars
  (`auth: inherit`), so the reader accepts both shapes. Nested objects inside
  `auth` (OAuth2 additional parameters) are not flattened into params; they stay
  in the source document for the writer.
- apigo maps this onto `Auth{Mode, Params}` without interpreting the fields, so a
  new Bruno auth mode survives a read.

### 8.4 Scripts, assertions, actions

```yaml
runtime:
  scripts:
    - type: before-request # before-request | after-response | tests | grpc:*
      code: |-
        bru.setVar('x', 1);
  assertions:
    - expression: res.status
      operator: eq
      value: '200'
      disabled: false
  actions:
    - type: set-variable
      phase: after-response
      selector: { expression: res.body.id, method: jsonq }
      variable: { name: userId, scope: runtime } # scope: runtime | request
```

- Script types are `before-request`, `after-response`, `tests` and the four
  `grpc:*` hooks. The reader also accepts `pre-request`/`post-response`, the
  `.bru` spelling, in case a file uses it.
- Post-response variables are `set-variable` actions; `selector.expression` is
  the value and `variable.scope: request` marks a local variable. They map to
  `Request.VarsPost`, and to `Folder.VarsPost` for the `request:` block of a
  folder or the collection root (`ResolvedVarsPost` merges the chain).
  `selector.method`, `phase` and non-`set-variable` actions are not modelled; they
  remain in the document for the writer.

### 8.5 Environments

```yaml
name: Local
extends: Base # or a list
color: '#4caf50'
variables:
  - name: host
    value: http://localhost:8081
  - name: retries
    value: { type: number, data: '3' }
  - name: token
    secret: true # no value on disk
externalSecrets:
  type: 1password
  variables: [{ name: apiKey, value: '' }]
```

- `extends` accepts a scalar or a list, matching the `.bru` environment file.
- A typed value is `{type, data}`; `string` is the implicit default and is
  omitted by the writer.

### 8.6 What the reader does with unknown data

`internal/opencollection` keeps the whole `yaml.Node` tree, including comments,
key order and unknown keys. `internal/collection` maps the fields it models onto
`Request`/`Folder`/`Environment` and **keeps the document on the model**
(`Request.Source`, `Folder.Source`), which is what makes the preserved data
reachable: `Source.Bytes()` still contains a key the model never read. The M1-14
writer patches those nodes and writes the tree back, so unmodelled fields survive
a save without being copied into the model.

Limits, all documented rather than silent:

- Merge keys (`<<: *defaults`) are not expanded when decoding to a `Node`, so a
  file relying on them reads as if the keys were absent. Bruno does not emit
  them.
- Only the first document of a multi-document YAML file is read.
- Duplicate mapping keys: the first wins.
- `docs.type` is not modelled; the writer always emits `text/markdown`.
- Examples are stored as YAML fragments (`Example.Content`) so they survive a
  read; the writer re-emits them as a list.
