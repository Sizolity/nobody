# World Runtime Storage And Evolution

## Purpose

This document records storage principles, the early runtime shape, and open design decisions for the `nobody` world runtime direction.

It is a design document, not an implementation plan, migration note, or repository audit.

## Capability Tiers

This document describes the full conceptual design. Current implementation status:

- Items marked **[T1]** are implemented in the current codebase.
- Items marked **[T2]** are identified as next development targets.
- Items unmarked or marked **[T3]** are aspirational — described for completeness but not yet needed by any product anchor.

See `docs/engineering/world-runtime/implementation-status.md` for detailed implementation status.

## Storage And Persistence

**[T1]** The storage model should stay local-readable and migration-friendly:

```text
world.json
entities/*.json
relations.json
facts.json
events.jsonl
memories.jsonl
threads.json
views or drafts as generated artifacts
```

Principles:

- **[T1]** Store durable state as structured JSON documents.
- **[T1]** Store event and memory history as append-friendly JSONL streams.
- **[T1]** Keep IDs safe for file paths.
- **[T1]** Treat `EventLog` as the audit trail for replay and debugging.
- **[T1]** Keep generated prose and rendered views separate from source-of-truth world state.
- **[T1]** Allow future replacement with a database, vector index, graph store, or hybrid storage behind interfaces.

## Early Runtime Shape

**[T1]** The first world runtime shape should establish durable data boundaries rather than attempting the full runtime at once.

Recommended early target:

```text
World
  Canon
  Clock
  Entities
  Relations
  Facts
  Rules
  EventLog
  EventQueue
  Memory
  Threads

Runtime
  directors
  step loop
  apply one proposed event
  validate with minimal rules
  extract/update memory
  update threads

Views
  debug view
  narrative summary view
  character context view
  GM and RPG views later
```

`Rules`, `Directors`, and `Views` can begin as runtime-side services before they become persisted world fields.

Character self-drive and random directors should come after the event and memory mechanisms are stable.

## Open Design Decisions

- **[T1]** ~~Whether `Facts` should be a first-class store, a relation subset, or a rule-query layer.~~ Decided: first-class store with `Fact` type.
- **[T1]** ~~Whether `Memory` records should be embedded under entities or stored globally with owner indexes.~~ Decided: global store with owner field.
- **[T1]** ~~Whether `Thread` belongs inside world state or in a higher-level narrative subsystem.~~ Decided: inside world state as `[]WorldThread`.
- **[T1]** ~~Whether initial event effects should be strict typed structs or a small tagged union.~~ Decided: typed `Effect` structs with `EffectKind`.
- **[T2]** Whether world views should be pure renderers or allowed to call LLM-backed summarizers.
- **[T2]** Whether `Director` output should be scheduled through a simple priority queue or a richer arbitration layer.

