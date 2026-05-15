# Nobody

Nobody is being reset into a **Narrative Harness** seed repository.

The old coding-agent product direction has been removed. What remains is migration material for a shared narrative engine that can later power:

- **Writer Mode**: structured long-form fiction generation.
- **Tavern Mode**: local-first roleplay and adventure worlds.

The core idea is documented in [`docs/product/narrative-harness-core.md`](docs/product/narrative-harness-core.md).

## Current Seed Contents

This repository intentionally keeps only a small set of reusable building blocks:

- `internal/inference/llamacpp`: local llama.cpp runtime, health checks, OpenAI-compatible chat and embedding clients, and managed process helpers.
- `internal/inference`: shared inference events and health-check contracts.
- `internal/config`: existing configuration model retained as migration material for the runtime.
- `internal/harness`: small logging/checkpoint/run metadata primitives worth reusing in the future narrative engine.
- `internal/skills`: minimal embedding interface for future recall/index work.
- `scripts`: llama.cpp build/start helper scripts.
- `docs/product`: new product direction and core engine notes.

Everything else should be rebuilt deliberately around the Narrative Harness base.

## Verification

```bash
go test ./...
```

## Next Design Step

Design the shared `internal/narrative` base:

- world bible schema
- story graph schema
- event and memory JSONL store
- draft store
- product-neutral beat loop
- five-agent narrative map: director, writer, continuity, memory, state
