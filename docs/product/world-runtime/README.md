# World Runtime Design Documents

This directory contains the durable project design documents for the `nobody` world runtime direction.

The documents are split by purpose so future readers can find the right level of detail without reading one large file.

## Reading Order

1. [`01-core-philosophy.md`](01-core-philosophy.md)  
   Core thesis, project direction, and design boundaries.

2. [`02-architecture.md`](02-architecture.md)  
   Layered architecture, dependency direction, and relationship to the existing narrative harness.

3. [`03-core-model-design.md`](03-core-model-design.md)  
   Concrete world model design: `World`, `Entity`, `MemoryRecord`, `WorldEvent`, `Rule`, and `WorldThread`.

4. [`04-runtime-mechanisms.md`](04-runtime-mechanisms.md)  
   Runtime logic and mechanisms: event pipeline, memory reconciliation, rule evaluation, directors, views, and LLM boundaries.

5. [`05-storage-and-evolution.md`](05-storage-and-evolution.md)  
   Storage direction, early implementation shape, and open design decisions.

## Scope

These are project design documents, not task plans. They describe the long-term conceptual model and architecture for `nobody` as a fictional world runtime.

Engineering analysis, repository reuse audits, risk controls, migration notes, implementation plans, and development records do not belong in this directory. Put those under `docs/engineering/` or `docs/superpowers/plans/` as appropriate.

