# ADR-0003: goja for user scripts

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

Bruno scripts are JavaScript with a Node-ish runtime: pre-request and
post-response scripts, `bru` APIs, `req`/`res`, `expect`, `test`, and modules
such as chai, moment, crypto-js, uuid and ajv. apigo is pure Go and
cannot embed Node without a large runtime, cgo or a subprocess.

Alternatives:

1. `goja` (pure Go ECMAScript engine).
2. QuickJS via WebAssembly (`wazero`) or cgo.
3. Spawn a bundled Node binary as a helper (violates single-binary/no-Node).
4. Do not support scripts (unacceptable for parity).

## Decision

Use `goja` for script execution, with a compatibility layer implemented in Go
and small embedded JS shims:

- `bru`, `req`, `res`, `console`, `test`, `expect` injected per run.
- Async/await supported by draining the promise job queue; scripts have a
  timeout and bounded output.
- A whitelist of modules (`chai`, `moment`, `crypto-js`, `uuid`, `ajv`,
  `jsonwebtoken`, `tv4`, `axios`, `path`) with documented subsets; unsupported
  `require` calls fail with an actionable error. Node-only modules such as
  `lodash` and `fs` are not available.
- No filesystem, network, process or native addon APIs.

## Consequences

Positive:

- Pure Go, no cgo, fast startup, one runtime per request execution.
- Interrupt-based timeouts and deterministic isolation.
- Easy to expose Go values (`req`, `res`, variables) with typed wrappers.

Negative:

- Not Node: scripts relying on Node internals, native modules, worker threads or
  subtle V8 behavior will diverge.
- goja's ES support and performance differ from V8; very heavy scripts are
  slower.
- Module shims need maintenance and compatibility tests.

Mitigations:

- Maintain a conformance corpus of real Bruno scripts and publish a divergence
  list in [SCRIPTING.md](../SCRIPTING.md).
- Fail closed with a clear message rather than silently returning wrong values.
- Revisit a second engine only with a new ADR and measured need.

## References

- [Scripting specification](../SCRIPTING.md)
- Bruno `packages/bruno-js`, `packages/bruno-requests/src/scripting`
