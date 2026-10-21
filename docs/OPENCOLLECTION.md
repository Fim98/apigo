# OpenCollection YAML (Bruno 2.x)

Status: **provisional**. Bruno 2.0 ships two collection formats:

- `.bru` + `bruno.json` (the format described in [BRU_FORMAT.md](BRU_FORMAT.md)).
- OpenCollection YAML (`opencollection.yml` + `folder.yml` + item `.yml` files),
  the default for new collections in Bruno 2.x.

apigo must edit both. M1-13 implements the reader, M1-14 the writer; M8 adds
bidirectional migration. This document records what is known from the Bruno
2.0.0 source; it is verified against fixtures in M1-13 before implementation.

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
    content: |
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

- Folders have their own YAML with `info.type: folder`, plus headers/auth/vars/
  scripts and child ordering.
- Environments live under `environments/<name>.yml` with variables, secrets and
  `extends`; the shape follows the OpenCollection environment types.
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

1. ~~Which format does Bruno 2.0 create by default?~~ **Answered:** filestore
   defaults to `yml`; existing `.bru` collections keep their format. Support both.
2. Exact `body`, `auth`, `assertions`, `actions` and `scripts` YAML shapes (use
   `packages/bruno-filestore/src/formats/yml/items/*` and its specs).
3. How `extensions.bruno` interacts with top-level `config` when both exist.
4. Environment `extends` syntax and secret representation (`name` + `secret: true`,
   no value on disk).
5. Folder file name is `folder.yml`; collection root settings are `collection.yml`;
   verify nested naming and the `mock` block.

Answer these from fixtures in
`packages/bruno-filestore/src/formats/yml/**/*.spec.ts` and record the results
here before writing the parser.
