# Testing

The project treats `.bru` files, wire requests, script results and CLI output as
contracts. Tests are organized around those contracts, not around internal
implementation details.

## 1. Test pyramid

| Layer       | What                                                                   | Where                                    | Speed     |
| ----------- | ---------------------------------------------------------------------- | ---------------------------------------- | --------- |
| Unit        | Parser tokens, model conversion, interpolation, auth signing, encoders | `internal/*/…_test.go`                   | ms        |
| Golden      | Parser/printer fixtures, CLI output, import/export snapshots           | `testdata/`                              | ms        |
| Integration | `httptest` servers, TLS, proxies, gRPC/WS/SSE servers                  | `internal/*/integration_test.go`         | 10–500 ms |
| Conformance | Real Bruno fixtures and CLI comparisons                                | `internal/conformance`, `testdata/bruno` | seconds   |
| UI          | `ui.NewTester` flows without a window                                  | `internal/ui/…_test.go`                  | ms        |
| E2E smoke   | Built app launched externally, package/update checks                   | CI-only                                  | minutes   |

`make test` runs unit/golden/integration/UI tests with `CGO_ENABLED=0`.
`make test-race` runs the same with the race detector. CI adds cross-platform
builds and packaging.

MyGo has no importable app-level GUI e2e harness (`internal/e2e` is internal to
the framework). App smoke tests drive the built binary externally; the UI state
machine is covered by `ui.NewTester` instead.

## 2. Conformance with Bruno

The reference snapshot is Bruno 2.0.0 at
`dd0281dad107247a4653ff514feb069ac42e7ff8` (2026-10-19).

### 2.1 Fixtures

- `internal/bru/testdata/bruno/` — curated `.bru` files from:
  - `packages/bruno-lang/v2/tests/` (settings, OAuth1, custom methods, examples,
    asserts, collections, environments)
  - `packages/bruno-toml/tests/` (if TOML support is added)
  - `tests/**/fixtures/**/*.bru` for feature-specific syntax
- `internal/collection/testdata/bruno/` — complete collection trees, including
  `bruno.json`, folder files, environments and `.env`.
- `internal/opencollection/testdata/bruno/` — OpenCollection YAML trees
  (`opencollection.yml`, `collection.yml`, `folder.yml`, item `.yml`,
  environments) from Bruno's filestore specs and test collections.
- `internal/script/testdata/bruno/` — scripting fixtures.
- `internal/httpx/testdata/bruno/` — request/response cases derived from
  `packages/bruno-tests/collection/` and the Express testbench.

Every copied file goes with the MIT notice in
[THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md). Do not copy source code,
only data/fixtures. Do not vendor a fixture above 2.5 MiB: Bruno's watcher
refuses to parse it, and our loader must exercise the `ErrTooLarge` path instead.

### 2.2 Parser conformance

```go
func TestBrunoFixtures(t *testing.T) {
    for _, path := range fixtures(t) {
        raw := os.ReadFile(path)
        ast, err := bru.Parse(path, raw)
        // 1. parse succeeds
        // 2. model conversion succeeds
        // 3. print(model) parses to an equal model
        // 4. unknown blocks are present in the output
        // 5. printing twice is byte-identical
    }
}
```

Failures print a diff of the model (not the whole file). A fixture that cannot
pass is skipped only with a `t.Skip` that names the reason and a row in the
capability matrix; skipped fixtures are counted and must not grow silently.

### 2.3 Runtime conformance

For a case directory:

```text
testdata/conformance/<case>/
├── collection/            # .bru files
├── environment.bru        # optional
├── request.json           # optional request selection
├── expected.json          # normalized result
└── server/                # optional Go test server config
```

`expected.json` normalizes volatile values:

- drop `Date`, `Server`, `Set-Cookie` timestamps, request IDs;
- replace durations with `">=0"`;
- sort headers that are unordered;
- keep status, body bytes (or a hash for binary), test results and exit code.

The harness runs apigo in-process. A separate optional script
(`scripts/compare-bruno.sh`, not part of `make verify`) runs the same collection
through `npx @usebruno/cli@1.16.0 run` in Docker and diffs normalized JSON. It is
run manually before a release or nightly, because it needs Node/Docker and
network access.

### 2.4 CLI contract

`cmd/apigo-cli` tests assert:

- exit code for every documented case (0, 1–13, 255);
- stdout/stderr separation;
- JSON shape (golden), JUnit XML (golden + schema validation), HTML contains
  the expected request names and no secrets;
- flag parsing errors and help text;
- no UI imports (a test walks the package import graph).

## 3. HTTP engine tests

Use `httptest.NewServer`, `httptest.NewTLSServer`, `httptest.NewUnstartedServer`
with custom TLS, and raw TCP listeners for malformed responses.

Required matrix:

- methods, custom methods, HEAD, no-body statuses (204/304);
- duplicate query params, path params, encoding, fragments;
- JSON/text/XML/SPARQL/file/form/multipart/GraphQL bodies;
- gzip, deflate, brotli; chunked; Content-Length; trailers;
- redirects: 301/302/303/307/308, relative Location, loops, maxRedirects,
  cross-origin auth stripping, multipart replay;
