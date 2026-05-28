# World Runtime Architecture

## Purpose

This document describes the project architecture for `nobody` as a fictional world runtime.

It focuses on layers, dependency direction, and how the new world runtime framing relates to the existing narrative harness direction.

## Capability Tiers

This document describes the full conceptual design. Current implementation status:

- Items marked **[T1]** are implemented in the current codebase.
- Items marked **[T2]** are identified as next development targets.
- Items unmarked or marked **[T3]** are aspirational — described for completeness but not yet needed by any product anchor.

See `docs/engineering/world-runtime/implementation-status.md` for detailed implementation status.

## Conceptual Architecture

**[T1]** At a high level, the project should be organized around five layers:

```text
Product Layer
  Writer, Tavern, RPG shell, CLI, UI, tools

View Layer
  Novel view, RPG view, character context, GM/debug views

Runtime Layer
  Directors, scheduler, rule evaluation, event application

World Model Layer
  World, entities, relations, facts, memory, threads, event log

Storage And Infrastructure Layer
  JSON documents, JSONL streams, indexes, model runtime, workspace utilities
```

**[T1]** The dependency direction should stay downward:

```text
Products use views.
Views read world state.
Runtime changes world through events.
World model owns durable concepts.
Storage persists model state and logs.
```

**[T1]** Products should not directly mutate the world. **[T2]** LLM agents should not receive unrestricted state unless they are explicitly acting as narrator, GM, debugger, or maintenance system.

## Layer Responsibilities

### Product Layer

**[T1]** The product layer contains Writer, Tavern, RPG shells, CLIs, UIs, and external tools.

**[T1]** Product code decides workflows and user experience. It should not own the fundamental world schema.

### View Layer

**[T1]** The view layer turns world state into context for humans, products, or agents.

Examples:

- **[T1]** novel prose context;
- **[T2]** RPG scene state;
- **[T2]** character subjective context;
- **[T2]** GM hidden state;
- **[T1]** debug and replay views.

**[T1]** Views should read world state and render projections. They should not directly apply state changes.

### Runtime Layer

**[T1]** The runtime layer owns the step loop:

```text
proposal -> validation -> scheduling -> effects -> memory -> threads -> views
```

**[T1]** It coordinates directors, rules, event application, memory extraction, and thread updates.

### World Model Layer

**[T1]** The world model layer contains durable concepts:

- `World`;
- entities;
- relations;
- facts;
- events;
- memory records;
- threads;
- rules.

**[T1]** This layer should remain product-neutral.

### Storage And Infrastructure Layer

**[T1]** The storage and infrastructure layer persists state and supports execution:

- **[T1]** JSON world documents;
- **[T1]** JSONL event and memory streams;
- **[T2]** indexes;
- **[T1]** model runtime adapters;
- **[T1]** workspace utilities;
- **[T1]** local-readable files before heavier database choices.

## Relationship To Existing Narrative Harness

**[T1]** The existing narrative harness direction remains useful, but this design moves the center of gravity down one layer.

Previous framing:

```text
Narrative Harness
  world bible
  story graph
  event log
  memory index
  beat engine
  agent map
```

Updated framing:

```text
World Runtime
  world state
  entity graph
  fact and relation model
  event mechanism
  memory mechanism
  thread mechanism
  rule mechanism
  directors
  views
```

**[T1]** The beat engine can remain as one higher-level narrative runtime on top of the world runtime. Writer and Tavern products can both use the lower world runtime while defining their own product-specific views and interaction loops.

