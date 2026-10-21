# ADR-0005: Git through the `git` CLI

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

Bruno's Git panel lets users version collections. Git behavior must match what
users get in a terminal: credential helpers, SSH agents, GPG signing, LFS,
worktrees, hooks and repository configuration. apigo could use a pure-Go Git
library or invoke the system `git`.

Alternatives:

1. Shell out to `git` with plumbing/porcelain output.
2. `go-git` (pure Go) embedded.
3. Do not ship Git integration (unacceptable for parity).

## Decision

Invoke the system `git` binary through a small wrapper (`internal/gitx`),
parsing stable porcelain formats (`--porcelain=v2 -z`, `--numstat -z`,
`for-each-ref`). Detect `git` at startup and show an actionable message when it
is missing. Never bundle Git.

## Consequences

Positive:

- Full compatibility with user credentials, SSH, signing, hooks, LFS and config.
- Small Go dependency footprint; no reimplementation of Git edge cases.
- Errors are the same ones users see in a terminal.

Negative:

- Requires Git to be installed (it is for the target audience; document it).
- Process management, cancellation and output parsing add code.
- Different Git versions may emit slightly different output; pin parsing to
  stable formats and test the minimum version.

Mitigations:

- Parse only documented porcelain output, never human-readable messages.
- Run with `-C <repo>`, a controlled environment and `context` cancellation.
- Integration tests use temporary repositories; CI has Git on all runners.

## References

- Bruno `packages/bruno-app/src/utils/git`, `simple-git`
- [Architecture](../ARCHITECTURE.md)
