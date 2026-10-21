# ADR-0001: Native UI with MyGo

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

The current repository is a MyGo native-UI app: windows are drawn by MyGo on
the GPU, views are Go functions, and there is no webview or frontend build. The
target is a Bruno-like API client with dense editors, tables, trees, tabs,
split panes and high-frequency keyboard input. Bruno itself is Electron + React
and uses CodeMirror, which is not available in a native-UI window.

Alternatives:

1. Keep native UI (current project).
2. Switch to a MyGo webview + TypeScript/React frontend, reusing CodeMirror and
   web component libraries.
3. Build a separate web frontend served by the Go backend.

## Decision

Build the desktop client with MyGo's native UI (`ui`, `ui/editor`, `ui/table`,
`ui/tree`, `ui/outline`, `ui/split`, `ui/tabs`, `ui/tabsbase`).

## Consequences

Positive:

- Single Go binary, no Chromium/Node runtime, fast startup and low memory.
- `CGO_ENABLED=0`, cross-compiles from any host; no webview dependency on Linux
  (`linux.webview = false`).
- Keyboard, focus, accessibility and native menus are first-class.
- The UI and engine share Go types, eliminating an IPC schema.

Negative:

- No CodeMirror/Monaco ecosystem; syntax highlighting and editor behavior come
  from `ui/editor` (which is Zed-like and sufficient, but not identical).
- More UI code for complex controls (key-value grids, JSON tree, tabs).
- Less reusable web styling; themes are MyGo themes.
- Some browser-only rendering (HTML preview) must be a webview pane or a
  text/plain fallback.

Mitigations:

- Use MyGo's built-in widgets rather than reimplementing: `editor` for bodies,
  scripts and diffs; `table`/`outline` for virtualized lists; `split` for panes.
- Keep all business logic in `internal/app` so a future web frontend is an
  adapter, not a rewrite.
- Budget performance in M3/M11; use `ui.Tester` for regression tests.

## References

- MyGo UI docs, `ui/doc.go`, `docs/ui/code-editor.md`, `docs/ui/table.md`
- [Architecture](../ARCHITECTURE.md)
