# Narrative Harness Core

## Product Direction

Nobody pivots from a coding-agent harness for local small models into a shared narrative foundation repository. In the current stage, this repository is the reusable base for narrative products, not a finished product itself.

The future product family will be split into independent repositories:

- **Writer Mode product**: a structured AI novel writer that manages world bible, plot tree, scene drafting, continuity checks, and revisions.
- **Tavern Mode product**: a local-first AI tavern / roleplay engine for custom worlds, private adventures, persistent NPC memory, and evolving game state.

The shared product thesis is:

> Strong models provide imagination and high-quality prose; the harness provides persistent world state, structured narrative control, memory, continuity, privacy options, and repeatable workflows.

## Core Engine Idea

The common base is a **Narrative Harness**. It should not be designed around one product first. Instead, it owns the reusable narrative substrate:

- `World Bible`: characters, locations, factions, rules, boundaries, style guide.
- `Story Graph`: plot tree, active beat, branches, hooks, unresolved conflicts.
- `Event Log`: append-only record of what happened.
- `Memory Index`: extracted facts, relationships, plot hooks, and state changes.
- `Draft Store`: generated prose, dialogue, scene drafts, and revision history.
- `Agent Map`: product-neutral narrative agents that operate on structured context.

The smallest shared unit is a **Beat**:

- In Writer Mode, a beat is a scene or scene fragment.
- In Tavern Mode, a beat is one player turn and narrative response.

## Repository Role

`nobody` is the shared core repository for now. Its job is to keep and reshape reusable infrastructure from the old project while deleting code that no longer serves the new direction.

The repository should contain:

- Product-neutral narrative domain models and engine contracts.
- Public `pkg/narrative`, `pkg/config`, `pkg/inference`, `pkg/workspace`, and `pkg/skills` facade packages that future product repositories can import.
- Public `pkg/narrative/agentio` helpers for strict decoding of model JSON output into typed contracts, including validate-on-decode helpers for agent outputs.
- Public `pkg/narrative/bootstrap` helpers for initializing a store with a validated world seed.
- Public `pkg/narrative/contract` helpers for validating agent outputs before returning them to the engine.
- Public `pkg/narrative/enginetest` deterministic agents for product tests that should not call an LLM.
- Public `pkg/narrative/id` helpers for validating store-safe identifiers before product code writes files.
- Public `pkg/narrative/memstore` in-memory Store implementation for product tests and short-lived workflows.
- Public `pkg/narrative/prompt` helpers for rendering stable context JSON for LLM-backed agents.
- Public `pkg/narrative/snapshot` helpers for product UIs and CLIs that need to inspect current world state.
- Public `pkg/narrative/storetest` conformance tests for future custom Store implementations.
- Local-readable storage, JSONL event streams, prompt loading, and runtime adapters when they serve the shared base.
- llama.cpp lifecycle and model runtime pieces that help the core run locally.
- Tests and harnesses that validate the shared contracts before product repositories depend on them.

The repository should not contain:

- A Writer-specific application workflow.
- A Tavern-specific application workflow.
- The old coding-agent UX, plan/execute/audit loop, or assumptions that local small models are the main intelligence layer.
- Compatibility layers for old product behavior that is not needed by the shared core.

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
- Product-neutral run metadata and recovery concepts, when they serve narrative workflows.
- Tool and sandbox infrastructure where useful.

Delete or leave behind code whose only value is preserving old product assumptions:

- Coding-task-first agent UX.
- Plan/execute/audit orchestrator as the primary product loop.
- Local small model as the main intelligence premise.
- Ollama compatibility.

The new repository history should start from this product thesis and rebuild deliberately around the Narrative Harness base.

## Near-Term Direction

1. Harden the shared narrative core: schemas, validation, file store behavior, engine contract tests, and deterministic fake-agent flows.
2. Audit old `nobody` code for reuse: keep runtime, storage, prompt, workspace, and testing pieces that directly support the shared base.
3. Remove coding-agent product code that does not support the shared base.
4. Keep Writer and Tavern requirements in docs as downstream constraints, but do not implement their product flows here.
5. When the core contract is stable enough, create separate Writer and Tavern product repositories that depend on this shared base.
