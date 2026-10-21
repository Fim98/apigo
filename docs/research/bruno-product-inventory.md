# Bruno product / UI / collection capability inventory

Source of truth: read-only inspection of `/Users/liyunfu/Documents/bruno` (commit `dd0281dad`).
All paths below are relative to that repository. Where a claim could not be verified in the
source it is marked **UNCERTAIN**. This is a clean-room behavior inventory, not copied code.

Scope reviewed: `packages/bruno-app`, `bruno-electron`, `bruno-filestore`, `bruno-lang`,
`bruno-schema`, `bruno-toml`, `bruno-converters` (plus `bruno-js`, `bruno-requests`,
`bruno-common` where the product behavior lives there).

---

## 0. Executive summary of the model

- Bruno 2.x supports **two on-disk formats for the same logical collection**: the original
  `.bru` text format and the OpenCollection YAML format (`opencollection.yml`, `folder.yml`,
  `*.yml` requests/environments). Both are parsed into one in-memory item model
  (`packages/bruno-schema/src/collections/index.js`) and can be converted to each other by
  `renderer:migrate-collection-to-yml` (`packages/bruno-electron/src/ipc/yml-migration.js`).
- A collection is a directory. A request is one file. A folder is a directory containing a
  `folder.bru` / `folder.yml`. Environments are files under `environments/`.
- Inheritance (headers/auth/vars/scripts/tests/tags) flows **collection root → folders → request**.
- Variables come in several scopes; `{{...}}` interpolation merges them in a fixed order, and
  scripts can mutate env/runtime/collection/global scopes at runtime.
- The desktop app is Electron + React/Redux; all filesystem, network, Git, mock-server and script
  execution happens in the Electron main process behind `ipcRenderer.invoke` channels.
- The current OSS checkout has **no Git source-control panel** (see §5); it has clone-repository
  and connect/disconnect-remote only, while the underlying Git utility still contains a full
  command surface.

---

## 1. Collection on-disk model

### 1.1 Directory layout

```
my-collection/                      # any directory name; collection name lives in config
├── bruno.json                      # bru-format collection config (required for bru format)
│   or opencollection.yml           # yml-format collection config (required for yml format)
├── collection.bru                  # optional collection-root settings (bru format)
├── collection.yml                  # optional collection-root settings (yml format)
├── environments/
│   ├── Local.bru  | Local.yml
│   └── Production.bru | Production.yml
├── .env, .env.local, .env.*        # process.env files (not collection vars)
├── folder-a/
│   ├── folder.bru | folder.yml     # folder metadata + request defaults
│   ├── get-users.bru | get-users.yml
│   └── nested/
│       ├── folder.bru | folder.yml
│       └── ping.bru | ping.yml
├── root-request.bru | root-request.yml
└── scripts/                        # ordinary folder unless it has folder.bru/.yml
    └── helper.js                   # arbitrary file; ignored by the request indexer
```

Format detection:

- `opencollection.yml` **wins** over `bruno.json` when both exist
  (`packages/bruno-electron/src/app/collections.js`, `getCollectionConfigFile`).
- A directory is a collection iff it contains either file
  (`packages/bruno-electron/src/utils/filesystem.js`, `isValidCollectionDirectory`).
- In-memory format is derived from the collection config file
  (`getCollectionFormat` in `packages/bruno-electron/src/utils/filesystem.js`).
- The filestore package default is `'yml'` (`packages/bruno-filestore/src/constants.ts`),
  but the app passes the detected collection format explicitly on save/import.

There is **no `.bru.json` file** in this source. `bruno.json` is plain JSON; `collection.bru`
is the root settings file. Do not invent a `.bru.json`.

### 1.2 `bruno.json` (bru-format config)

Validated by `configSchema` in `packages/bruno-electron/src/app/collections.js`:

```json
{
  "version": "1",
  "collectionVersion": "2.5",
  "name": "My API",
  "type": "collection",
  "ignore": ["dist", "tmp"],
  "presets": {
    "requestType": "http",
    "requestUrl": "{{baseUrl}}",
    "defaultEnvironment": "Local"
  },
  "openapi": [
    {
      "sourceUrl": "https://example.com/openapi.json",
      "lastSyncDate": "2026-01-01T00:00:00.000Z",
      "specHash": "…",
      "groupBy": "tags",
      "autoCheck": true,
      "autoCheckInterval": 60
    }
  ],
  "proxy": { "inherit": true, "config": { "...": "..." } },
  "clientCertificates": { "certs": [] },
  "protobuf": { "protoFiles": [], "importPaths": [] },
  "scripts": { "flow": "sandwich", "additionalContextRoots": [] }
}
```

- `name` and `type: "collection"` are required; unknown keys are tolerated by the Yup schema
  (no `noUnknown`).
- `ignore` is a user list. `node_modules` and `.git` are **always** added at open time
  (`packages/bruno-electron/src/app/collections.js`, `openCollection`).
- `version`/`collectionVersion` are used for collection version display and migration; the
  importer writes `version: "1"` and maps `collectionVersion` from the source config
  (`packages/bruno-electron/src/utils/collection-import.js`).

### 1.3 `.bru` grammar

Parser: Ohm grammar + semantics in `packages/bruno-lang/v2/src/bruToJson.js`.
Serializer: `packages/bruno-lang/v2/src/jsonToBru.js`.
Public wrapper: `packages/bruno-lang/src/index.js`
(`bruToJsonV2`, `jsonToBruV2`, `bruToEnvJsonV2`, `envJsonToBruV2`,
`collectionBruToJson`, `jsonToCollectionBru`, `dotenvToJson`, `outdentString`).
Higher-level, format-dispatching API: `packages/bruno-filestore/src/index.ts`
(`parseRequest`, `parseCollection`, `parseFolder`, `parseEnvironment`, `parseDotenv`,
`stringify*`, plus `*ViaWorker` variants).

Block kinds:

| Kind       | Example                                      | Notes                                |
| ---------- | -------------------------------------------- | ------------------------------------ |
| Dictionary | `headers { content-type: application/json }` | key/value pairs, `~` prefix disables |
| Text       | `body:json { { "a": 1 } }`                   | raw text, 2-space outdent on read    |
| List       | `tags [ smoke regression ]`                  | one item per line                    |

Lexical rules worth replicating exactly:

- Blocks end with `}` on a new line (`tagend = nl "}"`). There is **no comment syntax** in the
  request grammar; text outside a known block is a parse error.
- Disabled pairs: `~name: value` (headers, params, form fields, vars, assertions).
- Keys containing `:`, `"`, `{`, `}`, or spaces must be double-quoted
  (`getKeyString` in `packages/bruno-lang/v2/src/utils.js`).
- Multiline dictionary values and URL use `'''` delimiters (`getValueString`, `getValueUrl`).
- Annotations precede a pair on their own line: `@description('...')`, typed variable
  annotations `@number`, `@boolean`, `@object`, and multipart/file `@contentType(...)`.
  Arbitrary annotations are preserved in `item.annotations`
  (`serializeAnnotations`, `extractTypedAnnotations` in `packages/bruno-lang/v2/src/utils.js`).
- `meta.type` defaults to `http`; missing `meta.seq` defaults to `1`.

### 1.4 Request file blocks (bru)

From the grammar (`bruToJson.js`) and `parseBruRequest` in
`packages/bruno-filestore/src/formats/bru/index.ts`:

| Block                                                                           | Meaning                                         | Model field                                |
| ------------------------------------------------------------------------------- | ----------------------------------------------- | ------------------------------------------ |
| `meta { name, type, seq, tags }`                                                | name/type/ordering/tags                         | `name`, `type`, `seq`, `tags`              |
| `get/post/put/delete/patch/options/head/connect/trace { ... }`                  | standard HTTP method + `url`, `body`, `auth`    | `request.method/url/body.mode/auth.mode`   |
| `http { method: ..., url: ... }`                                                | custom/verbatim method                          | same                                       |
| `params:query { ... }`                                                          | query params                                    | `request.params[].type='query'`            |
| `params:path { ... }`                                                           | path params                                     | `request.params[].type='path'`             |
| `query { ... }`                                                                 | legacy alias of query params                    | same                                       |
| `headers { ... }`                                                               | headers                                         | `request.headers`                          |
| `metadata { ... }`                                                              | gRPC metadata                                   | `request.headers` (gRPC)                   |
| `auth:awsv4/basic/bearer/digest/ntlm/oauth1/oauth2/wsse/apikey/akamai-edgegrid` | auth config                                     | `request.auth[mode]`                       |
| `auth:oauth2:additional_params:*`                                               | extra auth/token/refresh request params         | `request.auth.oauth2.additionalParameters` |
| `body:json/text/xml/sparql`                                                     | text bodies                                     | `request.body.*`                           |
| `body:graphql`, `body:graphql:vars`                                             | GraphQL query/variables                         | `request.body.graphql`                     |
| `body:form-urlencoded { ... }`                                                  | URL-encoded form                                | `request.body.formUrlEncoded`              |
| `body:multipart-form { ... }`                                                   | multipart with `@file(...)`/`@contentType(...)` | `request.body.multipartForm`               |
| `body:file { file: @file(path) @contentType(type) }`                            | raw binary body, one selected file              | `request.body.file`                        |
| `body:grpc { name, content: '''…''' }`                                          | one gRPC message per block                      | `request.body.grpc[]`                      |
| `body:ws { name, type, selected, content }`                                     | one WebSocket message per block                 | `request.body.ws[]`                        |
| `vars:pre-request { ... }`                                                      | request-scoped vars; `@name:` = local           | `request.vars.req`                         |
| `vars:post-response { ... }`                                                    | post-response vars (JS expressions)             | `request.vars.res`                         |
| `assert { $res.status: 200 }`                                                   | assertions                                      | `request.assertions`                       |
| `script:pre-request`, `script:post-response`                                    | JS scripts                                      | `request.script.req/res`                   |
| `script:grpc:before-call-start` etc.                                            | gRPC lifecycle scripts                          | `request.script.beforeCallStart…`          |
| `tests { ... }`                                                                 | JS tests                                        | `request.tests`                            |
| `docs { ... }`                                                                  | Markdown/rich docs                              | `request.docs`                             |
| `settings { ... }`                                                              | per-request settings                            | `item.settings`                            |
| `app { enabled, code }`                                                         | in-app HTML/JS app for the request              | `item.app`                                 |
| `example { ... }`                                                               | saved request/response example                  | `item.examples[]`                          |

