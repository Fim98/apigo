# ADR-0002: Clean-room `.bru` compatibility

- Status: Accepted
- Date: 2026-10-21
- Deciders: project owner, implementation agents

## Context

Bruno stores collections in a plain-text format parsed by `@usebruno/lang`
(Ohm grammar, MIT). Users have existing collections in Git; compatibility is
the main reason to consider apigo. Reusing the JavaScript parser would require
a Node runtime and break the single-binary, pure-Go design.

## Decision

Implement a clean-room Go parser and printer that conforms to the `.bru` format
as observed in Bruno 2.0.0 and its fixtures. Do not port the Ohm grammar file or
JavaScript semantics line by line; use the file examples and behavior as the
specification. Every parser/printer change is gated by fixtures and round-trip
tests.

Compatibility goals:

1. Parse every curated fixture without loss.
2. Preserve unknown blocks/fields on save.
3. Produce deterministic, Git-friendly output.
4. Keep request semantics identical where the syntax maps to behavior.

## Consequences

Positive:

- No Node/Electron runtime; parser is fast, fuzzable and testable in Go.
- A single format implementation shared by the app, CLI and converters.
- Explicit compatibility corpus makes regressions visible.

Negative:

- Grammar edge cases (annotations, quoted keys, multiline text, comments,
  encoding, BOM) must be discovered from fixtures and tests.
- Byte-identical output is not guaranteed for files Bruno wrote; the invariant
  is semantic round-trip and stable canonical output.
- New Bruno syntax requires ongoing fixture updates.

Mitigations:

- `internal/bru` AST retains source spans and raw unknown blocks.
- A fuzz target and a fixture corpus run in CI.
- The format spec is documented in [BRU_FORMAT.md](../BRU_FORMAT.md) and updated
  with the parser.

## References

- Bruno `packages/bruno-lang/v2/src/bruToJson.js`, `jsonToBru.js`
- [.bru format](../BRU_FORMAT.md)
