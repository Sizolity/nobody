# World Runtime Risk-First Reuse Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prepare reusable repository code for the world runtime direction while first isolating naming, storage, event-log, and narrative-layer risks.

**Architecture:** Add a new `internal/world` boundary instead of renaming `internal/narrative`. Reuse storage and validation patterns from the narrative store, keep workspace telemetry separate from world event logs, and keep the beat engine as a higher-level narrative workflow.

**Tech Stack:** Go 1.25, `testing`, local JSON documents, append-only JSONL streams, existing inference/config/workspace facades.

**Commit Policy:** Do not commit during execution unless the user explicitly asks.

---

## File Structure

- Create: `internal/world/model/`
  Core world runtime value types and validation helpers.

- Create: `internal/world/store/`
  Store interface and file-backed implementation using JSON/JSONL patterns.

- Create: `internal/world/runtime/`
  Minimal event application boundary after model and store are stable.

- Create: `internal/world/view/`
  Debug and character context projections after memory ownership exists.

- Keep: `internal/narrative/`
  Beat engine, `StoryGraph`, drafts, narrative agents, and current narrative store.

- Keep: `internal/workspace/`
  Operational telemetry and run metadata only.

- Keep: `internal/inference/`
  LLM runtime adapters used by future directors and views.

## Task 1: Establish Risk Guardrails In Tests

**Files:**
- Create: `internal/world/model/doc.go`
- Create: `internal/world/model/id.go`
- Create: `internal/world/model/id_test.go`
- Read: `pkg/narrative/id/id.go`

- [ ] **Step 1: Read the existing safe ID behavior**

Read `pkg/narrative/id/id.go` and `internal/narrative/store/file_store.go` to confirm the current safe ID rules.

- [ ] **Step 2: Add package documentation**

Create `internal/world/model/doc.go`:

```go
// Package model defines product-neutral world runtime value types.
//
// World runtime state is separate from narrative beat state and workspace
// telemetry. WorldEvent records are canonical world changes; workspace logs are
// operational telemetry.
package model
```

- [ ] **Step 3: Write ID tests**

Create `internal/world/model/id_test.go`:

```go
package model

import "testing"

func TestValidateIDAcceptsStoreSafeIDs(t *testing.T) {
	for _, id := range []string{"world1", "world_1", "world-1", "A123"} {
		if err := ValidateID(id); err != nil {
			t.Fatalf("ValidateID(%q) returned error: %v", id, err)
		}
	}
}

func TestValidateIDRejectsUnsafeIDs(t *testing.T) {
	for _, id := range []string{"", "../world", "world/id", " world", "world id", ".hidden"} {
		if err := ValidateID(id); err == nil {
			t.Fatalf("ValidateID(%q) returned nil error", id)
		}
	}
}
```

- [ ] **Step 4: Run the failing test**

Run: `go test ./internal/world/model`

Expected: FAIL because `ValidateID` is undefined.

- [ ] **Step 5: Implement ID validation**

Create `internal/world/model/id.go`:

```go
package model

import (
	"fmt"
	"regexp"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func ValidateID(id string) error {
	if !safeIDPattern.MatchString(id) {
		return fmt.Errorf("unsafe id %q", id)
	}
	return nil
}
```

- [ ] **Step 6: Run the test**

Run: `go test ./internal/world/model`

Expected: PASS.

## Task 2: Add Core World Model Skeleton

**Files:**
- Create: `internal/world/model/world.go`
- Create: `internal/world/model/world_test.go`
- Reference: `docs/product/world-runtime/03-core-model-design.md`

- [ ] **Step 1: Write model validation tests**

Create `internal/world/model/world_test.go`:

```go
package model

import "testing"

func TestWorldValidateRequiresIDAndName(t *testing.T) {
	world := World{Name: "Test World"}
	if err := world.Validate(); err == nil {
		t.Fatal("Validate returned nil without ID")
	}

	world = World{ID: "test_world"}
	if err := world.Validate(); err == nil {
		t.Fatal("Validate returned nil without Name")
	}
}

func TestWorldValidateAcceptsMinimalWorld(t *testing.T) {
	world := World{
		ID:   "test_world",
		Name: "Test World",
		Clock: WorldClock{
			Current: WorldTime{Kind: WorldTimeTick, Tick: 1},
		},
	}
	if err := world.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run: `go test ./internal/world/model`

Expected: FAIL because `World` is undefined.

- [ ] **Step 3: Implement minimal world types**

Create `internal/world/model/world.go`:

```go
package model

import "fmt"

type WorldID string
type EntityID string
type EventID string
type MemoryID string
type RuleID string
type ThreadID string
type RelationID string
type FactID string

