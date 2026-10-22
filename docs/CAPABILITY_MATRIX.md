# Capability matrix

The parity contract. Every row is either implemented and tested, explicitly
divergent with an ADR/test, or out of scope. A row without evidence is not done.

Target: Bruno 2.0.0 (commit `dd0281d`, 2026-10-19), open-source desktop + CLI.
Evidence for Bruno behavior is in [research/](research/); research is a snapshot,
not a specification to copy.

Status values:

- `done` — implemented, tested, linked to evidence.
- `partial` — usable but a known part is missing; the gap is described.
- `todo` — planned, not implemented.
- `divergent` — implemented differently on purpose; ADR/test required.
- `dropped` — deliberately not implemented; reason required.
- `extension` — apigo adds behavior Bruno lacks; must not break parity.

Phase values refer to [ROADMAP.md](ROADMAP.md) milestones.

## A. Collection and format

| ID  | Capability                                                 | Bruno 2.0.0                                                                  | apigo design                           | Phase | Status    |
| --- | ---------------------------------------------------------- | ---------------------------------------------------------------------------- | -------------------------------------- | ----- | --------- |
| A01 | Open a `.bru` collection directory                         | `bruno.json` + `.bru` tree                                                   | Format detection + loader              | M1    | todo      |
| A02 | Read/preserve `bruno.json` config                          | proxy, scripts, certs, presets, ignore                                       | Typed config + raw unknown fields      | M1    | todo      |
| A03 | Nested folders, `folder.bru`, folder metadata              | Headers/auth/vars/scripts/tests/docs                                         | Same model, recursive                  | M1    | todo      |
| A04 | Ordering by name/`seq`, tags, disabled items               | `seq`, `tags`, `~` prefix                                                    | Same; stable sort                      | M1    | todo      |
| A05 | Environments, `extends`, secret names, color, typed vars   | `.bru` env grammar                                                           | Same; keychain for values              | M1    | todo      |
| A06 | `.env` / `process.env`                                     | dotenv over OS env                                                           | Same, separate namespace               | M1    | todo      |
| A07 | OpenCollection YAML (read)                                 | `opencollection.yml` + `folder.yml` + item YAML; default for new collections | `internal/opencollection` reader       | M1    | todo      |
| A08 | OpenCollection YAML (write)                                | Default format for new collections                                           | Writer, canonical YAML                 | M1    | todo      |
| A09 | Unknown block/key preservation, stable output              | Parser/printers                                                              | AST spans + `Extra`, canonical printer | M1    | todo      |
| A10 | Create/rename/duplicate/delete collection, folder, request | Sidebar CRUD                                                                 | Atomic file operations                 | M3    | todo      |
| A11 | File watching / external edits                             | chokidar, conflict prompts                                                   | Debounced watcher + reload             | M1    | todo      |
| A12 | Request docs (Markdown)                                    | `docs {}`                                                                    | Editor + preview                       | M3    | todo      |
| A13 | Examples (request/response snapshots)                      | `example {}`                                                                 | Model + UI                             | M1/M3 | todo      |
| A14 | Request/folder/collection presets, ignore rules            | `bruno.json`                                                                 | Same, editable                         | M3    | todo      |
| A15 | TOML request format                                        | `bruno-toml` package                                                         | Import/export only, not native         | M8    | divergent |
| A16 | Workspace global environments                              | `workspace.yml`                                                              | Read workspace envs                    | M6/M9 | todo      |
| A17 | `.bru` ↔ OpenCollection migration                          | UI migrates bru→yml only, destructive                                        | Bidirectional conversion with backup   | M8    | extension |

## B. Request engine

