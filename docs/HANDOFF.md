# Handoff

This is the **only** file that describes where the project is right now. Update
it at the end of every session, then verify with `make verify`.

## Snapshot

- Date: 2026-10-21 (Asia/Shanghai)
- Branch: `main`, tracking `origin` (`git@github.com:Fim98/apigo.git`), 3 commits
  ahead of `origin/main` (`31376a1`)
- Commits: foundation `f199eb4`, research/doc corrections, `d99ef9c` (M0-13 agent
  skills), `31376a1` (issue tracker), then M1: `6a61281` (bru), `99dd0b7`
  (collection), `8f62487` (cli)
- Version: `0.1.0` (foundation)
- Milestone: **M1 — Language & collection store** — parser, model, loader, watcher,
  CLI and the OpenCollection YAML reader done; dotenv and the YAML writer remain
- Next milestone: finish M1, then **M2 — Core request engine**
- Working tree: M1 docs for this session

## What is done

### Documentation and decisions

- [x] `AGENTS.md` — operating manual, invariants, workflow, DoD, handoff protocol.
- [x] `docs/ROADMAP.md` — M0–M12, dependencies, estimates, risks, open decisions.
- [x] `docs/BACKLOG.md` — claimable tasks with dependencies and acceptance criteria.
- [x] `docs/CAPABILITY_MATRIX.md` — 195-row parity contract.
- [x] `docs/ARCHITECTURE.md`, `docs/BRU_FORMAT.md`, `docs/OPENCOLLECTION.md`,
      `docs/REQUEST_ENGINE.md`, `docs/SCRIPTING.md`, `docs/TESTING.md`,
      `docs/RELEASE.md`, `docs/SECURITY.md`.
- [x] ADRs 0001–0008 and template.
- [x] Research: `docs/research/bruno-engine-inventory.md`,
      `docs/research/bruno-product-inventory.md`,
      `docs/research/mygo-capabilities.md`.

### Agent workflow (M0-13)

- [x] 20 engineering skills vendored from `mattpocock/skills` 1.3.1 (commit
      `5beb35b`) under `.agents/skills/`; notice in `THIRD_PARTY_NOTICES.md`.
- [x] `docs/agents/issue-tracker.md` — GitHub Issues in `Fim98/apigo`, live since
      the remote was pushed; records the `Backlog:` id rule that keeps issues and
      `BACKLOG.md` aligned.
- [x] `docs/agents/triage-labels.md` — five canonical roles mapped to backlog
      statuses (`blocked`, `todo`, `in_progress`, `dropped`).
