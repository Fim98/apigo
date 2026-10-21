# apigo

A fast, offline-first API client with a native Go interface, compatible with
[Bruno](https://github.com/usebruno/bruno) collections.

apigo keeps every collection in a folder of plain-text `.bru` files, so the
files stay reviewable in a pull request and portable between tools. It has no
account, no cloud sync and no telemetry: the collection on disk is the source
of truth.

> **Status: foundation (0.1.0).** The plan, specifications, toolchain, CI and
> release pipeline are in place; the API client itself is being built. The
> current window is a MyGo starter view and the current CLI only reports its
> version. See the [roadmap](docs/ROADMAP.md) and [backlog](docs/BACKLOG.md).
> Do not treat apigo as usable yet.

## What "compatible with Bruno" means

apigo targets **functional parity with Bruno 2.0.0 (MIT)**:

- Opens existing Bruno collections in both formats — `.bru` + `bruno.json`, and
  OpenCollection YAML (`opencollection.yml`, `folder.yml`, `*.yml`, the default
  for new Bruno collections) — and writes files Bruno can open again.
- Sends the same HTTP requests, with the same auth, variables, scripts,
  assertions, tests and runner semantics.
- Implements the same workflow: collections, environments, import/export, the
  runner, the CLI, protocols (GraphQL, WebSocket, SSE, gRPC) and secrets.
  Git panel, request history and non-English locales are apigo extensions: Bruno
  2.0.0 does not ship them.
- Divergences are explicit, tested and listed in the
  [capability matrix](docs/CAPABILITY_MATRIX.md); "pixel-identical UI" is not a
  goal. The interface is native, not a web page.

apigo is an independent clean-room implementation. It is not affiliated with,
endorsed by, or shipped by the Bruno project. "Bruno" is a trademark of its
owner; see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

## Stack

| Layer     | Choice                                                                                                   |
| --------- | -------------------------------------------------------------------------------------------------------- |
| Language  | Go 1.27, `CGO_ENABLED=0` everywhere                                                                      |
| Desktop   | [MyGo](https://github.com/egoist/mygo) 0.6 native UI (`ui`, `ui/editor`, `ui/tree`, `ui/table`)          |
| HTTP      | `net/http` with a per-request transport (HTTP/1.1 by default, HTTP/2 opt-in), proxy, TLS and `httptrace` |
| Scripting | [goja](https://github.com/dop251/goja), sandboxed, with a compatibility layer for Bruno's JS API         |
| Storage   | `.bru`/OpenCollection files; bbolt for history/cookies; OS keychain for secrets                          |
| CLI       | `apigo-cli`, sharing the engine with the app; no UI imports                                              |
| Release   | `go tool mygo build`, GitHub Actions, optional code signing/notarization                                 |

The full rationale is in the [architecture guide](docs/ARCHITECTURE.md) and the
[ADRs](docs/decisions/).

## Develop

Requirements: Go 1.27+, Node.js 22+ for the repository hooks (Husky, Prettier,
markdownlint). No Bun, no C compiler and no webview are required for the native
app.

```sh
npm install          # installs Husky hooks
go tool mygo doctor
make run             # go run ./cmd/apigo
make dev             # live-reload native app
make test            # CGO_ENABLED=0 go test ./...
make verify          # everything CI checks, locally
```

Build installers for this machine:

```sh
go tool mygo build
```

The output is in `build/<os>-<arch>/`; see [release engineering](docs/RELEASE.md)
for the tag-driven GitHub Actions pipeline.

## Repository map

```text
cmd/apigo/            native desktop app (MyGo)
cmd/apigo-cli/        terminal runner (no UI imports)
internal/bru/         .bru tokenizer, parser, AST, printer
internal/opencollection/ OpenCollection YAML reader/writer
internal/collection/  collection model, discovery, environments, watcher
internal/vars/        variable scopes and interpolation
internal/httpx/       request building, transports, cookies, timing
internal/auth/        auth providers
internal/script/      goja runtime and Bruno JS compatibility
internal/assert/      assertion DSL
internal/runner/      collection runner and reporters
internal/convert/     importers, exporters, code snippets
internal/gitx/        git CLI integration
internal/store/       settings, history, cookies, secrets
internal/ui/          native screens and reusable widgets
internal/app/         application services shared by the UI and CLI
docs/                 plan, specs, decisions, handoff state
```

## Documentation

Start with [AGENTS.md](AGENTS.md) if you are an agent or a new contributor.

| Document                                       | Purpose                                                   |
| ---------------------------------------------- | --------------------------------------------------------- |
| [Roadmap](docs/ROADMAP.md)                     | Milestones, sequencing, exit criteria and release trains  |
| [Backlog](docs/BACKLOG.md)                     | Claimable tasks with dependencies and acceptance criteria |
| [Handoff](docs/HANDOFF.md)                     | Living state: what is done, what is next, how to verify   |
| [Capability matrix](docs/CAPABILITY_MATRIX.md) | The parity contract, row by row                           |
| [Architecture](docs/ARCHITECTURE.md)           | Modules, data flow, concurrency, storage and error model  |
| [.bru format](docs/BRU_FORMAT.md)              | Grammar, file types, serialization and round-trip policy  |
| [Request engine](docs/REQUEST_ENGINE.md)       | Variables, encoding, auth, HTTP, cookies, proxies, timing |
| [Scripting](docs/SCRIPTING.md)                 | goja sandbox, `bru`/`req`/`res`, modules, divergences     |
| [Testing](docs/TESTING.md)                     | Test pyramid, conformance fixtures, UI tester, CI         |
| [Release](docs/RELEASE.md)                     | Versioning, changelog, signing, GitHub Actions, updates   |
| [Security](docs/SECURITY.md)                   | Secrets, TLS, script sandbox, supply chain                |
| [Decisions](docs/decisions/)                   | ADRs and the template for new ones                        |
| [Research](docs/research/)                     | Source-level inventories of Bruno 2.0.0 and MyGo 0.6.2    |

## Contributing

Read [AGENTS.md](AGENTS.md). In short: one backlog task per pull request, tests
for behavior, no `CGO_ENABLED=1`, no changes to the `.bru` format without an
ADR, and update `CHANGELOG.md` plus `docs/HANDOFF.md` in the same change.

Commits follow [Conventional Commits](https://www.conventionalcommits.org/):
`feat(bru): parse annotations`, `fix(request): keep duplicate query params`.

## License

A project license has not been chosen yet (`UNLICENSED` in `package.json`).
Do not distribute builds until the owner makes that decision. Bruno-derived
test fixtures are attributed in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