type World struct {
	ID          WorldID
	Name        string
	Description string
	Canon       Canon
	Clock       WorldClock
	Entities    map[EntityID]Entity
	Relations   []Relation
	Facts       []Fact
	Rules       []Rule
	Threads     []WorldThread
	EventLog    []WorldEvent
	EventQueue  []WorldEvent
	Memory      []MemoryRecord
	Metadata    WorldMetadata
}

func (w World) Validate() error {
	if err := ValidateID(string(w.ID)); err != nil {
		return fmt.Errorf("world.id: %w", err)
	}
	if w.Name == "" {
		return fmt.Errorf("world.name is required")
	}
	return nil
}

type Canon struct {
	Genre      []string
	Tone       []string
	StyleGuide []string
	Premise    string
	Laws       []string
	Boundaries []string
	Secrets    []EntityID
}

type WorldClock struct {
	Current   WorldTime
	Calendar  string
	TimeScale string
	Sequence  int64
}

type WorldTimeKind string

const (
	WorldTimeTick     WorldTimeKind = "tick"
	WorldTimeTurn     WorldTimeKind = "turn"
	WorldTimeScene    WorldTimeKind = "scene"
	WorldTimeChapter  WorldTimeKind = "chapter"
	WorldTimeDay      WorldTimeKind = "day"
	WorldTimeCalendar WorldTimeKind = "calendar_time"
)

type WorldTime struct {
	Kind     WorldTimeKind
	Tick     int64
	Label    string
	Calendar map[string]int
}

type WorldMetadata struct {
	SchemaVersion string
	Source        string
	Tags          []string
}
```

- [ ] **Step 4: Run the test**

Run: `go test ./internal/world/model`

Expected: FAIL because referenced types such as `Entity` are undefined.

- [ ] **Step 5: Add placeholder value types with real fields**

Append to `world.go`:

```go
type Value struct {
	Kind   string
	Raw    any
	Unit   string
	Source string
}

type Entity struct {
	ID          EntityID
	Type        string
	Name        string
	Description string
	Components  map[string]any
	State       map[string]Value
	Tags        []string
}

type Relation struct {
	ID       RelationID
	Type     string
	SourceID EntityID
	TargetID EntityID
}

type Fact struct {
	ID        FactID
	SubjectID EntityID
	Predicate string
	Value     Value
}

type Rule struct {
	ID      RuleID
	Name    string
	Kind    string
	Enabled bool
}

type WorldThread struct {
	ID       ThreadID
	Kind     string
	Title    string
	Summary  string
	Status   string
	Priority float64
	Tension  float64
}

type WorldEvent struct {
	ID          EventID
	Type        string
	Source      string
	ActorIDs    []EntityID
	TargetIDs   []EntityID
	LocationID  EntityID
	Intent      string
	Description string
	Effects     []Effect
}

type Effect struct {
	Kind     string
	TargetID string
	Payload  map[string]Value
}

type MemoryRecord struct {
	ID         MemoryID
	Owner      MemoryOwner
	Scope      string
	Kind       string
	SubjectIDs []EntityID
	EventIDs   []EventID
	Content    string
	Summary    string
	Confidence float64
	Importance float64
}

type MemoryOwner struct {
	Kind string
	ID   string
}
```

- [ ] **Step 6: Run the test**

Run: `go test ./internal/world/model`

Expected: PASS.

## Task 3: Prepare World Store Boundary Without Migrating Narrative Store

**Files:**
- Create: `internal/world/store/store.go`
- Create: `internal/world/store/file_store.go`
- Create: `internal/world/store/file_store_test.go`
- Read: `internal/narrative/store/file_store.go`

- [ ] **Step 1: Write file store test**

Create `internal/world/store/file_store_test.go`:

```go
package store

