# AGENTS.md

Operating manual for agentic contributors. Read this file before changing code,
then read [docs/HANDOFF.md](docs/HANDOFF.md) for the live state and
[docs/ROADMAP.md](docs/ROADMAP.md) for the milestone you are in.

The product is an offline-first, Bruno-compatible API client written in Go with
MyGo's native UI. The parity target is defined in
[docs/CAPABILITY_MATRIX.md](docs/CAPABILITY_MATRIX.md), not by "make it look like
Bruno".

## Read order

1. This file.
2. [docs/HANDOFF.md](docs/HANDOFF.md) — current milestone, next task, known gaps.
3. [docs/ROADMAP.md](docs/ROADMAP.md) — scope and exit criteria of the milestone.
4. [docs/BACKLOG.md](docs/BACKLOG.md) — the task you are claiming.
5. The relevant spec: [.bru format](docs/BRU_FORMAT.md),
   [request engine](docs/REQUEST_ENGINE.md), [scripting](docs/SCRIPTING.md),
   [testing](docs/TESTING.md), [release](docs/RELEASE.md).
6. The relevant ADR in [docs/decisions/](docs/decisions/).
7. `.agents/skills/mygo-maintenance/SKILL.md` before touching `internal/ui` or
   any MyGo API.

## Non-negotiables

1. **No cgo.** `CGO_ENABLED=0` must build and test all packages on macOS, Linux
   and Windows. Do not add a dependency that needs a C toolchain.
2. **No UI imports outside `internal/ui` and `cmd/apigo`.** The parser, engine,
   runner and CLI must be testable without a window. `cmd/apigo-cli` must never
   import `internal/ui`, `github.com/egoist/mygo/ui`, or `cmd/apigo`.
3. **`.bru` compatibility is a contract.** Every parser/printer change needs
   golden fixtures and a semantic round-trip test. Unknown blocks and fields
   must survive a load/save cycle. Do not silently normalize away user data.
4. **No blocking I/O on the UI thread.** Use a goroutine + `context`, then
   publish with `Window.Update`. Never capture a `*ui.Context`, `ui.Element` or
   widget parts in app state or workers.
5. **No secrets in files, logs or fixtures.** Secret variables live in the OS
   keychain; test secrets are obvious dummies. Never log authorization headers,
   cookies, passwords or script output containing them.
6. **Offline by default.** No telemetry, analytics, crash upload, cloud sync or
   background network calls that the user did not initiate.
7. **Every behavior change has a test.** Parser: golden/fuzz. Engine: `httptest`.
   UI: `ui.NewTester`. CLI: exit-code and output contract tests.
8. **Keep the docs true.** Update `CHANGELOG.md`, `docs/HANDOFF.md` and the
   task's status in `docs/BACKLOG.md` in the same change. If reality diverges
   from a spec, fix the spec or record an ADR; do not leave both.