| ID  | Capability                                                    | Bruno 2.0.0                           | apigo design                                       | Phase | Status    |
| --- | ------------------------------------------------------------- | ------------------------------------- | -------------------------------------------------- | ----- | --------- |
| B01 | Methods, custom method, HEAD/OPTIONS                          | Standard + `http` block               | Same                                               | M2    | todo      |
| B02 | URL interpolation, path params, query duplicates, `encodeUrl` | `interpolate-vars` + axios            | Dedicated builder, wire tests                      | M2    | todo      |
| B03 | Headers, disabled headers, metadata, `omitHeaders`            | Merge + default header rules          | Same; transport defaults exposed to scripts        | M2    | todo      |
| B04 | Redirects 301/302/303/307/308, max 5, auth stripping          | Client-side loop                      | Same; multipart replay                             | M2    | todo      |
| B05 | Timeout, `inherit`, default no timeout                        | Settings + preferences                | Same; CLI `--timeout` extension                    | M2    | todo      |
| B06 | Cookie jar, persistence, `Cookie` merge                       | tough-cookie                          | `cookiejar` + bbolt                                | M2/M9 | todo      |
| B07 | TLS verify on/off, custom CA                                  | Node TLS + preferences                | System roots + CA, explicit insecure               | M2    | todo      |
| B08 | Client certs PEM/PFX, domain globs                            | Collection/global certs               | Same, pure-Go PKCS#12                              | M4    | todo      |
| B09 | Proxy off/manual/system/PAC, SOCKS, auth, bypass              | Agents + system proxy                 | Same; PAC via goja                                 | M4    | todo      |
| B10 | gzip/deflate/br, chunked, BOM/UTF-16                          | axios/Node                            | Go + brotli; charset handling                      | M2    | todo      |
| B11 | Default headers and sent-header truth                         | UA/Accept/Encoding, `_header` capture | Same defaults (UA `apigo`), computed sent headers  | M2    | divergent |
| B12 | JSON/text/XML/SPARQL/none bodies                              | 9 body modes                          | Same                                               | M2    | todo      |
| B13 | form-urlencoded, repeated keys                                | urlencoded builder                    | Same                                               | M2    | todo      |
| B14 | Multipart, per-part content type, file parts, replay          | `form-data`                           | `multipart.Writer`, retained parts                 | M2    | todo      |
| B15 | File/binary bodies, >20 MiB streaming                         | read/stream threshold                 | Stream with size cap                               | M2    | todo      |
| B16 | GraphQL body + variables                                      | JSON body mode                        | Same                                               | M2/M7 | todo      |
| B17 | Basic, Bearer, API key                                        | Headers/query                         | Same                                               | M4    | todo      |
| B18 | Digest auth (MD5, qop)                                        | 401 challenge                         | Same                                               | M4    | todo      |
| B19 | NTLM                                                          | `axios-ntlm`                          | Pure-Go NTLM RoundTripper                          | M4    | todo      |
| B20 | OAuth1 (7 signature methods, body hash, 3 placements)         | RFC 5849                              | Same                                               | M4    | todo      |
| B21 | OAuth2 all four grants, PKCE, state, cache/refresh, placement | Desktop                               | Same                                               | M4    | todo      |
| B22 | OAuth2 in CLI                                                 | Interactive grants broken             | Non-interactive + browser loopback where available | M4/M6 | divergent |
| B23 | AWS SigV4, profiles, session token                            | aws4-axios                            | Pure-Go signer + `~/.aws`                          | M4    | todo      |
| B24 | WSSE                                                          | Non-standard digest                   | Reproduce by default; strict option                | M4    | divergent |
| B25 | Akamai EdgeGrid                                               | Helper                                | Same                                               | M4    | todo      |
| B26 | Auth inheritance                                              | Collection/folder/request             | Same; explicit none                                | M4    | todo      |
| B27 | Timing/timeline                                               | performance.now hops                  | `httptrace` + timeline                             | M2    | todo      |
| B28 | Cancellation                                                  | AbortController                       | context + tab generation                           | M2/M3 | todo      |
| B29 | HTTP/2                                                        | absent (HTTP/1.1 only)                | opt-in per request                                 | M11   | extension |
| B30 | Stream large responses to disk                                | fully buffered                        | 10 MiB threshold                                   | M2    | extension |
| B31 | DNS/localhost normalization                                   | IPv4/IPv6 probe                       | Resolver normalization                             | M2    | todo      |
| B32 | Variables: precedence, typed, unresolved, prompt              | 9 scopes                              | Same + resolver tests                              | M2    | done      |
| B33 | 120 dynamic faker variables                                   | faker v9                              | 120 names, own word lists (same shapes)            | M2    | divergent |
| B34 | Request settings (redirects, timeout, omit, encode)           | `settings {}`                         | Same                                               | M2    | todo      |
| B35 | Prompt variables in desktop, skip in runner                   | UI prompt / skip                      | Same                                               | M3/M6 | todo      |

