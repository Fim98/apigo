# Third-party notices

apigo is an independent implementation. It is not affiliated with, endorsed by,
or sponsored by the Bruno project. "Bruno" is a trademark of its respective
owner.

## Bruno

Parts of the specification, test fixtures and research in this repository are
derived from the open-source Bruno project:

- Project: https://github.com/usebruno/bruno
- Version studied: 2.0.0, commit `dd0281dad107247a4653ff514feb069ac42e7ff8`
  (2026-10-19)
- License: MIT

Bruno source code is **not** copied into apigo. The Go implementation is
clean-room; only file-format behavior and test data are used for compatibility.
When fixtures are vendored under `internal/**/testdata/bruno/` or
`internal/**/testdata/bruno-yaml/`, they retain their original license notice
below.

```text
MIT License

Copyright (c) 2022 Anoop M D, Anusree P S and Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## MyGo

- Project: https://github.com/egoist/mygo
- Version: 0.6.2
- License: MIT (see the module's LICENSE file)

MyGo is used as a Go module and pinned as a `tool` in `go.mod`.

## Matt Pocock engineering skills

Agent skill definitions vendored into `.agents/skills/`:

- Project: https://github.com/mattpocock/skills
- Version: 1.3.1, commit `5beb35b040e3de97e54fa9e386d7e42d15c295c2`
  (2026-10-19)
- Path vendored: `skills/engineering/` (20 skills, unmodified copies)
- License: MIT, Copyright (c) 2026 Matt Pocock

These are instructions for coding agents, not compiled code. They are not part
of the app; removing the directory removes the dependency. The per-repo
configuration they read lives in `docs/agents/` and is this project's own work.

## Go dependencies

Runtime dependencies and their licenses:

- `github.com/fsnotify/fsnotify` v1.9.0 — BSD-3-Clause, pure Go (ADR-0009).
- `gopkg.in/yaml.v3` v3.0.1 — Apache-2.0 with MIT-licensed portions, pure Go
  (ADR-0010).

Licenses ship inside the modules. Other module licenses are recorded by
`go list -m -json all` and are included in the SBOM once the release pipeline adds
it (M12). New dependencies must pass a license review (see
[docs/SECURITY.md](docs/SECURITY.md)).
