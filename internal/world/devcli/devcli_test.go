package devcli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	worldview "github.com/sizolity/nobody/internal/world/view"
)

func TestRunInitCreatesWorldSnapshot(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"init",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--name", "Test World",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	st := store.NewFileStore(workspace)
	world, err := st.LoadSnapshot(context.Background(), "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if world.ID != "test_world" || world.Name != "Test World" {
		t.Fatalf("world mismatch: %#v", world)
	}
}

func TestRunApplyEventPersistsEvent(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	eventPath := filepath.Join(workspace, "event.json")
	writeTestJSON(t, eventPath, model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	})

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"apply-event",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--event-file", eventPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	world, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(world.EventLog) != 1 || world.EventLog[0].ID != "event_1" {
		t.Fatalf("event not persisted: %#v", world.EventLog)
	}
}

func TestRunApplyEventAcceptsSnakeCaseEventJSON(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	eventPath := filepath.Join(workspace, "event.json")
	const eventJSON = `{
  "id": "event_1",
  "type": "world_fact_changed",
  "source": "test",
  "effects": [
    {
      "kind": "set_fact",
      "target_id": "fact_1",
      "payload": {
        "subject_id": {"kind": "entity_ref", "raw": "king"},
        "predicate": {"kind": "string", "raw": "status"},
        "value": {"kind": "string", "raw": "dead"}
      }
    }
  ]
}`
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"apply-event",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--event-file", eventPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	world, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(world.Facts) != 1 || world.Facts[0].ID != "fact_1" || world.Facts[0].SubjectID != "king" {
		t.Fatalf("fact not applied from snake_case JSON: %#v", world.Facts)
	}
}

func TestRunApplyEventPersistsReconciledMemory(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	initial := model.World{
		ID:   "test_world",
		Name: "Test World",
		Memory: []model.MemoryRecord{{
			ID:          "memory_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
			Scope:       model.MemoryScopeSubjective,
			Kind:        model.MemoryKindBelief,
			Content:     "A killed the king.",
			TruthStatus: model.TruthStatusUnknown,
			Confidence:  0.8,
			Importance:  0.7,
		}},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	eventPath := filepath.Join(workspace, "event.json")
	const eventJSON = `{
  "id": "event_reconcile_1",
  "type": "remember",
  "source": "test",
  "effects": [
    {
      "kind": "reconcile_memory",
      "target_id": "memory_1",
      "payload": {
        "truth_status": {"kind": "string", "raw": "disputed"},
        "confidence_delta": {"kind": "number", "raw": -0.5},
        "summary": {"kind": "string", "raw": "New evidence disputes C's old belief."},
        "add_memory_id": {"kind": "string", "raw": "memory_2"},
        "add_memory_content": {"kind": "string", "raw": "C starts to suspect A was framed."}
      }
    }
  ]
}`
	if err := os.WriteFile(eventPath, []byte(eventJSON), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"apply-event",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--event-file", eventPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	world, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(world.Memory) != 2 {
		t.Fatalf("Memory length = %d, want 2: %#v", len(world.Memory), world.Memory)
	}
	if world.Memory[0].TruthStatus != model.TruthStatusDisputed {
		t.Fatalf("memory truth status not reconciled: %#v", world.Memory[0])
	}
	if world.Memory[0].Confidence < 0.299999 || world.Memory[0].Confidence > 0.300001 {
		t.Fatalf("memory confidence = %v, want 0.3", world.Memory[0].Confidence)
	}
	if world.Memory[0].Summary != "New evidence disputes C's old belief." {
		t.Fatalf("memory summary not reconciled: %#v", world.Memory[0])
	}
	if world.Memory[1].ID != "memory_2" || world.Memory[1].Owner.ID != "char_c" {
		t.Fatalf("follow-up memory mismatch: %#v", world.Memory[1])
	}
	if world.Memory[1].Content != "C starts to suspect A was framed." {
		t.Fatalf("follow-up memory content mismatch: %#v", world.Memory[1])
	}
	if len(world.EventLog) != 1 || world.EventLog[0].ID != "event_reconcile_1" {
		t.Fatalf("reconcile event not persisted: %#v", world.EventLog)
	}
}

func TestRunShowPrintsSnapshotJSON(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"show",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	var world model.World
	if err := json.Unmarshal(stdout.Bytes(), &world); err != nil {
		t.Fatalf("stdout is not world JSON: %v\n%s", err, stdout.String())
	}
	if world.ID != "test_world" || world.Name != "Test World" {
		t.Fatalf("world mismatch: %#v", world)
	}
}

func TestRunDebugViewPrintsWorldDebugContextJSON(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, inspectionWorld()); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"debug-view",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	var got worldview.WorldDebugContext
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not debug context JSON: %v\n%s", err, stdout.String())
	}
	if got.World.ID != "test_world" {
		t.Fatalf("debug world id = %q, want test_world", got.World.ID)
	}
	if len(got.Memories) != 3 {
		t.Fatalf("debug memories count = %d, want 3: %#v", len(got.Memories), got.Memories)
	}
	hasSecret := false
	hasPrivate := false
	for _, memory := range got.Memories {
		if memory.TruthStatus == model.TruthStatusSecret {
			hasSecret = true
		}
		if memory.Owner.Kind == model.MemoryOwnerKindCharacter {
			hasPrivate = true
		}
	}
	if !hasSecret || !hasPrivate {
		t.Fatalf("debug view should expose secret and private memories: %#v", got.Memories)
	}
	if len(got.EventLog) != 3 {
		t.Fatalf("debug event log count = %d, want 3", len(got.EventLog))
	}
}

