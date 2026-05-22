# World Runtime Core Design

This is the entry point for the `nobody` world runtime project design documents.

The design documents are split by document type:

- [`world-runtime/01-core-philosophy.md`](world-runtime/01-core-philosophy.md): core thesis, project direction, and design boundaries.
- [`world-runtime/02-architecture.md`](world-runtime/02-architecture.md): layered architecture and relationship to the existing narrative harness.
- [`world-runtime/03-core-model-design.md`](world-runtime/03-core-model-design.md): concrete model design for `World`, `Entity`, `MemoryRecord`, `WorldEvent`, `Rule`, `WorldThread`, and related model types.
- [`world-runtime/04-runtime-mechanisms.md`](world-runtime/04-runtime-mechanisms.md): event, memory, rule, thread, director, runtime, view, and LLM mechanisms.
- [`world-runtime/05-storage-and-evolution.md`](world-runtime/05-storage-and-evolution.md): storage direction, early implementation shape, and open design decisions.

Start with [`world-runtime/README.md`](world-runtime/README.md) for the recommended reading order.

Engineering analysis, repository reuse audits, risk controls, migration notes, implementation plans, and development records live outside this product design document set. Use `docs/engineering/` for engineering analysis and `docs/superpowers/plans/` for executable implementation plans.

