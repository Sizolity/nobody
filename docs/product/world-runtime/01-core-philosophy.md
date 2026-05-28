# World Runtime Core Philosophy

## Status

Project design reference for the long-term direction of `nobody`.

This document records the core thesis and conceptual boundaries. It explains why the project should be centered on `World` instead of story text, game UI, or individual characters.

## Capability Tiers

This document describes the full conceptual design. Current implementation status:

- Items marked **[T1]** are implemented in the current codebase.
- Items marked **[T2]** are identified as next development targets.
- Items unmarked or marked **[T3]** are aspirational — described for completeness but not yet needed by any product anchor.

See `docs/engineering/world-runtime/implementation-status.md` for detailed implementation status.

## Project Direction

**[T1]** `nobody` should evolve from a narrow narrative harness into a reusable **fictional world runtime**.

The project should provide the lower-level engine for systems such as:

- **[T1]** novel generation and continuation;
- **[T2]** RPG and open-world roleplay;
- **[T2]** persistent character simulation;
- **[T3]** self-driven fictional worlds;
- **[T2]** externally driven scenario execution;
- **[T1]** random incident generation;
- **[T2]** rehearsal, deduction, and counterfactual exploration;
- **[T3]** structured state optimization for long-running stories.

**[T1]** The recommended center is `World`, with a simulation-style runtime. `World` owns durable state. Runtime systems propose, validate, apply, remember, and render changes.

## Core Thesis

**[T1]** `World` is the stable root abstraction.

**[T1]** `nobody` should not make `Story`, `Game`, or `Character` the root of the system:

- `Story` is a rendered view of world state and event history.
- `Game` is a world plus interaction rules, progress structure, and product UI.
- `Character` is an important entity type inside the world, not the only root object.

**[T1]** The practical flow should be:

```text
World -> Events -> State Changes -> Narrative View -> Story Text
```

That means story text is a projection, not the bottom-level data model.

**[T1]** The core engine should preserve this separation:

```text
World is state.
Event is change.
Rule is constraint and resolution logic.
Memory is continuity and subjective belief.
Thread is long-running dramatic or systemic pressure.
Director is a source of event proposals.
Runtime is the execution loop.
View is a projection for humans, agents, or products.
```

**[T1]** This lets the same lower layer support fiction writing, RPG roleplay, world simulation, debug replay, and product-specific interfaces.

## Design Boundaries

**[T1]** The world runtime should treat generated prose, UI state, and product-specific game loops as projections of the underlying world, not as the world itself.

Core boundaries:

- **[T1]** Products use the runtime; they do not define the core data model.
- **[T1]** Views render world state; they do not become the source of truth.
- **[T1]** Runtime changes world state only through events.
- **[T1]** Rules validate and resolve changes before they apply.
- **[T1]** Memory records can represent subjective belief, not only objective facts.
- **[T2]** Characters should act on their own visible context, not omniscient world truth.

## Important Non-Goals

**[T1]** The core runtime should avoid these early traps:

- Do not make `Story` the core state model.
- Do not let character agents access omniscient world truth by default.
- Do not split characters, items, and locations into unrelated systems too early.
- Do not make rules a complex DSL before the Go interface proves the model.
- Do not make memory only a text summary; it must model owner, confidence, truth status, and visibility.
- Do not preserve compatibility with old coding-agent behavior that does not serve the world runtime.