## C. Scripting, assertions, tests

| ID  | Capability                                                          | Bruno 2.0.0                 | apigo design                         | Phase | Status    |
| --- | ------------------------------------------------------------------- | --------------------------- | ------------------------------------ | ----- | --------- |
| C01 | Safe JS sandbox (QuickJS)                                           | `quickjs-emscripten`        | goja, timeout, bounded output        | M5    | divergent |
| C02 | Pre-request, post-response, tests phases                            | Script runtime              | Same                                 | M5    | todo      |
| C03 | `sandwich` / `sequential` script flow                               | `scripts.flow`              | Same                                 | M5    | todo      |
| C04 | `bru` env/runtime/collection/folder vars                            | Full API                    | Same method names                    | M5    | todo      |
| C05 | `bru.sendRequest`, `runRequest`, cookies, utils, results            | Host bridge                 | Same, axios-shaped config            | M5    | todo      |
| C06 | `req` object (URL/path/query/headers/body/timeout/onFail)           | `BrunoRequest`              | Same fields/methods                  | M5    | todo      |
| C07 | `res` callable, getters, path query, `jq` in asserts                | `BrunoResponse`             | Same, path parser in Go              | M5    | todo      |
| C08 | Safe modules (chai, moment, crypto-js, uuid, ajv, tv4, axios, path) | Bundled QuickJS libs        | Embedded shims                       | M5    | todo      |
| C09 | Local `require('./x.js')`, `__dirname`, module cache                | QuickJS CJS loader          | CJS loader rooted at collection      | M5    | todo      |
| C10 | async/await, promises, `setTimeout` sleep, nested tests             | Async IIFE + pending awaits | goja jobs + host sleep               | M5    | todo      |
| C11 | Console capture, script errors, partial results, line mapping       | QuickJS runtime             | Same contract                        | M5    | todo      |
| C12 | `test()` + chai `expect`/`assert` + custom matchers                 | chai 4 + chai-string        | JS shim + Go deep equality           | M5    | todo      |
| C13 | Assert DSL (28 operators, typed RHS, `res`/`$res` LHS)              | Assert runtime              | Parser + evaluator                   | M5    | todo      |
| C14 | `vars:post-response` expressions                                    | Vars runtime                | Same, before post script             | M5    | todo      |
| C15 | `jwt` global in tests                                               | jsonwebtoken                | HMAC shim                            | M5    | todo      |
| C16 | Developer sandbox (`fs`, npm, Node builtins)                        | Node `vm`                   | Not supported; explicit error        | —     | dropped   |
| C17 | Scripts/tests UI, results, run                                      | CodeMirror panes            | MyGo editor + result list            | M5    | todo      |
| C18 | Script timeout / runaway containment                                | QuickJS interrupt           | goja interrupt + kill                | M5    | todo      |
| C19 | gRPC script hooks                                                   | Four hooks                  | Same                                 | M10   | todo      |
| C20 | Script compatibility corpus                                         | 42 unit + E2E suites        | Curated fixtures + divergence report | M5    | todo      |

## D. Runner and CLI

