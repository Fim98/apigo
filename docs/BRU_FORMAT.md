# The `.bru` format

Status: specification for compatibility with Bruno 2.0.0. This is a clean-room
description derived from Bruno's grammar, serializer and fixtures. The Go
implementation lives in `internal/bru`.

## 1. Compatibility contract

apigo must be able to:

1. Open a Bruno collection directory and read every request, folder,
   environment and collection setting that Bruno 2.0.0 reads.
2. Write files that Bruno opens without conversion.
3. Preserve unknown blocks, unknown keys and unknown annotations across a
   load/save cycle.
4. Keep output deterministic so Git diffs are meaningful.

The parser is **permissive on input, canonical on output**. It never rejects a
file just because it contains a block this version does not understand; the
block is kept raw and printed back.

Byte-for-byte identity with Bruno's serializer is not guaranteed. The invariants
are:

- **Semantic round-trip:** `parse(print(model)) == model`.
- **Stability:** `print(parse(print(model))) == print(model)`.
- **Unknown-data preservation:** a file containing an unsupported block prints
  that block unchanged (byte range, position relative to known blocks).

## 2. Collection layout

```text
my-collection/
├── bruno.json                 # collection configuration (JSON, required by Bruno)
├── collection.bru             # collection-level headers/auth/vars/scripts/tests/docs
├── .env                       # optional process env for scripts/interpolation
├── environments/
│   ├── Local.bru
│   ├── Staging.bru
│   └── Production.bru
├── folder.bru                 # optional, one per folder
├── users/
│   ├── folder.bru
│   ├── list.bru
│   └── get-user.bru
└── README.md                  # ignored by the loader
```

The loader walks recursively, skipping `.git`, `node_modules`, `environments`
(handled separately) and hidden directories. Folders with a `folder.bru` carry
their own metadata; a folder without one is still a folder.

`bruno.json` is **not** a `.bru` file. It holds collection identity and runtime
configuration:

```json
{
  "version": "1",
  "name": "my-collection",
  "type": "collection",
  "proxy": {
    "inherit": true,
    "config": {
      "protocol": "http",
      "hostname": "proxy.local",
      "port": 8080,
      "auth": { "username": "u", "password": "p", "disabled": true },
      "bypassProxy": "localhost,127.0.0.1"
    }
  },
  "scripts": {
    "moduleWhitelist": ["crypto", "buffer"],
    "additionalContextRoots": ["../shared-lib"]
  },
  "clientCertificates": { "enabled": true, "certs": [] },
  "presets": { "requestType": "http", "requestUrl": "http://localhost:3000" }
}
```

apigo reads it, preserves unknown fields (round-trip through a JSON document
model that keeps key order where practical) and writes it back only when the
user changes collection settings. The `bruno.json` schema is owned by
`internal/collection/config.go`; see the research inventory for the fields
Bruno 2.0.0 uses.

## 3. Lexical rules

- Encoding: UTF-8. A UTF-8 BOM is accepted and not written back.
- Line endings: LF or CRLF accepted; the printer writes LF.
- Indentation: two spaces per level in `.bru` request/folder/collection files.
  Environment multiline values are conventionally four spaces.
- A file is a sequence of **blocks**. Block order is semantically irrelevant on
  input; the printer uses a canonical order.
- Whitespace outside strings is not significant except for indentation inside
  text blocks (which is stripped relative to the block).
- Comments are not a first-class grammar construct. `//` inside `script`,
  `tests` and `docs` is content and is preserved; do not strip it in the
  parser. (Execution may strip JS comments as Bruno does, but the file keeps
  them.)

### Block header

```text
<name> {
  <body>
}
```

or, for list blocks:

```text
<name> [
  <item>
]
```

Names may contain letters, digits, `_`, `-`, `:` and `.`, e.g.
`auth:oauth2`, `vars:pre-request`, `body:multipart-form`.

## 4. Blocks

### 4.1 `meta` (request and folder)

```bru
meta {
  name: Get users
  type: http
  seq: 2
  tags: [
    smoke
    users
  ]
}
```

- `name`: display name.
- `type`: `http`, `graphql`, `grpc`, `ws`, or `collection` (collection.bru).
- `seq`: ordering hint (integer; missing defaults to 1).
- `tags`: list block (request and folder tags; inherited during runs).
- Unknown keys are preserved.

### 4.2 HTTP method block

Exactly one of `get`, `post`, `put`, `patch`, `delete`, `head`, `options`,
`trace`, `connect`, or `http` with a `method:` field for custom methods.

