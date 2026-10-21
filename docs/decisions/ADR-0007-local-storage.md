# ADR-0007: bbolt for local state

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

The app needs request history, persistent cookie jars, recent collections and
UI state. Collections must stay as `.bru` files; these stores are machine-local
and not portable. Bruno uses SQLite for history and cookies.

Alternatives:

1. JSON/JSONL files.
2. bbolt (pure-Go B+tree key/value store).
3. SQLite via a pure-Go driver or MyGo's prebuilt SQLite plugin.
4. Do not persist history/cookies.

## Decision

Use bbolt for history and cookies, JSON for small settings/state, and the OS
keychain for secrets. No SQL database in 1.0.

## Consequences

Positive:

- Pure Go, no cgo and no runtime library download; small and fast.
- Single-file databases with transactions and crash safety.
- Easy backup/delete; no schema migrations for simple key-value data.

Negative:

- No SQL queries; search is implemented in Go over indexes/buckets.
- History retention and search need explicit bucket design.
- If mock-server logs or analytics need SQL later, a second store may be added.

Mitigations:

- Design buckets with stable keys: `history/<collectionID>/<timestamp>-<id>`,
  `cookies/<collectionID>/<domain>`.
- Cap history entries and vacuum/prune on startup.
- Revisit SQLite with a new ADR only when a real query need appears.

## References

- [Architecture](../ARCHITECTURE.md#8-storage-layout)