| ID  | Capability                                   | Bruno 2.0.0            | apigo design                       | Phase  | Status    |
| --- | -------------------------------------------- | ---------------------- | ---------------------------------- | ------ | --------- |
| D01 | Run request, folder, collection              | `bru run`              | Same                               | M6     | todo      |
| D02 | Recursion, `seq` order, disabled/prompt skip | CLI + desktop          | Same                               | M6     | todo      |
| D03 | Tags include/exclude, `--tests-only`         | CLI                    | Same                               | M6     | todo      |
| D04 | Environment by name/file, `extends`          | CLI                    | Same                               | M6     | todo      |
| D05 | Global env + workspace                       | CLI/desktop            | Same                               | M6/M9  | todo      |
| D06 | `--env-var`, `--global-env-var`              | CLI                    | Same                               | M6     | todo      |
| D07 | Persist env/collection var mutations         | CLI + desktop          | Same, secrets blanked              | M6     | todo      |
| D08 | Delay, bail, `setNextRequest`, stop/skip     | CLI/desktop divergence | Union, documented                  | M6     | todo      |
| D09 | `req.onFail`                                 | Script runtime         | Same                               | M6     | todo      |
| D10 | Reporters: console, JSON, JUnit, HTML        | CLI                    | Same shapes                        | M6     | todo      |
| D11 | Reporter redaction flags                     | CLI                    | Same + default secret redaction    | M6     | todo      |
| D12 | Exit codes 0–13, 255                         | CLI                    | Same contract                      | M6     | todo      |
| D13 | `import openapi/wsdl`                        | CLI                    | Same + more formats                | M8     | todo      |
| D14 | `docs generate` HTML                         | CLI                    | Same                               | M8     | todo      |
| D15 | Docker images (alpine/debian, amd64/arm64)   | CLI                    | Same, `apigo-cli`                  | M6     | todo      |
| D16 | Sandbox flag safe/developer                  | CLI                    | safe only; developer errors        | —      | dropped   |
| D17 | Module cache flag                            | CLI (developer)        | Not supported                      | —      | dropped   |
| D18 | No config file / `BRU_*` env vars            | CLI                    | Same; flags only                   | M6     | todo      |
| D19 | Parallel, retries, data-file iterations      | absent                 | not implemented (no parity target) | —      | dropped   |
| D20 | Runner UI with progress/results              | Desktop                | MyGo runner screen                 | M6     | todo      |
| D21 | Runner event stream + response persistence   | Desktop/SQLite         | In-process events + bbolt          | M6     | todo      |
| D22 | gRPC/WS in runner                            | skipped                | skip by default, opt-in execution  | M6/M10 | extension |

## E. Protocols

| ID  | Capability                                             | Bruno 2.0.0          | apigo design                         | Phase | Status    |
| --- | ------------------------------------------------------ | -------------------- | ------------------------------------ | ----- | --------- |
| E01 | GraphQL request (query, variables, errors)             | HTTP body mode + UI  | Same                                 | M7    | todo      |
| E02 | GraphQL introspection + schema docs                    | Desktop              | Same                                 | M7    | todo      |
| E03 | WebSocket connect, headers, auth, subprotocols, TLS    | `ws` client          | gorilla/websocket equivalent         | M7    | todo      |
| E04 | WS send/receive, text/binary, ping, close codes, queue | `WsClient`           | Same, bounded log                    | M7    | todo      |
| E05 | WS in runner                                           | skipped              | skip by default                      | M7    | todo      |
| E06 | SSE streaming, events, post-scripts on close           | Desktop              | Same, pause/clear, bounded           | M7    | todo      |
| E07 | SSE in CLI                                             | buffered             | streaming with `--raw` option        | M7    | extension |
| E08 | gRPC proto files, import paths                         | `@grpc/proto-loader` | `protoreflect` + parser              | M10   | todo      |
| E09 | gRPC reflection v1/v1alpha                             | Desktop              | Same                                 | M10   | todo      |
| E10 | gRPC unary, metadata, auth, status/trailers            | Desktop              | Same                                 | M10   | todo      |
| E11 | gRPC client/server/bidi streaming                      | Desktop              | Same                                 | M10   | todo      |
| E12 | gRPC TLS/mTLS, proxy, script hooks                     | Desktop              | Same (HTTP proxy first)              | M10   | todo      |
| E13 | Mock server routes, rules, delay, request log          | Express server       | Go HTTP server, same model           | M10   | todo      |
| E14 | MCP                                                    | absent in 2.0.0      | not implemented                      | —     | dropped   |
| E15 | AI chat/autocomplete                                   | desktop AI providers | optional, opt-in, no default network | M10   | todo      |
| E16 | Easter egg method `WOOF`                               | present              | omitted                              | —     | dropped   |