9. **No copied Bruno source.** Bruno is MIT; behavior and file format may be
   studied, but Go code is written clean-room. Fixtures copied from Bruno go in
   `internal/**/testdata/bruno/` with the notice in
   [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
10. **Do not rename the app, identifier or .bru format** without the owner's
    decision. The identifier `com.example.apigo` is a placeholder that must be
    replaced before the first public release.

## Package boundaries

```text
cmd/apigo ──► internal/ui ──► internal/app ──► internal/{collection,httpx,auth,script,runner,gitx,store}
cmd/apigo-cli ──► internal/runner ──► internal/{collection,httpx,auth,script,assert}
internal/collection ──► internal/bru
internal/httpx ──► internal/{bru,collection,vars}
internal/script ──► internal/{bru,collection,httpx}   (data types only)
```

- `internal/bru` is a leaf: no app, UI, network or filesystem globals.
- `internal/app` owns services and orchestration; it must not import `ui`.
- `internal/ui` is an adapter: read state, render, dispatch commands. Business
  logic belongs in `internal/app` or below.
- Define interfaces at the consumer, not next to the implementation.
- No package-level mutable state except immutable tables and `sync.Once`-style
  caches. Pass dependencies explicitly.
- `context.Context` is the first parameter of anything that does I/O or may
  block. Never store a context in a struct.

## MyGo rules that bite

Read the full skill (`.agents/skills/mygo-maintenance/SKILL.md`) before UI work.
The short version:

- A view rebuilds several times per event. Do side effects in handled actions,
  not in unconditional rendering code.
- Elements and parts expire before the next build pass. Store data, widget
  state and `ui.Handle`, never elements.
- Give stateful widgets a stable `c.Key(...)`, and list rows a stable
  `.ItemKey`. Container keys come before children.
- Bind text inputs to persistent model fields. `Changed`/`Submitted` arrive in
  the next build pass; `OnClick`/`OnShortcut` run after the current build.
- Use handles for focus (`Bind`, `FocusBind`, `FocusedValue`) and scoped
  shortcuts (`handle.OnShortcut(c, ...)`), not element references. Menu
  accelerators are window-wide; per-tab commands must be routed in the view or
  through a bound handle.
- Use `ui.Outline`/`OutlineTable`/`List`/`Table` for large data. `ui.Tree`
  builds every item. Use `ui.TabsBase` for closable/dirty/reorderable tabs;
  `ui.Tabs` has no close button.
- Long work runs in goroutines and publishes with `Window.Update`. Cancel stale
  work when tabs close or the collection changes.
- Verify with `ui.NewTester`, `go tool mygo vet .` and
  `CGO_ENABLED=0 go test ./...`. Add extra frames around elements that disappear
  and return.

## Workflow

### Claim a task

1. Pick the first `todo` task in the current milestone whose dependencies are
   `done` from [docs/BACKLOG.md](docs/BACKLOG.md).
2. Change its status to `in_progress` and put your branch/agent name in the
   row in the same edit.
3. Create a branch: `feat/M1-03-request-model`, `fix/...`, `docs/...`.
4. Keep the change focused on one task. Split incidental refactors out.

### Implement

- Write the test first for parser/engine behavior. For UI, build the smallest
  view that demonstrates the state transition, then test it.
- Prefer stdlib. Add a dependency only with an ADR or a note in the task, and
  check `CGO_ENABLED=0` plus the three target OSes.
- Run `make fmt` as you go. Run `make test` before every commit; `make verify`
  before every push (the pre-push hook does this).
- Update `CHANGELOG.md` under `## [Unreleased]` with one line per user-visible
  change. Internal-only changes may omit it.

### Finish

Definition of Done:

- [ ] Acceptance criteria in the task row are met, with evidence (test names).
- [ ] New behavior has tests; `make test` passes.
- [ ] `make verify` passes locally (format, vet, mygo vet, tests, cross-build,
      docs lint).
- [ ] Public types and packages have doc comments; exported behavior is
      documented in the relevant spec.
- [ ] `CHANGELOG.md` updated (user-visible) and `docs/HANDOFF.md` updated
      (always).
- [ ] Task status in `docs/BACKLOG.md` is `done` with a link to the commit/PR.
- [ ] No secrets, build outputs, `.mygo/`, or generated fixtures without notice
      committed.
- [ ] If the change alters a documented decision, add an ADR and link it.

Commit with Conventional Commits, scopes from `commitlint.config.mjs`:

```text
feat(bru): parse disabled pairs and annotations
fix(httpx): preserve duplicate query parameters
docs(plan): add M2 acceptance criteria
test(script): add Bruno pre-request compatibility fixtures
```

Open a PR using `.github/pull_request_template.md`. The PR description must
name the task ID and the verification commands run.

## Handoff protocol

`docs/HANDOFF.md` is the only file that says where the project is _now_. At the
end of every session:

1. Set `Last updated` to the current date.
2. List what changed, with task IDs.
3. List the next three tasks in order.
4. Record blockers and open questions.
5. Record the exact commands run and their result (pass/fail, platform).
6. Note any temporary divergence from the specs.

A fresh agent must be able to continue from that file alone, then verify with
`make verify`.

## Commands

```sh
npm install                 # Husky hooks (once)
make run                    # go run ./cmd/apigo
make dev                    # live-reload native app
make test                   # unit tests, CGO disabled
make test-race              # race detector
make fmt                    # gofumpt + Prettier
make vet                    # go vet + go tool mygo vet .
make verify                 # full local gate (pre-push)
go tool mygo build          # package for this machine
go run ./cmd/apigo-cli version
```

CI runs the same gates on macOS, Ubuntu and Windows; see
`.github/workflows/ci.yml`.

## Go conventions

- Errors: wrap with `%w`, name sentinel errors `ErrX`, expose typed errors only
  when callers need to branch. No `panic` for user input.
- Return early; keep functions under ~80 lines where practical. Table-driven
  tests with `t.Run`.
- Use `slog` for structured logs in the app; never format secrets into log
  messages. The CLI writes user-facing results to stdout and diagnostics to
  stderr.
- JSON: use explicit structs with `json` tags. Do not `map[string]any` for the
  collection model. For arbitrary JSON display, use `encoding/json` into
  `any` only at the view boundary.
- Time: `time.Duration` in APIs, milliseconds only at the UI/CLI boundary.
- File paths: `filepath` everywhere; never assume `/`. Respect
  `mygo.App.Path(mygo.PathUserData)` for app data.
- Tests must not depend on wall-clock timing. Use channels/conditions and
  `httptest`; if a timeout is unavoidable, it is a hang guard, not a delay.

## .bru compatibility rules

- Parse into an AST with source spans; convert to a typed model in
  `internal/collection`. The printer writes canonical formatting.
- Preserve unknown blocks/fields in an `Extra` map and write them back at the
  end, in original order.
- Disabled pairs use the `~` prefix; annotations start with `@`; text blocks
  use `'''` when a line would break the grammar. Round-trip fixtures cover all
  three.
- The semantic round-trip invariant: parse → print → parse yields an equal
  model. The byte-stability invariant: an unmodified file prints identically
  after the first canonical save.
- Never key logic off file names alone: request type comes from `meta.type`,
  protocol block and body block, not the extension.

## Security rules

- TLS verification is on by default. "Disable verification" is per-request,
  visibly marked, and never persisted globally without an explicit setting.
- Scripts get a timeout (default 5s), bounded output, and no filesystem or
  network APIs. Whitelisted modules only. Document the divergence from Node.
- Secret values are masked in the UI, redacted in logs and omitted from
  exported collections. Git integration warns when a `.bru` file contains a
  likely secret.
- Update artifacts must be signed before enabling auto-update; see
  [docs/RELEASE.md](docs/RELEASE.md).

## Decisions and questions

Do not decide these alone; ask the owner:

- Project license and public repository/owner name.
- Bundle identifier and app display name.
- Code-signing identities, notarization credentials and update keys.
- Whether to ship AI-provider integrations (Bruno 2.0 has them) and which
  providers.

Everything else is in scope for an ADR with a clear recommendation.
