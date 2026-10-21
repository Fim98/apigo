# Backlog

This is the claimable task board. One task per pull request. Status values:
`todo`, `in_progress`, `blocked`, `done`, `divergent`, `dropped`.

Rules:

- Pick the first `todo` task in the current milestone whose dependencies are
  `done`; change it to `in_progress` with your name/branch.
- A task is `done` only when its acceptance criteria are covered by tests and
  `make verify` passes. Link the commit or PR in the Evidence column.
- Do not renumber tasks; add `M<number>-<nn>a` for follow-ups.
- If a task grows beyond one focused PR, split it and record the new IDs in
  [HANDOFF.md](HANDOFF.md).
- Milestone exit criteria live in [ROADMAP.md](ROADMAP.md); this file does not
  repeat them.

## M0 — Foundation

| ID    | Task                                                                                                | Deps      | Acceptance / evidence                                                                                | Status  |
| ----- | --------------------------------------------------------------------------------------------------- | --------- | ---------------------------------------------------------------------------------------------------- | ------- |
| M0-01 | Write AGENTS, roadmap, architecture, format, engine, scripting, testing, release, security and ADRs | —         | Docs exist, links resolve, `markdownlint` passes                                                     | done    |
| M0-02 | Restructure into `cmd/apigo`, `cmd/apigo-cli`, `internal/version`; set `mygo.json` `main`           | —         | `make run`, `go run ./cmd/apigo-cli version`, `make test` pass                                       | done    |
| M0-03 | gofumpt, Prettier, markdownlint, Husky hooks, commitlint, Makefile, verify script                   | —         | `make verify` passes; a bad commit message is rejected                                               | done    |
| M0-04 | Initialize Git on `main`, add `.gitignore`/`.gitattributes`/`.editorconfig`, first commit           | M0-01..03 | `git log` shows the foundation commit; `git status` clean                                            | done    |
| M0-05 | CI workflow: docs, lint, tests on macOS/Ubuntu/Windows, packaging build                             | M0-02     | Workflow is valid YAML; all jobs pass on the first push                                              | partial |
| M0-06 | Release workflow: tag/version check, build matrix, draft release, checksums                         | M0-05     | A `v0.1.0` dry run uploads dmg/exe/deb/tar.gz to a draft                                             | partial |
| M0-07 | CLI skeleton: `version`, `help`, exit codes, tests                                                  | M0-02     | `go test ./cmd/apigo-cli` passes; `--help` exits 0, unknown exits 2                                  | done    |
| M0-08 | Add PR template, issue templates and Dependabot config                                              | M0-04     | Files present; Dependabot config parses                                                              | done    |
| M0-09 | Add third-party notices for Bruno fixtures and research                                             | M0-01     | Notice lists Bruno 2.0.0, MIT, commit `dd0281d`                                                      | done    |
| M0-10 | Choose project license and final app identifier                                                     | —         | Owner decision recorded in ADR; `package.json`/`mygo.json` updated                                   | blocked |
| M0-11 | Create GitHub repository, push `main`, set secrets, enable branch protection                        | M0-10     | `Fim98/apigo` exists; `main` pushed as `d99ef9c`. Left: branch protection, Actions secrets, CI green | partial |
| M0-12 | Validate a real `mygo build` locally and record artifact names                                      | M0-02     | `go tool mygo build` succeeds; output paths in HANDOFF                                               | done    |
| M0-13 | Vendor the Matt Pocock engineering skills and configure `docs/agents/` for this repo                | —         | 20 skills under `.agents/skills/`; `docs/agents/` matches reality; pushed as `d99ef9c`               | done    |

## M1 — Language & collection store