Request `settings` keys (grammar + `Settings` UI):

```
encodeUrl, followRedirects, maxRedirects, timeout (ms or "inherit"),
forwardAuthorizationHeader, keepAliveInterval (WS), omitHeaders [ ... ]
```

Timeout `"inherit"` means "use collection/folder/global default"; `0` means no timeout.

### 1.5 Synthesized request example (from `packages/bruno-lang/v2/tests/fixtures/request.bru`)

```bru
meta {
  name: Send Bulk SMS
  type: http
  seq: 1
  tags: [
    smoke
    regression
  ]
}

post {
  url: https://api.example.com/send/:id
  body: json
  auth: bearer
}

params:query {
  apiKey: {{apiKey}}
  "key with spaces": is allowed
  ~message: hello
}

params:path {
  id: 123
}

headers {
  content-type: application/json
  Authorization: Bearer {{token}}
  ~transaction-id: {{transactionId}}
}

auth:bearer {
  token: {{token}}
}

body:json {
  {
    "hello": "world"
  }
}

vars:pre-request {
  departingDate: 2020-01-01
  @orderId: 42
}

vars:post-response {
  token: $res.body.token
  ~petId: $res.body.id
}

assert {
  $res.status: 200
  ~$res.body.message: success
}

script:pre-request {
  const foo = 'bar';
}

script:post-response {
  bru.setVar('seen', res.getStatus());
}

tests {
  test('returns 200', function () {
    expect(res.getStatus()).to.equal(200);
  });
}

docs {
  Needs a bearer token.
}

settings {
  encodeUrl: true
  timeout: 5000
  followRedirects: true
  maxRedirects: 5
}
```

### 1.6 `collection.bru` and `folder.bru`

`collection.bru` uses a _different_ grammar
(`packages/bruno-lang/v2/src/collectionBruToJson.js`). Allowed blocks:
`meta`, `auth { mode }`, every `auth:*`, `headers`, `vars:pre-request`,
`vars:post-response`, `script:pre-request`, `script:post-response`, `tests`, `docs`,
`oauth2 additional_params`.

```bru
meta {
  type: collection
}

headers {
  content-type: application/json
}

auth {
  mode: basic
}

auth:basic {
  username: john
  password: {{password}}
}

vars:pre-request {
  baseUrl: https://api.example.com
}

script:pre-request {
  console.log('collection pre');
}

docs {
  Shared defaults for every request.
}
```

`folder.bru` adds `meta { name, seq, tags }` and often `auth { mode: inherit }`.
Real fixture (`tests/collection/generate-docs/fixtures/collection/Zoo/folder.bru`):

```bru
meta {
  name: Zoo
  seq: 1
}

auth {
  mode: inherit
}
```

Folder/collection root parse result (`parseBruCollection`) is
`{ request: { headers, auth, script, vars, tests }, docs, meta? }`, matching
`folderRootSchema` in `packages/bruno-schema/src/collections/index.js`.

### 1.7 Environments (bru)

Grammar: `packages/bruno-lang/v2/src/envToJson.js`; serializer `jsonToEnv.js`.
Blocks: `vars`, `vars:secret [ ... ]`, `extends` / `extends [...]`, `color:`,
`vars:externalsecrets:<provider>`.

```bru
extends: Local

vars {
  host: https://api.example.com
  retries: 3
  @number: 3
}

vars:secret [
  apiKey,
  password
]

color: #ff8800
```

- `vars:secret` stores **names only**; values live in the Electron `secrets` store,
  encrypted with Electron `safeStorage` (`packages/bruno-electron/src/store/env-secrets.js`,
  `utils/encryption.js`).
- Environment inheritance: `extends` can be a single name or a list; the app resolves
  inheritance in `resolveEnvironmentInheritance` (`@usebruno/common/utils`), and the
  environment editor has an "Inherits from" control
  (`packages/bruno-app/src/components/Environments/Common/InheritsFrom`).
- `vars:externalsecrets:<type>` is parsed and preserved (`type` seen in tests: `vault`,
  `infisical`), but **no editor UI for external secrets was found in the OSS app** — the
  data survives load/save only. **UNCERTAIN** whether a paid edition adds the UI.
- `.env` / `.env.*` are separate from environments: they feed `process.env`.
  Priority is **collection `.env` > workspace `.env` > OS `process.env`**
  (`packages/bruno-electron/src/store/process-env.js`).
  Only `.env` is loaded into `process.env`; `.env.*` files are watched/displayed but do not
  override it (`packages/bruno-electron/src/app/dotenv-watcher.js`).

### 1.8 Examples

- File syntax is a nested "JS-object-like" grammar (`request: { ... }`, `response: { ... }`)
  parsed by `packages/bruno-lang/v2/src/example/bruToJson.js` and written by
  `example/jsonToBru.js`. Text bodies inside examples use `'''`.
- Model schema: `exampleSchema` in `packages/bruno-schema/src/collections/index.js`
  (`name`, `description`, `type`, `request { url, method, headers, params, body }`,
  `response { status, statusText, headers, body { type, content } }`).
- Example fixture: `packages/bruno-lang/v2/tests/examples/fixtures/bru/multiple-examples-variations.bru`.
- UI: examples appear as sidebar children (`CollectionItem/ExampleItem`), open in a
  dedicated `response-example` tab (`components/ResponseExample`), and can be executed
  ("Try") and used for code generation.

### 1.9 OpenCollection YAML format

Entry points: `packages/bruno-filestore/src/formats/yml/`
(`parseCollection.ts`, `parseItem.ts`, `parseFolder.ts`, `parseEnvironment.ts`,
`stringify*.ts`). Constants: `DEFAULT_COLLECTION_FORMAT = 'yml'`.

File names: `opencollection.yml` (collection root), `folder.yml`, `<name>.yml` for
requests and for `environments/<name>.yml`.

Collection root:

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
  actions: # post-response variable setters
    - type: set-variable
      phase: after-response
      selector:
        expression: res.body.token
        method: jsonq
      variable:
        name: token
        scope: runtime
docs:
  content: Shared defaults
  type: text/markdown
config:
  proxy:
    inherit: true
    config:
      {
        protocol: http,
        hostname: '',
        port: '',
        auth: { username: '', password: '' },
        bypassProxy: '',
      }
  clientCertificates:
    - domain: localhost
      type: pem
      certificateFilePath: ./certs/client.pem
      privateKeyFilePath: ./certs/client.key
  protobuf:
    protoFiles: [{ path: ./proto/service.proto }]
    importPaths: [{ path: ./proto }]
extensions:
  bruno:
    ignore: [dist]
    presets:
      request: { type: http, url: '{{baseUrl}}' }
      defaultEnvironment: Local
    scripts:
      flow: sequential
      additionalContextRoots: [../libs]
    openapi: []
```

Request item (real fixtures
`tests/collection/zip-git-metadata/fixtures/collection/users/get-user.yml`,
`tests/scripting/quickjs-safe-mode/fixtures/collections/quickjs-safe-mode/04-sync-vars.yml`,
`tests/auth/akamai-edgegrid/fixtures/collections/yml/EdgeGrid POST JSON 200.yml`):

```yaml
info:
  name: Get User
  type: http
  seq: 2
  tags: [users]

http:
  method: GET
  url: '{{baseUrl}}/users/1'
  headers:
    - name: accept
      value: application/json
  params:
    - name: page
      value: '1'
  auth:
    type: akamai-edgegrid
    accessToken: token
    clientToken: client
    clientSecret: secret
  body:
    type: json
    data: |-
      {"name":"bruno"}

runtime:
  variables:
    - name: count
      value: { type: number, data: '42' }
  scripts:
    - type: tests
      code: |-
        test("ok", function () {
          expect(res.getStatus()).to.equal(200);
        });
  assertions:
    - expression: res.status
      operator: eq
      value: '200'
  actions:
    - type: set-variable
      phase: after-response
      selector: { expression: res.body.token, method: jsonq }
      variable: { name: token, scope: runtime }

settings:
  encodeUrl: true
  timeout: 0
  followRedirects: true
  maxRedirects: 5
  forwardAuthorizationHeader: true
```

Environment (`tests/scripting/bru-api/.../environments/Test.yml`):

```yaml
name: Test
variables:
  - name: host
    value: https://testbench-sanity.usebruno.com
  - name: apiKey
    secret: true
