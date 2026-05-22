# World Runtime Architecture

## Purpose

This document describes the project architecture for `nobody` as a fictional world runtime.

It focuses on layers, dependency direction, and how the new world runtime framing relates to the existing narrative harness direction.

## Conceptual Architecture

At a high level, the project should be organized around five layers:

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

The dependency direction should stay downward:

```text
Products use views.
Views read world state.
Runtime changes world through events.
World model owns durable concepts.
Storage persists model state and logs.
```

Products should not directly mutate the world. LLM agents should not receive unrestricted state unless they are explicitly acting as narrator, GM, debugger, or maintenance system.

## Layer Responsibilities

### Product Layer

The product layer contains Writer, Tavern, RPG shells, CLIs, UIs, and external tools.

Product code decides workflows and user experience. It should not own the fundamental world schema.

### View Layer

The view layer turns world state into context for humans, products, or agents.

Examples:

- novel prose context;
- RPG scene state;
- character subjective context;
- GM hidden state;
- debug and replay views.

Views should read world state and render projections. They should not directly apply state changes.

### Runtime Layer

The runtime layer owns the step loop:

```text
proposal -> validation -> scheduling -> effects -> memory -> threads -> views
```

It coordinates directors, rules, event application, memory extraction, and thread updates.

### World Model Layer

The world model layer contains durable concepts:

- `World`;
- entities;
- relations;
- facts;
- events;
- memory records;
- threads;
- rules.

This layer should remain product-neutral.

### Storage And Infrastructure Layer

The storage and infrastructure layer persists state and supports execution:

- JSON world documents;
- JSONL event and memory streams;
- indexes;
- model runtime adapters;
- workspace utilities;
- local-readable files before heavier database choices.

## Relationship To Existing Narrative Harness

The existing narrative harness direction remains useful, but this design moves the center of gravity down one layer.

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

The beat engine can remain as one higher-level narrative runtime on top of the world runtime. Writer and Tavern products can both use the lower world runtime while defining their own product-specific views and interaction loops.