| ID     | Task                                                                                                            | Deps  | Acceptance / evidence                                                                                                                                                                               | Status |
| ------ | --------------------------------------------------------------------------------------------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| M1-01  | Tokenizer with byte spans, line/column tracking, error recovery for unknown blocks                              | —     | Token table tests in `internal/bru/lexer_test.go`; `go test ./internal/bru` passes (6a61281)                                                                                                        | done   |
| M1-02  | Parser for dictionary blocks, list blocks, text blocks, annotations, quoted keys, disabled pairs                | M1-01 | Fixtures for request/folder/collection/environment; errors carry line/col (6a61281)                                                                                                                 | done   |
| M1-03  | AST node types + source-faithful printer contract                                                               | M1-02 | `DictBlock`/`TextBlock`/`RawBlock` cover every block; unknown blocks kept verbatim (6a61281)                                                                                                        | done   |
| M1-04  | Typed request model: method, URL, params, headers, body modes, auth, scripts, tests, assertions, docs, examples | M1-03 | `internal/collection/parse.go` + `format.go`; model→AST→model is idempotent (99dd0b7)                                                                                                               | done   |
| M1-05  | Collection/folder/environment model: nested tree, ordering, inheritance links, secret names                     | M1-04 | 3-level sample collection; inherited headers/auth/vars/scripts tested (99dd0b7)                                                                                                                     | done   |
| M1-06  | Canonical printer: stable block order, 2-space indent, LF, trailing newline, unknown block passthrough          | M1-03 | Golden files per fixture; `Format` is idempotent (6a61281)                                                                                                                                          | done   |
| M1-07  | Collection loader: `.bru` discovery, ignore rules, `seq` sort, folder.bru, collection.bru, environments         | M1-05 | Loads `testdata/sample`; broken/oversized files become warnings, not panics (99dd0b7)                                                                                                               | done   |
| M1-08  | Environment files: `vars`, `vars:secret`, extends, secret masking                                               | M1-05 | Extends resolution and cycle detection tested; secret values never parsed from file (99dd0b7)                                                                                                       | done   |
| M1-08a | Dotenv (`.env`) reading and `process.env` precedence                                                            | M1-08 | `.env` parsed, `.env.*` inert, precedence tests                                                                                                                                                     | todo   |
| M1-09  | File watcher: debounce, targeted reload, external-change events                                                 | M1-07 | `watcher_test.go`: create/update/remove, rename-into-place, move-out, removed dir, oversized, burst coalescing, ignored dirs, new subdirs, close, cancel; `-race -count=3` clean (bd800ad, 0c196ce) | done   |
| M1-10  | Vendor a curated Bruno fixture corpus with MIT notice                                                           | M1-07 | `internal/bru/testdata/bruno/` with `NOTICE.md`; corpus test round-trips 9 files (6a61281)                                                                                                          | done   |
| M1-11  | Semantic round-trip + fuzz tests                                                                                | M1-06 | `FuzzParse` (60 s / 448k execs clean, seeds committed); round-trip test per fixture (6a61281)                                                                                                       | done   |
| M1-12  | `apigo-cli fmt` and `inspect`                                                                                   | M1-07 | `fmt --check` exit 0/1/2 matrix; `inspect` tree and `--json`; CLI tests (8f62487)                                                                                                                   | done   |
| M1-13  | OpenCollection YAML reader (`opencollection.yml` + item files)                                                  | M1-05 | Bruno 2.0 YAML fixture opens read-only; unknown fields preserved                                                                                                                                    | todo   |
| M1-14  | OpenCollection YAML writer + canonical output                                                                   | M1-13 | Round-trip YAML fixtures; Bruno opens the result                                                                                                                                                    | todo   |

## M2 — Core request engine

