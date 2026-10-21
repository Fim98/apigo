# Handoff

This is the **only** file that describes where the project is right now. Update
it at the end of every session, then verify with `make verify`.

## Snapshot

- Date: 2026-10-21 (Asia/Shanghai)
- Branch: `main`
- Commits: `git log --oneline`; foundation `f199eb4`, then OpenCollection notes
  and product research corrections (latest docs commit).
- Version: `0.1.0` (foundation)
- Milestone: **M0 — Foundation** files complete; CI/release validation needs a remote
- Next milestone: **M1 — Language & collection store**
- Working tree: clean

## What is done

### Documentation and decisions

- [x] `AGENTS.md` — operating manual, invariants, workflow, DoD, handoff protocol.
- [x] `docs/ROADMAP.md` — M0–M12, dependencies, estimates, risks, open decisions.
- [x] `docs/BACKLOG.md` — claimable tasks with dependencies and acceptance criteria.
- [x] `docs/CAPABILITY_MATRIX.md` — 195-row parity contract (3 done, 167 todo,
      6 divergent, 12 extension, 7 dropped).
- [x] `docs/ARCHITECTURE.md` — package graph, flows, storage, concurrency, budgets.
- [x] `docs/BRU_FORMAT.md` — grammar, blocks, environments, canonical printing.
- [x] `docs/OPENCOLLECTION.md` — Bruno 2.x YAML format and detection.
- [x] `docs/REQUEST_ENGINE.md` — variables, auth, HTTP, bodies, assertions, exit codes.
- [x] `docs/SCRIPTING.md` — goja sandbox, `bru`/`req`/`res`, modules, divergences.
- [x] `docs/TESTING.md` — test pyramid, fixtures, conformance, UI tester.
- [x] `docs/RELEASE.md` — versioning, signing, CI, updates, checklist.
- [x] `docs/SECURITY.md` — threat model, secrets, sandbox, supply chain.
- [x] ADRs 0001–0008 and template.
- [x] Research: `docs/research/bruno-engine-inventory.md`,
      `docs/research/bruno-product-inventory.md`,
      `docs/research/mygo-capabilities.md`.

### Code and tooling

- [x] `cmd/apigo` (moved from the repository root), `cmd/apigo-cli`,
      `internal/version`; `mygo.json` `main` points at `./cmd/apigo`.
- [x] `apigo-cli` skeleton: `version`, `help`; unknown command exits 2.
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

| Command                                    | Result                                                                             |
| ------------------------------------------ | ---------------------------------------------------------------------------------- |
| `CGO_ENABLED=0 go test ./...`              | pass (`cmd/apigo`, `cmd/apigo-cli`, `internal/version`)                            |
| `go vet ./...`                             | pass                                                                               |
| `go tool mygo vet .`                       | pass                                                                               |
| `go run ./cmd/apigo-cli version`           | `apigo dev`                                                                        |
| `npm install`                              | pass (161 packages; Husky installed after `git init`)                              |
| `make test-race`                           | pass (`cmd/apigo`, `cmd/apigo-cli`)                                                |
| `make verify`                              | pass (gofumpt, vet, mygo vet, tests, cross-build, Prettier, markdownlint)          |
| `go tool mygo build`                       | pass — `build/darwin-arm64/apigo.app` (9.0 MB), `apigo 0.1.0 arm64.dmg` (3.0 MB)   |
| `go tool mygo build -platform linux/amd64` | pass — `apigo_0.1.0_amd64.deb` (4.0 MB), `apigo-0.1.0-linux-amd64.tar.gz` (4.0 MB) |
| GitHub Actions                             | not run (no remote yet)                                                            |
| Tag dry-run                                | not run (no remote yet)                                                            |

After adding `docs/research/bruno-product-inventory.md`, run `make fmt` again;
Prettier will reformat it. markdownlint excludes `docs/research/**` because the
tables contain code spans with literal pipes.

## Next three tasks

1. **M1-01 — `.bru` tokenizer.**
   Create `internal/bru`, write the token model (name, value, punctuation,
   newline, text, EOF) with byte spans and line/column tracking, and table tests
   for every token. Do not start the HTTP engine until the parser round-trips the
   corpus.
2. **M1-02 — block parser.**
   Parse dictionary, list and text blocks, annotations, quoted keys and disabled
   pairs for request, folder, collection and environment grammars. Errors carry
   line/column. Use `packages/bruno-lang/v2/src/bruToJson.js` as behavior
   reference only; write Go clean-room.