- cookies: domain/path/expiry/secure/httpOnly/`__Host-`, jar + explicit header;
- TLS: self-signed, custom CA, client cert (PEM and PKCS#12), hostname mismatch;
- proxies: HTTP, HTTPS CONNECT, SOCKS5, auth, bypass, PAC;
- auth: every provider, inheritance, redirect behavior;
- timeouts and cancellation (server blocks; context cancels);
- large bodies (memory bound), binary detection, BOM/UTF-16;
- default headers, `omitHeaders`, `content-type: false`.

Tests assert the **wire** request when possible (a handler reads the raw
request), not only the Go struct, because Go normalizes some values.

## 4. Script tests

- Unit: each module shim, `bru` API, `req`/`res` shape, interpolation.
- Golden: script result JSON (variable mutations, test results, logs).
- Security: `fs`, `process`, `require('lodash')` and network attempts fail;
  path traversal from `require('./../..')` fails; timeouts stop a loop.
- Conformance: cases from Bruno's scripting collection, with a divergence list.
- Async: promises, `setTimeout`, nested `test()`, rejected promises.
- Error mapping: a syntax error on line N reports the `.bru` line.

A script test must never depend on wall-clock timing. Use a fake clock or a
channel the test controls; the only real timeout is a hang guard.

## 5. UI tests

Use `ui.NewTester(view, width, height)` and interact as a user:

```go
func TestOpenSendSave(t *testing.T) {
    app := newTestApp(t) // temp collection, fake engine
    tt := ui.NewTester(app.view, 1200, 800)

    if err := tt.Click("Open collection"); err != nil { t.Fatal(err) }
    // choose the temp folder through an injected dialog service
    tt.Type("https://example.test/users")
    if err := tt.Click("Send"); err != nil { t.Fatal(err) }
    app.engine.Wait() // test hook: blocks until the fake request finishes
    if !tt.HasText("200 OK") { t.Fatalf("texts: %q", tt.Texts()) }
}
```

Rules:

- Inject services (dialogs, clock, HTTP client, keychain) through `app.Deps`;
  never open real OS dialogs in tests.
- Give every stateful widget a key; tests find controls by label/key.
- Add extra frames after elements disappear and return; assert focus and
  shortcut behavior.
- Test the state machine (tab close, dirty prompt, stale response) separately
  from rendering.
- Keep a screenshot/golden test only for layout-critical screens; text-based
  assertions are less brittle.

## 6. Fuzzing and race detection

- `FuzzParse` in `internal/bru`: no panic, bounded memory, error path includes
  position.
- `FuzzInterpolate`: no infinite recursion, bounded output.
- `FuzzImportPostman`/`FuzzImportOpenAPI`: no panic on arbitrary JSON/YAML.
- `go test -race ./...` in CI (Linux) and locally for concurrency changes.
- `go vet` + `go tool mygo vet .` catch MyGo lifetime mistakes.

## 7. Performance tests

Benchmarks live next to the code (`BenchmarkParse`, `BenchmarkJSONRender`,
`BenchmarkInterpolate`). The budgets in
[ARCHITECTURE.md](ARCHITECTURE.md#10-performance-budgets) are checked manually
before a release; a regression > 20% needs a note in the PR.

UI frame time is measured with the MyGo inspector on a release build; a
10 MB JSON response and a 100k-line body are the reference cases.

## 8. CI matrix

| Job     | OS                     | What                                                      |
| ------- | ---------------------- | --------------------------------------------------------- |
| docs    | Ubuntu                 | Prettier + markdownlint                                   |
| lint    | Ubuntu                 | gofumpt check, go vet, mygo vet, golangci-lint            |
| test    | macOS, Ubuntu, Windows | `CGO_ENABLED=0 go test ./...`                             |
| race    | Ubuntu                 | `go test -race ./...`                                     |
| build   | macOS, Ubuntu          | `go tool mygo build` for darwin/universal, windows, linux |
| release | tags                   | draft release + artifacts (see RELEASE.md)                |

Windows/macOS-only tests use build tags or `runtime.GOOS` skips with a reason.
A skipped test must not hide a cross-platform bug: the corresponding behavior
gets a platform-independent unit test where possible.

## 9. Test data hygiene

- No real secrets, tokens or personal data. Use `example.test` domains and
  `test` credentials.
- Binary fixtures are small (< 1 MiB) and committed with `.gitattributes`
  marking them binary.
- Golden files are reviewed like code; update them with a diff, never blind
  `-update`.
- Fixtures that come from Bruno keep their original path in a header comment
  or `SOURCE` file.

## 10. Definition of test coverage

There is no global coverage percentage gate. Instead:

- Every exported behavior has a test.
- Every capability matrix row marked `done` links to at least one test.
- Parser/printer and auth code aim for > 90% statement coverage; UI state
  machines aim for every transition.
- Bugs get a regression test that fails before the fix.
