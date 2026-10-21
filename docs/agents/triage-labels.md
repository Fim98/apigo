# Triage labels

The skills speak in terms of five canonical triage roles. This file maps those
roles to the actual label strings used in this repo's issue tracker, and to the
status vocabulary already used by [docs/BACKLOG.md](../BACKLOG.md).

| Label in mattpocock/skills | Label in our tracker | Backlog status it implies | Meaning                                  |
| -------------------------- | -------------------- | ------------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | — (no row yet)            | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | `blocked`                 | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | `todo`                    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | `in_progress`             | Requires human implementation            |
| `wontfix`                  | `wontfix`            | `dropped`                 | Will not be actioned                     |

When a skill mentions a role (e.g. "apply the AFK-ready triage label"), use the
corresponding label string from this table.

## Repo-specific rules

- A ticket labelled `ready-for-agent` must name its `Backlog:` id and carry
  acceptance criteria that a test can check; otherwise it is `ready-for-human`.
- `divergent` and `done` are backlog outcomes, not triage states. Do not create
  labels for them.
- These labels do not exist yet: there is no remote until M0-11. See
  [issue-tracker.md](./issue-tracker.md).

Edit the "Label in our tracker" column if the tracker vocabulary ever changes.
