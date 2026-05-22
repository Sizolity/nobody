# Nobody

Nobody is the shared foundation repository for a **Narrative Harness**.

The old coding-agent product direction has been removed. This repository now keeps the reusable core that future narrative products will depend on:

- **Writer Mode product**: structured long-form fiction generation.
- **Tavern Mode product**: local-first roleplay and adventure worlds.

Writer and Tavern should become separate product repositories. This repository should stay product-neutral.

The core idea is documented in [`docs/product/narrative-harness-core.md`](docs/product/narrative-harness-core.md). The first base design is in [`docs/product/narrative-engine-base-design.md`](docs/product/narrative-engine-base-design.md).

## Current Contents

This repository intentionally keeps only a small set of reusable building blocks:

- `internal/inference/llamacpp`: local llama.cpp runtime, health checks, OpenAI-compatible chat and embedding clients, and managed process helpers.
- `internal/inference`: shared inference events and health-check contracts.
- `internal/config`: shared model/runtime configuration for the reusable core.
- `internal/workspace`: small logging and run metadata primitives worth reusing in future narrative products.
- `internal/narrative`: product-neutral world/story schemas, file-backed narrative storage, and a deterministic beat loop.
- `pkg/narrative`, `pkg/config`, `pkg/inference`, `pkg/workspace`, and `pkg/skills`: public facade packages for future Writer and Tavern repositories.
- `internal/skills`: minimal embedding interface for future recall/index work.
- `scripts`: llama.cpp build/start helper scripts.
- `docs/product`: new product direction and core engine notes.

Everything else should be rebuilt deliberately around the Narrative Harness base. Product-specific workflows, prompts, UI, and packaging belong in the future Writer and Tavern repositories.

## Public Import Surface

Future product repositories should import the shared narrative core through:

- `github.com/sizolity/nobody/pkg/narrative`
- `github.com/sizolity/nobody/pkg/narrative/agentio`
- `github.com/sizolity/nobody/pkg/narrative/bootstrap`
- `github.com/sizolity/nobody/pkg/narrative/contract`
- `github.com/sizolity/nobody/pkg/narrative/store`
- `github.com/sizolity/nobody/pkg/narrative/engine`
- `github.com/sizolity/nobody/pkg/narrative/enginetest`
- `github.com/sizolity/nobody/pkg/narrative/id`
- `github.com/sizolity/nobody/pkg/narrative/memstore`
- `github.com/sizolity/nobody/pkg/narrative/prompt`
- `github.com/sizolity/nobody/pkg/narrative/snapshot`
- `github.com/sizolity/nobody/pkg/narrative/storetest`
- `github.com/sizolity/nobody/pkg/config`
- `github.com/sizolity/nobody/pkg/inference`
- `github.com/sizolity/nobody/pkg/inference/llamacpp`
- `github.com/sizolity/nobody/pkg/workspace`
- `github.com/sizolity/nobody/pkg/skills`

Packages under `internal/` are implementation detail and should not be treated as the downstream product API.

`pkg/narrative/enginetest` is intended for product tests only. It provides deterministic agent doubles so Writer and Tavern can test product workflows without calling an LLM.

`pkg/narrative/storetest` is intended for custom Store implementation tests, such as future database-backed stores.

`pkg/narrative/agentio` decodes model JSON output into typed contracts and includes validate-on-decode helpers for beat plans, drafts, memory deltas, and state deltas.

## Verification

```bash
go test ./...
```

## Narrative Base Slice

The current shared base includes:

- world bible schema
- story graph schema
- event and memory JSONL store
- draft store
- product-neutral beat loop
- five-agent narrative map: director, writer, continuity, memory, state

Writer Mode and Tavern Mode should be designed as downstream products on top of this base, not as long-term modes inside this repository.
