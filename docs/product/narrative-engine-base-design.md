# Narrative Engine Base Design

## Status

Draft seed design for the shared narrative engine base. This document supersedes the older coding-agent assumptions in the imported migration material.

## Goal

Create a product-neutral **Narrative Harness** base that can later power both:

- **Writer Mode**: long-form fiction planning, drafting, continuity checks, and revisions.
- **Tavern Mode**: local-first roleplay, custom worlds, persistent NPC memory, and turn-based adventures.

The base must not choose either product shell first. It should define the durable substrate: world data, story graph, event log, memory records, drafts, and a beat-oriented agent loop.

## Design Principles

1. **Structured state before prompt cleverness**  
   The system should keep narrative facts in explicit data structures so models operate on small, relevant context bundles instead of remembering the whole world.

2. **Local-readable storage first**  
   Use JSON documents, JSONL event streams, and Markdown drafts. Database-backed storage can replace the file store later behind interfaces.

3. **One shared unit: Beat**  
   A Beat is the smallest product-neutral narrative step. Writer Mode can interpret a beat as a scene fragment; Tavern Mode can interpret it as one player turn.

4. **Agent map, not monolithic chat**  
   The first loop uses five narrow agents: director, writer, continuity, memory, and state. Each communicates through typed contracts.

5. **Model tiers are pluggable**  
   Remote Qwen/DeepSeek-style APIs should be easy to add later. The seed keeps llama.cpp runtime as the local/private model path.

## Package Layout

```text
internal/narrative/
  model.go          # product-neutral domain schemas
  validate.go       # lightweight schema validation
  store/
    store.go        # Store interface
    file_store.go   # file-backed implementation
  engine/
    engine.go       # Beat loop orchestration
    agents.go       # agent interfaces and contracts
```

The narrative packages must not import `internal/harness`. Shared logging/storage ideas can be copied or extracted later, but the narrative domain should stay independent.

## Storage Layout

For a workspace root and world id:

```text
<workspace>/narrative/worlds/<world_id>/
  world.json
  story_graph.json
  characters/
    <character_id>.json
  locations/
    <location_id>.json
  events.jsonl
  memories.jsonl
  drafts/
    <draft_id>.md
```

The file store should use atomic JSON writes for documents and append-only JSONL for events/memories. IDs used in file paths must be safe slugs, not arbitrary user text.

## Domain Schemas

### World

Fields:

- `id`
- `title`
- `genre`
- `tone`
- `rules`
- `boundaries`
- `style_guide`

### Character

Fields:

- `id`
- `name`
- `role`
- `traits`
- `goals`
- `secrets`
- `relationships`

### Location

Fields:

- `id`
- `name`
- `description`
- `tags`
- `connected_location_ids`

### StoryGraph and StoryNode

`StoryGraph` contains nodes and the current node id.

`StoryNode` fields:

- `id`
- `type`
- `parent_id`
- `status`
- `goal`
- `character_ids`
- `location_id`
- `hooks`

### NarrativeEvent

Append-only record of what happened.

Fields:

- `id`
- `beat_id`
- `type`
- `summary`
- `participant_ids`
- `effects`
- `source_text`
- `created_at`

### Memory

Durable extracted knowledge.

Fields:

- `id`
- `type`
- `subject`
- `text`
- `tags`
- `source_event_id`
- `importance`
- `created_at`

### Draft

Human-readable generated artifact.

Fields:

- `id`
- `beat_id`
- `title`
- `kind`
- `text`
- `created_at`

Markdown files carry a small JSON front matter block plus a Markdown body so generated prose stays local-readable.

## Beat Engine

Input:

- world id
- current story graph state
- optional user input
- recent event/memory window

Loop:

1. Load world, story graph, current-node characters, current-node location, recent events, and memories.
2. Build a `ContextBundle`.
3. Call `DirectorAgent` for `BeatPlan`.
4. Call `SceneWriterAgent` for `Draft`.
5. Call `ContinuityAgent` for issues.
6. Call `MemoryAgent` for new memories and events.
7. Call `StateAgent` for story graph updates.
8. Persist draft, events, memories, and graph changes.

Output:

- beat id
- draft id
- continuity issues
- persisted event ids
- persisted memory ids
- updated current story node id

## MVP Agent Contracts

Agents are interfaces. The first tests use fake deterministic agents.

- `DirectorAgent.PlanBeat(ctx, ContextBundle) (BeatPlan, error)`
- `SceneWriterAgent.WriteBeat(ctx, ContextBundle, BeatPlan) (Draft, error)`
- `ContinuityAgent.Check(ctx, ContextBundle, Draft) (ContinuityReport, error)`
- `MemoryAgent.Extract(ctx, ContextBundle, Draft) (MemoryDelta, error)`
- `StateAgent.Apply(ctx, ContextBundle, BeatPlan, MemoryDelta) (StateDelta, error)`

## Validation Strategy

- Schema validation tests for required IDs and valid graph references.
- Store round-trip tests for world, characters, locations, graph, events, memories, and drafts.
- JSONL append/read tests for events and memories.
- Deterministic fake-agent test for one full beat.
- Context bundle test to keep agent inputs stable.

## Non-Goals

- No full novel generator.
- No open-world infinite tavern.
- No UI.
- No graph database service.
- No remote Qwen/DeepSeek runtime yet.
- No product-specific Writer/Tavern prompts yet.
- No resurrection of the old coding-agent orchestrator.

## First Implementation Slice

1. Add domain schemas and validation.
2. Add file-backed store with tests.
3. Add engine interfaces and fake-agent loop test.
4. Leave CLI/product modes for a follow-up.

