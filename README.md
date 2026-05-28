# Nobody

Nobody is a product-neutral **fictional world runtime** framework written in
Go. It provides durable concepts (worlds, entities, events, threads,
memories, rules) plus the runtime machinery that mutates them — without
prescribing any particular narrative product.

Downstream products build their own beat loops, prompts, UIs, and tools on
top of this framework. The first such product is
[`github.com/sizolity/worldline`](https://github.com/sizolity/worldline), a
local-first narrative RPG runtime.

## Design Documents

The conceptual model and architecture live under
[`docs/product/world-runtime/`](docs/product/world-runtime/) (read in
numbered order). Engineering notes live under
[`docs/engineering/`](docs/engineering/).

## Repository Layout

```text
cmd/nobody-world/             framework CLI (init, apply-event, step, run,
                              checkpoint / rollback / fork / lineage, diff,
                              merge, validate, ingest-source, …)
internal/world/
├── model/                    World, Entity, WorldEvent, MemoryRecord,
│                             WorldThread, Rule, Relation, Fact, Clock, ...
├── runtime/                  ApplyEvent, Step, Run, builtin rules
├── store/                    FileStore (JSON / JSONL) + WorldTemplate
│                             scaffold (no concrete templates ship here)
├── view/                     NarrativeView, WorldDebugView,
│                             CharacterContextView projections
├── ingest/                   Draft → World compile pipeline + Parser /
│                             ChunkParser interfaces
├── director/                 Event proposal interface + script / random /
│                             reconcile / event-table / LLM directors
├── runner/                   Multi-step runner over (Runtime, Directors)
├── system/                   Event-builder helpers: typed functions that
│                             construct valid WorldEvent values for spatial,
│                             inventory, stats, and actor mutations.
│                             Called by products, not by runtime automatically.
└── devcli/                   The nobody-world CLI implementation
internal/textchunk/           Text chunking utilities (currently unused,
                              reserved for ingest pipeline)
```

## Design Boundaries (enforced by code reviews and the docs)

The framework deliberately stays product-neutral:

- `internal/world/model` exposes `Entity.Type string` — values like
  `character` / `location` / `item` are **product conventions**, not enums
  baked into the framework.
- `internal/world/store` ships the `WorldTemplate` struct but **no concrete
  templates**. Products supply their own templates (e.g. Worldline's
  `rpg/template/{fantasy,scifi,modern,mystery}.go`).
- The framework never imports a downstream product. Any `internal/world/*`
  package importing a product (e.g. `rpg/`) is a layering violation. The
  prior `manage-rule` / `--template` couplings were removed when the RPG
  product was split into the Worldline repository.

## Quick Start

```fish
go build -o ./bin/nobody-world ./cmd/nobody-world

./bin/nobody-world init --workspace /tmp/nb --world-id demo --name "Demo World"
./bin/nobody-world show --workspace /tmp/nb --world-id demo
```

## Verification

```fish
go test ./...
go vet  ./...
go build ./...
```

## Downstream Products

- [Worldline](https://github.com/sizolity/worldline) — local-first narrative
  RPG: streaming beats, Lorekeeper knowledge sedimentation, WorldLine
  drift / milestone scheduler, REPL CLI. Currently a fork-and-copy of the
  `internal/world/{model,director,runtime,store,view,ingest}` subset rather
  than a Go import dependency; see Worldline's `README.md` for the
  provenance pointer.