import (
	"context"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestFileStoreSaveLoadWorld(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := NewFileStore(t.TempDir())
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeTick, Tick: 1},
		},
	}

	if err := st.SaveWorld(ctx, world); err != nil {
		t.Fatalf("SaveWorld returned error: %v", err)
	}
	got, err := st.LoadWorld(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadWorld returned error: %v", err)
	}
	if got.ID != world.ID || got.Name != world.Name {
		t.Fatalf("loaded world mismatch: got %#v want %#v", got, world)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run: `go test ./internal/world/store`

Expected: FAIL because package files are missing.

- [ ] **Step 3: Define store interface**

Create `internal/world/store/store.go`:

```go
package store

import (
	"context"

	"github.com/sizolity/nobody/internal/world/model"
)

type Store interface {
	SaveWorld(context.Context, model.World) error
	LoadWorld(context.Context, string) (model.World, error)
}
```

- [ ] **Step 4: Implement minimal file store**

Create `internal/world/store/file_store.go`:

```go
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sizolity/nobody/internal/world/model"
)

type FileStore struct {
	root string
}

func NewFileStore(workspace string) *FileStore {
	return &FileStore{root: filepath.Join(workspace, "worlds")}
}

func (s *FileStore) SaveWorld(_ context.Context, world model.World) error {
	if err := world.Validate(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.worldDir(string(world.ID)), "world.json"), world)
}

func (s *FileStore) LoadWorld(_ context.Context, worldID string) (model.World, error) {
	if err := model.ValidateID(worldID); err != nil {
		return model.World{}, err
	}
	var world model.World
	if err := readJSON(filepath.Join(s.worldDir(worldID), "world.json"), &world); err != nil {
		return model.World{}, err
	}
	if string(world.ID) != worldID {
		return model.World{}, fmt.Errorf("world id %q does not match path id %q", world.ID, worldID)
	}
	return world, world.Validate()
}

func (s *FileStore) worldDir(worldID string) string {
	return filepath.Join(s.root, worldID)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
```

- [ ] **Step 5: Run store tests**

Run: `go test ./internal/world/...`

Expected: PASS.

## Task 4: Add Risk Separation Tests Before Runtime Logic

**Files:**
- Create: `internal/world/runtime/doc.go`
- Create: `internal/world/runtime/runtime.go`
- Create: `internal/world/runtime/runtime_test.go`

- [ ] **Step 1: Write test for event-only mutation boundary**

Create `internal/world/runtime/runtime_test.go`:

```go
package runtime

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestRuntimeRejectsEmptyEvent(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	_, err := rt.ApplyEvent(world, model.WorldEvent{})
	if err == nil {
		t.Fatal("ApplyEvent returned nil for empty event")
	}
}

func TestRuntimeAppliesEventToEventLog(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:          "event_1",
		Type:        "note",
		Source:      "test",
		Description: "test event",
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 || got.EventLog[0].ID != event.ID {
		t.Fatalf("EventLog mismatch: %#v", got.EventLog)
	}
}
```

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/world/runtime`

Expected: FAIL because `Runtime` is undefined.

- [ ] **Step 3: Add runtime package docs**

Create `internal/world/runtime/doc.go`:

```go
// Package runtime applies proposed world events through a controlled boundary.
//
// It must not use workspace operational logs as world history and must not let
// narrative agents mutate world state directly.
package runtime
```

- [ ] **Step 4: Implement minimal ApplyEvent**

Create `internal/world/runtime/runtime.go`:

```go
package runtime

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

type Runtime struct{}

func (r Runtime) ApplyEvent(world model.World, event model.WorldEvent) (model.World, error) {
	if err := model.ValidateID(string(event.ID)); err != nil {
		return model.World{}, fmt.Errorf("event.id: %w", err)
	}
	if event.Type == "" {
		return model.World{}, fmt.Errorf("event.type is required")
	}
	world.EventLog = append(world.EventLog, event)
	return world, nil
}
```

- [ ] **Step 5: Run runtime tests**

Run: `go test ./internal/world/...`

Expected: PASS.

## Task 5: Verify Existing Narrative Layer Still Builds

**Files:**
- Verify only: `internal/narrative/...`, `pkg/narrative/...`, `internal/world/...`

- [ ] **Step 1: Run focused tests**

Run: `go test ./internal/world/... ./internal/narrative/... ./pkg/narrative/...`

Expected: PASS.

- [ ] **Step 2: Run all tests**

Run: `go test ./...`

Expected: PASS. If unrelated pre-existing failures appear, record them before changing code.

## Task 6: Update Reuse Documentation After Code Prep

**Files:**
- Modify: `docs/engineering/world-runtime/repository-reuse-and-risk-controls.md`
- Modify if needed: `docs/product/world-runtime/05-storage-and-evolution.md`

- [ ] **Step 1: Add implementation status note**

Update `docs/engineering/world-runtime/repository-reuse-and-risk-controls.md` with a short status section listing which risk controls now have code boundaries:

```markdown
## Implementation Status

- `internal/world/model` owns initial world runtime value types.
- `internal/world/store` owns the first world file-store boundary.
- `internal/world/runtime` owns the first event-only mutation boundary.
- `internal/narrative` remains the beat/narrative layer.
- `internal/workspace` remains operational telemetry only.
```

- [ ] **Step 2: Search for accidental world-runtime imports from narrative**

Run: `rg "internal/world" internal/narrative pkg/narrative`

Expected: no matches during this preparation phase.

- [ ] **Step 3: Run documentation lint**

Use IDE diagnostics or markdown lints if available. Expected: no new diagnostics.