```bru
get {
  url: {{baseUrl}}/users/:id
  body: none
  auth: inherit
}
```

- `url`: required; may contain `{{variables}}` and `:path` params.
- `body`: body mode reference (`none`, `json`, `text`, `xml`, `sparql`, `file`,
  `formUrlEncoded`, `multipartForm`, `graphql`) or omitted.
- `auth`: auth mode reference (`inherit`, `none`, `basic`, ...) or omitted.
- `method` is only used by the custom `http` block.

### 4.3 `grpc` and `ws`

```bru
grpc {
  url: grpcs://localhost:50051
  method: /users.UserService/GetUser
  body: body:grpc
  protoPath: ./proto/user.proto
  auth: inherit
  methodType: unary
}
```

```bru
ws {
  url: wss://localhost:8080/socket
  body: body:ws
  auth: none
  methodType: websocket
}
```

`methodType` values: `unary`, `client-streaming`, `server-streaming`,
`bidirectional` for gRPC; `websocket` for WS.

### 4.4 `params:query` and `params:path`

```bru
params:query {
  limit: 20
  ~offset: 40        # disabled
  @description('Page size')
  page: 1
}

params:path {
  id: 42
}
```

- Query params are key/value pairs; duplicates are allowed and preserved in
  order.
- A key prefixed with `~` is disabled. Disabled values are preserved.
- `@description(...)` and other annotations attach to the following pair.
- Path params substitute `:name` segments (and OData-style `Func(:name)`).

### 4.5 `headers` and `metadata`

```bru
headers {
  Content-Type: application/json
  ~X-Debug: 1
}

metadata {
  authorization: Bearer {{token}}
}
```

`metadata` is the gRPC metadata equivalent. Header names and values may contain
variables; do not validate them as HTTP tokens at parse time.

### 4.6 `auth` and `auth:<mode>`

`auth` selects the mode; the mode-specific block holds the fields:

```bru
auth {
  mode: oauth2
}

auth:oauth2 {
  grant_type: authorization_code
  access_token_url: {{baseUrl}}/oauth/token
  refresh_token_url: {{baseUrl}}/oauth/token
  client_id: apigo
  client_secret: {{clientSecret}}
  scope: openid profile
  credentials_placement: basic_auth_header
  token_placement: header
  token_header_prefix: Bearer
  auto_fetch_token: true
  auto_refresh_token: true
}
```

