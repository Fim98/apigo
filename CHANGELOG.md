# Changelog

All notable changes to apigo are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Nothing yet. The API client work starts with M1; see `docs/BACKLOG.md`.

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
