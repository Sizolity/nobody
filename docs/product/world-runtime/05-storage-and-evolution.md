# World Runtime Storage And Evolution

## Purpose

This document records storage principles, the early runtime shape, and open design decisions for the `nobody` world runtime direction.

It is a design document, not an implementation plan, migration note, or repository audit.

## Storage And Persistence

The storage model should stay local-readable and migration-friendly:

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

- Store durable state as structured JSON documents.
- Store event and memory history as append-friendly JSONL streams.
- Keep IDs safe for file paths.
- Treat `EventLog` as the audit trail for replay and debugging.
- Keep generated prose and rendered views separate from source-of-truth world state.
- Allow future replacement with a database, vector index, graph store, or hybrid storage behind interfaces.

## Early Runtime Shape

The first world runtime shape should establish durable data boundaries rather than attempting the full runtime at once.

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

- Whether `Facts` should be a first-class store, a relation subset, or a rule-query layer.
- Whether `Memory` records should be embedded under entities or stored globally with owner indexes.
- Whether `Thread` belongs inside world state or in a higher-level narrative subsystem.
- Whether initial event effects should be strict typed structs or a small tagged union.
- Whether world views should be pure renderers or allowed to call LLM-backed summarizers.
- Whether `Director` output should be scheduled through a simple priority queue or a richer arbitration layer.