3. **M1-03 — AST node types.**
   Cover meta/method/grpc/ws/params/headers/metadata/auth/body/vars/assert/
   scripts/tests/docs/settings/app/example. Keep source spans and an `Extra`
   list for unknown blocks.

## Blockers and open questions

| Item                         | Status      | Owner decision needed                                                         |
| ---------------------------- | ----------- | ----------------------------------------------------------------------------- |
| Project license              | Blocked     | Which license? `package.json` currently says `UNLICENSED`; no `LICENSE` file. |
| Bundle identifier / app name | Blocked     | Replace `com.example.apigo` before 0.2.0 or first public release.             |
| GitHub repository            | Blocked     | Create `OWNER/apigo`, push `main`, set branch protection; update badges/URLs. |
| Signing/notarization secrets | Later (M12) | Apple Developer ID, Windows certificate, update key.                          |
| AI features                  | M10         | Include provider integrations? Which providers? Default is off.               |
| Git panel scope              | M9          | Bruno 2.0 does not ship one; add in M9 or defer after 1.0.                    |
| i18n scope                   | M11         | Bruno 2.0 is English-only; Chinese/other locales are an apigo extension.      |
| External secret providers    | M10+        | `vars:externalsecrets` is parsed but has no editor/resolver in Bruno OSS.     |

## Important implementation notes (from the research)

- **Two formats, one model.** `.bru` + `bruno.json`, and OpenCollection YAML
  (`opencollection.yml`, `collection.yml`, `folder.yml`, item `.yml`). New Bruno
  collections default to YAML; existing `.bru` collections keep their format.
  `opencollection.yml` wins when both config files exist. There is no
  `.bru.json`.
- **No Git panel in Bruno 2.0.** Only clone/remote-connect are wired; a 1.8k-line
  backend is dormant. Treat the full Git panel as an apigo extension.
- **No request history, command palette, MCP, or non-English i18n** in Bruno
  2.0. Those are extensions/out of scope, not parity rows.
- **No curl/HAR collection import.** Curl works by pasting into the URL bar.
  Bruno imports Postman, OpenAPI, Swagger, Insomnia, WSDL, Bruno ZIP/JSON,
  OpenCollection, Git repos and spec URLs.
- **Strict schemas in Bruno** (`noUnknown(true)`) reject unknown fields. apigo
  preserves unknown blocks/keys instead; this is a deliberate divergence.
- **Watcher cap: 2.5 MiB** per request file. Larger files require an explicit
  open action.
- **Cookies are global**, not per collection. Client certificates are global or
  collection-level; proxy is global or collection-level (no per-request proxy in
  Bruno).
- **Variable precedence:** global env < collection < environment < folder <
  request < OAuth2 credentials < runtime < prompt; `process.env` is separate.
  Only plain `.env` loads; `.env.*` is inert.
- **Assertions** accept `res.*` and legacy `$res.*` on the left.
- **Runner** supports recursive folders, delay, tags and a selected subset; it
  explicitly skips WS/gRPC. No data files, parallel or retries.
- **Scripts** run in QuickJS safe mode by default; NodeVM developer mode is not
  supported by apigo (documented). `lodash`/`fs`/npm are absent.
- **MyGo constraints:** no HTTP/WS/SSE/Git/scripting APIs; no system-proxy API;
  `Secrets` caps at 2,560 bytes; no native print/PDF; `ui.Tree` is not
  virtualized; `ui.Tabs` has no close button. See
  `docs/research/mygo-capabilities.md`.
- The current `cmd/apigo/main.go` is still the MyGo counter demo; replace it in
  M3-01.
- Do not use `mygo.NewDocuments` for the request workbench; see ADR-0008.
- HTTP/1.1 is the parity default; HTTP/2 is opt-in.

## Environment

- macOS 26 (Apple silicon), Go 1.27.1, Node 24.18.0, npm 11.16.0.
- MyGo 0.6.2 pinned; `go tool mygo` works.
- No Bun required (native UI).
- `git` 2.50.1 available; `gh` 2.96.0 available but no remote configured.

## Handoff template for the next session

```markdown
## Snapshot

- Date:
- Branch / commit:
- Milestone:
- Working tree:

## What changed

- M1-01: ...
- Evidence: `go test ./internal/bru`

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
