# Changelog

All notable changes to apigo are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `.bru` language support (`internal/bru`): lexer with byte spans and
  line/column tracking, block parser for request, folder, collection and
  environment files, canonical printer, and a parser that preserves unknown
  blocks and duplicate keys.
- Collection model and loader (`internal/collection`): typed requests, folders,
  auth, body modes, variables, assertions, scripts and settings; collection →
  folder → request inheritance for headers, auth, variables and scripts;
  `bruno.json`, `collection.bru`, `folder.bru` and `environments/` loading with
  ignore rules, `seq` ordering and a 2.5 MiB per-file cap.
- Environment resolution with `extends` chains, cycle detection and
  `vars:secret` names whose values never touch the file.
- OpenCollection YAML writer: `MarshalYAMLRequest`/`Folder`/`Collection`/
  `Environment` patch a clone of the file's `yaml.Node` tree, so unknown keys —
  including ones inside headers, variables, auth blocks and actions — survive an
  edit. Output is canonical (2-space indent, LF, version key first).
- `apigo-cli fmt` now canonicalises OpenCollection files too, and refuses to
  touch YAML that does not belong to a collection (a compose file or CI config
  is never rewritten).
- OpenCollection YAML reader (`internal/opencollection`, `internal/collection`):
  `opencollection.yml`, `folder.yml`, item `.yml` and environment `.yml` files
  load into the same request/folder/environment model as `.bru`, with unknown
  keys and comments preserved through `yaml.Node` (ADR-0010). The loader picks
  YAML when both formats are present and warns.
- OpenCollection shape notes verified against Bruno fixtures: raw bodies use
  `data:`, auth may be a bare scalar, scripts are `before-request` /
  `after-response` / `tests` / `grpc:*`, environments carry `extends`, `color`,
  `secret: true` and `externalSecrets`.
- Variable scopes (`internal/vars`): the eight-layer precedence from
  REQUEST_ENGINE §2 (global, collection, environment, folder, request, OAuth2,
  runtime, prompt), disabled-variable fall-through, nearest-folder-wins ordering,
  `--env-var` overrides, and `process.env` as an explicit namespace. Lookups take
  an exact name before a dot/array path, so `$oauth2.<id>.<field>` and object
  variables both work.
- Dotenv (`internal/collection`): `<collection>/.env` is read with npm dotenv
  v16 grammar (quotes, multiline values, `export`, `KEY: value`, comments) into
  `Collection.DotEnv`; `BuildProcessEnv` merges OS environment, workspace `.env`
  and collection `.env` for the `process.env` namespace. `.env.*` files stay
  inert, and a malformed line is a warning, not a failed load.
- `apigo-cli inspect` reports `.env` variable names (never values) in the tree
  summary and in `--json`.
- File watching (`internal/collection`): recursive `fsnotify` watcher with a
  per-path debounce window, create/update/remove events, the loader's ignore
  rules, and clean shutdown through `Close` or a cancelled context (ADR-0009).
- `apigo-cli fmt` (rewrite or `--check` `.bru` files in canonical form) and
  `apigo-cli inspect` (tree and `--json` summary of a collection).
- Parser conformance corpus: hand-written fixtures with golden output plus
  vendored Bruno 2.0.0 files under `internal/bru/testdata/bruno/`.
- Agent workflow: 20 engineering skills vendored under `.agents/skills/`, with
  per-repo configuration in `docs/agents/` (issue tracker, triage labels, domain
  docs) and a seeded root `GLOSSARY.md`.
- Product-level research inventory of Bruno 2.0.0 (UI, collections, formats,
  import/export, Git, persistence) and a provisional OpenCollection YAML spec.
- Capability matrix corrections: dual-format collections, Git panel/history/i18n
  as extensions, no curl/HAR importer, OpenCollection write in M1.
- Interpolation (`internal/vars`): `{{name}}` tokens resolved through the
  scopes, `{{process.env.NAME}}`, `{{$oauth2.<id>.<field>}}` and `{{?Prompt}}`,
  with nested values bounded by a cycle check and a depth limit, unresolved
  tokens kept literally and reported, JSON-context escaping, and a typed result
  when the whole string is one token.
- `apigo vars` prints the variable layers a request would see, lowest priority
  first, marking shadowed entries, masking secret values and listing `.env` by
  name only; `--text` interpolates one string and reports unresolved names.

### Changed

- M1 now includes the OpenCollection writer, because new Bruno collections
  default to YAML.
- Environment inheritance now matches Bruno: an environment's own variables beat
  everything it inherits, while among inherited variables a secret still wins
  over a plain one. Previously an inherited secret could not be overridden.

## [0.1.0] - 2026-10-21

### Added

- Foundation for a native, offline-first, Bruno-compatible API client.
- Repository layout: `cmd/apigo` desktop app, `cmd/apigo-cli` terminal runner,
  `internal/version`; `mygo.json` points at `./cmd/apigo`.
- Planning suite: `AGENTS.md`, roadmap, backlog, capability matrix,
  architecture, `.bru` format, request engine, scripting, testing, release and
  security specifications.
- Architecture decision records for native UI, `.bru` compatibility, goja,
  `net/http`, Git CLI, CLI binary, bbolt storage and the tabbed workbench.
- Source-level research inventories for Bruno 2.0.0 and MyGo 0.6.2.
- Developer tooling: gofumpt, Prettier, markdownlint, Husky pre-commit,
  commit-msg and pre-push hooks, commitlint, Makefile and `scripts/verify.sh`.
- GitHub Actions: docs/lint/test matrix, race detector, packaging builds, and a
  tag-driven draft release with installers, CLI binaries and checksums.
- Minimal `apigo-cli` with `version` and `help`; exit codes 0 and 2.
- Git repository initialized on `main`; `.gitignore`, `.gitattributes` and
  `.editorconfig`.

### Notes

- The desktop window is still the MyGo starter view. It is not a usable API
  client; see the roadmap.
- The bundle identifier `com.example.apigo` is a placeholder.
- Builds are unsigned/ad-hoc until signing credentials are provided.