```

Typed values are `{ type: number|boolean|object|string, data: "…" }`; `type: string` is the
implicit default and is not written. Secrets are `secret: true` with no value on disk.

### 1.10 Ordering

- Each request/folder carries `seq`. `meta.seq` is written for every item; missing seq
  defaults to 1 on parse (bru) but is deliberately left absent for yml folders when the
  source has none (`stringifyFolder.ts` comment).
- UI ordering: `sortByNameThenSequence` (`packages/bruno-common`, re-exported by
  `packages/bruno-app/src/utils/common/index.js`):
  - sort all items alphabetically by name;
  - items **without** a valid positive integer `seq` keep alphabetical order;
  - items **with** `seq` are inserted at index `seq - 1` (collisions group together in
    sequence order).
- Requests inside a folder are sorted by `seq` after folders are placed
  (`sortFolder` in `packages/bruno-electron/src/utils/collection.js`).
- Drag-and-drop and "resequence" rewrites seq values
  (`renderer:resequence-items` in `packages/bruno-electron/src/ipc/collection.js`).
- Collection order in a workspace is stored in `workspace.yml`, not in the collection.

### 1.11 Ignore rules

- Hard-coded: `node_modules`, `.git` (`app/collections.js`; also `DENY_DIRS` in
  `utils/mount.js` includes `.svn`, `.hg`, `.bruno`).
- Default denylist globs: `**/.DS_Store`, `**/Thumbs.db` (`utils/mount.js`).
- User `ignore` entries: plain names match as path prefixes, glob patterns use
  `path.matchesGlob` (`utils/mount.js`, `isDenied`).
- UI: per-folder "Ignore" action writes the folder into `bruno.json.ignore`
  (`renderer:ignore-folder` in `ipc/collection.js`, `CollectionItem/IgnoreCollectionItem`).
- Workspace `.gitignore` template (`DEFAULT_GITIGNORE` in `utils/filesystem.js`):
  `.env*`, `node_modules`, `.DS_Store`, `Thumbs.db`.

### 1.12 Parser / serializer entry points

| Purpose                  | Path                                                                                                                  | API                                                                                                        |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `.bru` request → JSON    | `packages/bruno-lang/v2/src/bruToJson.js`                                                                             | default export parser                                                                                      |
| JSON → `.bru` request    | `packages/bruno-lang/v2/src/jsonToBru.js`                                                                             | default export                                                                                             |
| collection/folder `.bru` | `bruno-lang/v2/src/collectionBruToJson.js`, `jsonToCollectionBru.js`                                                  |                                                                                                            |
| environment `.bru`       | `bruno-lang/v2/src/envToJson.js`, `jsonToEnv.js`                                                                      |                                                                                                            |
| `.env`                   | `bruno-lang/v2/src/dotenvToJson.js`                                                                                   |                                                                                                            |
| examples                 | `bruno-lang/v2/src/example/**`                                                                                        | own grammar                                                                                                |
| public `@usebruno/lang`  | `packages/bruno-lang/src/index.js`                                                                                    | `bruToJsonV2`, `jsonToBruV2`, …                                                                            |
| format dispatch          | `packages/bruno-filestore/src/index.ts`                                                                               | `parseRequest`, `parseCollection`, `parseFolder`, `parseEnvironment`, `parseDotenv`, `stringifyRequest`, … |
| bru adapter/normalizer   | `packages/bruno-filestore/src/formats/bru/index.ts`                                                                   | `parseBruRequest`, `stringifyBruRequest`, `parseBruCollection`, …                                          |
| yml adapter              | `packages/bruno-filestore/src/formats/yml/*.ts`                                                                       | `parseYmlItem`, `stringifyYmlItem`, …                                                                      |
| worker (off-main-thread) | `packages/bruno-filestore/src/workers/*`, `BruParserWorker`                                                           | `parseRequestViaWorker`, `stringifyRequestViaWorker`                                                       |
| redaction of huge bodies | `packages/bruno-filestore/src/formats/bru/utils/redact-large-text-blocks.ts`, `request-parse-and-redact-body-data.ts` | `parseRequestAndRedactBody`, `restoreRedactedBlocks`                                                       |
| validation               | `packages/bruno-schema/src/collections/index.js`                                                                      | `requestSchema`, `itemSchema`, `environmentSchema`, `collectionSchema`                                     |

File-size guard: the watcher refuses to parse request files above
`MAX_FILE_SIZE = 2.5 * 1024 * 1024` bytes
(`packages/bruno-electron/src/app/collection-watcher.js`); the app asks the user to open
large files explicitly (`renderer:load-large-request`).

### 1.13 `app` items

A request file may carry an HTML/JS app that replaces the request/response panes:

```bru
meta { name: dashboard  type: app  seq: 1 }

app {
  enabled: true
  code: '''
    <html>…</html>
  '''
}
```

- Parsed by `parseBruRequest` (`meta.type === 'app'` branch) and serialized back.
- YAML equivalent uses an `app` block (`formats/yml/items/parseApp.ts`, `stringifyApp.ts`).
- UI: `RequestPane/AppCodeEditor`, preview tab (`AppPreviewKeepAlive`), enabled via the
  request Settings tab. The `app` request-pane tab is hidden unless `app.enabled`.

---

## 2. UI surface

### 2.1 Shell and sidebar

`packages/bruno-app/src/pages/Bruno/index.js` composes: title bar, sidebar, request tabs,
request/response panes, status bar, optional AI chat sidebar, devtools console.

Sidebar sections (`components/Sidebar/Sections/*`, `SidebarContent.js`):

| Section      | Contents                                                                                 |
| ------------ | ---------------------------------------------------------------------------------------- |
| Collections  | workspace collections tree, folders, requests, examples, JS files, "invalid file" badges |
| API Specs    | OpenAPI spec files registered in the workspace (`Sidebar/ApiSpecs`)                      |
| Mock Servers | mock server instances (`Sidebar/Sections/MockServersSection`)                            |

Collection row actions (`components/Sidebar/Collections/Collection/CollectionRow`):
new request, new folder, new app, run collection, clone, rename, remove, move to workspace,
share/export, generate documentation, create mock server, open in terminal/devtools.

Sidebar extras: `CollectionSearch` / `SearchInput` (collection-scoped filter), bookmark-style
reveal, multi-select + bulk actions (`BulkActionsDropdown`, `BulkActionsMenu`),
drag-and-drop move/reorder (`react-dnd`), collapse-all, keyboard navigation
(`useCollectionFolderTree`, `useSidebarSelectionClick`).

Global search: `components/GlobalSearchModal` — fuzzy search over collections, folders,
requests, docs, plus a documentation link; opened with `Cmd/Ctrl+K`. **No command palette**
was found (search for "command palette" returns nothing).

### 2.2 Tabs

Redux slice: `providers/ReduxStore/slices/tabs.js`. Tab types observed:
`request`, `http-request`, `graphql-request`, `grpc-request`, `ws-request`,
`response-example`, `folder-settings`, `collection-settings`, `environment-settings`,
`global-environment-settings`, `preferences`, `collection-runner`, `variables`,
`workspaceOverview`, `mock-server`, `mock-response`, `openapi-sync`, `openapi-spec`,
`api-spec`, `changelog`, plus transient request tabs.

Behavior:

- Duplicate tabs are focused instead of duplicated; some types are singleton per collection
  (`nonReplaceableTabTypes` in `addTab`).
- Preview tabs (single-click) are replaced by the next preview; double-click pins
  (`makeTabPermanent`).
- Recently closed tabs stack (max 50) and `reopenLastClosedTab`.
- Drag-reorder (`DraggableTab`), overflow chevrons, tab context menu, close/close-all,
  close-others, reveal in sidebar.
- Per-tab persisted UI state: `requestPaneTab`, `responsePaneTab`, `responseFormat`,
  `responseViewTab`, pane sizes/collapse, `tabState` (environment selection, docs, etc.).

Request tab bar: `components/RequestTabs` (collection header, tab list, new-request action).

### 2.3 Request builder tabs

HTTP (`components/RequestPane/HttpRequestPane/index.js`, `TAB_CONFIG`):
`Params`, `Body`, `Headers`, `Auth`, `Vars`, `Script`, `Assert`, `Tests`, `Docs`,
`App` (only when enabled), `Settings`.
Tab indicators show enabled counts (params/headers/vars/asserts) or a dot (body/auth/
script/tests/docs/app/tags) and an error dot for script/test errors.

GraphQL (`GraphQLRequestPane/index.js`): `Query`, `Headers`, `Auth`, `Vars`, `Script`,
`Assert`, `Tests`, `Docs`, `Settings`. Query pane adds a GraphQL schema explorer
(`@usebruno/graphql-docs` DocExplorer), introspection load/refresh, load schema from file,
and a visual Query Builder side panel (`components/RequestPane/QueryBuilder`).

gRPC (`GrpcRequestPane/index.js`): `Message`, `Metadata`, `Auth`, `Script`, `Docs`.
gRPC scripts expose four lifecycle hooks; proto file or server reflection supplies methods;
`grpcurl` command generation is available (`GrpcQueryUrl/GrpcurlModal`).

WebSocket (`WSRequestPane/index.js`): `Message`, `Headers`, `Auth`, `Settings`, `Docs`.
Settings: connection timeout and keep-alive interval.

Tab bodies are mounted from `TAB_PANELS`; only the active panel is rendered.

### 2.4 Response viewer

`components/ResponsePane/index.js` tabs: `Response`, `Headers` (count), `Timeline`, `Tests`.

- Response formats (`QueryResult/index.js`): `JSON`, `HTML`, `XML`, `JavaScript`,
  `Raw`, `Hex`, `Base64`.
- Preview modes (`previewMode`): `preview-web`, `preview-json`, `preview-xml`,
  `preview-text`, `preview-image`, `preview-pdf`, `preview-audio`, `preview-video`.
- JSON filter (JSONPath-ish) with expandable filter bar (`QueryResultFilter`).
- Status code, response time, size, live stopwatch for streams, "large response" warning.
- Actions: clear response, download body (`ResponseActions` → `ResponseClear`,
  `ResponseDownload`), copy body, bookmark? (`ResponseBookmark` exists), set-as-variable
  popover (`SetAsVariablePopover`).
- Headers tab: `ResponseHeaders`.
- Timeline tab: `Timeline` + `RunnerTimeline`, entry types for request/response, OAuth2
  debug, gRPC, redirects.
- Tests tab: `TestResults` aggregated from `preRequestTestResults`, `testResults`,
  `assertionResults`, `postResponseTestResults`; pass/fail labels.
- Errors: `ResponseAlertsSection`, `ResponseErrorsCard`, `NetworkError`,
  `UnresolvedVariablesInfo`, `SkippedRequest`.
- Streaming responses (SSE + WebSocket) render `WSMessagesList` instead of the static viewer
  and keep a live elapsed-time counter.

### 2.5 Environment selector and environment editor

- Selector: `components/Environments/Common/EnvironmentSelection` (dropdown with search,
  colors, global vs collection environments, shortcuts `useEnvironmentShortcuts`).
- Environment editor tab: `components/Environments/EnvironmentSettings` →
  `EnvironmentList` → `EnvironmentDetails` →
  `EnvironmentVariables` → `components/EnvironmentVariablesTable`.
- Tabs inside an environment: **Variables** and **Secrets** (`hooks/useEnvironmentTabs`);
  both tabs edit the same `variables[]`, partitioned by `secret: true`.
  Supported per-variable fields: name, value, enabled, secret, dataType
  (`string|number|boolean|object`), description, annotations.
- Environment list also exposes `.env` files: create/delete/rename, table view and raw view
  (`Environments/DotEnvFileEditor`, `DotEnvFileDetails`), with save/reset.
- Environment actions: save all, rename, copy, delete, export, import, color, inherits-from.
- Global environments (workspace-level) have their own settings tab
  (`GlobalEnvironmentSettings`).

### 2.6 Runner UI

`components/RunnerResults` (opened as a `collection-runner` tab):

- Header/toolbar: run, re-run, cancel, clear; compact/tiny overflow levels.
- `RunConfigurationPanel`: flat request list with checkboxes, drag-reorder, select all,
  per-request enable/disable, delay input, recursive folder toggle, tag include/exclude.
- `RunnerFilter`: All / Passed / Failed / Skipped.
- `RunnerTags`: include/exclude tags for the run.
- Per-request result rows show status, test/assert/pre/post test statuses, duration, size,
  and an inline response pane (`RunnerResults/ResponsePane`).
- WS and gRPC requests are explicitly disabled in the runner
  (`RunConfigurationPanel`, `isRequestDisabled`), matching the runner backend’s skip behavior.
- Runner configuration is persisted per collection (`runnerConfiguration`, `runnerTags`,
  `runnerTagsEnabled` in `providers/ReduxStore/slices/collections/index.js`).

### 2.7 Import / export UI

- `components/Sidebar/ImportCollection`: tabs **File**, **Git Repository**, **URL**
  (drag-drop + browse, multi-file selection, ZIP).
- `components/Sidebar/Collections/Collection/ExportCollection/ExportToPostman`.
- `components/ShareCollection`: **ZIP**, **YAML (OpenCollection)**, **Postman**; warns when
  the collection contains gRPC/WebSocket requests that Postman cannot represent.
- Environment export modal: `Environments/Common/ExportEnvironmentModal`
  (JSON envelope with environments, `format:'json'`, `exportFormat`).
- Postman import can surface a package report and offer to install npm packages referenced by
  imported scripts (`ImportCollection/PostmanPackageReport`,
  `renderer:install-postman-packages`).

### 2.8 Git panel

**Absent in this checkout.** Verified:

- `packages/bruno-app/src/components/Git/` contains only `GitNotFoundModal` and
  `VisualDiffViewer` (the latter is imported by OpenAPI Sync, not by a Git panel).
- No `git` tab type in `providers/ReduxStore/slices/tabs.js`.
- IPC registrations are only `renderer:clone-git-repository`,
  `renderer:get-collection-git-remote-url`, `renderer:list-remote-branches-for-url`
  (`packages/bruno-electron/src/ipc/git.js`).
- Git UI that does exist: clone repository (`Sidebar/CloneGitRespository`, used by the
  workspace home and import flow), connect/disconnect collection remote
  (`WorkspaceHome/WorkspaceOverview/CollectionsList/ConnectGitRemote|RemoveGitRemote`).
  See §5 for the dormant backend surface.

### 2.9 Preferences

`components/Preferences/index.js` tabs:

| Tab                 | Contents (component)                                                                                                                                                       |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| General             | SSL verification, custom CA, keep default CAs, store/send cookies, system-browser OAuth2, auto-save toggle+delay, request timeout, default collection location (`General`) |
| Themes              | theme picker (`Themes`)                                                                                                                                                    |
| Display             | code font/font size, zoom (`Display`)                                                                                                                                      |
| Proxy               | proxy source manual/PAC/inherit, protocol/host/port/auth/bypass (`ProxySettings`)                                                                                          |
| Client Certificates | global client certs (PEM/pfx, passphrase, per-domain) (`ClientCertSettings`)                                                                                               |
| Keybindings         | editable per-action bindings with display formatting (`Keybindings`)                                                                                                       |
| AI                  | providers, models, OpenAI-compatible endpoints, autocomplete, redaction security (`AI`)                                                                                    |
| Cache               | SSL session cache, file cache (`Cache`)                                                                                                                                    |
| Support             | version info, links (`Support`)                                                                                                                                            |
| Beta                | feature toggles incl. `openapi-sync`, `mock-server` (`Beta`)                                                                                                               |

### 2.10 Themes

`packages/bruno-app/src/themes/index.js` registers 22 themes:

`light`, `light-monochrome`, `light-pastel`, `catppuccin-latte`,
`gruvbox-light-soft|medium|hard`, `dark`, `dark-monochrome`, `dark-pastel`,
`catppuccin-frappe`, `catppuccin-macchiato`, `catppuccin-mocha`,
`gruvbox-dark-soft|medium|hard`, `nord`, `dracula`, `vscode-light`, `vscode-dark`,
`cyberdream-light`, `cyberdream-dark`.

Theme metadata (`themeRegistry`) carries id/name/mode; theme files export CSS variables and
editor token colors. There is also a design-system palette viewer
(`themes/PaletteViewer`, `themes/DesignSystem`).

### 2.11 Keyboard shortcuts

Defaults from `providers/Hotkeys/keyMappings.js` (Mousetrap; Mac shown, Windows uses `ctrl`):

| Action                             | Default                               |
| ---------------------------------- | ------------------------------------- |
| Send request                       | `Cmd+Enter`                           |
| Close tab / close all tabs         | `Cmd+W` / `Cmd+Shift+W`               |
| Save / save all                    | `Cmd+S` / `Cmd+Shift+S`               |
| Reopen last closed tab             | `Cmd+Shift+T`                         |
| Switch tab 1–8 / last              | `Cmd+1…8` / `Cmd+9`                   |
| Prev / next tab                    | `Shift+Cmd+[` / `Shift+Cmd+]`         |
| Move tab left / right              | `Cmd+[` / `Cmd+]`                     |
| Search sidebar                     | `Cmd+F`                               |
| Copy / paste / clone / rename item | `Cmd+C` / `Cmd+V` / `Cmd+D` / `Cmd+R` |
| Collapse sidebar                   | `Cmd+\`                               |
| Change orientation                 | `Cmd+J`                               |
| Trigger autocomplete               | `Ctrl+Space`                          |
| Import collection                  | `Cmd+O`                               |
| Edit environment                   | `Cmd+E`                               |
| New request                        | `Cmd+N`                               |
| Global search                      | `Cmd+K`                               |
| Zoom in / out / reset              | `Cmd+=` / `Cmd+-` / `Cmd+0`           |
| Open terminal                      | `Cmd+T`                               |
| Preferences                        | `Cmd+,`                               |
| Close Bruno                        | `Cmd+Q`                               |

User overrides stored in `preferences.keyBindings`; `keybindingsEnabled` can disable all.
Storage encoding uses `+bind+` between keys (to allow a literal `+` key).

### 2.12 Panes, drag-drop, confirmations, safety

- Resizable/collapsible request and response panes, horizontal or vertical layout
  (`useTabPaneBoundaries`, `paneSize.js`, `preferences.layout.responsePaneOrientation`);
  keyboard/mouse collapse thresholds; AI sidebar docking auto-collapses the response pane.
- Drag-drop: sidebar move/reorder, tab reorder, runner request reorder, multipart/file drop.
- Confirmations/safety: unsaved-changes save prompt on close
  (`providers/App/ConfirmAppClose`, `SaveRequestsModal`), delete confirmations for
  collection/folder/request/environment/cookie, bulk-delete confirmation, "Hold on.."
  destructive modals, invalid-file badges + skipped-paths warning
  (`SkippedPathsWarning`), sensitive-field warnings (`SensitiveFieldWarning`),
  duplicate secret-name validation, JS sandbox mode (Safe/Developer) per collection
  (`components/SecuritySettings/JsSandboxMode`), AI redaction settings.
- Mock server UI: `components/MockServer/MockServerDashboard` (responses/routes tabs,
  start/stop, port, global delay, request log, per-route responses), create/clone/rename/
  delete modals; gated by beta flag `mock-server` (default on in preferences).
- Cookies modal: `components/Cookies` (grouped by domain, add/edit/delete/clear, search).
- Devtools console: `components/Devtools` with Console/Network/Debug tabs and xterm terminal.

---

## 3. Collection and request features

### 3.1 Folders and inheritance

- Folder = directory + `folder.bru`/`folder.yml`; nesting arbitrary.
- Folder root fields: `meta { name, seq, tags }`, `request { headers, auth, vars, script,
tests }`, `docs`.
- Resolved per request in `getTreePathFromCollectionToItem` and merged by
  `mergeHeaders`, `mergeVars`, `mergeScripts`, `mergeAuth`
  (`packages/bruno-electron/src/utils/collection.js`).
- Auth mode `inherit` means "use nearest ancestor auth"; `hasEffectiveAuth` in the UI
  mirrors this. Headers merge case-insensitively for `content-type`.
- Script execution flow is configurable per collection: `sandwich` (default; request/folder/
  collection pre scripts run outer→inner, post scripts inner→outer) or `sequential`
  (`brunoConfig.scripts.flow`; `mergeScripts` in `utils/collection.js`).
- Tags are inherited and used by the runner include/exclude filter
  (`getEffectiveTags`, `isRequestTagsIncluded` in `@usebruno/common`).
- Folders can be ignored and can be run recursively by the runner.

### 3.2 Variables and precedence

Scopes and where they live:

| Scope                  | Source                                               | Setter API                                                |
| ---------------------- | ---------------------------------------------------- | --------------------------------------------------------- |
| Global environment     | workspace `environments/*.yml`                       | `bru.setGlobalEnvVar`                                     |
| Collection vars        | `collection.bru`/`collection.yml` `vars:pre-request` | `bru.setCollectionVar`                                    |
| Environment vars       | active environment file                              | `bru.setEnvVar`                                           |
| Folder vars            | `folder.bru/yml` `vars:pre-request`                  | read-only (`bru.getFolderVar`)                            |
| Request vars           | `vars:pre-request` of the request                    | read-only (`bru.getRequestVar`)                           |
| OAuth2 credential vars | in-memory `$oauth2.<credentialId>.<key>`             | `bru.getOauth2CredentialVar`, `bru.resetOauth2Credential` |
| Runtime vars           | in-memory per run/tab                                | `bru.setVar` / `getVar` / `deleteVar`                     |
| Prompt vars            | `{{?Prompt Text}}` collected before send             | UI modal `PromptVariablesModal`                           |
| `process.env`          | `.env`, workspace `.env`, OS env                     | `bru.getProcessEnv` (read-only)                           |

Merge order for `{{var}}` (later wins), from `interpolateVars`
(`packages/bruno-electron/src/ipc/network/interpolate-vars.js`) and
`getAllVariables` (`packages/bruno-app/src/utils/collections/index.js`):

```
globalEnv < collectionVars < envVars < folderVars < requestVars
         < oauth2CredentialVars < runtimeVars < promptVars
process.env is available separately under {{process.env.NAME}}
```

Other interpolation behavior:

- Dynamic values `{{$randomInt}}`, `{{$randomUUID}}`, `{{$timestamp}}`, `{{$isoTimestamp}}`,
  plus the full faker-backed set in `packages/bruno-common/src/utils/faker-functions.ts`
  (`{{$randomFirstName}}`, `{{$randomIP}}`, `{{$randomBankAccount}}`, …).
- `{{process.env.X}}` and env vars that themselves contain `{{process.env.X}}` are resolved.
- Unresolved variables are collected and surfaced in the response pane
  (`UnresolvedVariablesInfo`, `trackUnresolvedVariables`).
- Variable data types (`string|number|boolean|object`) are coerced at execution time
  (`parseValueByDataType`), including from the unsaved draft (`resolveTypedValue`).
- Variable names must match `^[\w-.]*$` (`assertValidVariableName` in `bru.js`).
- Secret variables are masked in the UI and resolved from the encrypted store; they are
  never written into `.bru`/`.yml`.

### 3.3 Auth modes

| Mode            | `.bru` block                      | Fields                                                                                                                                                                    | Implementation                                                          |
| --------------- | --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| none            | `auth: none` in method block      | —                                                                                                                                                                         | —                                                                       |
| inherit         | `auth { mode: inherit }` (folder) | —                                                                                                                                                                         | `mergeAuth`                                                             |
| basic           | `auth:basic`                      | username, password                                                                                                                                                        | axios `auth`                                                            |
| bearer          | `auth:bearer`                     | token                                                                                                                                                                     | `Authorization: Bearer`                                                 |
| apikey          | `auth:apikey`                     | key, value, placement (`header                                                                                                                                            | queryparams`)                                                           | header or query append |
| digest          | `auth:digest`                     | username, password                                                                                                                                                        | `addDigestInterceptor`                                                  |
| ntlm            | `auth:ntlm`                       | username, password, domain                                                                                                                                                | `axios-ntlm` adapter                                                    |
| oauth1          | `auth:oauth1`                     | consumerKey/Secret, accessToken/Secret, callbackUrl, verifier, signatureMethod (`HMAC-SHA1/256/512`, `RSA-SHA1/256/512`, `PLAINTEXT`), privateKey + privateKeyType (`file | text`), timestamp, nonce, version, realm, placement (`header            | query                  | body`), includeBodyHash | `@usebruno/requests/auth/oauth1-request-authorization.ts` |
| oauth2          | `auth:oauth2`                     | see below                                                                                                                                                                 | `utils/oauth2.js` + `bruno-requests/auth/oauth2-helper.ts`              |
| wsse            | `auth:wsse`                       | username, password                                                                                                                                                        | SHA-1 digest header `X-WSSE` constructed inline in `prepare-request.js` |
| awsv4           | `auth:awsv4`                      | accessKeyId, secretAccessKey, sessionToken, service, region, profileName                                                                                                  | `awsv4auth-helper.js` (SigV4, profile lookup)                           |
| akamai-edgegrid | `auth:akamai-edgegrid`            | accessToken, clientToken, clientSecret, nonce, timestamp, baseURL, headersToSign, maxBodySize                                                                             | `bruno-requests/auth/edgegrid-helper.ts`                                |

OAuth2 grant types and fields (`authOAuth2` semantics + `oauth2Schema`):

- `authorization_code`: callbackUrl, authorizationUrl, accessTokenUrl, refreshTokenUrl,
  clientId, clientSecret, scope, state, pkce, credentialsPlacement (`body|basic_auth_header`),
  credentialsId, tokenSource (`access_token|id_token`), tokenPlacement (`header|url`),
  tokenHeaderPrefix, tokenQueryKey, autoFetchToken, autoRefreshToken,
  additionalParameters {authorization[], token[], refresh[]}.
- `implicit`: callbackUrl, authorizationUrl, clientId, scope, state, credentialsId,
  tokenSource, tokenPlacement, tokenHeaderPrefix/tokenQueryKey, autoFetchToken.
- `client_credentials`: accessTokenUrl, refreshTokenUrl, clientId, clientSecret, scope,
  credentialsPlacement/credentialsId, token placement, autoFetchToken, autoRefreshToken.
- `password`: accessTokenUrl, refreshTokenUrl, username, password, clientId, clientSecret,
  scope, credentials placement, token placement, auto flags.
- OAuth2 additional parameters are serialized into separate `.bru` blocks
  (`auth:oauth2:additional_params:auth_req:headers|queryparams`,
  `access_token_req:headers|queryparams|body`,
  `refresh_token_req:headers|queryparams|body`).
- Token cache/session is stored per collection in the Electron `oauth2` store
  (`store/oauth2.js`), and can be reset from scripts.
- Authorization can open in a Bruno window or the system browser
  (`preferences.request.oauth2.useSystemBrowser`;
  `ipc/network/authorize-user-in-window.js`, `authorize-user-in-system-browser.js`).
- **No device-code flow** was found.

### 3.4 Body types

`requestBodySchema` (`packages/bruno-schema/src/collections/index.js`):

| `mode`           | Meaning                          | `.bru` block                         |
| ---------------- | -------------------------------- | ------------------------------------ |
| `none`           | no body                          | —                                    |
| `json`           | raw JSON text                    | `body:json`                          |
| `text`           | plain text                       | `body:text`                          |
| `xml`            | XML text                         | `body:xml`                           |
| `sparql`         | SPARQL text                      | `body:sparql`                        |
| `graphql`        | `{query, variables}`             | `body:graphql` + `body:graphql:vars` |
| `formUrlEncoded` | key/value list                   | `body:form-urlencoded`               |
| `multipartForm`  | text/file fields                 | `body:multipart-form`                |
| `file`           | raw file body, one selected file | `body:file`                          |

Notes:

- Multipart file fields serialize as `name: @file(path1|path2) @contentType(type)`; multiple
  paths are joined by `|`. Files are streamed above 20 MB
  (`STREAMING_FILE_SIZE_THRESHOLD` in `prepare-request.js`).
- `body:file` has no user-visible name; each entry is `file: @file(path)` with `~` meaning
  unselected. `selected` drives which file is sent.
- GraphQL is also represented as `http { body: graphql }`, but the body block is the source
  of truth for query/variables.
- Binary response previews are content-type driven (image/pdf/audio/video/zip).
- Duplicate query parameters are preserved (array of params), but interpolation may collapse
  duplicates if the API consumer assumes a map — the model itself allows duplicates.
- Path params replace `:name` segments at send time (`getPath` in `bruno-request.js` and
  URL interpolation in `prepare-request.js`).

### 3.5 Assertions

Syntax in `.bru`: `assert { $res.status: 200 }` (disabled `~`).
The value is modeled as a **string containing the operator and RHS**, e.g. `eq 200`;
unary operators have no RHS.

Operators (from `AssertRuntime` in `packages/bruno-js/src/runtime/assert-runtime.js`
and mirrored in the UI `Assertions/AssertionRow`):

`eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `notIn`, `contains`, `notContains`,
`length`, `matches`, `notMatches`, `startsWith`, `endsWith`, `between`,
`isEmpty`, `isNotEmpty`, `isNull`, `isUndefined`, `isDefined`, `isTruthy`, `isFalsy`,
`isJson`, `isNumber`, `isString`, `isBoolean`, `isArray`.

LHS is a JS expression evaluated with `res`/`req`/`bru` in scope; results are reported
separately from tests (`assertionResults`).

### 3.6 Tests and scripts

- `tests { ... }` is JavaScript run post-response with Chai:
  `test('name', fn)`, `expect`, `assert`, `res`, `req`, `bru`.
- Pre-request scripts run before the request; post-response scripts after the response and
  before post-response vars/assertions/tests (order is visible in
  `ipc/network/index.js`).
- JS runtime default is **QuickJS** sandbox (`runtime: 'quickjs'`); collections can opt into
  `nodevm` "Developer Mode" (`utils/collection.js`, `save-collection-security-config`).
- Script APIs (`packages/bruno-js/src/bru.js`):
  - vars: `setVar/getVar/deleteVar/deleteAllVars/getAllVars/hasVar`,
    `getEnvVar/setEnvVar/deleteEnvVar/getAllEnvVars/hasEnvVar`,
    `getGlobalEnvVar/setGlobalEnvVar/deleteGlobalEnvVar`,
    `getCollectionVar/setCollectionVar/deleteCollectionVar`,
    `getFolderVar`, `getRequestVar`, `getProcessEnv`, `getPromptVar` (**UNCERTAIN** name;
    prompt values are merged into interpolation but a dedicated getter was not confirmed),
  - `getEnvName`, `getCollectionName`, `cwd`, `interpolate`, `sleep`, `isSafeMode`,
    `resetOauth2Credential`, `getOauth2CredentialVar`,
  - `bru.runner.skipRequest/stopExecution/setNextRequest`,
  - `bru.runRequest(path)` (nested request execution; WS/gRPC skipped),
  - `bru.sendRequest(...)` (records a scripted request entry),
  - `bru.cookies` (CookieList: get/set/delete for the current URL),
  - `bru.utils.minifyJson/minifyXml`,
  - console log/debug/info/warn/error captured into the timeline.
- `req` object: `getUrl/setUrl/getHost/getPath/getQueryString/getMethod/getAuthMode`,
  `getHeaders/getHeader/setHeader/deleteHeader`, `headerList` (PropertyList),
  `getBody/setBody`, `getTimeout`, `getPathParams`, `getTags`, `getName`, etc.
  `res` object is callable as a JSON-query accessor (`res('data.id')`), plus
  `getStatus/getStatusText/getHeader/getHeaders/getBody/getResponseTime/getUrl/getSize`,
  `setBody`, and a read-only `headerList`.
- Postman script translation (`packages/bruno-converters/src/postman/postman-translations.js`)
  maps `pm.*` APIs to `bru.*`/`req.*`/`expect`.

### 3.7 Docs

- Request/folder/collection `docs` blocks are Markdown.
- Docs tab (`components/Documentation`) has a rich-text editor (`ui/RichTextEditor`) and a
  Markdown source mode, with view/edit toggle, persisted scroll, save shortcut.
- Docs can be generated into a standalone interactive HTML site
  (`components/Sidebar/Collections/Collection/GenerateDocumentation`,
  `generateApiDocsHtml` in `@usebruno/common`), with environment and tag filters and an
  optional Git link.

### 3.8 Protocol coverage

| Protocol                                                    | Supported                                              | Where                                                                                                     |
| ----------------------------------------------------------- | ------------------------------------------------------ | --------------------------------------------------------------------------------------------------------- |
| HTTP/1.1, HTTP/2                                            | yes                                                    | axios-based `ipc/network`                                                                                 |
| HTTPS with custom CA / client certs / disabled verification | yes                                                    | `cert-utils.js`, prefs                                                                                    |
| GraphQL over HTTP                                           | yes                                                    | `GraphQLRequestPane`, introspection, schema docs, query builder                                           |
| WebSocket (`ws://`, `wss://`)                               | yes                                                    | `bruno-requests/src/ws/ws-client.js`, `WSRequestPane`, stream message list; client certificates supported |
| SSE (`text/event-stream`)                                   | yes (as a streaming HTTP response)                     | stream detection in `ipc/network/index.js`; UI `WSMessagesList` + elapsed timer; tests `tests/sse`        |
| gRPC (unary, client/server/bidi streaming)                  | yes                                                    | `bruno-requests/src/grpc`, `GrpcRequestPane`, reflection, proto files, four script hooks, certificates    |
| SOAP/WSDL                                                   | import-only (WSDL → HTTP requests with SOAP envelope)  | `bruno-converters/src/wsdl`                                                                               |
| MCP                                                         | **not present** (no MCP code in app/electron/requests) | —                                                                                                         |
| HTTP/3, QUIC                                                | not present                                            | —                                                                                                         |

### 3.9 OAuth2 / AWS / other auth caveats

- AWS SigV4 supports static credentials and named profiles; session token supported.
  IMDS/credential-chain behavior was not verified.
- Digest auth is implemented via an axios interceptor that replays the challenge.
- NTLM is implemented via `axios-ntlm`; only tested/used on Windows servers in practice.
- OAuth1 RSA signing reads the private key from a file path or inline text.
- EdgeGrid `maxBodySize` limits the body hash; nonce/timestamp can be overridden.
- WSSE uses SHA-1 password digest and a random nonce (not WS-Security policy negotiation).

---

## 4. Import / export converters

### 4.1 Converter package API

`packages/bruno-converters/src/index.js`:

```js
(postmanToBruno,
  postmanToBrunoEnvironment,
  brunoToPostman,
  openApiToBruno,
  insomniaToBruno,
  wsdlToBruno,
  postmanTranslation,
  openCollectionToBruno,
  brunoToOpenCollection);
```

### 4.2 Import formats

| Source                         | Detection                                                                      | Converter path                                                    | UI entry                     | Caveats                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ------------------------------ | ------------------------------------------------------------------------------ | ----------------------------------------------------------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Postman collection v2.x        | `isPostmanCollection` in `bruno-app/src/utils/importers/postman-collection.js` | `bruno-converters/src/postman/postman-to-bruno.js`                | File / JSON                  | Scripts translated `pm.*`→`bru.*`; auth types mapped (basic/bearer/awsv4/apikey/digest/oauth1/oauth2/edgegrid/ntlm); unknown auth logs a warning and sets mode without config; GraphQL body parsed from Postman’s string; OAuth2 grant mapping may fall back with a warning; npm packages referenced by scripts detected and can be installed (`postman-package-detector.js`, `renderer:install-postman-packages`); folder/collection variables become vars; examples converted |
| Postman environment            | `postman-env-to-bruno-env.js`                                                  | `src/postman/postman-env-to-bruno-env.js`                         | Environment import modal     | Values only; secrets not inferred                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| OpenAPI 3.x (JSON/YAML)        | `isOpenApiSpec`                                                                | `src/openapi/openapi-to-bruno.js` + `openapi-common.js`           | File / URL                   | Explicit `TODO: handle allOf/anyOf/oneOf` at `openapi-to-bruno.js:529`; `$ref` resolution errors reject the import; grouping by tags or path; examples synthesized from schema (`getExampleFromSchema`); parameter serialization (form/explode/pipe/space delimited) implemented; servers become environments (export path)                                                                                                                                                     |
| Swagger 2.0                    | same detector                                                                  | `src/openapi/swagger2-to-bruno.js`                                | File / URL                   | Separate converter; same ref-resolution failure mode                                                                                                                                                                                                                                                                                                                                                                                                                            |
| Insomnia v4 / v5               | `isInsomniaCollection`                                                         | `src/insomnia/insomnia-to-bruno.js`                               | File                         | v5 export must contain a collection; throws on invalid item arrays; environment conversion via `env-utils.js`                                                                                                                                                                                                                                                                                                                                                                   |
| WSDL                           | `isWSDLCollection`                                                             | `src/wsdl/wsdl-to-bruno.js` (+ `parse-xml.js`, `schema-graph.js`) | File `.wsdl` / XML           | Generates SOAP envelopes (`soap` prefix, SOAP 1.1 namespace); warning array for unsupported constructs; requires valid XML and `definitions`                                                                                                                                                                                                                                                                                                                                    |
| Bruno collection (ZIP)         | `renderer:is-bruno-collection-zip`                                             | `ipc/collection.js` ZIP import, `utils/collection-import.js`      | File `.zip`                  | ZIP must contain `bruno.json`/`opencollection.yml`; extracts to chosen location                                                                                                                                                                                                                                                                                                                                                                                                 |
| Bruno collection (JSON export) | `isBrunoCollection`                                                            | `bruno-app/src/utils/importers/bruno-collection.js`               | File `.json`                 | Inverse of the JSON export model; `bruno.json`-less payload needs a location                                                                                                                                                                                                                                                                                                                                                                                                    |
| OpenCollection YAML            | `isOpenCollection`                                                             | `bruno-converters/src/opencollection/opencollection-to-bruno.ts`  | File `.yml`                  | Supports info/config/runtime/scripts/assertions/actions; unknown extensions preserved only partially                                                                                                                                                                                                                                                                                                                                                                            |
| Git repository URL             | URL validation via `git-url-parse`                                             | `ipc/git.js` clone + `utils/collection-import.js`                 | Sidebar “Git Repository” tab | Requires `git` binary; can specify branch; clone then open                                                                                                                                                                                                                                                                                                                                                                                                                      |
| OpenAPI spec URL               | URL fetch (`ipc/swagger-fetch.js`)                                             | same OpenAPI converter                                            | Import “URL” tab             | Network required                                                                                                                                                                                                                                                                                                                                                                                                                                                                |

**Curl**: there is **no collection import** for curl/HAR. Curl is supported only by
paste-into-URL-bar (`components/RequestPane/QueryUrl/index.js` uses
`utils/curl/curl-to-json.js` + `parse-curl.js`, detecting `^\s*curl\s`). There is no HAR
importer/exporter in this source.

### 4.3 Export formats

| Target                   | Path                                                                                                       | Entry                                    | Caveats                                                                                                                                                                  |
| ------------------------ | ---------------------------------------------------------------------------------------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Postman collection v2.1  | `bruno-app/src/utils/exporters/postman-collection.js` → `bruno-converters/src/postman/bruno-to-postman.js` | Collection menu → Export → Postman       | “Preserve scripts” option skips translation back to `pm.*`; gRPC/WS not representable (Share modal warns); transient items filtered                                      |
| OpenCollection YAML      | `bruno-app/src/utils/exporters/opencollection.js` → `bruno-to-opencollection.ts`                           | Share → YAML; collection export          | Adds `extensions.bruno.exportedAt/exportedUsing`; single `.yml` file (not a folder tree)                                                                                 |
| Bruno collection ZIP     | `renderer:export-collection-zip` (`ipc/collection.js`)                                                     | Share → ZIP                              | Uses current on-disk collection; strips transient items; may include `.git`? (zip-git-metadata tests suggest Git metadata handling)                                      |
| Environment JSON         | `bruno-app/src/utils/exporters/bruno-environment.js` → `renderer:export-environment`                       | Environment export modal                 | Bruno-specific JSON envelope; not Postman env format                                                                                                                     |
| Standalone API docs HTML | `@usebruno/common` `generateApiDocsHtml`                                                                   | Collection menu → Generate Documentation | Optional environments, tags, Git link                                                                                                                                    |
| OpenAPI spec             | `bruno-app/src/utils/exporters/openapi-spec.js`                                                            | API Specs → Create API Spec              | Converts collection requests back to an OpenAPI document; baseUrl from collection/env vars becomes `servers`; non-HTTP/GraphQL items filtered; warns on lossy constructs |

### 4.4 Format conversion (bru ↔ yml) and migration

- `renderer:migrate-collection-to-yml` (`ipc/yml-migration.js`) rewrites an entire collection
  to OpenCollection, including requests, folders, environments, docs, and `bru.runRequest`
  script paths (drops the `.bru` suffix so the runtime appends the current format extension).
- `renderer:cancel-migrate-collection-to-yml` cooperatively cancels between file writes.
- Migration UI: `components/MigrateCollectionToYmlModal`; collections show a migration pill.
- Caveat: the migration is destructive after confirmation (original files removed); there is
  no automatic yml→bru down-migration path in the UI.

---

## 5. Git integration

### 5.1 What is actually wired up

Only three IPC handlers are registered
(`packages/bruno-electron/src/ipc/git.js`):

| Channel                                  | Behavior                                                                                 |
| ---------------------------------------- | ---------------------------------------------------------------------------------------- |
| `renderer:clone-git-repository`          | Creates the target dir, `simple-git` clone (optional branch), removes the dir on failure |
| `renderer:get-collection-git-remote-url` | Reads `origin` URL (`listRemote --get-url origin`)                                       |
| `renderer:list-remote-branches-for-url`  | `ls-remote --heads` with a 10 s timeout; rejects URLs starting with `-`                  |

Plus preferences call `getGitVersion` (`ipc/preferences.js`).
Workspace-level:
`renderer:connect-collection-to-git` / `renderer:disconnect-collection-from-git`
(`ipc/workspace.js`) write/remove a managed block in the workspace `.gitignore`
(`# Bruno managed collection remotes`, `utils/workspace-config.js`).

### 5.2 Dormant full Git backend

`packages/bruno-electron/src/utils/git.js` is 1,862 lines and exports a complete
source-control feature set, but **no IPC handler or React component in this checkout calls
most of it**. Exported functions include:

`getCollectionGitRootPath`, `getCollectionGitRepoUrl`, `stageChanges`, `unstageChanges`,
`discardChanges`, `commitChanges`, `getChangedFilesInCollectionGit`,
`getCollectionGitBranches`, `getDefaultGitBranch`, `getCurrentGitBranch`,
`checkoutGitBranch`, `getCollectionGitLogs`, `getCollectionGitTagsWithDetails`, `canPush`,
`pushGitChanges`, `pullGitChanges`, `initGit`, `getCollectionGitData`,
`getStagedFileDiff`, `getUnstagedFileDiff`, `getRenamedFileDiff`, `cloneGitRepository`,
`listBranchesForRemoteUrl`, `parseRemoteBranches`, `fetchChanges`, `fetchRemotes`,
`fetchRemoteBranches`, `checkoutRemoteGitBranch`, `getGitVersion`, `addRemote`,
`removeRemote`, `getAheadBehindCount`, `getAheadCount`, `getBehindCount`,
`abortConflictResolution`, `continueMerge`, `getCommitFiles`, `getCommitFileDiff`,
`getCommitCompareFiles`, `getCommitCompareFileDiff`, `getFileGitHistory`, `getGitGraph`,
`createStash`, `listStashes`, `applyStash`, `dropStash`, `getStashFiles`,
`getStashFileDiff`, `getFileContentAtCommit`, `getFileContentForVisualDiff`,
`getWorkingFileContentForVisualDiff`, `getStashFileContentForVisualDiff`.

Implementation notes if parity is desired:

- `simple-git`, one cached instance per repo root; repo root is found by walking up until
  `.git` exists (`findGitRootPath`).
- `initGit` runs `git init` then `git branch -M main`.
- Diff APIs use `git diff --no-prefix`, `git show`, `git stash show`, `git log` etc., and
  return unified diff text consumed by a visual diff renderer.
- Progress is streamed via `main:update-git-operation-progress` (`handleGitOutput`).
- Credentials are delegated to the system `git` (SSH agent/credential helper); no token
  storage was found. `bruno-app/src/utils/git/index.js` filters GitHub tokens out of
  displayed remote URLs (`containsGitHubToken`, `getSafeGitRemoteUrls`).
- `.git` and `node_modules` are ignored by watchers; the workspace `.gitignore` template
  excludes `.env*`.

**Conclusion for planning:** treat “Git panel (status/diff/commit/branch/log/stash/merge)”
as **not shipped in the OSS desktop app at this commit**, even though the backend exists.
Only clone/remote-connect are product-visible. Build Git parity as a separate, optional
milestone and do not assume Bruno’s UI semantics for it.

---

## 6. Persistence and settings

### 6.1 Electron `electron-store` files

Names from `new Store({ name })` in `packages/bruno-electron/src/store/*`:

| Store name            | Contents                                                                                                 | Key file examples          |
| --------------------- | -------------------------------------------------------------------------------------------------------- | -------------------------- |
| `preferences`         | `preferences` object, window state, last opened collections/workspaces, generate-code prefs, keybindings | `preferences.json`         |
| `cookies`             | `{ encryptedPasskey, cookies: { domain: [cookie] } }`, values encrypted with a random passkey            | `cookies.json`             |
| `secrets`             | `{ collections: [{ path, environments: [{ name, secrets: [{name, value}] }] }] }`, values encrypted      | `secrets.json`             |
| `oauth2`              | per-collection OAuth2 session/credentials cache                                                          | `oauth2.json`              |
| `global-environments` | global environment list/values                                                                           | `global-environments.json` |
| `collection-security` | `{ collections: [{ path, securityConfig: { jsSandboxMode } }] }`                                         | `collection-security.json` |
| `ai-keys`             | AI provider API keys                                                                                     | `ai-keys.json`             |
| `env-secrets` (class) | same as `secrets` (class `EnvironmentSecretsStore`)                                                      | —                          |

Preferences defaults (`store/preferences.js`):

```
request.sslVerification=true
request.customCaCertificate {enabled:false, filePath:null}
request.keepDefaultCaCertificates {enabled:true}
request.storeCookies=true, request.sendCookies=true
request.timeout=0
request.oauth2.useSystemBrowser=false
request.clientCertificates.certs=[]
font.codeFont='default', font.codeFontSize=13
proxy.source='inherit', proxy.pac.source='', proxy.config{protocol,hostname,port,auth,bypassProxy}
layout.responsePaneOrientation='horizontal'
mockServer.mode='isolated', mockServer.instances[]
beta {'openapi-sync':false, 'mock-server':true}
onboarding {hasLaunchedBefore,hasSeenWelcomeModal,lastSeenVersion}
general {defaultLocation, defaultWorkspacePath}
autoSave {enabled:false, interval:1000}
display.zoomPercentage=100
cache {sslSession.enabled:false, file.enabled:false}
ai {enabled:false, providers, models, defaultModel, openaiCompatibleEndpoints, autocomplete, security redaction flags}
```

Preferences are validated by Yup (`preferencesSchema`) and migrated in code
(old proxy formats, font size 14→13, `defaultCollectionLocation`→`defaultLocation`,
mock-server beta default).

### 6.2 Workspace model

- `workspace.yml` at the workspace root, OpenCollection-style:
  `opencollection: 1.0.0`, `info {name, type: workspace}`, `collections: [{name, path,
remote?}]`, `specs: [{name, path}]`, `docs`.
- Workspace directory contains `collections/` and a `.gitignore`.
- Workspaces have their own environments (`environments/*.yml`), docs, API specs, and
  scratch requests (`renderer:mount-workspace-scratch`).
- Workspace-level state: `lastOpenedWorkspaces`, `activeWorkspaceUid`, workspace watcher.

### 6.3 Cookies

- Stored globally (not per collection) in the `cookies` store, encrypted values with a
  passkey protected by Electron `safeStorage` (`utils/encryption.js`).
- Debounced persist (5 s) from the in-memory tough-cookie jar
  (`store/cookies.js`, `utils/cookies.js` → `@usebruno/requests` cookies module).
- Preferences toggle automatic store/send.
- UI can add/edit/delete/clear per domain (`components/Cookies`).

### 6.4 History

**No request-history feature was found** in this checkout. Searches for
`RequestHistory`/`requestHistory` and for a History UI return nothing outside the AI chat
sidebar and code-editor state. Timeline is per-request/per-collection ephemeral, not a
persisted history. Do not plan for it as an existing Bruno OSS feature without confirming
against another release.

### 6.5 Themes and i18n

- Themes: 22 registered in `bruno-app/src/themes/index.js` (see §2.10); stored in
  preferences as theme id; light/dark mode metadata.
- i18n: `packages/bruno-app/src/i18n/index.js` initializes i18next with **English only**
  (`translation/en.json`). No other locale files exist. Language switching is not exposed
  as a feature; adding locales is a clean-room decision, not parity.

### 6.6 Logs and diagnostics

- `slices/logs.js` + `components/Devtools` provide an in-app console (console/network/debug)
  and terminal (xterm). Logs are in-memory, not a persisted audit log.
- Support tab exposes version/git version and external links.

---

## 7. Observed limitations / TODOs a parity plan must know

1. **Git panel is not shipped** in this OSS checkout despite a 1.8k-line backend (§5).
2. **No request history** and **no command palette** in the OSS app.
3. **English-only i18n**; no locale switching.
4. **No curl/HAR collection import.** Curl works only by pasting into the URL bar.
   HAR is absent entirely.
5. **No MCP support** anywhere in app/electron/requests.
6. **External secrets** (`vars:externalsecrets:<provider>`) are parsed/preserved but have no
   editor UI; values are not resolved by the app in this checkout (only `vault`/`infisical`
   appear in tests). **UNCERTAIN** if resolution exists in a paid build.
7. **JS items (`type: 'js'`) are vestigial**: `itemSchema` and import/export handle them,
   but the file indexer only classifies `.bru`/`.yml`/`.yaml`, so they do not appear as
   normal sidebar items (`utils/mount.js`, `defaultClassify`).
8. **Runner excludes WebSocket and gRPC** requests (`RunConfigurationPanel`,
   `isRequestDisabled`); the backend records skipped rows for `bru.runRequest` on those
   protocols.
9. **gRPC folder/collection script inheritance is unimplemented** — `stringifyFolder.ts`
   has `TODO: Widen scope to include GRPC scripts once Collection/Folder level inheritance
is added to GRPC.`
10. **OpenAPI import cannot handle `allOf`/`anyOf`/`oneOf` fully**
    (`openapi-to-bruno.js:529`), and fails the whole import on unresolved `$ref`.
11. **Postman OAuth2/auth mapping is lossy**; unknown auth types log warnings and keep the
    mode without configuration (`postman-to-bruno.js` around line 449–457).
12. **Duplicate keys collapse for auth/meta dictionaries** (`mapPairListToKeyValPair` uses
    `_.merge`), but header/param/form lists preserve duplicates.
13. **No comments in `.bru`**; the Ohm grammar has no comment rule. Unknown blocks fail the
    whole file parse. (Round-trip robustness for unknown _pairs_ is not guaranteed either;
    the model is validated strictly by `bruno-schema` with `noUnknown(true)`.)
14. **Strict schema**: `itemSchema`, `requestSchema`, `environmentSchema`,
    `collectionSchema` all use `noUnknown(true).strict()`. Unknown JSON fields from
    third-party imports are dropped or cause validation errors, depending on the path.
15. **Large files**: watcher refuses >2.5 MB requests; a redaction path exists for huge text
    blocks in the filestore (`redact-large-text-blocks.ts`), but the UI must explicitly load
    large requests.
16. **`.env` vs `.env.*`**: only `.env` populates `process.env`; other dotenv files are
    editable but inert. This may surprise users.
17. **Proxy** is global (preferences) or collection-level (`bruno.json.proxy`); no
    per-request proxy. PAC source is a URL string; system proxy is inherited.
18. **Client certificates** can be global or collection-level; per-request certs were not
    found. Cert types: PEM (`cert` + `key`) and PFX (`pfx`, passphrase), per-domain, with a
    `disabled` flag.
19. **OAuth2 has no device-code flow**; four grants only.
20. **Cookie storage is global**, not per collection/workspace; clearing a domain affects all
    collections.
21. **Collection format default is yml** in `@usebruno/filestore`, but existing `.bru`
    collections keep their format. Any parity work must support both indefinitely: the app
    auto-detects, and the migration UI converts bru→yml only.
22. **File naming**: request filenames are sanitized (`sanitizeName`) and can differ from
    the display name; renaming an item can rename the file (`rename-item-name` vs
    `rename-item-filename` are separate operations). A clean-room implementation should keep
    the display name in `meta.name` and treat filenames as an implementation detail.
23. **No content hashing / stable IDs on disk**: item UIDs are derived at runtime
    (`generateUidBasedOnHash` for collections, hashed path for requests, `uuid()` for
    unsaved items). Tabs/examples reference paths or generated UIDs; do not assume a
    persistent per-item ID in files (only examples/`uid` in the in-memory model).
24. **Examples** are embedded in the request file, not separate files; there is no
    `.example` file format.
25. **Safety**: default scripts are sandboxed (QuickJS, no fs/network), but collections can
    switch to Developer Mode (Node VM with fs/exec) per collection. This is a security
    decision a Go implementation must consciously match or diverge from — Go cannot run
    arbitrary Node; plan a JS engine (goja/QuickJS) and document API gaps.

---

## 8. Quick parity checklist (derived, not exhaustive)

- `.bru` parser/printer with: disabled `~`, quoted keys, `'''`, annotations/typed vars,
  every block in §1.4, collection/folder/env grammars, examples grammar.
- `.yml`/OpenCollection parser/printer for collection, folder, request, environment, mocks.
- Directory model + watcher with ignore semantics and 2.5 MB guard.
- Inheritance (headers/auth/vars/scripts/tests/tags) and variable precedence §3.2.
- Auth matrix §3.3 (incl. 4 OAuth2 grants + PKCE + token cache + additional params).
- Body matrix §3.4 incl. multipart/file streaming and GraphQL.
- Script sandbox + `bru`/`req`/`res` APIs + Chai `test`.
- Assertions operators, runner with tags/delay/recursion/filters, skip WS/gRPC.
- Import: Postman, OpenAPI 3, Swagger 2, Insomnia, WSDL, Bruno ZIP, OpenCollection.
- Export: Postman, OpenCollection YAML, ZIP, env JSON, HTML docs, OpenAPI spec.
- UI: sidebar tree, tabs (request/example/folder/collection/env/settings), request tabs per
  protocol, response viewer formats/preview/timeline/tests, environment selector with
  Variables/Secrets/dotenv, runner panel, preferences, themes, keybindings, global search.
- Deliberately defer/decide: Git panel, request history, command palette, MCP, i18n,
  external-secret providers, AI features.

---

## 9. Source index (most load-bearing files)

| Area                    | Files                                                                                                                                                                                      |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `.bru` grammar/print    | `packages/bruno-lang/v2/src/bruToJson.js`, `jsonToBru.js`, `collectionBruToJson.js`, `jsonToCollectionBru.js`, `envToJson.js`, `jsonToEnv.js`, `dotenvToJson.js`, `utils.js`, `example/**` |
| Format dispatch         | `packages/bruno-filestore/src/index.ts`, `formats/bru/index.ts`, `formats/yml/**`                                                                                                          |
| Model/schema            | `packages/bruno-schema/src/collections/index.js`                                                                                                                                           |
| Collection open/watch   | `packages/bruno-electron/src/app/collections.js`, `app/collection-watcher.js`, `app/dotenv-watcher.js`, `utils/mount.js`, `utils/collection-reader.js`, `utils/filesystem.js`              |
| Request execution       | `packages/bruno-electron/src/ipc/network/index.js`, `prepare-request.js`, `interpolate-vars.js`, `cert-utils.js`, `awsv4auth-helper.js`                                                    |
| Scripts                 | `packages/bruno-js/src/{bru,bruno-request,bruno-response,test}.js`, `src/runtime/*`, `src/sandbox/*`, `src/grpc/*`                                                                         |
| Network primitives      | `packages/bruno-requests/src/{auth,cookies,grpc,ws,network,scripting,utils}/*`                                                                                                             |
| Converters              | `packages/bruno-converters/src/{index.js,postman,openapi,insomnia,wsdl,opencollection,common,constants}`                                                                                   |
| Import/export IPC       | `packages/bruno-electron/src/ipc/collection.js`, `utils/collection-import.js`                                                                                                              |
| UI shell/tabs           | `packages/bruno-app/src/pages/Bruno/index.js`, `components/Sidebar/**`, `components/RequestTabs/**`, `components/RequestTabPanel/**`, `providers/ReduxStore/slices/tabs.js`                |
| Request pane            | `components/RequestPane/**`                                                                                                                                                                |
| Response pane           | `components/ResponsePane/**`                                                                                                                                                               |
| Environments            | `components/Environments/**`, `components/EnvironmentVariablesTable/**`                                                                                                                    |
| Runner                  | `components/RunnerResults/**`                                                                                                                                                              |
| Preferences/themes/i18n | `components/Preferences/**`, `themes/index.js`, `i18n/index.js`                                                                                                                            |
| Keybindings             | `providers/Hotkeys/keyMappings.js`, `providers/Hotkeys/index.js`                                                                                                                           |
| Persistence             | `packages/bruno-electron/src/store/*.js`, `utils/encryption.js`                                                                                                                            |
| Git (dormant)           | `packages/bruno-electron/src/utils/git.js`, `ipc/git.js`, `ipc/workspace.js`                                                                                                               |
| Mock server             | `packages/bruno-electron/src/app/mock-server/**`, `ipc/mock-server/index.js`, `packages/bruno-app/src/components/MockServer/**`                                                            |
| Workspace               | `packages/bruno-electron/src/utils/workspace-config.js`, `ipc/workspace.js`, `store/default-workspace.js`                                                                                  |

---

_Prepared as read-only research. No Bruno source was modified. Items marked UNCERTAIN or
“not found” should be verified against a packaged build before being treated as product
requirements._
