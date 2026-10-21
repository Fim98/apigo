# Architecture decision records

One file per decision that changes the architecture, a public contract or the
release process. Use [TEMPLATE.md](TEMPLATE.md).

| ADR                                   | Decision                        | Status   |
| ------------------------------------- | ------------------------------- | -------- |
| [0001](ADR-0001-native-ui.md)         | Native UI with MyGo             | Accepted |
| [0002](ADR-0002-bru-compatibility.md) | Clean-room `.bru` compatibility | Accepted |
| [0003](ADR-0003-scripting-engine.md)  | goja for user scripts           | Accepted |
| [0004](ADR-0004-http-transport.md)    | `net/http` request execution    | Accepted |
| [0005](ADR-0005-git-cli.md)           | Git through the `git` CLI       | Accepted |
| [0006](ADR-0006-cli-binary.md)        | Separate CLI binary             | Accepted |
| [0007](ADR-0007-local-storage.md)     | bbolt for local state           | Accepted |
| [0008](ADR-0008-tabbed-workbench.md)  | Single-window tabbed workbench  | Accepted |

Rules:

- ADRs are immutable once accepted; supersede with a new ADR that links back.
- A change to the `.bru` format, the scripting engine, the storage backend, the
  release signing model or the public CLI contract needs an ADR.
- Internal refactors do not.