| ID    | Task                                                                                                                      | Deps      | Acceptance / evidence                                                                | Status |
| ----- | ------------------------------------------------------------------------------------------------------------------------- | --------- | ------------------------------------------------------------------------------------ | ------ |
| M2-01 | Variable scopes: process, collection, folder, environment, request, runtime; precedence                                   | M1-05     | Precedence table tests match documented Bruno order                                  | todo   |
| M2-02 | Interpolation: `{{var}}`, nested, unresolved markers, escaping, `process.env`                                             | M2-01     | Table tests including missing vars and recursive references (bounded)                | todo   |
| M2-03 | Dynamic variables: `$uuid`, `$timestamp`, `$isoTimestamp`, `$randomInt`, faker basics                                     | M2-02     | Deterministic with a seeded source; documented list                                  | todo   |
| M2-04 | URL builder: path params, query params, duplicates, disabled entries, `encodeUrl`                                         | M2-02     | Wire-level tests compare raw request URIs                                            | todo   |
| M2-05 | Body builders: JSON, text, XML, SPARQL, GraphQL, form-urlencoded                                                          | M2-04     | Content-Type and byte-for-byte body tests                                            | todo   |
| M2-06 | Multipart and file bodies: file refs, content types, multiple files, disabled parts                                       | M2-05     | Test server parses multipart; file paths with spaces/unicode                         | todo   |
| M2-07 | HTTP transport: timeout, redirects, max redirects, HTTP/1.1 baseline (HTTP/2 opt-in), keep-alive, header forwarding rules | M2-04     | `httptest` suite; redirect loop terminates; auth header forwarding matches spec      | todo   |
| M2-08 | TLS: custom CA, client cert hooks, verify on/off per request                                                              | M2-07     | Self-signed test server; insecure only when requested                                | todo   |
| M2-09 | Cookie jar per collection with persistence interface                                                                      | M2-07     | Cookies survive restart; domain/path/expiry semantics via `cookiejar`                | todo   |
| M2-10 | Response capture: status, headers, cookies, text/binary body, redirect chain, size, truncation                            | M2-07     | Large body streams to disk; binary detection; no unbounded memory                    | todo   |
| M2-11 | Timing via `httptrace`: DNS, connect, TLS, TTFB, download, total                                                          | M2-10     | Timeline fields populated; test tolerates zero on cached/local                       | todo   |
| M2-12 | Error taxonomy + cancellation                                                                                             | M2-07     | DNS/TLS/timeout/cancel/HTTP errors classified; context cancellation returns promptly | todo   |
| M2-13 | Request conformance suite: 100+ cases                                                                                     | M2-01..12 | Table-driven; race detector passes; documented in TESTING.md                         | todo   |
| M2-14 | `apigo-cli run request.bru` (single request, `--raw`)                                                                     | M2-10     | Exit codes; JSON pretty by default; test against `httptest`                          | todo   |

## M3 — Native workbench