- [x] `docs/agents/domain.md` — single-context, ADRs at `docs/decisions/` (not the
      skill's default `docs/adr/`), numbering continues at `ADR-0009`.
- [x] `GLOSSARY.md` seeded with the project vocabulary, each term pointing at the
      spec that owns the detail.
- [x] `AGENTS.md` gained `## Agent skills` and a read-order entry for
      `docs/agents/`.
- [x] Local wiring: `.workbuddy/skills/` symlinks to `.agents/skills/`
      (gitignored); `.scratch/` and `.workbuddy/` ignored.
- [x] Remote `git@github.com:Fim98/apigo.git` added, `main` pushed as `d99ef9c`,
      tracking set; the five triage labels created on the remote.

### M1 — language and collection store

- [x] `internal/bru` (M1-01/02/03): on-demand lexer with byte spans and
      line/column tracking; permissive block parser; `DictBlock`, `TextBlock` and
      `RawBlock` AST; separate environment grammar; errors carry
      path/line/column while a partial AST is still returned.
- [x] `internal/bru` printer (M1-06): canonical block order per BRU_FORMAT §7,
      two-space indent, LF, one trailing newline, disabled pairs last, unknown
      blocks verbatim.
- [x] Fixtures and conformance (M1-10/11): six hand-written fixtures with
      `.golden` output, nine vendored Bruno 2.0.0 files
      (`internal/bru/testdata/bruno/`, MIT, see its `NOTICE.md`), a semantic
      round-trip per fixture and `FuzzParse`.
- [x] `internal/collection` (M1-04/05): typed request, folder, collection and
      environment models; `RequestToFile`/`FolderToFile` for the reverse
      direction; collection → folder → request inheritance for headers, auth,
      pre-request variables and lifecycle scripts.
- [x] Loader (M1-07): `bruno.json` (unknown keys preserved), `collection.bru`,
      `folder.bru`, `environments/`, ignore rules, hidden directories, `seq`
      ordering, the 2.5 MiB cap, and per-file warnings instead of a failed load.
- [x] Environments (M1-08): `vars`, `vars:secret` names, `extends` resolution with
      cycle detection, external secret blocks preserved verbatim.
- [x] `apigo-cli fmt` / `inspect` (M1-12): atomic rewrites, `--check` exit 1,
      `--stdout`, recursive directory walk; tree and `--json` output.
- [x] OpenCollection YAML reader (M1-13): `internal/opencollection` keeps the
      `yaml.Node` tree (unknown keys, comments and order survive), and
      `internal/collection/yaml.go` maps `opencollection.yml`, `folder.yml`, item
      `.yml` and `environments/*.yml` onto the same model as `.bru`. The loader
      detects the format, prefers YAML when both markers exist and warns.
- [x] Watcher (M1-09): recursive `fsnotify` watcher with a per-path debounce,
      create/update/remove events, `IsDir` preserved for removed directories,
      `TooLarge` for files above the 2.5 MiB cap, new subdirectories watched as
      they appear, idempotent `Close` and `Run(ctx)`; ADR-0009 records the
      dependency choice.

### Code and tooling

- [x] `cmd/apigo` (moved from the repository root), `cmd/apigo-cli`,
      `internal/version`; `mygo.json` `main` points at `./cmd/apigo`.
- [x] Go tool pin: `go tool mygo` (0.6.2) and `go tool gofumpt` (0.12.0).
- [x] `package.json` (Husky, lint-staged, Prettier, markdownlint, commitlint),
      `package-lock.json`, `.prettierrc.json`, `.markdownlint-cli2.jsonc`,
      `.editorconfig`, `.gitattributes`, `.gitignore`.
- [x] Husky `pre-commit` (lint-staged + `go vet`), `commit-msg` (commitlint),
      `pre-push` (`scripts/verify.sh`).
- [x] `Makefile`, `scripts/verify.sh`, `scripts/check-release.sh`,
      `scripts/changelog-section.sh`.
- [x] `.golangci.yml`, Dependabot, PR/issue templates.
- [x] GitHub Actions `ci.yml` (docs/lint/test/race/package) and `release.yml`
      (validate/build/CLI/draft release/checksums).
- [x] `CHANGELOG.md`, `THIRD_PARTY_NOTICES.md`, documentation index.

## Verification

Run from the repository root:

```sh
npm ci
make fmt
make test
make vet
make verify
```

At the time of writing:

| Command                                                              | Result                                                                                       |
| -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `CGO_ENABLED=0 go test ./...`                                        | pass (`cmd/apigo`, `cmd/apigo-cli`, `internal/bru`, `internal/collection`)                   |
| `go vet ./...`                                                       | pass                                                                                         |
| `go tool mygo vet .`                                                 | pass                                                                                         |
| `go run ./cmd/apigo-cli version`                                     | `apigo dev`                                                                                  |
| `make test-race`                                                     | pass (`cmd/apigo`, `cmd/apigo-cli`)                                                          |
| `make verify`                                                        | pass (gofumpt, vet, mygo vet, tests, import boundary, cross-build, Prettier, markdownlint)   |
| `go tool mygo build`                                                 | pass — `build/darwin-arm64/apigo.app` (9.0 MB), `apigo 0.1.0 arm64.dmg` (3.0 MB)             |
| `go tool mygo build -platform linux/amd64`                           | pass — `apigo_0.1.0_amd64.deb` (4.0 MB), `apigo-0.1.0-linux-amd64.tar.gz` (4.0 MB)           |
| `go test -fuzz=FuzzParse -fuzztime=60s ./internal/bru`               | pass, 448k executions; seeds committed under `internal/bru/testdata/fuzz`                    |
| Bruno corpus sweep (789 files in `~/Documents/bruno`)                | 784 parse and print idempotently; 5 expected failures (3 legacy v1 files, 2 broken fixtures) |
| `go run ./cmd/apigo-cli inspect internal/collection/testdata/sample` | tree with 3 requests and 2 environments                                                      |
| GitHub Actions                                                       | `d99ef9c`/`31376a1` ran; `Go lint` red (see blockers). M1 commits not pushed yet.            |
| Tag dry-run                                                          | not run                                                                                      |

The 789-file sweep was a local check, not a committed test; the committed corpus
test covers the nine vendored files.

### Verification for M0-13 (2026-10-21)

| Command                                                | Result                                                                                                                                                                                                                                |
| ------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `npx prettier --check "**/*.{md,yml,yaml,json,jsonc}"` | pass — "All matched files use Prettier code style"                                                                                                                                                                                    |
| `npx markdownlint-cli2 "**/*.md" …`                    | pass — 32 files, 0 issues (`.agents/skills`, `.workbuddy`, `.scratch` excluded)                                                                                                                                                       |
| `git push -u origin main`                              | pass — `d99ef9c`; branch tracks `origin/main`                                                                                                                                                                                         |
| GitHub Actions on `d99ef9c`                            | **failure** in `Go lint` only. Docs, Test (3 OS) and Race pass. golangci-lint's bundled gofumpt (`version: latest`) flags `cmd/apigo-cli/main.go`, while the pinned `go tool gofumpt` 0.12.0 passes: two formatter versions disagree. |

## Next three tasks

1. **M1-14 — OpenCollection YAML writer.**
   Re-read the file, patch the modelled nodes on the preserved `yaml.Node` tree,
   and write canonical YAML (2-space indent, LF). Every vendored fixture must
   round-trip, and an unknown key must survive an edit. This is what makes a YAML
   collection editable instead of read-only.
2. **M1-08a — dotenv.**
   Read `<collection>/.env` into a `process.env` map (plain `.env` only; `.env.*`
   is inert), keep it out of the normal variable scopes, and test precedence.
3. **M1-12a — CLI format awareness.**
   `apigo-cli inspect` already loads YAML collections through `collection.Load`;
   add `fmt` support for `.yml` files (canonical YAML output) once M1-14 lands,
   and cover the YAML path in the CLI tests.

## Blockers and open questions

| Item                               | Status      | Owner decision needed                                                                                                                                                                                                                      |
| ---------------------------------- | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| CI `Go lint` red                   | Blocked     | `golangci-lint-action` uses `version: latest`; its bundled gofumpt disagrees with the pinned `go tool gofumpt` 0.12.0. Pin the linter version, or drop `gofumpt` from `.golangci.yml` formatters now that the pinned step owns formatting. |
| Project license                    | Blocked     | The repo is **public** with no `LICENSE`; default is all rights reserved. Decide before outside contributions.                                                                                                                             |
| Bundle identifier / app name       | Blocked     | Replace `com.example.apigo` before 0.2.0 or first public release.                                                                                                                                                                          |
| GitHub repository                  | Partial     | `Fim98/apigo` exists and `main` is pushed. Left: branch protection, badges/URLs, first green CI, pushing M1.                                                                                                                               |
| Concurrent agent sessions          | Watch       | Two sessions edited this workspace at once on 2026-10-21 (files briefly vanished). Commit early; no parallel `git` commands.                                                                                                               |
| YAML dependency for OpenCollection | M1-13       | `gopkg.in/yaml.v3` is cgo-free; confirm in an ADR, including comment/order preservation expectations.                                                                                                                                      |
| Signing/notarization secrets       | Later (M12) | Apple Developer ID, Windows certificate, update key.                                                                                                                                                                                       |
| AI features                        | M10         | Include provider integrations? Which providers? Default is off.                                                                                                                                                                            |
| Git panel scope                    | M9          | Bruno 2.0 does not ship one; add in M9 or defer after 1.0.                                                                                                                                                                                 |
| i18n scope                         | M11         | Bruno 2.0 is English-only; Chinese/other locales are an apigo extension.                                                                                                                                                                   |
| External secret providers          | M10+        | `vars:externalsecrets` is parsed and preserved, but has no resolver.                                                                                                                                                                       |

## Important implementation notes (research and M1)

- **Two formats, one model.** `.bru` + `bruno.json`, and OpenCollection YAML
  (`opencollection.yml`, `folder.yml`, item `.yml`, `environments/*.yml`). New
  Bruno collections default to YAML; existing `.bru` collections keep their
  format. `opencollection.yml` wins when both config files exist and the loader
  records a warning. There is no `collection.yml` in the 2.0.0 snapshot.
- **OpenCollection shapes that surprise.** Raw bodies use `data:` (not
  `content:`); GraphQL bodies omit `type` and carry `query`/`variables`; `auth`
  may be a bare scalar (`auth: inherit`); scripts are `before-request`,
  `after-response`, `tests` and `grpc:*`; post-response variables are
  `set-variable` actions. Verified in docs/OPENCOLLECTION.md §8.
- **YAML unknown data** stays in the `yaml.Node` tree inside
  `internal/opencollection`; the model only takes the fields it understands, and
  the M1-14 writer patches the tree so unmodelled fields survive a save.
- **Text blocks keep their source indentation.** `TextBlock.Text` and
  `Request.Docs`/`Scripts`/`Tests` store block content exactly as written (the
  closing brace must be at column 1). Display and script execution call
  `collection.Outdent`, so a save never re-indents user content. Bruno applies
  `outdentString` on read instead.
- **Unknown blocks are verbatim.** A block name outside the known set becomes
  `RawBlock{Raw}` and prints back unchanged; unknown keys in known dictionary
  blocks land in `Meta.Extra` / `Settings.Extra`.
- **`~` disables a pair**, and disabled pairs print after enabled ones. Duplicate
  keys are kept as separate ordered pairs.
- **Annotations precede their pair** (`@number`, `@boolean`, `@object`,
  `@description('...')`) and are preserved on variables, params and headers.
- **`auth: none` is explicit.** `ResolvedAuth` walks the chain: `inherit` keeps
  walking, `none` stops with no auth, a concrete mode wins.
- **No byte-for-byte identity with Bruno.** The contracts are semantic round-trip,
  print idempotency and unknown-data preservation (BRU_FORMAT §1).
- **Legacy v1 `.bru` files are out of scope.** The `type http-request` / `params`
  … `/params` format is not parsed; Bruno 2.0 migrates it on open.
- **No Git panel in Bruno 2.0.** Only clone/remote-connect are wired; a 1.8k-line
  backend is dormant. Treat the full Git panel as an apigo extension.
- **No request history, command palette, MCP, or non-English i18n** in Bruno 2.0.
  Those are extensions/out of scope, not parity rows.
- **No curl/HAR collection import.** Curl works by pasting into the URL bar.
  Bruno imports Postman, OpenAPI, Swagger, Insomnia, WSDL, Bruno ZIP/JSON,
  OpenCollection, Git repos and spec URLs.
- **Strict schemas in Bruno** (`noUnknown(true)`) reject unknown fields. apigo
  preserves unknown blocks/keys instead; this is a deliberate divergence.
- **Watcher cap: 2.5 MiB** per request file (`collection.MaxFileSize`). The
  watcher sets `Event.TooLarge` from the stat it already does, so the consumer
  never parses an oversized file by accident.
- **Watcher semantics.** The debounce (150 ms default) lives inside
  `collection.Watcher`, per path; a burst collapses to one event, and a rename is
  an update when the path still exists and a remove when it does not (editors
  save by renaming over the target). Removed directories keep `IsDir`, and new
  subdirectories are watched as soon as they appear. Events are also emitted for
  `environments/*.bru`, which the loader reads outside the request tree.
- **Cookies are global**, not per collection. Client certificates are global or
  collection-level; proxy is global or collection-level (no per-request proxy in
  Bruno).
- **Variable precedence:** global env < collection < environment < folder <
  request < OAuth2 credentials < runtime < prompt; `process.env` is separate. Only
  plain `.env` loads; `.env.*` is inert.
- **Assertions** accept `res.*` and legacy `$res.*` on the left.
- **Runner** supports recursive folders, delay, tags and a selected subset; it
  explicitly skips WS/gRPC. No data files, parallel or retries.
- **Scripts** run in QuickJS safe mode by default; NodeVM developer mode is not
  supported by apigo (documented). `lodash`/`fs`/npm are absent.
- **MyGo constraints:** no HTTP/WS/SSE/Git/scripting APIs; no system-proxy API;
  `Secrets` caps at 2,560 bytes; no native print/PDF; `ui.Tree` is not virtualized;
  `ui.Tabs` has no close button. See `docs/research/mygo-capabilities.md`.
- The current `cmd/apigo/main.go` is still the MyGo counter demo; replace it in
  M3-01.
- Do not use `mygo.NewDocuments` for the request workbench; see ADR-0008.
- HTTP/1.1 is the parity default; HTTP/2 is opt-in.

## Environment

- macOS 26 (Apple silicon), Go 1.27.1, Node 24.18.0, npm 11.16.0.
- MyGo 0.6.2 pinned; `go tool mygo` works.
- No Bun required (native UI).
- `git` 2.50.1 and `gh` 2.96.0 available; `origin` is
  `git@github.com:Fim98/apigo.git`.
- A full Bruno 2.0.0 checkout at `~/Documents/bruno` is useful as a read-only
  behaviour reference (789 `.bru` files for corpus checks).

## Handoff template for the next session

```markdown
## Snapshot

- Date:
- Branch / commit:
- Milestone:
- Working tree:

## What changed

- M1-09: ...
- Evidence: `go test ./internal/collection`

## Verification

| Command          | Result |
| ---------------- | ------ |
| `make verify`    |        |
| `make test-race` |        |
| Platform         |        |

## Next three tasks

1.
2.
3.

## Blockers

-

## Notes / divergences

-
```
