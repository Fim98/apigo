# Domain docs

How the engineering skills should consume this repo's domain documentation when
exploring the codebase.

## Before exploring, read these

- **`GLOSSARY.md`** at the repo root: the project's vocabulary (collection,
  block, disabled pair, round-trip, divergent, extension, …). It points at the
  authoritative spec for each term.
- **`docs/decisions/`**: read the ADRs that touch the area you're about to work
  in. The index is [docs/decisions/README.md](../decisions/README.md).
- **`docs/research/`**: source-level inventories of Bruno 2.0.0 and MyGo 0.6.2.
  Read the relevant one before changing parser, engine or UI behavior; they
  record what the upstream actually does, which the specs compress.

If `GLOSSARY.md` is missing, proceed silently. `/domain-modeling` creates and
sharpens it lazily when terms actually get resolved.

## File structure

Single-context repo (this one):

```text
/
├── GLOSSARY.md
├── AGENTS.md
├── docs/
│   ├── decisions/                 ← ADRs, one file per decision
│   │   ├── README.md              ← index table; keep it in sync
│   │   ├── TEMPLATE.md
│   │   └── ADR-0001-native-ui.md
│   ├── research/                  ← upstream inventories, cited
│   └── BACKLOG.md                 ← milestone contract
└── internal/
```

This repo does **not** use `docs/adr/`. The ADR directory is `docs/decisions/`,
files are named `ADR-NNNN-<slug>.md`, and numbering continues from the highest
existing number (currently `ADR-0008`).

## Writing an ADR

1. Copy [docs/decisions/TEMPLATE.md](../decisions/TEMPLATE.md); keep its
   heading, status line, date and the Context / Decision / Consequences /
   References sections.
2. Add the row to the index table in
   [docs/decisions/README.md](../decisions/README.md).
3. Link the ADR from the spec it governs and from the backlog task that
   implements it.

ADRs are immutable once accepted: supersede with a new ADR that links back, and
change the old one's status line. A change to the `.bru` format, the scripting
engine, the storage backend, the release signing model or the public CLI
contract needs an ADR; an internal refactor does not.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal,
a hypothesis, a test name), use the term as defined in `GLOSSARY.md`. Don't
drift to synonyms the glossary explicitly avoids: it is _collection_ not
"workspace", _request file_ not "endpoint", _disabled pair_ not "commented
param".

If the concept you need isn't in the glossary yet, that's a signal: either
you're inventing language the project doesn't use (reconsider) or there's a real
gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than
silently overriding:

> _Contradicts ADR-0008 (single-window tabbed workbench), but worth reopening
> because…_

## Specs are not ADRs

A spec describes how something works and changes when the code changes. An ADR
records why a hard choice was made and stays put. Do not put a spec in
`docs/decisions/`, and do not put a decision in a spec's changelog line.
