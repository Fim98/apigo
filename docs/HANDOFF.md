# Handoff

This is the **only** file that describes where the project is right now. Update
it at the end of every session, then verify with `make verify`.

## Snapshot

- Date: 2026-10-21 (Asia/Shanghai)
- Branch: `main`
- Commit: `f199eb4` (foundation)
- Version: `0.1.0` (foundation)
- Milestone: **M0 — Foundation** (nearly complete)
- Next milestone: **M1 — Language & collection store**
- Working tree at handoff: the initial commit is being prepared; see "Verification".

## What is done

### Documentation and decisions

- [x] `AGENTS.md` — operating manual, invariants, workflow, DoD, handoff protocol.
- [x] `docs/ROADMAP.md` — M0–M12, dependencies, estimates, risks, open decisions.
- [x] `docs/BACKLOG.md` — claimable tasks with dependencies and acceptance criteria.
- [x] `docs/CAPABILITY_MATRIX.md` — 180-row parity contract.
- [x] `docs/ARCHITECTURE.md` — package graph, flows, storage, concurrency, budgets.
- [x] `docs/BRU_FORMAT.md` — grammar, blocks, environments, canonical printing.
- [x] `docs/REQUEST_ENGINE.md` — variables, auth, HTTP, bodies, assertions, exit codes.
- [x] `docs/SCRIPTING.md` — goja sandbox, `bru`/`req`/`res`, modules, divergences.
- [x] `docs/TESTING.md` — test pyramid, fixtures, conformance, UI tester.
- [x] `docs/RELEASE.md` — versioning, signing, CI, updates, checklist.
- [x] `docs/SECURITY.md` — threat model, secrets, sandbox, supply chain.
- [x] ADRs 0001–0008 and template.
- [x] Research: `docs/research/bruno-engine-inventory.md`,
      `docs/research/mygo-capabilities.md`.
- [ ] Research: `docs/research/bruno-product-inventory.md` (agent still running
      at handoff; if absent, the matrix's product rows are based on source sampling
      and must be verified in M1/M3).

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

Record the results here when you run them. At the time of writing:

| Command                          | Result                                                   |
| -------------------------------- | -------------------------------------------------------- |
| `CGO_ENABLED=0 go test ./...`    | pass (`cmd/apigo`, `cmd/apigo-cli`, `internal/version`)  |
| `go vet ./...`                   | pass                                                     |
| `go tool mygo vet .`             | pass                                                     |
| `go run ./cmd/apigo-cli version` | `apigo dev`                                              |
| `npm install`                    | pass (161 packages; Husky installed after `git init`)    |
| `make verify`                    | **not yet run after the docs were added** — run it first |
| `go tool mygo build`             | **not yet run** — M0-12                                  |
| GitHub Actions                   | not run (no remote yet)                                  |
| Tag dry-run                      | not run (no remote yet)                                  |

Known lint risks: Prettier may reformat the large Markdown tables; run
`make fmt` and commit the formatting. markdownlint excludes `CHANGELOG.md` but
not research docs.

## Next three tasks

1. **M0-12 — validate a real `mygo build` locally.**
   Run `go tool mygo build`, confirm `build/darwin-arm64/apigo.app` and the DMG,
   then update this file with the artifact names. If packaging fails, fix it
   before M1; the release workflow depends on the same command.
2. **M0-08 — finish repository templates/metadata.**
   Confirm the issue templates parse, remove the placeholder `OWNER/apigo` URL
   in `.github/ISSUE_TEMPLATE/config.yml` (or point it at the real remote), and
   mark M0-08 done in the backlog.
3. **M1-01 — `.bru` tokenizer.**
   Create `internal/bru`, write the token model (name, value, punctuation,
   newline, text, EOF) with byte spans and line/column tracking, and table tests
   for every token. Then M1-02 (block parser). Do not start the HTTP engine until
   the parser round-trips the corpus.

## Blockers and open questions

| Item                           | Status      | Owner decision needed                                                             |
| ------------------------------ | ----------- | --------------------------------------------------------------------------------- |
| Project license                | Blocked     | Which license? `package.json` currently says `UNLICENSED`; no `LICENSE` file.     |
| Bundle identifier / app name   | Blocked     | Replace `com.example.apigo` before 0.2.0 or first public release.                 |
| GitHub repository              | Blocked     | Create `OWNER/apigo`, push `main`, set branch protection; then update badge/URLs. |
| Signing/notarization secrets   | Later (M12) | Apple Developer ID, Windows certificate, update key.                              |
| AI features                    | M10         | Include provider integrations? Which providers? Default is off.                   |
| OpenCollection format          | Planned     | M1 read-only, M8 write; verify the actual Bruno 2.0 default for new collections.  |
| `bruno.json` vs OpenCollection | Planned     | Model format detection; no decision needed yet.                                   |
| System proxy                   | Planned     | MyGo has no API; implement per-OS detection in M4.                                |
| Product inventory              | In progress | The agent's product inventory may land after this handoff; read it before M3.     |

## Important implementation notes

- The current `cmd/apigo/main.go` is still the MyGo counter demo. It is a smoke
  test for the UI toolkit, not product code. Replace it in M3-01.
- Do not use `mygo.NewDocuments` for the request workbench; see ADR-0008.
- Use `ui.Outline`/`OutlineTable` for collections/JSON, not `ui.Tree`.
- Use `ui.TabsBase` for request tabs; `ui.Tabs` has no close/dirty support.
- MyGo's `Secrets` cap is 2,560 bytes; large client certificates need the
  encrypted-blob scheme in `docs/ARCHITECTURE.md` §8.
- MyGo provides no HTTP/WebSocket/SSE/Git/scripting APIs; those are app code.
- HTTP/1.1 is the parity default; HTTP/2 is opt-in.
- The runner has no data-file/parallel/retry features in Bruno 2.0.0; do not add
  them as "parity".
- MCP is absent from Bruno 2.0.0; do not plan it.
- `docs/SCRIPTING.md` is the authority for modules; `lodash`/`fs` are absent.

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
