# ADR-0009: `fsnotify` for collection file watching

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

Opening a collection must reflect edits made outside apigo: another editor, a
`git checkout`, a sync client. Bruno uses chokidar (FSEvents on macOS, inotify on
Linux, ReadDirectoryChangesW on Windows). The watcher feeds the UI's external
change banner and the conflict prompt on save (M3-15).

The alternatives:

1. `github.com/fsnotify/fsnotify` — a thin, widely used wrapper over the three
   OS notification APIs. Pure Go, no cgo, ~8k LOC including tests.
2. Polling with `os.ReadDir`/`Stat` every few hundred milliseconds. Stdlib only,
   but it burns CPU on large collections, misses short-lived writes, and gives no
   event kind without a full diff.
3. Writing our own per-OS watcher — FSEvents, inotify and
   `ReadDirectoryChangesW` bindings, plus three sets of tests. Not worth owning.

`AGENTS.md` allows a dependency with an ADR and a `CGO_ENABLED=0` check on all
three target OSes.

## Decision

Use `fsnotify` v1.9.0 for file watching, confined to
`internal/collection/watcher.go`. The public surface is apigo's own:

- `collection.NewWatcher(root, Options)` — options carry the debounce window and
  the ignore rules from the loader.
- `(*Watcher).Events() <-chan Event` — `Event{Path, RelPath, Op}` where `Op` is
  `OpCreate`, `OpUpdate` or `OpRemove`, already debounced and coalesced.
- `(*Watcher).Errors() <-chan error` — watcher-level failures (inotify limit,
  directory removed).
- `(*Watcher).Run(ctx)` and `(*Watcher).Close()` — both stop cleanly and close
  the channels exactly once.

fsnotify is not imported anywhere else; the app talks to `Watcher` only, so the
backend can be replaced without touching callers.

## Consequences

Positive:

- Events arrive within milliseconds instead of on a poll interval, and the event
  kind is known without diffing the tree.
- One dependency, MIT-style BSD-3 license, no cgo, used by many Go projects.
- Tests can drive a real temp directory and assert create/update/remove, which is
  the behavior the UI depends on.

Negative:

- A build dependency outside the standard library; it must keep working on
  macOS, Linux and Windows, and cross-builds now cover it.
- fsnotify does not watch new subdirectories automatically; the watcher adds
  recursive watches when a directory is created, and drops them on removal.
- Some editors write via rename, producing `Chmod`/`Rename` noise; the debounce
  layer coalesces bursts and maps rename-into-place to `OpUpdate`.

Mitigations:

- `make verify` cross-compiles for all three OSes, so a binding regression fails
  the build before release.
- The watcher is tested against a temp tree, not against OS timing: the debounce
  window is configurable and set low in tests.
- Oversized files stay out of the watcher's parse path: the consumer checks
  `collection.MaxFileSize` and surfaces `ErrTooLarge` instead.
