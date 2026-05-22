# World Runtime Repository Reuse And Risk Controls

## Purpose

This engineering document compares the current repository shape with the world runtime design and records the risk controls that must be handled before code reuse.

The goal is to avoid accidentally rebuilding the old narrative harness or coding-agent harness under new names.

## Risk Controls

### Risk: Three Event Logs Become Confused

The repository now has or will have three different event streams:

```text
workspace logs
  operational telemetry, model runtime events, run/session observations

narrative events
  beat-level narrative events emitted by the current narrative engine

world events
  canonical world-state changes with intent, visibility, status, causes, and typed effects
```

Control:

- Keep `internal/workspace.EventLogger` as operational telemetry only.
- Keep current `narrative.NarrativeEvent` as a beat-layer artifact until migrated.
- Introduce `WorldEvent` under a separate world runtime package.
- Do not reuse `workspace/logs/*.jsonl` as a world `EventLog`.
- Do not treat old `narrative/events.jsonl` as the final world event log without a schema migration.

### Risk: StoryGraph Becomes World Truth

`StoryGraph` and `StoryNode` are useful, but they describe authored or beat-level narrative structure. They are not the durable world state.

Control:

- Keep `StoryGraph` in the narrative layer.
- Model durable dramatic pressure as `WorldThread`.
- Model authored quest chains or plot structures as `Script`.
- Treat generated prose and drafts as view outputs, not source-of-truth state.

### Risk: Agents Mutate World State Directly

The existing narrative engine allows agents to emit memory deltas and state deltas that are persisted by the beat loop.

World runtime design requires:

```text
agent output -> typed proposal -> WorldEvent -> rule validation -> effects -> persistence
```

Control:

- Existing `MemoryAgent` and `StateAgent` stay narrative-layer until replaced.
- New world-facing agents or directors must propose `WorldEvent` values.
- Only runtime effect application should mutate world state.

### Risk: Omniscient Context Leaks Into Character Agents

Current prompt/context helpers can load broad world, event, and memory context. That conflicts with owner-aware memory and character views.

Control:

- Keep broad snapshots for debug and GM use.
- Add `CharacterContextView` before character self-drive.
- Require owner-aware memory retrieval for character agents.
- Never pass hidden narrator truth to a character agent unless an event revealed it.

### Risk: Old Coding-Agent Harness Concepts Reappear

Old harness concepts include checkpoint envelopes, plan/execute/audit loops, coding presets, shell-timeout-oriented config, and harness-specific run assumptions.

Control:

- Do not resurrect `internal/harness`.
- Keep `internal/workspace` as generic run metadata and telemetry.
- Review config fields before using them in world runtime.
- Avoid names such as coding, audit, checkpoint, orchestrator, and shell loop in world-runtime APIs unless they describe a genuine world concept.

### Risk: Package Names Blur Layer Boundaries

Putting world runtime types under `pkg/narrative` would make the lower world layer look like a narrative product feature.

Control:

- Prefer `internal/world` for the first implementation.
- Add `pkg/world` only after the model and store are stable.
- Keep `internal/narrative` as the beat/story layer above world runtime.

## Reuse Categories

### Reuse Directly

These pieces are already product-neutral enough to reuse:

```text
internal/inference/
internal/inference/llamacpp/
pkg/inference/
pkg/inference/llamacpp/
internal/workspace/
pkg/workspace/
pkg/narrative/agentio/
pkg/narrative/id/
```

Reuse notes:

- Inference remains the model runtime layer.
- Workspace logs remain operational telemetry.
- `agentio` gives a useful typed JSON decode pattern for future director outputs.
- `id` gives safe file-path identifier validation patterns.

### Reuse By Forking Patterns

These are useful as implementation patterns, but their current schemas belong to the narrative layer:

```text
internal/narrative/store/file_store.go
pkg/narrative/memstore/
pkg/narrative/storetest/
pkg/narrative/bootstrap/
pkg/narrative/snapshot/
pkg/narrative/prompt/
```

Reuse notes:

- Copy the file-store mechanics: atomic JSON writes, append-only JSONL, safe IDs, local-readable layout.
- Do not copy the exact storage schema as the final world schema.
- Replace `characters/` and `locations/` with `entities/`.
- Add `relations.json`, `facts.json`, `threads.json`, and a world `events.jsonl`.
- Split omniscient prompt/snapshot behavior into debug, narrative, and character views.

### Keep As Narrative Layer

These should remain above world runtime:

```text
internal/narrative/engine/
internal/narrative.StoryGraph
internal/narrative.StoryNode
internal/narrative.Draft
pkg/narrative/engine/
pkg/narrative/contract/
pkg/narrative/enginetest/
```

Reuse notes:

- `RunBeat` can later call world runtime.
- `Draft` remains generated prose.
- `StoryGraph` can inform `Script` or narrative views, but it should not become `WorldThread`.

### Reshape Before Reuse

These current concepts are close to the target but not safe to reuse as-is:

```text
narrative.World        -> Canon + world identity seed
narrative.Character    -> Entity with Profile, Actor, Memory, Stats components
narrative.Location     -> Entity with Profile, Location, Spatial components
narrative.NarrativeEvent -> WorldEvent only after schema expansion
narrative.Memory       -> MemoryRecord only after owner/truth/visibility expansion
Character.Relationships -> Relation graph
World.Rules []string   -> Canon text or seed rules, not executable RuleSet
```

### Do Not Reuse As World Core

These should not become world-runtime foundations:

```text
internal/harness
harness checkpoint store
plan/execute/audit product loop
beat-scoped state mutation as the only runtime
workspace event logs as world event logs
story graph as world truth
omnisicent context bundle for character agents
```

## Recommended Preparation Order

1. Establish `internal/world/model` with the core value types.
2. Add store-safe ID tests for world identifiers.
3. Create a minimal world store by forking the narrative file-store pattern.
4. Keep operational logs separate from world event logs.
5. Add minimal `WorldEvent` validation before writing runtime logic.
6. Add a conservative runtime that applies one proposed event.
7. Add debug and character context views before character self-drive.
8. Integrate narrative `RunBeat` only after world event application is stable.

## Repository Review Questions

Before implementation, inspect the current repository and answer:

1. Which existing `internal/narrative` schemas can be evolved into `World`, `Entity`, `Event`, `Memory`, and `Thread`?
2. Does the current beat engine map to `Runtime.Step`, or should it remain a higher-level narrative loop above the world runtime?
3. Which store interfaces already support JSON documents and JSONL event/memory streams?
4. Should `pkg/narrative` expose early aliases for world runtime types, or should the design stay internal until stable?
5. Which old harness/workspace utilities are still useful for event logs, run metadata, and replay?
6. How should existing product docs be updated from "Narrative Harness" toward "World Runtime" without losing Writer/Tavern constraints?

## Package Boundary Target

```text
internal/world/model
  World, Canon, Clock, Entity, Relation, Fact, WorldEvent, MemoryRecord, WorldThread

internal/world/store
  WorldStore interface and file-backed implementation

internal/world/runtime
  RuleSet, Director, Step, effect application

internal/world/view
  DebugView, NarrativeView, CharacterContextView

internal/narrative
  Beat engine, StoryGraph, Drafts, narrative agents

internal/workspace
  Run metadata and operational JSONL logs

internal/inference
  LLM runtime adapters
```

## First Reuse Target

The first reusable code target should be the file-store pattern, not the narrative domain model.

Reason:

- Storage mechanics are already close to the world-runtime storage principles.
- Domain types need deeper semantic changes.
- A new store package can be tested independently without disturbing the current narrative engine.