## F. Import, export, code generation

| ID  | Capability                              | Bruno 2.0.0         | apigo design                              | Phase | Status  |
| --- | --------------------------------------- | ------------------- | ----------------------------------------- | ----- | ------- |
| F01 | Postman v2.1 collection import          | Converter           | Same, with translation warnings           | M8    | todo    |
| F02 | Postman environment import              | Converter           | Same                                      | M8    | todo    |
| F03 | Bruno → Postman export                  | Converter           | Same                                      | M8    | todo    |
| F04 | OpenAPI 3.x / Swagger 2.0 import        | Converter           | Same, operation grouping                  | M8    | todo    |
| F05 | Insomnia v4/v5 import                   | Converter           | Same                                      | M8    | todo    |
| F06 | WSDL import                             | Converter           | Same                                      | M8    | todo    |
| F07 | Curl paste into URL bar                 | UI parser           | Same (not a collection importer)          | M8    | todo    |
| F08 | OpenCollection import/export            | Converter           | Same                                      | M8    | todo    |
| F09 | `.bru` ↔ internal model (no conversion) | Native              | Native                                    | M1    | todo    |
| F14 | Bruno ZIP / JSON collection import      | ZIP + JSON          | Same, with path validation                | M8    | todo    |
| F15 | Git repository import (clone)           | Sidebar URL         | Same, delegates to git                    | M8    | todo    |
| F16 | OpenAPI / WSDL / spec URL import        | URL fetch           | Same, with size/time limits               | M8    | todo    |
| F17 | OpenAPI spec export                     | Collection → spec   | Same, lossy warnings                      | M8    | todo    |
| F18 | Environment JSON export, HTML docs      | CLI/UI              | Same                                      | M8    | todo    |
| F10 | Code generation (HTTPSnippet targets)   | `httpsnippet`       | Go generator, curl first + common targets | M8    | todo    |
| F11 | Import/export UI wizard                 | Desktop             | Same                                      | M8    | todo    |
| F12 | HAR import/export                       | absent              | not implemented                           | —     | dropped |
| F13 | API docs site generation                | CLI `docs generate` | Same                                      | M8    | todo    |

## G. Desktop UI and workflow

