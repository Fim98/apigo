# Research snapshots

These are source-level inventories, not specifications. They were produced by
reading the local checkouts on 2026-10-21 and are the evidence behind
[CAPABILITY_MATRIX.md](../CAPABILITY_MATRIX.md).

| File                                                     | Source                                                                                        | Lines |
| -------------------------------------------------------- | --------------------------------------------------------------------------------------------- | ----- |
| [bruno-engine-inventory.md](bruno-engine-inventory.md)   | Bruno 2.0.0 `dd0281d` — execution, auth, bodies, scripting, runner, CLI, variables, protocols | 646   |
| [bruno-product-inventory.md](bruno-product-inventory.md) | Bruno 2.0.0 `dd0281d` — collections, formats, UI, import/export, Git, persistence             | 1375  |
| [mygo-capabilities.md](mygo-capabilities.md)             | MyGo 0.6.2 — app/window/menu/document APIs, UI widgets, desktop APIs, packaging, gaps         | 600   |

## How to use them

- Treat them as **leads**, not truth. Verify a symbol/behavior against the pinned
  source before implementing it. Paths and line numbers are from the snapshots
  above.
- Do not copy source code. These documents describe behavior; the Go code is
  clean-room. Fixtures may be vendored under `internal/**/testdata/bruno/` with
  the MIT notice in [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md).
- Where a document says **UNCERTAIN** or **absent**, do not promote the row to
  `done` without a fixture or a source check.
- markdownlint excludes this directory because the tables contain code spans with
  literal pipes. Prettier still formats it.

## Corrections that changed the plan

The first draft of the roadmap assumed features that Bruno 2.0.0 does not ship.
The product inventory corrected them:

- Git panel, request history, command palette, MCP and non-English i18n are
  absent (Git has a dormant backend; history and palette do not exist).
- New collections default to OpenCollection YAML; `.bru` collections keep their
  format.
- There is no curl/HAR collection import; curl is a URL-bar paste.
- Bruno's schemas are strict (`noUnknown(true)`); apigo preserves unknown data as
  a deliberate divergence.
- Cookies are global; proxy and client certificates are global or
  collection-level, not per-request.
- The watcher refuses request files above 2.5 MiB.

If a future Bruno version changes any of these, update the inventory and the
matrix together.
