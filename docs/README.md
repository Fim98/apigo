# Documentation index

Start with [AGENTS.md](../AGENTS.md) and [HANDOFF.md](HANDOFF.md).

| Document                                  | What it answers                                                        |
| ----------------------------------------- | ---------------------------------------------------------------------- |
| [Roadmap](ROADMAP.md)                     | What are we building, in what order, and when is a milestone done?     |
| [Backlog](BACKLOG.md)                     | What can I claim right now? What are its dependencies?                 |
| [Capability matrix](CAPABILITY_MATRIX.md) | Does apigo match Bruno for this behavior?                              |
| [Architecture](ARCHITECTURE.md)           | How are packages, data flow, concurrency and storage organized?        |
| [.bru format](BRU_FORMAT.md)              | What is the file grammar, serialization order and round-trip contract? |
| [Request engine](REQUEST_ENGINE.md)       | How is a request built, sent, authenticated and measured?              |
| [Scripting](SCRIPTING.md)                 | What JS runs, with which globals/modules, and what diverges?           |
| [Testing](TESTING.md)                     | How do we prove parity and prevent regressions?                        |
| [Release](RELEASE.md)                     | How do we version, build, sign and publish?                            |
| [Security](SECURITY.md)                   | What is the threat model and the secret/script policy?                 |
| [Decisions](decisions/)                   | Why did we choose this architecture?                                   |
| [Research](research/)                     | Source-level inventories of Bruno 2.0.0 and MyGo 0.6.2                 |
| [Handoff](HANDOFF.md)                     | Where are we now, what is next, what is blocked?                       |

## Reading paths

- **Implementing M1:** AGENTS → HANDOFF → ROADMAP M1 → BACKLOG M1 → BRU_FORMAT →
  TESTING → research/bruno-product-inventory.md.
- **Implementing the UI:** AGENTS → research/mygo-capabilities.md →
  ARCHITECTURE §7 → ADR-0008 → MyGo skill.
- **Implementing the engine:** REQUEST_ENGINE → research/bruno-engine-inventory.md
  → TESTING §3.
- **Releasing:** RELEASE → CHANGELOG → scripts/check-release.sh → CI workflow.