| ID    | Task                                                                                     | Deps         | Acceptance / evidence                                                     | Status |
| ----- | ---------------------------------------------------------------------------------------- | ------------ | ------------------------------------------------------------------------- | ------ |
| M3-01 | App state model + window shell (sidebar/request/response/status), persisted splits       | M2-07        | App opens; layout survives restart; `ui.Tester` renders shell             | todo   |
| M3-02 | Menus and shortcuts: send, save, new/close tab, find, open collection                    | M3-01        | Menu actions dispatch; shortcuts work with focus in editor and URL bar    | todo   |
| M3-03 | Open collection dialog, recent collections, drag-and-drop folder                         | M1-07, M3-01 | Open a real collection; recent list persists; invalid folder shows error  | todo   |
| M3-04 | Sidebar collection tree: folders, requests, methods, search/filter, context menu         | M3-03        | 5,000-row tree scrolls; search filters; context actions dispatch          | todo   |
| M3-05 | Collection CRUD: new request/folder, rename, duplicate, delete, reveal, ordering         | M3-04        | File operations tested on temp dirs; no data loss on cancel               | todo   |
| M3-06 | Tab model and closable tab bar with dirty indicators                                     | M3-01        | Reorder, close, restore session; unsaved prompt; stable keys              | todo   |
| M3-07 | Request bar: method select, URL input, Send/Cancel, loading/error states                 | M3-06, M2-14 | End-to-end send from UI; stale response rejected after tab switch         | todo   |
| M3-08 | Params and headers key-value editors (enable, add, remove, reorder, bulk edit)           | M3-07        | Edits persist to model; disabled rows; duplicate keys                     | todo   |
| M3-09 | Body editor by mode (JSON/text/XML/GraphQL/form/multipart/file) with syntax highlighting | M3-08        | Switching modes preserves per-mode content; file picker; 1 MB body smooth | todo   |
| M3-10 | Auth tab: basic, bearer, API key; inherited-auth indicator                               | M3-09        | Secrets masked; changing auth updates request; UI test                    | todo   |
| M3-11 | Scripts/tests/docs tabs with editor (read-only execution arrives in M5)                  | M3-09        | Content saves; editor shortcuts work; tab state persists                  | todo   |
| M3-12 | Response body viewer: pretty JSON tree, raw, preview (image/HTML text), copy/save        | M3-07, M2-10 | 10 MB JSON < 200 ms; collapsing large arrays; binary placeholder          | todo   |
| M3-13 | Response headers, cookies and timeline tables                                            | M3-12        | Sort/copy/filter; timings displayed; empty states                         | todo   |
| M3-17 | Response formats JSON/HTML/XML/JS/Raw/Hex/Base64 + previews                              | M3-12        | Toggle formats; image/PDF/audio/video preview; 10 MB cap                  | todo   |
| M3-18 | Bulk key/value editor for params/headers/form                                            | M3-08        | Paste bulk, parse, preserve duplicates                                    | todo   |
| M3-14 | Environment selector + variable viewer (masked secrets)                                  | M3-03, M2-01 | Switching env re-resolves; secrets never shown unmasked                   | todo   |
| M3-15 | Save flow: dirty state, atomic write, watcher conflict prompt, external change banner    | M3-06, M1-09 | Save/undo/conflict tests; no partial files                                | todo   |
| M3-16 | UI flow tests + performance budget check                                                 | M3-01..15    | Open→edit→send→save test; startup < 150 ms; no UI-thread I/O              | todo   |

## M4 — Auth, TLS, proxy

