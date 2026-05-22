# Shared Core Transition Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `nobody` into a clean shared narrative foundation repository by hardening the narrative core, preserving reusable runtime infrastructure, and removing old coding-agent leftovers.

**Architecture:** Keep `internal/narrative` as the product-neutral domain and beat engine. Keep `internal/inference` and `internal/inference/llamacpp` as reusable model runtime support. Thin or remove old harness/config pieces that encode the removed coding-agent product loop.

**Tech Stack:** Go 1.25, `testing`, `testify`, YAML config loading, file-backed JSON/JSONL storage, llama.cpp OpenAI-compatible runtime.

**Commit Policy:** Do not commit during execution unless the user explicitly asks. Treat commit steps from the generic process as local verification checkpoints.

---

## File Structure

- `internal/narrative/`: shared product-neutral narrative domain, store, and beat engine. This is the first stable core surface.
- `internal/inference/`: product-neutral LLM runtime contracts. Keep.
- `internal/inference/llamacpp/`: local llama.cpp runtime lifecycle and adapters. Keep.
- `internal/config/`: shared model/runtime configuration after removing old coding-agent fields.
- `internal/workspace/eventlog.go`: reusable JSONL observability.
- `internal/workspace/run_meta.go`: reusable run metadata for future product repositories.
- `internal/harness/`: removed; old Eino/orchestrator checkpoint storage was deleted.
- `docs/product/`: product and architecture docs. Keep aligned with shared-core positioning.

### Task 1: Establish Baseline

**Files:**
- Read: `README.md`
- Read: `internal/config/config.go`
- Read: `internal/harness/file_checkpoint_store.go`
- Read: `internal/harness/eventlog.go`
- Read: `internal/harness/run_meta.go`
- Verify: all Go packages

- [ ] **Step 1: Run the full Go test suite**

Run: `go test ./...`

Expected: PASS. If it fails, record the failing package and fix only failures related to the current transition.

- [ ] **Step 2: Search for active imports of legacy harness checkpoint code**

Run: `rg "file_checkpoint_store|Checkpoint|stateEnvelope|orchestrator" internal`

Expected: either only `internal/harness/file_checkpoint_store.go` references itself, or any external references are listed before deletion.

- [ ] **Step 3: Search for removed product-loop vocabulary**

Run: `rg "plan/execute/audit|mandatory_audit|Orchestrator|handoff|AGENT.md|coding-agent|cmd/harness|planner|auditor" README.md docs internal nobody.yaml`

Expected: all matches are reviewed and classified as either still useful documentation or stale old-product residue.

### Task 2: Remove Dead Checkpoint Store

**Files:**
- Delete: `internal/harness/file_checkpoint_store.go`
- Verify: all Go packages

- [ ] **Step 1: Confirm there are no active imports**

Run: `rg "FileCheckpointStore|NewFileCheckpointStore|LoadCheckpoint|SaveCheckpoint|stateEnvelope" internal`

Expected: only `internal/harness/file_checkpoint_store.go` matches.

- [ ] **Step 2: Delete the file**

Remove `internal/harness/file_checkpoint_store.go`.

- [ ] **Step 3: Run package tests**

Run: `go test ./...`

Expected: PASS. If deleting the file exposes references, either remove those references if stale or stop and reassess before restoring behavior elsewhere.

### Task 3: Thin Config Around Shared Runtime

**Files:**
- Modify: `internal/config/config.go`
- Modify if needed: `internal/config/loader.go`
- Modify if needed: `nobody.yaml`
- Test: add or update config tests if they exist

- [ ] **Step 1: Identify config fields used by live packages**

Run: `rg "Config|RuntimeConfig|Model|Llama|Orchestrator|Sandbox|Budget|Confirm|Context" internal`

Expected: fields used only by old comments or removed product loops are marked for deletion or legacy quarantine.

- [ ] **Step 2: Preserve runtime config used by inference**

Keep model provider, llama.cpp server, embedding, timeout, and OpenAI-compatible options needed by `internal/inference` and `internal/inference/llamacpp`.

- [ ] **Step 3: Remove stale coding-agent fields**

Remove config fields whose only purpose is the old orchestrator, coding-agent audit loop, AGENT.md flow, sandbox confirmation flow, run handoff, or tool spillover.

- [ ] **Step 4: Update YAML examples**

Edit `nobody.yaml` so it shows only shared-core runtime settings still accepted by `internal/config`.

- [ ] **Step 5: Run config and inference tests**

Run: `go test ./internal/config ./internal/inference/...`

Expected: PASS.

### Task 4: Rename Generic Workspace Utilities

**Files:**
- Create: `internal/workspace/eventlog.go`
- Create: `internal/workspace/run_meta.go`
- Delete: `internal/harness/eventlog.go`
- Delete: `internal/harness/run_meta.go`
- Modify imports if any packages use them

- [ ] **Step 1: Check current imports**

Run: `rg "internal/harness|EventLogger|RunMeta|runs-index" internal`

Expected: current usage is clear before moving anything.

- [ ] **Step 2: Decide package boundary**

Move generic observability and run metadata utilities to `internal/workspace`.

- [ ] **Step 3: Update comments**

Remove comments that refer to removed files such as `session_handoff.go` or the old coding-agent lifecycle.

- [ ] **Step 4: Run tests**

Run: `go test ./...`

Expected: PASS.

### Task 5: Lock Narrative Core Contract

**Files:**
- Modify if needed: `internal/narrative/model.go`
- Modify if needed: `internal/narrative/store/file_store.go`
- Modify if needed: `internal/narrative/engine/engine.go`
- Test: `internal/narrative/...`

- [ ] **Step 1: Run narrative tests**

Run: `go test ./internal/narrative/...`

Expected: PASS.

- [ ] **Step 2: Add missing tests only for observed gaps**

Add tests only if baseline or review finds missing validation around current fields, file path safety, JSONL errors, or engine agent-output validation.

- [ ] **Step 3: Keep product-specific behavior out**

Reject any Writer-only or Tavern-only prompt, workflow, CLI, or UX logic from `internal/narrative`.

- [ ] **Step 4: Run full tests**

Run: `go test ./...`

Expected: PASS.

### Task 6: Align Documentation

**Files:**
- Modify: `docs/product/narrative-harness-core.md`
- Modify: `docs/product/narrative-engine-base-design.md`
- Modify if needed: `README.md`
- Modify if needed: `nobody.yaml`

- [ ] **Step 1: Search for stale product claims**

Run: `rg "coding-agent|cmd/harness|plan/execute/audit|Writer Mode.*same repo|Tavern Mode.*same repo|Ollama" README.md docs nobody.yaml internal`

Expected: stale claims are removed or rewritten as historical migration notes.

- [ ] **Step 2: Document the shared-core boundary**

Ensure docs state that `nobody` is the shared core repository and Writer/Tavern are future separate product repositories.

- [ ] **Step 3: Run final verification**

Run: `go test ./...`

Expected: PASS.

## Self-Review

- Spec coverage: The plan covers shared-core positioning, old code reuse/deletion, narrative hardening, config cleanup, harness utility cleanup, and future Writer/Tavern repo boundaries.
- Placeholder scan: No TBD/TODO placeholders remain.
- Type consistency: Package names and paths match the current repository snapshot.