| ID  | Capability                                                 | Bruno 2.0.0           | apigo design                       | Phase | Status    |
| --- | ---------------------------------------------------------- | --------------------- | ---------------------------------- | ----- | --------- |
| G01 | Main three-pane layout, resizable, persisted               | React layout          | MyGo split + state                 | M3    | todo      |
| G02 | Collection sidebar tree, methods, folders                  | Sidebar               | `ui.Tree`/`ui.Outline`             | M3    | todo      |
| G03 | Search/filter collections and requests                     | Sidebar search        | Model index + facet search         | M3/M9 | todo      |
| G04 | Request tabs, dirty state, close prompt, restore           | Redux tabs            | Tab model + keys                   | M3    | todo      |
| G05 | Method + URL bar, send/cancel                              | Request pane          | MyGo controls                      | M3    | todo      |
| G06 | Params/headers key-value editors with enable/reorder       | Grid components       | Virtualized table editor           | M3    | todo      |
| G07 | Body editors per mode with highlighting                    | CodeMirror            | `ui/editor`                        | M3    | todo      |
| G08 | Auth tabs for all modes, inheritance indicator             | Auth components       | Generated form per mode            | M3/M4 | todo      |
| G09 | Scripts/tests/docs editors                                 | CodeMirror            | `ui/editor`, tabs                  | M3/M5 | todo      |
| G10 | Response pretty/raw/preview, JSON tree                     | Response pane         | Virtualized tree + editor          | M3    | todo      |
| G11 | Response headers/cookies/timeline                          | Tables                | `ui.Table`                         | M3    | todo      |
| G12 | Environment selector/editor, masked secrets                | Env UI                | Same                               | M3    | todo      |
| G13 | Collection/folder settings (headers/auth/vars/scripts)     | Settings screens      | Same                               | M3    | todo      |
| G14 | Request settings (redirects/timeout/encode/omit)           | Settings pane         | Same                               | M3    | todo      |
| G15 | Examples list/detail, set as request/response              | Example UI            | Same                               | M3    | todo      |
| G16 | Docs editor/preview                                        | Markdown editor       | `ui/editor` + markdown             | M3    | todo      |
| G17 | Runner screen (select, env, run, results)                  | Runner UI             | MyGo screen                        | M6    | todo      |
| G18 | Git panel (status/diff/commit/branch/push/pull)            | Git UI                | MyGo panel + diff viewer           | M9    | extension |
| G19 | Preferences (general/proxy/certs/cookies/history/themes)   | Preferences           | Settings model + screens           | M3/M9 | todo      |
| G20 | Light/dark/system themes, density                          | Themes                | MyGo theme + custom tokens         | M11   | todo      |
| G21 | Full keyboard shortcut set + menu integration              | Hotkeys               | MyGo menus/handles                 | M3    | todo      |
| G22 | Drag/drop collections and files                            | DnD                   | MyGo drag/drop                     | M3    | todo      |
| G23 | Request history                                            | absent in Bruno 2.0.0 | bbolt + screen                     | M9    | extension |
| G24 | Cookie manager UI                                          | Cookie UI             | Table + clear per domain           | M9    | todo      |
| G25 | Proxy/cert UI with validation                              | Settings              | Same                               | M4/M9 | todo      |
| G26 | Mock server UI                                             | Desktop               | Same                               | M10   | todo      |
| G27 | Import/export UI                                           | Modals                | Same                               | M8    | todo      |
| G28 | Code generation UI (language/library picker)               | Modal                 | Same                               | M8    | todo      |
| G29 | i18n (English-only in Bruno 2.0.0)                         | `en.json` only        | MyGo i18n; English/Chinese first   | M11   | extension |
| G30 | Accessibility (keyboard, labels, screen readers)           | Web a11y              | MyGo roles + tests                 | M11   | todo      |
| G31 | Recent collections, welcome/empty states                   | Home                  | Same                               | M3    | todo      |
| G33 | Response formats: JSON/HTML/XML/JS/Raw/Hex/Base64          | QueryResult           | Same                               | M3    | todo      |
| G34 | Response previews: web/json/xml/text/image/PDF/audio/video | Response pane         | Same (webview for web/PDF)         | M3    | todo      |
| G35 | GraphQL schema explorer + query builder                    | GraphQL pane          | MyGo tree + builder                | M7    | todo      |
| G36 | Global search (`Cmd+K`)                                    | Modal                 | MyGo panel + fuzzy index           | M9    | todo      |
| G37 | Bulk key/value editor                                      | BulkEditor            | Same                               | M3    | todo      |
| G38 | Workspaces, global environments, scratch requests          | `workspace.yml`       | Same                               | M9    | todo      |
| G39 | App items (`meta.type: app`, HTML/JS pane)                 | App preview           | WebView sandbox                    | M10   | todo      |
| G40 | Per-collection security settings (sandbox mode)            | Security settings     | Safe mode only, documented         | M5    | divergent |
| G41 | 22 themes + keybinding editor                              | Themes/Keybindings    | MyGo theme set + editable bindings | M11   | todo      |
| G42 | AI chat/autocomplete                                       | AI sidebar            | Opt-in, off by default             | M10   | todo      |
| G32 | Save/copy/download response                                | Response actions      | Same + stream to file              | M3    | todo      |

## H. Desktop integration

