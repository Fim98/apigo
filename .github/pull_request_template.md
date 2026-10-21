# Pull request

## Task

<!-- Backlog ID, e.g. M1-03. One task per PR. -->

## What changed

<!-- A short description of the behavior, not a file list. -->

## Why

<!-- The user-visible reason or the spec/ADR this implements. -->

## Verification

- [ ] `make verify`
- [ ] `make test-race` (if concurrency changed)
- [ ] Platform checked (macOS / Linux / Windows):
- [ ] Manual flow (if UI):

## Checklist

- [ ] Task status and evidence updated in `docs/BACKLOG.md`
- [ ] `CHANGELOG.md` updated for user-visible changes
- [ ] `docs/HANDOFF.md` updated
- [ ] No secrets, build output, `.mygo/` or unrelated files
- [ ] New dependency? Linked ADR or justification:
- [ ] `.bru` format/parser change? Fixtures + round-trip test:
- [ ] Public API documented; tests named in the PR description