| ID    | Task                                                                             | Deps  | Acceptance / evidence                                                | Status |
| ----- | -------------------------------------------------------------------------------- | ----- | -------------------------------------------------------------------- | ------ |
| M4-01 | Auth provider interface + inheritance resolution                                 | M2-07 | Collection/folder/request inheritance tests; explicit none overrides | todo   |
| M4-02 | Basic, Bearer, API key (header/query)                                            | M4-01 | Wire assertions; no duplicate Authorization headers                  | todo   |
| M4-03 | OAuth1 (HMAC-SHA1, PLAINTEXT, RSA-SHA1)                                          | M4-01 | Test vectors; nonce/timestamp injection                              | todo   |
| M4-04 | OAuth2: auth code + PKCE, client credentials, password, refresh, token placement | M4-01 | Loopback server test; token cache; refresh on 401; secrets masked    | todo   |
| M4-05 | Digest auth (MD5/SHA-256, qop)                                                   | M4-01 | RFC 7616 vectors + live test server                                  | todo   |
| M4-06 | NTLM                                                                             | M4-01 | Local NTLM test server; connection reuse                             | todo   |
| M4-07 | AWS SigV4, profiles, session token, service/region                               | M4-01 | AWS documented vectors; `~/.aws` profile discovery                   | todo   |
| M4-08 | WSSE and Akamai EdgeGrid                                                         | M4-01 | Vector tests from provider docs                                      | todo   |
| M4-09 | Client certs (PEM/PKCS#12), custom CA, proxy (HTTP/HTTPS/SOCKS5, auth, no-proxy) | M2-08 | mTLS test server; SOCKS proxy test; proxy auth                       | todo   |
| M4-10 | Auth UI for all modes + validation                                               | M3-10 | Every mode has fields, help text, masked secrets; UI tests           | todo   |

## M5 — Scripting, assertions, tests

| ID    | Task                                                                                                                                    | Deps         | Acceptance / evidence                                                     | Status |
| ----- | --------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------------------------------------------------------------------------- | ------ |
| M5-01 | goja runtime lifecycle: create, interrupt timeout, bounded console output                                                               | M2-12        | Runaway script stops; no goroutine leak; panic-free on bad input          | todo   |
| M5-02 | `bru` API: vars, env vars, headers, next request, process env, assertions/tests access                                                  | M5-01        | API table tests; mutations visible to runner                              | todo   |
| M5-03 | `req` and `res` objects with documented fields                                                                                          | M5-01        | Snapshot tests for request/response shapes                                | todo   |
| M5-04 | Async/await support + microtask draining                                                                                                | M5-01        | Async script fixture resolves and mutates vars                            | todo   |
| M5-05 | `console` bridging, error stack mapping, script timeline                                                                                | M5-01        | Errors show script line; output bounded                                   | todo   |
| M5-06 | `test`, `expect` and chai-compatible matchers                                                                                           | M5-01        | Matcher matrix; failures carry messages                                   | todo   |
| M5-07 | Module shims: chai, moment, crypto-js, uuid, ajv, jwt, tv4 (documented subsets)                                                         | M5-01        | Per-module fixtures; unsupported calls give actionable errors             | todo   |
| M5-08 | Assertion DSL evaluator (`eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `contains`, `matches`, `isJson`, `isDefined`, truthiness, length) | M1-04        | Operator table; nested paths; type coercion rules documented              | todo   |
| M5-09 | Scripts/tests UI with run, output and result badges                                                                                     | M3-11, M5-06 | Run one request; failing test highlighted; editor state kept              | todo   |
| M5-10 | Bruno script compatibility corpus + divergence report                                                                                   | M5-02..08    | Curated scripts pass/fail identically; divergences listed in SCRIPTING.md | todo   |

## M6 — Runner & CLI

| ID    | Task                                                                                        | Deps      | Acceptance / evidence                                        | Status |
| ----- | ------------------------------------------------------------------------------------------- | --------- | ------------------------------------------------------------ | ------ |
| M6-01 | Runner engine: selection, recursion, `seq`, disabled requests, folder/collection iterations | M2, M5    | Fixture collections produce expected order/results           | todo   |
| M6-02 | Environment resolution (extends, global envs), tag filters, prompt/runnable selection       | M6-01     | Inheritance/tag tests; skipped requests carry a reason       | todo   |
| M6-03 | Flow control: `setNextRequest`, delays, bail, `stopExecution`, `skipRequest`                | M6-01     | Loop/termination tests; bail stops promptly                  | todo   |
| M6-04 | Reporters: console, JSON, JUnit XML, HTML                                                   | M6-01     | Golden outputs; JUnit validates against schema; HTML escapes | todo   |
| M6-05 | CLI `run` flags and exit codes                                                              | M6-01     | Contract tests for every flag; 0/1/2/3 exit-code matrix      | todo   |
| M6-06 | CLI `import` and `docs` commands                                                            | M6-05, M8 | Commands work on fixtures; help text matches docs            | todo   |
| M6-07 | Docker image for the CLI (alpine + debian, amd64/arm64)                                     | M6-05     | Image runs a collection; documented in RELEASE.md            | todo   |
| M6-08 | CI examples (GitHub Actions, GitLab, Jenkins)                                               | M6-05     | Example workflows lint and run against a sample collection   | todo   |
| M6-09 | Bruno CLI conformance harness                                                               | M6-04     | Same collection, normalized output; differences triaged      | todo   |
| M6-10 | Runner UI (select collection/folder, env, run, progress, results)                           | M6-04     | UI test; cancellation; result drill-down                     | todo   |

## M7 — Protocols

| ID    | Task                                                                   | Deps      | Acceptance / evidence                                        | Status |
| ----- | ---------------------------------------------------------------------- | --------- | ------------------------------------------------------------ | ------ |
| M7-01 | GraphQL request model, editor, variables, response errors              | M3-09     | Query/mutation fixtures; variables JSON; syntax highlighting | todo   |
| M7-02 | GraphQL introspection + schema docs                                    | M7-01     | Introspection against test server; cached schema; docs pane  | todo   |
| M7-03 | WebSocket connect/auth/headers/subprotocols, send/receive, message log | M3-07, M4 | Echo server tests; binary/text; close codes; reconnect       | todo   |
| M7-04 | SSE request + event stream viewer                                      | M3-07, M4 | Parses event/id/retry/data; pause/clear; bounded buffer      | todo   |
| M7-05 | Protocol tabs and response layouts                                     | M7-01..04 | Each protocol persists state per tab; UI tests               | todo   |
| M7-06 | Protocol integration tests on all platforms                            | M7-01..05 | CI green on macOS/Linux/Windows                              | todo   |

## M8 — Import/export & snippets

| ID    | Task                                                                 | Deps      | Acceptance / evidence                                    | Status |
| ----- | -------------------------------------------------------------------- | --------- | -------------------------------------------------------- | ------ |
| M8-01 | Importer interface + collection writer                               | M1-07     | Fixture-driven; warnings for lossy fields                | todo   |
| M8-02 | Postman v2.1 collection + environment import                         | M8-01     | Corpus round-trip; auth/body/scripts mapped or warned    | todo   |
| M8-03 | OpenAPI 3.x + Swagger 2.0 import                                     | M8-01     | Petstore + edge cases; operation grouping                | todo   |
| M8-04 | Insomnia v4, curl, HAR import                                        | M8-01     | Fixtures; curl flag parsing; HAR entries                 | todo   |
| M8-05 | Bruno export (and Postman export where feasible)                     | M8-01     | Exported collection opens in Bruno/Postman               | todo   |
| M8-09 | Bruno ZIP/JSON import, Git clone import, spec URL import             | M8-01     | Fixtures + path/size validation; clone uses git          | todo   |
| M8-10 | OpenAPI spec export, environment JSON, HTML docs                     | M8-05     | Output opens in Bruno/Swagger; warnings for lossy fields | todo   |
| M8-06 | Code snippets: HTTPSnippet-compatible targets (curl first)           | M2        | Snapshot tests; secrets redacted                         | todo   |
| M8-11 | OpenCollection YAML conversion + `.bru` ↔ YAML migration with backup | M1-13     | Round-trip both ways; Bruno opens the result             | todo   |
| M8-07 | UI import/export wizard                                              | M8-02..06 | Pick file, preview, choose target, progress, errors      | todo   |
| M8-08 | Import/export conformance corpus and docs                            | M8-02..07 | Matrix of supported fields; known losses documented      | todo   |

## M9 — Git, secrets, history

| ID    | Task                                                                               | Deps         | Acceptance / evidence                                        | Status |
| ----- | ---------------------------------------------------------------------------------- | ------------ | ------------------------------------------------------------ | ------ |
| M9-01 | Git CLI wrapper: detect repo, status, log, diff, branches (extension)              | M3-03        | Integration tests in temp repos; missing git handled         | todo   |
| M9-02 | Git panel UI: changes, stage/unstage, commit, pull/push, branch switch (extension) | M9-01        | Real repo flow; cancellable; errors actionable               | todo   |
| M9-03 | Diff viewer integration + conflict resolution (extension)                          | M9-02        | Ours/theirs/edit; merge abort; no data loss                  | todo   |
| M9-04 | Secret store in OS keychain + prompt/mask flows                                    | M1-08        | Keychain tests (platform-skipped where needed); no plaintext | todo   |
| M9-05 | Request history (bbolt) + history UI (extension; Bruno has none)                   | M2-10        | Persists, searches, prunes; redacted secrets                 | todo   |
| M9-06 | Cookie persistence (bbolt jar)                                                     | M2-09        | Survives restart; clear per collection                       | todo   |
| M9-07 | Global search across collections (Cmd+K)                                           | M3-04        | Query by name/URL/method; performance on 10k requests        | todo   |
| M9-11 | Workspaces: `workspace.yml`, global environments, scratch requests                 | M1-07        | Workspace fixture opens; global envs resolve                 | todo   |
| M9-08 | Command palette + recent/favorite collections                                      | M3-02        | Keyboard-only; actions match menus                           | todo   |
| M9-09 | Git ignore suggestions + secret-leak warning                                       | M9-02, M9-04 | Warns before committing likely secrets                       | todo   |
| M9-10 | Git/secret/history tests on all platforms                                          | M9-01..09    | CI green; documented platform caveats                        | todo   |

## M10 — gRPC, mock server, advanced

| ID     | Task                                                                         | Deps       | Acceptance / evidence                                 | Status |
| ------ | ---------------------------------------------------------------------------- | ---------- | ----------------------------------------------------- | ------ |
| M10-01 | Proto loading (files/import paths) + descriptor model                        | M1         | Real proto fixtures; errors with line numbers         | todo   |
| M10-02 | gRPC unary calls, metadata, auth, status/trailers                            | M10-01, M4 | Interop against Go test server                        | todo   |
| M10-03 | gRPC streaming (client/server/bidi) + message log                            | M10-02     | Stream cancellation; message ordering                 | todo   |
| M10-04 | gRPC reflection                                                              | M10-02     | Reflection-enabled test server; no proto files needed | todo   |
| M10-05 | Mock server: serve collection routes, latency, logs                          | M2, M6     | Runner passes against mock; port conflict handling    | todo   |
| M10-06 | AI scope decision + implementation if opted in (MCP absent from Bruno 2.0.0) | M6         | ADR; off by default; no data leaves without action    | todo   |

## M11 — Parity audit & polish

| ID     | Task                                                                              | Deps    | Acceptance / evidence                                           | Status |
| ------ | --------------------------------------------------------------------------------- | ------- | --------------------------------------------------------------- | ------ |
| M11-01 | Capability matrix audit, row by row                                               | all     | Every row `done`/`divergent`/`dropped` with evidence            | todo   |
| M11-02 | i18n infrastructure + English/Chinese catalogs (extension; Bruno is English-only) | M3      | Language switch; catalog check passes; no hard-coded UI strings | todo   |
| M11-03 | Themes, density, typography, system appearance                                    | M3      | Light/dark/system; persisted                                    | todo   |
| M11-04 | Accessibility pass (labels, focus, keyboard-only, screen reader)                  | M3      | Checklist; no keyboard traps; roles announced                   | todo   |
| M11-05 | Performance pass: startup, collection load, response render, memory               | all     | Budgets in ARCHITECTURE.md met with benchmarks                  | todo   |
| M11-06 | Conformance sweep (parser, HTTP, script, runner, CLI, import)                     | all     | Full suite green; fixtures pinned                               | todo   |
| M11-07 | User guide + Bruno migration guide                                                | M3..M10 | Docs cover every P0 workflow                                    | todo   |
| M11-08 | Beta bug bash + triage                                                            | all     | No open P0 bugs; P1 decisions recorded                          | todo   |

## M12 — Release engineering

| ID     | Task                                                    | Deps       | Acceptance / evidence                                         | Status |
| ------ | ------------------------------------------------------- | ---------- | ------------------------------------------------------------- | ------ |
| M12-01 | Final identity: name, bundle ID, icon, trademark review | M0-10      | ADR + `mygo.json` updated; no Bruno marks                     | todo   |
| M12-02 | macOS signing + notarization in CI                      | M12-01     | Signed DMG passes Gatekeeper on a clean Mac                   | todo   |
| M12-03 | Windows Authenticode signing in CI                      | M12-01     | Signed installer passes SmartScreen check                     | todo   |
| M12-04 | Linux packages (deb/rpm/snap/Flatpak)                   | M12-01     | Install/uninstall on clean distro images                      | todo   |
| M12-05 | Signed auto-updates + key management + rollback         | M12-02..04 | Update verifies signature; rollback works                     | todo   |
| M12-06 | Package managers (Homebrew/winget/Scoop/Chocolatey)     | M12-05     | Install commands documented and tested                        | todo   |
| M12-07 | SBOM, checksums, release checklist, 1.0 notes           | M12-06     | Release artifact set complete; notes generated from CHANGELOG | todo   |
