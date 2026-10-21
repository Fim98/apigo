# ADR-0010: `gopkg.in/yaml.v3` for OpenCollection

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

Bruno 2.x adds OpenCollection YAML as the default format for new collections
(`opencollection.yml`, `folder.yml`, item `.yml`,
`environments/*.yml`). M1-13 reads it, M1-14 writes it, M8 converts between the
two formats. See [docs/OPENCOLLECTION.md](../OPENCOLLECTION.md).

The format needs three properties a naive `map[string]any` decoder cannot give:

1. **Unknown keys survive.** A file written by a newer Bruno must not lose
   fields apigo does not model yet (BRU_FORMAT's "unknown data" contract).
2. **Key order is stable.** Canonical output should keep the author's order
   instead of sorting keys, so diffs stay readable.
3. **Source positions for errors.** A malformed collection must point at a line,
   the same way `.bru` parse errors do.

The alternatives:

1. `gopkg.in/yaml.v3` — `yaml.Node` exposes the document tree with comments,
   styles, order and line numbers. Pure Go, no cgo, Apache-2.0/MIT.
2. `sigs.k8s.io/yaml` — converts YAML to JSON and loses order and positions.
3. A hand-written YAML parser — not a good use of this project's time.
4. `github.com/goccy/go-yaml` — capable, but larger and newer than needed.

## Decision

Use `gopkg.in/yaml.v3` v3.0.1, confined to `internal/opencollection`, a leaf
package that only knows about YAML documents:

- `opencollection.Parse(src)` returns a `*Document` wrapping the `yaml.Node`
  tree, with ordered `Get`/`Each`, scalar/sequence accessors and line numbers.
- `Document.Root()` and the `Node` accessors let `internal/collection` map the
  document onto the model without importing a YAML library itself.
- `Document.Bytes()` re-marshals the preserved tree, which is what the M1-14
  writer will patch before saving.

`internal/collection` imports `internal/opencollection`, never `yaml.v3`
directly, so the dependency stays behind one seam.

## Consequences

Positive:

- Unknown keys, comments and ordering survive a read, which is the whole point
  of `yaml.Node`.
- Errors carry the line number yaml.v3 reports, so a broken item points at the
  offending line. yaml.v3 does not report a column for every error kind, so
  `Error.Column` stays zero when the decoder is silent.
- One small dependency, cgo-free, cross-builds for macOS, Linux and Windows.
- The writer in M1-14 can patch nodes instead of re-serializing the whole
  document, so fields apigo does not model stay byte-identical.

Negative:

- `yaml.Node` is verbose: every read is a tree walk, and the mapping code has to
  handle missing nodes.
- Anchors are kept as `AliasNode` and followed by the accessors, but **merge keys
  (`<<: *defaults`) are not expanded** by yaml.v3 when decoding to a `Node`, so a
  file relying on them reads as if the merged keys were absent. Bruno never emits
  them; documented in OPENCOLLECTION.md §8.6. Multi-document YAML is also out of
  scope: only the first document is read.

Mitigations:

- `internal/opencollection` has its own tests for ordering, unknown keys and
  positions, so the mapping layer can assume a sane API.
- The writer (M1-14) is required to round-trip every vendored fixture, which
  keeps anchor/order regressions visible.