func TestRunNarrativeViewPrintsNarrativeContextJSON(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, inspectionWorld()); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"narrative-view",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--recent-events", "2",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	var got worldview.NarrativeContext
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not narrative context JSON: %v\n%s", err, stdout.String())
	}
	if got.World.ID != "test_world" {
		t.Fatalf("narrative world id = %q, want test_world", got.World.ID)
	}
	if len(got.RecentEvents) != 2 || got.RecentEvents[0].ID != "event_2" || got.RecentEvents[1].ID != "event_3" {
		t.Fatalf("recent events mismatch: %#v", got.RecentEvents)
	}
	if len(got.ActiveThreads) != 1 || got.ActiveThreads[0].ID != "thread_open" {
		t.Fatalf("active threads mismatch: %#v", got.ActiveThreads)
	}
	if len(got.PublicMemories) != 1 || got.PublicMemories[0].ID != "memory_world_public" {
		t.Fatalf("public memories mismatch: %#v", got.PublicMemories)
	}
}

func TestRunStepScriptPersistsAppliedEvents(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	eventsPath := filepath.Join(workspace, "events.json")
	const eventsJSON = `[
  {
    "id": "event_1",
    "type": "world_fact_changed",
    "source": "director",
    "effects": [
      {
        "kind": "set_fact",
        "target_id": "fact_1",
        "payload": {
          "subject_id": {"kind": "entity_ref", "raw": "tower"},
          "predicate": {"kind": "string", "raw": "status"},
          "value": {"kind": "string", "raw": "sealed"}
        }
      }
    ]
  }
]`
	if err := os.WriteFile(eventsPath, []byte(eventsJSON), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"step-script",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--events-file", eventsPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	world, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(world.Facts) != 1 || world.Facts[0].ID != "fact_1" {
		t.Fatalf("fact not persisted: %#v", world.Facts)
	}
	if len(world.EventLog) != 1 || world.EventLog[0].ID != "event_1" {
		t.Fatalf("event not persisted: %#v", world.EventLog)
	}
	if want := "applied 1 events to world test_world\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunStepScriptDoesNotPersistFailedStep(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	eventsPath := filepath.Join(workspace, "events.json")
	writeTestJSON(t, eventsPath, []model.WorldEvent{
		{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
		{ID: "event_2"},
	})

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"step-script",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--events-file", eventsPath,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("Run exit code = 0, want failure; stdout=%s", stdout.String())
	}

	world, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(world.EventLog) != 0 {
		t.Fatalf("failed step was persisted: %#v", world.EventLog)
	}
}