Supported mode blocks (fields are listed in
[REQUEST_ENGINE.md](REQUEST_ENGINE.md#4-authentication)):

| Block                  | Mode            |
| ---------------------- | --------------- |
| `auth:basic`           | basic           |
| `auth:bearer`          | bearer          |
| `auth:digest`          | digest          |
| `auth:ntlm`            | ntlm            |
| `auth:oauth1`          | oauth1          |
| `auth:oauth2`          | oauth2          |
| `auth:apikey`          | apikey          |
| `auth:awsv4`           | awsv4           |
| `auth:wsse`            | wsse            |
| `auth:akamai-edgegrid` | akamai-edgegrid |

OAuth2 additional-parameter blocks use the exact Bruno names:
`auth:oauth2:additional_params:{auth_req|access_token_req|refresh_token_req}:{headers|queryparams|body}`.

`auth.mode: inherit` means "use the nearest enclosing auth" (folder, then
collection); `none` means no auth and overrides inherited auth.

### 4.7 `body:<mode>`

| Block                  | Content                            | Notes                                                    |
| ---------------------- | ---------------------------------- | -------------------------------------------------------- |
| `body`                 | raw text                           | Generic body; rarely written                             |
| `body:json`            | JSON text                          | Comments are preserved in the file, stripped before send |
| `body:text`            | text                               | `text/plain`                                             |
| `body:xml`             | XML                                | `application/xml`                                        |
| `body:sparql`          | SPARQL                             | `application/sparql-query`                               |
| `body:graphql`         | query document                     | Sent as JSON with `body:graphql:vars`                    |
| `body:graphql:vars`    | JSON object text                   | Parsed after interpolation                               |
| `body:form-urlencoded` | key/value pairs                    | `~` disables a field                                     |
| `body:multipart-form`  | key/value pairs                    | `@file(...)`, `@contentType(...)`                        |
| `body:file`            | one or more file entries           | `@file(path)`; binary content                            |
| `body:grpc`            | `name`/`content` pairs             | Content is a `'''` block                                 |
| `body:ws`              | `name`/`type`/`content`/`selected` | Content is a `'''` block                                 |

Multipart and file syntax:

```bru
body:multipart-form {
  field: value
  upload: @file(/tmp/a.txt) @contentType(text/plain)
  photo: @file(/tmp/a.png|/tmp/b.png)
}

body:file {
  file: @file(/tmp/payload.bin) @contentType(application/octet-stream)
}
```

`@file(...)` values are paths relative to the collection root (absolute paths
are allowed if Bruno allows them; validate on send). `@contentType(...)` is a
trailing annotation. Multiple files are separated by `|`.

### 4.8 `vars:pre-request` and `vars:post-response`

```bru
vars:pre-request {
  baseUrl: https://api.example.com
  @number
  retries: 3
  @boolean
  debug: false
  @object
  payload: '''
    {"id": 1}
  '''
}

vars:post-response {
  userId: res.body.id
}
```

- Values are strings unless an annotation types them (`@number`, `@boolean`,
  `@object`).
- A leading `@` on a variable key marks it **local** (not persisted to the
  environment when set from scripts); a leading `~` disables it. The
  combination `~@` is possible.
- Secret variables are not written with values. In request files, secret names
  appear only in the environment's `vars:secret` list.

### 4.9 `assert`

```bru
assert {
  res.status: eq 200
  $res.body.id: isNumber
  res.body.name: eq "Ada"
  ~res.body.debug: isDefined
}
```

- Left side is a response expression; both `res.*` and the legacy `$res.*`
  spelling are accepted.
- Right side is `<operator> <operand>`; a missing operator defaults to `eq`.
- `~` disables an assertion.
- Operators are listed in [REQUEST_ENGINE.md](REQUEST_ENGINE.md#7-assertions-and-tests).

### 4.10 Scripts and tests

```bru
script:pre-request {
  bru.setVar('token', 'abc');
}

script:post-response {
  console.log(res.getStatus());
}

tests {
  test('returns a user', function() {
    expect(res.getStatus()).to.equal(200);
  });
}
```

Text blocks run from the line after `{` to the line before `}`. Indentation is
preserved relative to the block; the runner strips the common indentation only
for display, never from the stored file.

gRPC script hooks:

- `script:grpc:before-call-start`
- `script:grpc:before-message-send`
- `script:grpc:after-message-receive`
- `script:grpc:after-call-end`

### 4.11 `settings`

```bru
settings {
  encodeUrl: true
  followRedirects: true
  maxRedirects: 5
  timeout: 30000
  forwardAuthorizationHeader: true
  omitHeaders: [
    User-Agent
    Accept-Encoding
  ]
}
```

- `timeout`: milliseconds, or `inherit` (desktop resolves to the app
  preference; CLI treats it as no timeout).
- Missing values mean Bruno defaults, not Go zero values.
- `omitHeaders` removes headers that would otherwise be added by the transport.

### 4.12 `app`, `docs`, `example`

```bru
app {
  enabled: true
  code: '''
    console.log('collection-level app script');
  '''
}

docs {
  # Users API

  Markdown documentation for this request.
}
```

`docs` is Markdown content, not parsed further. `example` blocks contain a
request/response snapshot; see §5.

### 4.13 `vars` and collection-level blocks

`collection.bru` supports `meta`, `headers`, `auth`, `auth:<mode>`,
`vars:pre-request`, `vars:post-response`, `script:pre-request`,
`script:post-response`, `tests`, `docs` and the OAuth2 additional-parameter
blocks. `folder.bru` supports `meta` plus the same request-default blocks.

## 5. `example` blocks

Examples are multiple blocks:

```bru
example {
  name: Success
  description: A user is returned

  request: {
    url: https://api.example.com/users
    method: GET
    mode: json

    headers: {
      Accept: application/json
    }

    body:json: {
      {}
    }
  }

  response: {
    headers: {
      Content-Type: application/json
    }

    status: {
      code: 200
      text: OK
    }

    body: {
      type: application/json
      content: '''
        {"id": 1}
      '''
    }
  }
}
```

The example grammar is simpler than the request grammar: root properties
(`name`, `description`) plus `request: { ... }` and `response: { ... }` blocks
with colon-suffixed names. Request content is parsed by the request grammar;
response content is a status, headers and a typed body. Older single-object
example forms may exist in the wild; preserve what the parser does not
understand.

- Each example is preserved as an ordered block.
- Unknown fields inside examples are preserved.
- The request/response bodies use the same text-block rules.
- `examples` are edited and reordered in the UI; their serialization order is
  file order.

## 6. Environment files

Environment files live in `environments/<name>.bru` and use a separate grammar:

```bru
vars {
  baseUrl: https://staging.example.com
  token: '''
    multi
    line
  '''
  @number
  retries: 3
}

vars:secret [
  apiKey
  clientSecret
]

extends: Base
color: #4caf50
```

- `vars:secret` is a **list of names**, not values. Values live in the OS
  keychain (desktop) or are injected at runtime (CLI).
- `extends:` or `extends [...]` inherits variables from other environment
  files; resolve root-ancestor first, detect cycles, and let secrets win over
  non-secrets on collision.
- `vars:externalsecrets:<type>` (e.g. 1Password, Vault) may appear in newer
  Bruno; preserve it and mark it unsupported until a provider is implemented.
- `color:` is a UI hint.
- `@description` annotations attach to variables.
- Environment names may contain spaces; filenames use the display name.

`dotenv` files (`.env`) are a separate, simpler format:

```dotenv
BASE_URL=https://api.example.com
TOKEN=dev-only
```

Precedence: OS environment < collection `.env` (for `process.env.*` and
`{{process.env.X}}`). Do not merge `.env` into the normal variable scopes.

## 7. Canonical serialization

The printer writes blocks in this order, omitting empty ones:

1. `meta`
2. method block (`get`/`post`/...) or `http`, then `grpc`, `ws`
3. `params:query`, `params:path`
4. `headers`, `metadata`
5. `auth`, `auth:<mode>`, OAuth2 additional-parameter blocks
6. body block (`body:*`) and gRPC/WS body blocks
7. `vars:pre-request`, `vars:post-response`
8. `assert`
9. script blocks in order: pre-request, post-response, gRPC hooks
10. `tests`
11. `app`
12. `settings`
13. `docs`
14. unknown blocks (in their original relative order)
15. `example` blocks

Formatting rules:

- Two-space indentation, one blank line between blocks, no trailing spaces.
- A file ends with exactly one newline (except empty files).
- Disabled pairs are written after enabled pairs, each group in model order.
- Arrays (`tags`, `vars:secret`, `omitHeaders`) use one item per line.
- Multiline values use `'''` delimiters with four-space indentation inside, as
  Bruno's environment writer does; request bodies use two-space indentation.
- Quotes: keys containing `:`, `{`, `}`, leading/trailing spaces or a leading
  `~` are quoted with `"..."` and inner quotes escaped. Values stay unquoted
  unless they need a text block.
- Never write `content-type: false`; that is an execution sentinel, not file
  content.

## 8. Parser behavior and errors

- Errors carry `Path`, `Line`, `Column`, `Offset`, and a short message.
- A malformed block must not panic; the parser returns a partial AST plus an
  error. The UI shows the file and offers "open as text".
- Unknown block names are collected as `RawBlock{Name, Raw}` and printed after
  known blocks. Unknown keys inside known dictionary blocks are preserved in
  `Extra`.
- Recursion depth is bounded; a pathological file (10 MB of nested blocks)
  returns an error rather than exhausting the stack.
- Duplicate keys are preserved as ordered pairs, not collapsed.
- BOM is stripped; invalid UTF-8 is replaced with U+FFFD and reported as a
  warning (never silently corrupt on write: keep the original bytes if the
  user did not edit).
- Files above 2.5 MiB are not parsed automatically (Bruno watcher cap); the
  loader returns `ErrTooLarge` and the UI asks before loading.

## 9. Versioning and migration

- The `.bru` format has no version field. `meta.type` (including `app`) and
  block names are the compatibility surface.
- Bruno validates the model with strict Yup schemas (`noUnknown(true)`) and
  fails a file with unknown blocks. apigo preserves unknown data instead; this
  is a deliberate, documented divergence (ADR-0002) and never changes known
  fields.
- New Bruno fields are preserved as unknown until implemented.
- apigo may add **new** blocks only under an `apigo` namespace with an ADR, and
  only if Bruno ignores them safely. Prefer not adding any.
- A format change (renaming, removing, reordering semantics) requires an ADR,
  a migration in `internal/bru/migrate.go`, and fixtures for both old and new
  forms.

## 10. Fixtures

The conformance corpus lives in:

- `internal/bru/testdata/fixtures/` — small, hand-written cases per syntax rule.
- `internal/bru/testdata/bruno/` — curated files from Bruno 2.0.0 with the MIT
  notice.
- `internal/collection/testdata/` — full collection trees.

Each fixture has an optional `.golden` file for canonical output and a
`.json` model snapshot where useful. CI runs:

```sh
go test ./internal/bru ./internal/collection
go test -fuzz=FuzzParse -fuzztime=10m ./internal/bru
```