| ID  | Capability                                     | Bruno 2.0.0              | apigo design              | Phase  | Status    |
| --- | ---------------------------------------------- | ------------------------ | ------------------------- | ------ | --------- |
| H01 | Git status/diff/log                            | simple-git + UI          | `git` CLI + porcelain     | M9     | extension |
| H02 | Stage/unstage/commit                           | Git UI                   | Same, confirmation        | M9     | extension |
| H03 | Pull/push/fetch, branch create/switch          | Git UI                   | Same, credential helpers  | M9     | extension |
| H04 | Conflict resolution (ours/theirs/edit, abort)  | Git UI                   | Same + diff viewer        | M9     | extension |
| H05 | Secret storage in OS keychain                  | encrypted electron-store | MyGo secrets/keychain     | M9     | todo      |
| H06 | Persistent cookies                             | cookie store             | bbolt jar                 | M9     | todo      |
| H07 | Persistent history                             | SQLite/IndexedDB         | bbolt + pruned            | M9     | todo      |
| H08 | Recent collections                             | electron-store           | JSON + menu               | M3     | todo      |
| H09 | Native menus, app menu, context menus          | Electron menu            | MyGo menus                | M3     | todo      |
| H10 | Single instance, deep links, file associations | Electron                 | MyGo APIs                 | M11    | todo      |
| H11 | Notifications (run finished, update)           | Electron                 | MyGo notifications        | M6/M12 | todo      |
| H12 | Auto-update with signed manifests              | Electron updater         | MyGo updater              | M12    | todo      |
| H13 | App state persistence (tabs, panes, window)    | electron-store           | JSON state + window state | M3     | todo      |

## I. Release, quality, operations

| ID  | Capability                       | Bruno 2.0.0              | apigo design                 | Phase | Status |
| --- | -------------------------------- | ------------------------ | ---------------------------- | ----- | ------ |
| I01 | macOS signing + notarization     | electron-builder         | `mygo build` + Developer ID  | M12   | todo   |
| I02 | Windows Authenticode             | electron-builder         | `mygo build` + cert          | M12   | todo   |
| I03 | Linux deb/tar.gz install         | electron-builder         | `mygo build`                 | M12   | todo   |
| I04 | rpm/snap/Flatpak                 | electron-builder targets | add per package manager      | M12   | todo   |
| I05 | Homebrew/winget/Scoop/Chocolatey | external manifests       | external manifests           | M12   | todo   |
| I06 | CLI Docker images                | usebruno/cli             | apigo-cli images             | M6    | todo   |
| I07 | CI on macOS/Linux/Windows        | GitHub Actions           | Same                         | M0    | done   |
| I08 | Checksums + SBOM                 | release assets           | Same                         | M12   | todo   |
| I09 | Performance budgets              | Electron baseline        | native budgets               | M11   | todo   |
| I10 | License/trademark separation     | Bruno trademark          | own name, no marks           | M12   | todo   |
| I11 | Changelog + release notes        | release notes            | Keep a Changelog + generated | M0    | done   |
| I12 | Documentation set                | docs site                | docs/ in repo                | M0    | done   |

## Summary

| Area                | Rows    | Done  | Todo    | Divergent | Extension | Dropped |
| ------------------- | ------- | ----- | ------- | --------- | --------- | ------- |
| A Collection/format | 17      | 0     | 15      | 1         | 1         | 0       |
| B Request engine    | 35      | 0     | 30      | 3         | 2         | 0       |
| C Scripting/tests   | 20      | 0     | 18      | 1         | 0         | 1       |
| D Runner/CLI        | 22      | 0     | 18      | 0         | 1         | 3       |
| E Protocols         | 16      | 0     | 13      | 0         | 1         | 2       |
| F Import/export     | 18      | 0     | 17      | 0         | 0         | 1       |
| G UI                | 42      | 0     | 38      | 1         | 3         | 0       |
| H Desktop           | 13      | 0     | 9       | 0         | 4         | 0       |
| I Release/quality   | 12      | 3     | 9       | 0         | 0         | 0       |
| **Total**           | **195** | **3** | **167** | **6**     | **12**    | **7**   |

The totals are a planning aid, not a score. A row moves to `done` only with a
test and, for parity behavior, a conformance fixture. Update this table and the
summary whenever a row changes.
