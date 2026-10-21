# ADR-0006: Separate CLI binary

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

Bruno ships a desktop app and a separate `bru` CLI (npm, now also Docker). The
CLI is used in CI to run collections and produce reports. apigo needs the same
capability without the UI, and the CLI must be installable on servers without a
display or GTK.

Alternatives:

1. One binary with `apigo run ...` subcommands and a GUI when no subcommand.
2. Two binaries: `apigo` (GUI) and `apigo-cli` (terminal).
3. A separate module/repository.

## Decision

Ship two commands from one module:

- `cmd/apigo` — the native desktop app.
- `cmd/apigo-cli` — the runner and converters, with no UI imports.

They share `internal/runner` and all engine packages. The CLI is a static
`CGO_ENABLED=0` binary; the app is packaged by MyGo. Both are built from the
same tag and released together.

## Consequences

Positive:

- The CLI stays small and runs headless on Linux servers/containers.
- No accidental UI dependency in CI code paths (enforced by an import test).
- Shared behavior is guaranteed by shared packages, not duplication.

Negative:

- Two binaries to build, sign and distribute.
- Users need to know which one to install; naming must be clear.
- Shared code must not grow platform UI assumptions.

Mitigations:

- A CI check fails if `cmd/apigo-cli` imports `internal/ui`, `mygo/ui` or
  `cmd/apigo`.
- Release assets name both binaries and document their use.
- The CLI's `version` is injected with ldflags from `mygo.json`.

## References

- [Release engineering](../RELEASE.md)
- `cmd/apigo-cli/main.go`