func TestRunStepReconcilePersistsConfiguredCases(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	initial := model.World{
		ID:   "test_world",
		Name: "Test World",
		Memory: []model.MemoryRecord{{
			ID:          "memory_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
			Scope:       model.MemoryScopeSubjective,
			Kind:        model.MemoryKindBelief,
			Content:     "A killed the king.",
			TruthStatus: model.TruthStatusUnknown,
			Confidence:  0.8,
		}},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	casesPath := filepath.Join(workspace, "reconcile_cases.json")
	const casesJSON = `[
  {
    "event_id": "event_reconcile_1",
    "target_memory_id": "memory_1",
    "when_truth_status": "unknown",
    "truth_status": "disputed",
    "confidence_delta": -0.5,
    "summary": "New evidence disputes C's old belief.",
    "add_memory_id": "memory_2",
    "add_memory_content": "C starts to suspect A was framed."
  }
]`
	if err := os.WriteFile(casesPath, []byte(casesJSON), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"step-reconcile",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--cases-file", casesPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	world, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(world.Memory) != 2 {
		t.Fatalf("Memory length = %d, want 2: %#v", len(world.Memory), world.Memory)
	}
	if world.Memory[0].TruthStatus != model.TruthStatusDisputed {
		t.Fatalf("memory truth status not reconciled: %#v", world.Memory[0])
	}
	if world.Memory[0].Confidence < 0.299999 || world.Memory[0].Confidence > 0.300001 {
		t.Fatalf("memory confidence = %v, want 0.3", world.Memory[0].Confidence)
	}
	if world.Memory[1].ID != "memory_2" || world.Memory[1].Content != "C starts to suspect A was framed." {
		t.Fatalf("follow-up memory mismatch: %#v", world.Memory[1])
	}
	if len(world.EventLog) != 1 || world.EventLog[0].ID != "event_reconcile_1" {
		t.Fatalf("reconcile event not persisted: %#v", world.EventLog)
	}
	if want := "applied 1 reconciliation events to world test_world\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunStepConfigPersistsConfiguredDirectors(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	initial := model.World{
		ID:   "test_world",
		Name: "Test World",
		Memory: []model.MemoryRecord{{
			ID:          "memory_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
			Scope:       model.MemoryScopeSubjective,
			Kind:        model.MemoryKindBelief,
			Content:     "A killed the king.",
			TruthStatus: model.TruthStatusUnknown,
			Confidence:  0.8,
		}},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	configPath := filepath.Join(workspace, "directors.json")
	const configJSON = `{
  "directors": [
    {
      "id": "script_1",
      "kind": "script",
      "events": [
        {
          "id": "event_script_1",
          "type": "world_fact_changed",
          "source": "director",
          "effects": [
            {
              "kind": "set_fact",
              "target_id": "fact_1",
              "payload": {
                "subject_id": {"kind": "entity_ref", "raw": "tower"},
                "predicate": {"kind": "string", "raw": "status"},
                "value": {"kind": "string", "raw": "sealed"}
              }
            }
          ]
        }
      ]
    },
    {
      "id": "reconcile_1",
      "kind": "reconcile",
      "cases": [
        {
          "event_id": "event_reconcile_1",
          "target_memory_id": "memory_1",
          "when_truth_status": "unknown",
          "truth_status": "disputed",
          "confidence_delta": -0.5
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"step-config",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--config-file", configPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	world, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(world.Facts) != 1 || world.Facts[0].ID != "fact_1" {
		t.Fatalf("script fact not persisted: %#v", world.Facts)
	}
	if len(world.Memory) != 1 || world.Memory[0].TruthStatus != model.TruthStatusDisputed {
		t.Fatalf("reconcile memory not persisted: %#v", world.Memory)
	}
	if len(world.EventLog) != 2 || world.EventLog[0].ID != "event_script_1" || world.EventLog[1].ID != "event_reconcile_1" {
		t.Fatalf("configured events not persisted in order: %#v", world.EventLog)
	}
	if want := "applied 2 configured events to world test_world\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunRunPersistsMultipleConfiguredSteps(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{
		ID:   "test_world",
		Name: "Test World",
		Clock: model.WorldClock{
			Current:  model.WorldTime{Kind: model.WorldTimeTick, Tick: 0},
			Sequence: 0,
		},
	}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	configPath := filepath.Join(workspace, "directors.json")
	writeTestJSON(t, configPath, json.RawMessage(`{"directors":[{"kind":"script","id":"s1","events":[{"id":"event_s","type":"note","source":"director"}]}]}`))

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"run",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--config-file", configPath,
		"--steps", "3",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	world, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if world.Clock.Sequence != 3 {
		t.Fatalf("Clock.Sequence = %d, want 3", world.Clock.Sequence)
	}
	if len(world.EventLog) != 3 {
		t.Fatalf("EventLog length = %d, want 3", len(world.EventLog))
	}
	if want := "ran 3 steps on world test_world (3 events applied)\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunBridgeContextOutputsNarrativeBundle(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{
		ID:   "test_world",
		Name: "The Fall of Kingdoms",
		Canon: model.Canon{
			Genre: []string{"fantasy"},
			Laws:  []string{"Magic requires sacrifice"},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {ID: "char_alice", Type: "character", Name: "Alice"},
			"tavern":     {ID: "tavern", Type: "location", Name: "The Tavern"},
		},
		Threads: []model.WorldThread{
			{ID: "thread_1", Kind: model.ThreadKindMystery, Title: "Find the killer", Status: model.ThreadStatusActive},
		},
	}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"bridge-context",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run exit code = %d, stderr=%s", code, stderr.String())
	}

	var bundle map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &bundle); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
	}
	world, ok := bundle["world"].(map[string]any)
	if !ok {
		t.Fatalf("bundle missing world: %#v", bundle)
	}
	if world["title"] != "The Fall of Kingdoms" {
		t.Fatalf("world.title = %v", world["title"])
	}
	characters, ok := bundle["characters"].([]any)
	if !ok || len(characters) != 1 {
		t.Fatalf("characters mismatch: %#v", bundle["characters"])
	}
	locations, ok := bundle["locations"].([]any)
	if !ok || len(locations) != 1 {
		t.Fatalf("locations mismatch: %#v", bundle["locations"])
	}
}

func writeTestJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func inspectionWorld() model.World {
	return model.World{
		ID:   "test_world",
		Name: "Test World",
		Facts: []model.Fact{{
			ID:        "fact_1",
			SubjectID: "char_c",
			Predicate: "location",
			Value:     model.Value{Kind: model.ValueKindString, Raw: "tower"},
		}},
		Threads: []model.WorldThread{
			{ID: "thread_open", Kind: model.ThreadKindMystery, Title: "Open mystery", Status: model.ThreadStatusOpen},
			{ID: "thread_done", Kind: model.ThreadKindQuest, Title: "Done quest", Status: model.ThreadStatusResolved},
		},
		EventLog: []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest},
			{ID: "event_2", Type: model.EventTypeNote, Source: model.EventSourceTest},
			{ID: "event_3", Type: model.EventTypeNote, Source: model.EventSourceTest},
		},
		Memory: []model.MemoryRecord{
			{
				ID:          "memory_world_public",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "The tower bell rang.",
				TruthStatus: model.TruthStatusTrue,
			},
			{
				ID:          "memory_world_secret",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "The assassin escaped.",
				TruthStatus: model.TruthStatusSecret,
			},
			{
				ID:          "memory_character_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "I am being followed.",
				TruthStatus: model.TruthStatusUnknown,
			},
		},
	}
}
