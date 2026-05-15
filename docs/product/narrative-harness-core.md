# Narrative Harness Core

## Product Direction

Nobody pivots from a coding-agent harness for local small models into a shared narrative engine base. The product family will support two future shells:

- **Writer Mode**: a structured AI novel writer that manages world bible, plot tree, scene drafting, continuity checks, and revisions.
- **Tavern Mode**: a local-first AI tavern / roleplay engine for custom worlds, private adventures, persistent NPC memory, and evolving game state.

The shared product thesis is:

> Strong models provide imagination and high-quality prose; the harness provides persistent world state, structured narrative control, memory, continuity, privacy options, and repeatable workflows.

## Core Engine Idea

The common base is a **Narrative Harness**. It should not be designed around one product shell first. Instead, it owns the reusable narrative substrate:

- `World Bible`: characters, locations, factions, rules, boundaries, style guide.
- `Story Graph`: plot tree, active beat, branches, hooks, unresolved conflicts.
- `Event Log`: append-only record of what happened.
- `Memory Index`: extracted facts, relationships, plot hooks, and state changes.
- `Draft Store`: generated prose, dialogue, scene drafts, and revision history.
- `Agent Map`: product-neutral narrative agents that operate on structured context.

The smallest shared unit is a **Beat**:

- In Writer Mode, a beat is a scene or scene fragment.
- In Tavern Mode, a beat is one player turn and narrative response.

## MVP Agent Map

Start with five product-neutral agents:

- `DirectorAgent`: chooses the next beat objective from current story state.
- `SceneWriterAgent`: writes prose or dialogue for the beat.
- `ContinuityAgent`: checks contradictions against world, memory, and story graph.
- `MemoryAgent`: extracts durable facts, relationships, hooks, and summaries.
- `StateAgent`: applies structured state updates to world/story data.

These agents should communicate through typed JSON contracts, not loose chat text.

## Storage Principle

Start local, readable, and migration-friendly:

- JSON documents for world, characters, locations, and story graph.
- JSONL for events and memories.
- Markdown for drafts and human-editable narrative material.
- File-backed storage first; database-backed storage later behind interfaces.

This gives a document/event style store without committing early to a heavy database. Future options include embedded KV, vector index, graph DB, or hybrid document/graph storage.

## Model Strategy

The engine should support model tiers:

- Remote Qwen / DeepSeek style APIs for high-quality planning, prose, and complex revision.
- Local llama.cpp models for privacy mode, lightweight roleplay turns, memory extraction, summarization, and low-cost support tasks.

The goal is not to make local small models replace strong remote models. The goal is to reduce dependence on strong models by making tasks smaller, structured, retrievable, and verifiable.

## Migration Stance

Keep reusable infrastructure from the old project where valuable:

- Run lifecycle and workspace layout.
- JSONL event logging.
- Prompt loading.
- Model runtime facade.
- Handoff/checkpoint concepts.
- Tool and sandbox infrastructure where useful.

Do not preserve old product assumptions:

- Coding-task-first agent UX.
- Plan/execute/audit orchestrator as the primary product loop.
- Local small model as the main intelligence premise.
- Ollama compatibility.

The new repository history should start from this product thesis and rebuild deliberately around the Narrative Harness base.
