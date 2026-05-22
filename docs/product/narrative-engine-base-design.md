# Narrative Engine Base Design

## Status

Draft seed design for the shared narrative engine base. This document supersedes the older coding-agent assumptions in the imported migration material.

`nobody` is treated as the shared core repository during this phase. Writer and Tavern are downstream products that should become separate repositories later; this design defines the core they will depend on.

## Goal

Create a product-neutral **Narrative Harness** base that can later power both:

- **Writer Mode**: long-form fiction planning, drafting, continuity checks, and revisions.
- **Tavern Mode**: local-first roleplay, custom worlds, persistent NPC memory, and turn-based adventures.

The base must not choose either product first. It should define the durable substrate: world data, story graph, event log, memory records, drafts, and a beat-oriented agent loop. Product-specific workflows, UX, prompts, and release packaging belong in future Writer and Tavern repositories.

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

The narrative packages must not import product-shell packages. Shared workspace logging and run metadata live under `internal/workspace`; the narrative domain should stay independent from those operational concerns unless an interface is introduced.

If old `nobody` packages are reused, they should either be reshaped into product-neutral support packages or copied into the narrative core with old coding-agent assumptions removed. The shared core should not preserve compatibility with the old product loop.

Future product repositories should import the shared narrative API through thin public facade packages:

```text
pkg/narrative/          # public aliases for domain schemas
pkg/narrative/agentio/  # strict model JSON output decoding helpers
pkg/narrative/bootstrap/ # product-neutral world initialization helpers
pkg/narrative/contract/ # public agent output validation helpers
pkg/narrative/store/    # public Store contract and file store constructor
pkg/narrative/engine/   # public beat engine and agent contracts
pkg/narrative/enginetest/ # deterministic agent doubles for product tests
pkg/narrative/id/       # public safe identifier validation
pkg/narrative/memstore/ # public in-memory Store for tests and short-lived workflows
pkg/narrative/prompt/   # stable context bundle JSON/prompt rendering
pkg/narrative/snapshot/ # product UI/CLI world snapshot loader
pkg/narrative/storetest/ # Store implementation conformance tests
pkg/config/             # public shared model/runtime config loader
pkg/inference/          # public inference contracts and event constants
pkg/inference/llamacpp/ # public llama.cpp runtime constructor
pkg/workspace/          # public event log and run metadata helpers
pkg/skills/             # public shared capability contracts
```

`internal/narrative` remains the implementation home for now so this repository can keep evolving the core without exposing every helper as API.

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

## Phase 1 Hardening

The first hardening pass makes the base stricter before any product-specific mode is added:

- `World`, `Character`, `Location`, `StoryGraph`, `StoryNode`, `Draft`, `NarrativeEvent`, and `Memory` validate required fields.
- The file store rejects invalid characters, locations, events, memories, and drafts before writing them, and revalidates local-readable files after reading them.
- The beat engine validates agent outputs before persistence, including beat plan target node references, draft/event beat IDs, memory source event references, and state graph consistency.
- `ContextBundle` uses stable JSON field names so future prompt input tests can lock down model-facing shape.
- Corrupt JSONL files return explicit read errors instead of being silently ignored.

## Repository Transition Plan

This repository is no longer treated as the long-term home of a coding-agent product. It is the staging ground for a shared narrative foundation.

Near-term work:

1. Finish hardening `internal/narrative` as a product-neutral core.
2. Audit old `nobody` packages and classify them as keep, reshape, copy, or delete.
3. Reuse only infrastructure that supports the shared core: model runtime, llama.cpp lifecycle, prompt loading, workspace layout, JSONL/event storage, and useful test harnesses.
4. Delete old coding-agent product paths once they are confirmed unnecessary for the shared base.
5. Keep Writer and Tavern requirements as downstream constraints, then create independent product repositories after the core APIs stabilize.

## Non-Goals

- No full novel generator.
- No open-world infinite tavern.
- No UI.
- No graph database service.
- No remote Qwen/DeepSeek runtime yet.
- No product-specific Writer/Tavern prompts yet.
- No resurrection of the old coding-agent orchestrator.
- No long-term monorepo product strategy for Writer and Tavern.

## First Implementation Slice

1. Add domain schemas and validation.
2. Add file-backed store with tests.
3. Add engine interfaces and fake-agent loop test.
4. Leave CLI/product modes for a follow-up.

