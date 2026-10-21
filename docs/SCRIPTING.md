# Scripting

Status: specification for `internal/script`. It targets Bruno 2.0.0's **safe**
(QuickJS) sandbox surface. The developer Node VM is deliberately not
implemented; see §10.

## 1. Runtime choice

- Engine: [goja](https://github.com/dop251/goja), pure Go, one runtime per
  script phase.
- No filesystem, no network, no `process`, no native modules.
- Every phase has a timeout (default 5s), an output cap (default 1 MiB) and a
  cancellation context.
- The engine runs on a worker goroutine, never on the UI thread.

goja supports async/await and promises. The runner drains the promise job queue
after resolving host promises; a pending promise that never settles is stopped
by the timeout interrupt.

## 2. Phases and globals

| Global                       | Pre-request     | Post-response   | Tests                   |
| ---------------------------- | --------------- | --------------- | ----------------------- |
| `bru`                        | yes             | yes             | yes                     |
| `req`                        | yes             | yes             | yes                     |
| `res`                        | no              | yes             | yes                     |
| `test(desc, fn)`             | yes             | yes             | yes                     |
| `expect` / `assert`          | yes             | yes             | yes                     |
| `console`                    | yes             | yes             | yes                     |
| `require`                    | yes (whitelist) | yes (whitelist) | yes (whitelist)         |
| `jwt`                        | no              | no              | yes (jsonwebtoken shim) |
| `process`, `Buffer`, `fetch` | no              | no              | no                      |

`res` is callable: `res('a.b[0].c')` resolves a path. It also exposes
`getStatus`, `getStatusText`, `getHeader`, `getHeaders`, `getBody`, `setBody`,
`getResponseTime`, `getUrl`, `getSize`, `getDataBuffer`, `headerList`, and
read-only proxy properties `status`, `statusText`, `headers`, `body`,
`responseTime`, `url`.

`req` exposes `getUrl/setUrl`, `getHost`, `getPath`, `getQueryString`,
`getMethod/setMethod`, `getHeaders/setHeaders/deleteHeaders`,
`getHeader/setHeader/deleteHeader`, `getBody/setBody`, `setMaxRedirects`,
`getTimeout/setTimeout`, `onFail(cb)`, `disableParsingResponseJson()`,
`getExecutionMode`, `getName`, `getPathParams`, `getTags`, `getAuthMode`.

`res.jq(expr)` exists only in assertion/post-response variable expressions, not
inside `script`/`tests` (matching Bruno).

## 3. `bru` API

| Group              | Methods                                                                                                                              |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------ |
| Meta               | `cwd()`, `getEnvName()`, `getCollectionName()`, `isSafeMode()`, `interpolate(x)`                                                     |
| Environment        | `getEnvVar`, `setEnvVar`, `hasEnvVar`, `deleteEnvVar`, `getAllEnvVars`, `deleteAllEnvVars`                                           |
| Global environment | `getGlobalEnvVar`, `setGlobalEnvVar`, `hasGlobalEnvVar`, `deleteGlobalEnvVar`, `getAllGlobalEnvVars`, `deleteAllGlobalEnvVars`       |
| Runtime            | `getVar`, `setVar`, `hasVar`, `deleteVar`, `getAllVars`, `deleteAllVars`, `setNextRequest`                                           |
| Collection         | `getCollectionVar`, `setCollectionVar`, `hasCollectionVar`, `deleteCollectionVar`, `getAllCollectionVars`, `deleteAllCollectionVars` |
| Folder/request     | `getFolderVar`, `getRequestVar`                                                                                                      |
| Process env        | `getProcessEnv(key)`                                                                                                                 |
| OAuth2             | `getOauth2CredentialVar(key)`, `resetOauth2Credential(id)`                                                                           |
| Runner             | `runner.skipRequest()`, `runner.stopExecution()`, `runner.setNextRequest(name)`                                                      |
| HTTP               | `sendRequest(config[, cb])`, `runRequest(relativePath)`                                                                              |
| Cookies            | `cookies.get/set/has/delete/getAll`                                                                                                  |
| Utilities          | `utils.minifyJson`, `utils.minifyXml`, `sleep(ms)`                                                                                   |
| Results            | `getTestResults()`, `getAssertionResults()`                                                                                          |
| gRPC               | `grpc.*` inside gRPC hooks                                                                                                           |

Variable names are validated against `^[\w.-]*$`. Writes are tracked per scope so
untouched scopes are not persisted.

`bru.sendRequest(config)` accepts an axios-shaped config:
`{method, url, headers, data, timeout}` and returns a promise; the callback form
`(err, res)` is supported. It uses the same auth/TLS/proxy/cookie settings as the
current request and records a timeline entry.

## 4. Script flow and inheritance

- Pre-request: collection → folders (outer to inner) → request.
- Post-response and tests: request → folders (inner to outer) → collection,
  unless `brunoConfig.scripts.flow = "sequential"`, which uses
  collection → folders → request for all phases.
- Script segments are executed separately, each with its own `__dirname` and
  `__filename`; `return` exits only the segment.
- Scripts are comment-stripped with a JS-aware stripper before execution (the
  file keeps comments). `decomment` semantics are reproduced; regex/string
  literals containing `//` are not stripped.
- A pre-request script error aborts the request. A post-response/test script
  error is recorded as a failed synthetic row and the run continues.

## 5. Async and timers

- `test()` callbacks and the script body are awaited.
- `setTimeout(fn, ms)` waits `ms` then calls `fn`; it is implemented as a host
  sleep, not a timer. `setInterval` is unavailable (matching safe mode).
- Pending test promises are awaited after the script body; nested `test()` calls
  are tracked.
- A never-settling promise is interrupted at the timeout.

## 6. `require` and modules

Safe-mode module registry (name → shim):

| Module                | Surface                                                                                    |
| --------------------- | ------------------------------------------------------------------------------------------ |
| `chai`                | `expect`, `assert` (chai 4 + chai-string + custom matchers)                                |
| `buffer`              | `Buffer` subset (from/alloc/concat/toString/base64/hex)                                    |
| `moment`              | date parsing/formatting subset, documented per method                                      |
| `crypto-js`           | hashes (MD5/SHA1/SHA256/SHA512), HMAC, base64/hex encoders; ciphers only if implemented    |
| `uuid`                | `v1/v3/v4/v5/v6/v7`, `validate`, `parse`, `stringify`, `version`                           |
| `nanoid`              | `nanoid()`                                                                                 |
| `jsonwebtoken`        | `sign`, `verify`, `decode` (HMAC algorithms)                                               |
| `ajv` + `ajv-formats` | draft-07 validation, `allErrors` option                                                    |
| `tv4`                 | legacy JSON Schema validation                                                              |
| `axios`               | `bru.sendRequest`-backed minimal client (`get/post/request`)                               |
| `path`                | POSIX path subset (`join`, `dirname`, `basename`, `extname`, `resolve` within `bru.cwd()`) |
| local files           | `require('./x.js')` relative to cwd, CJS wrapper, module cache                             |

Not available: `lodash`, `xml2js`, `cheerio`, any Node builtin (`fs`, `os`,
`crypto`, `stream`, ...), any npm package, and network APIs. `lodash` and the
other Node-only modules are developer-mode features in Bruno and are explicitly
**out of scope**; scripts using them fail with:

```text
Error: require('lodash') is not available in apigo's safe sandbox.
Supported modules: chai, buffer, moment, crypto-js, uuid, nanoid, jsonwebtoken, ajv, tv4, axios, path.
```

`require('./file.js')` is restricted to the collection root (and configured
additional context roots, if we add them later); symlinks must not escape it.

## 7. Assertions and tests

Assertion DSL and `test()` semantics are defined in
[REQUEST_ENGINE.md](REQUEST_ENGINE.md#7-assertions-and-tests). Implementation
notes:

- `expect` is JS, not Go: a small chai-compatible implementation shipped as an
  embedded JS bundle, with host-backed deep equality and JSON Schema.
- Errors carry `actual`, `expected`, `message`, `stack`, `errorName` where
  applicable, so test reports look like Bruno's.
- Assertion results are `{lhsExpr, rhsExpr, operator, status, error}`; test
  results are `{description, status, error}`.

## 8. Error reporting

- Syntax errors: `ScriptSyntaxError` with line/column relative to the script
  block.
- Runtime errors: `ScriptRuntimeError` with a mapped line in the `.bru` file
  where possible (segment offset + block start line).
- Timeout: `ScriptTimeoutError`, includes phase and elapsed time.
- Output overflow: `ScriptOutputError`; the first 1 MiB is kept.
- Every error preserves partial results (variable mutations, tests registered,
  `nextRequest`, `skipRequest`, `stopExecution`).

## 9. Security model

Scripts are untrusted by default.

- No filesystem, network, process, environment mutation or native code.
- Bounded CPU time and output.
- No access to auth secrets except through the variable APIs that the request
  already exposes (matching Bruno).
- `require` is a whitelist; local modules cannot escape the collection root.
- The app warns when a collection contains scripts before running it for the
  first time, with a "don't ask again for this collection" option.

## 10. Divergences from Bruno

| Topic                         | Bruno safe mode            | apigo                                           |
| ----------------------------- | -------------------------- | ----------------------------------------------- |
| Engine                        | QuickJS WASM               | goja                                            |
| `lodash`, `xml2js`, `cheerio` | not available              | not available (same)                            |
| Developer mode (`fs`, npm)    | available via Node VM      | not available; explicit error                   |
| `setTimeout`                  | host sleep                 | host sleep                                      |
| `setInterval`                 | unavailable                | unavailable                                     |
| `res.jq`                      | only in assertions/vars    | same                                            |
| `process.env` in assertions   | bare globals, no `process` | bare globals + `process.env` (superset)         |
| Error stack text              | V8/QuickJS format          | goja format; line numbers mapped                |
| `crypto-js` ciphers           | full library               | subset documented in code/tests                 |
| `ajv`                         | draft-07 + formats         | draft-07 + formats; other dialects fail clearly |

Any new divergence needs a test, a row in
[CAPABILITY_MATRIX.md](CAPABILITY_MATRIX.md) and a note here.

## 11. Compatibility corpus

- `internal/script/testdata/bruno/` — scripts copied from Bruno's
  `packages/bruno-tests/collection/scripting/` and `tests/scripting/` with the
  MIT notice.
- Each case has a `.bru` request, an expected result JSON (status, variables,
  test results) and an optional server fixture.
- Run with `go test ./internal/script -run TestBrunoCompatibility`.
- A divergence list is generated from skipped cases; it must stay empty for P0
  modules and documented for the rest.

## 12. Implementation checklist

1. goja runtime wrapper with interrupt/timeout and job-queue draining.
2. Host bridge for `bru`, `req`, `res`, `test`, `expect`.
3. Embedded JS bundles for chai-compatible matchers and module shims.
4. Comment stripper (JS-aware) and segment offset tracking.
5. Error mapping to `.bru` line numbers.
6. Result aggregation with partial results on error.
7. Conformance fixtures and divergence report.
