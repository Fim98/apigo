# ADR-0008: Single-window tabbed workbench

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

Bruno presents one desktop window with a collection sidebar, request tabs and a
response pane. MyGo offers two relevant models:

1. `mygo.NewDocuments[T]` — one window per document with autosave, crash
   recovery, external-change detection, recent documents and file associations.
2. A custom window with an app-owned tab model — one window, many tabs, but the
   app implements dirty state, save, recovery and conflict handling.

A collection is a directory, not a single file, and users expect one window with
many request tabs. MyGo's `ui.Tabs` is a label strip without close buttons,
dirty dots or reordering; `ui.TabsBase` exposes the parts needed to build them.

## Decision

Use a **single main window** with an app-owned tab model across collections
(Shape 2). Build the tab bar with `ui.TabsBase` + `TabsParts.Tab(i)` and a
custom close button/dirty indicator. Do not use `mygo.NewDocuments` for the
request workbench.

Persist open tabs, active tab, pane sizes and per-tab drafts to
`PathUserData/state.json`; write drafts to a recovery directory on a debounce so
a crash does not lose unsaved edits. Use `Window.SetRepresentedFile` and
`SetDocumentEdited` for the active collection to integrate with the OS.

## Consequences

Positive:

- One window matches the workflow and keeps the collection visible while
  switching requests.
- Tabs can span collections, split panes and preserve per-tab editor state.
- Full control over dirty prompts, autosave policy and recovery.
- Avoids one OS window per request.

Negative:

- We reimplement autosave/recovery/conflict handling that `Documents` provides.
- File associations open the app, not a specific request window; the app must
  route to the request after startup.
- More UI state to test and migrate.

Mitigations:

- A small, tested `internal/app/tabs` state machine with generation IDs.
- Recovery snapshots every 2s after a change and on quit; restore on launch.
- Watch the collection root (not individual documents) with `mygo.WatchFiles`.
- Keep the door open for a future `Documents`-based multi-window mode without
  changing the engine.

## References

- MyGo `docs/documents.md`, `docs/ui/tabs.md`, `docs/ui/outline.md`
- [Architecture](../ARCHITECTURE.md)
- Research: `docs/research/mygo-capabilities.md` §1.4 and §9
