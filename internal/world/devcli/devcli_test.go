package devcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	bridgenarrative "github.com/sizolity/nobody/internal/bridge/narrative"
	"github.com/sizolity/nobody/internal/narrative/engine"
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

func TestRunStepLLMRequiresAPIKey(t *testing.T) {
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	t.Setenv("DEEPSEEK_API_KEY", "")
	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"step-llm",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("step-llm should fail without DEEPSEEK_API_KEY")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("DEEPSEEK_API_KEY")) {
		t.Fatalf("stderr should mention DEEPSEEK_API_KEY: %s", stderr.String())
	}
}

func TestRunStepLLMRequiresFlags(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"step-llm"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("step-llm with no flags should exit 2, got %d", code)
	}
}

func TestRunStepConfigWithLLMKindRequiresAPIKey(t *testing.T) {
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, model.World{ID: "test_world", Name: "Test World"}); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	configPath := filepath.Join(workspace, "directors.json")
	const configJSON = `{
  "directors": [
    {"id": "llm_1", "kind": "llm", "provider": "deepseek"}
  ]
}`
	if err := os.WriteFile(configPath, []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("DEEPSEEK_API_KEY", "")
	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"step-config",
		"--workspace", workspace,
		"--world-id", "test_world",
		"--config-file", configPath,
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("step-config with llm kind should fail without DEEPSEEK_API_KEY")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("DEEPSEEK_API_KEY")) {
		t.Fatalf("stderr should mention DEEPSEEK_API_KEY: %s", stderr.String())
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

func TestRunCheckpointAndRollback(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	world := model.World{
		ID:    "test_world",
		Name:  "Before Checkpoint",
		Clock: model.WorldClock{Sequence: 5},
		EventLog: []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest},
		},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"checkpoint",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("checkpoint exit code = %d, stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("sequence 5")) {
		t.Fatalf("checkpoint output missing sequence: %s", stdout.String())
	}

	world.Name = "After Change"
	world.Clock.Sequence = 10
	world.EventLog = append(world.EventLog, model.WorldEvent{
		ID: "event_2", Type: model.EventTypeNote, Source: model.EventSourceTest,
	})
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{
		"rollback",
		"--workspace", workspace,
		"--world-id", "test_world",
		"5",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rollback exit code = %d, stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("sequence 5")) {
		t.Fatalf("rollback output missing sequence: %s", stdout.String())
	}

	restored, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if restored.Name != "Before Checkpoint" {
		t.Fatalf("world not rolled back: name = %q", restored.Name)
	}
	if len(restored.EventLog) != 1 {
		t.Fatalf("event log not rolled back: %d events", len(restored.EventLog))
	}
}

func TestRunListCheckpoints(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	world := model.World{ID: "test_world", Name: "Test World", Clock: model.WorldClock{Sequence: 3}}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"list-checkpoints",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list-checkpoints exit code = %d, stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("no checkpoints")) {
		t.Fatalf("expected 'no checkpoints', got: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{
		"checkpoint",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("checkpoint exit code = %d, stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{
		"list-checkpoints",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list-checkpoints exit code = %d, stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("checkpoint at sequence 3")) {
		t.Fatalf("expected checkpoint listing, got: %s", stdout.String())
	}
}

func TestRunForkFromCurrentState(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	world := model.World{
		ID:    "source_w",
		Name:  "Source",
		Clock: model.WorldClock{Sequence: 4},
		Entities: map[model.EntityID]model.Entity{
			"hero": {ID: "hero", Type: "character", Name: "Hero"},
		},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"fork",
		"--workspace", workspace,
		"--world-id", "source_w",
		"--new-id", "branch_a",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("fork exit code = %d, stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("branch_a")) {
		t.Fatalf("fork output missing new ID: %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("sequence 4")) {
		t.Fatalf("fork output missing sequence: %s", stdout.String())
	}

	loaded, err := st.LoadSnapshot(ctx, "branch_a")
	if err != nil {
		t.Fatalf("LoadSnapshot branch_a returned error: %v", err)
	}
	if loaded.Name != "Source" {
		t.Fatalf("forked name = %q", loaded.Name)
	}
	if loaded.Metadata.Fork == nil {
		t.Fatal("forked world missing ForkInfo")
	}
}

func TestRunForkFromCheckpoint(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	world := model.World{
		ID:    "source_w",
		Name:  "At Seq 2",
		Clock: model.WorldClock{Sequence: 2},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	if _, err := st.SaveCheckpoint(ctx, "source_w"); err != nil {
		t.Fatalf("SaveCheckpoint returned error: %v", err)
	}

	world.Name = "At Seq 7"
	world.Clock.Sequence = 7
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"fork",
		"--workspace", workspace,
		"--world-id", "source_w",
		"--new-id", "branch_b",
		"--at-sequence", "2",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("fork exit code = %d, stderr=%s", code, stderr.String())
	}

	loaded, err := st.LoadSnapshot(ctx, "branch_b")
	if err != nil {
		t.Fatalf("LoadSnapshot branch_b returned error: %v", err)
	}
	if loaded.Name != "At Seq 2" {
		t.Fatalf("forked name = %q, want 'At Seq 2'", loaded.Name)
	}
}

func TestRunForkRequiresNewID(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"fork",
		"--workspace", workspace,
		"--world-id", "source",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRunRollbackRequiresSequenceArg(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"rollback",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

// --- beat command tests ---

func TestRunBeatRequiresFlags(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"beat"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("beat with no flags should exit 2, got %d", code)
	}
}

func TestRunBeatRequiresAPIKey(t *testing.T) {
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)
	if err := st.SaveSnapshot(ctx, inspectionWorld()); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	t.Setenv("DEEPSEEK_API_KEY", "")
	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"beat",
		"--workspace", workspace,
		"--world-id", "test_world",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("beat should fail without DEEPSEEK_API_KEY")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("DEEPSEEK_API_KEY")) {
		t.Fatalf("stderr should mention DEEPSEEK_API_KEY: %s", stderr.String())
	}
}

func TestRunBeatMissingWorld(t *testing.T) {
	workspace := t.TempDir()
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"beat",
		"--workspace", workspace,
		"--world-id", "nonexistent",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 for missing world, got %d", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("load world")) {
		t.Fatalf("stderr should mention load world: %s", stderr.String())
	}
}

func TestExecuteBeatPipelineReturnsOutput(t *testing.T) {
	t.Parallel()

	world := inspectionWorld()
	bundle := bridgenarrative.AdaptWorld(world, bridgenarrative.Options{RecentEvents: 10})

	gen := &beatFakeGenerator{responses: []string{
		`{"beat_id":"beat_test","objective":"Test beat","target_node_id":"thread_open"}`,
		`{"id":"draft_1","beat_id":"beat_test","title":"Test Scene","kind":"scene","text":"The world shifted."}`,
		`{"issues":[{"code":"TONE_MISMATCH","summary":"Too dark for a lighthearted world."}]}`,
		`{"events":[{"id":"ev_1","beat_id":"beat_test","type":"scene","summary":"World shifted."}],"memories":[{"id":"mem_1","type":"observation","subject":"world","text":"Something changed.","importance":3}]}`,
		`{"graph":{"current_node_id":"thread_open","nodes":[{"id":"thread_open","type":"mystery","status":"active","goal":"Open mystery"}]}}`,
	}}

	var stderr bytes.Buffer
	result, err := executeBeatPipeline(context.Background(), gen, world, bundle, nil, 0, &stderr)
	if err != nil {
		t.Fatalf("executeBeatPipeline error: %v, stderr=%s", err, stderr.String())
	}

	out := result.Output
	if out.WorldID != "test_world" {
		t.Errorf("world_id = %q", out.WorldID)
	}
	if out.Plan.BeatID != "beat_test" {
		t.Errorf("plan.beat_id = %q", out.Plan.BeatID)
	}
	if out.Draft.Title != "Test Scene" {
		t.Errorf("draft.title = %q", out.Draft.Title)
	}
	if out.Draft.Text != "The world shifted." {
		t.Errorf("draft.text = %q", out.Draft.Text)
	}
	if len(out.ContinuityIssues) != 1 {
		t.Fatalf("continuity issues = %d, want 1", len(out.ContinuityIssues))
	}
	if out.ContinuityIssues[0].Code != "TONE_MISMATCH" {
		t.Errorf("issue code = %q", out.ContinuityIssues[0].Code)
	}
	if len(out.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(out.Events))
	}
	if out.Events[0].ID != "ev_1" {
		t.Errorf("event id = %q", out.Events[0].ID)
	}
	if len(out.Memories) != 1 {
		t.Fatalf("memories = %d, want 1", len(out.Memories))
	}
	if out.Memories[0].Importance != 3 {
		t.Errorf("memory importance = %d", out.Memories[0].Importance)
	}
	if out.Graph.CurrentNodeID != "thread_open" {
		t.Errorf("graph current = %q", out.Graph.CurrentNodeID)
	}
	if out.Graph.NodeCount != 1 {
		t.Errorf("graph nodes = %d", out.Graph.NodeCount)
	}

	for _, want := range []string{"beat planned", "draft written", "continuity:", "memory:", "graph:"} {
		if !bytes.Contains(stderr.Bytes(), []byte(want)) {
			t.Errorf("stderr missing %q: %s", want, stderr.String())
		}
	}
}

func TestExecuteBeatPipelineDirectorError(t *testing.T) {
	t.Parallel()

	world := inspectionWorld()
	bundle := bridgenarrative.AdaptWorld(world, bridgenarrative.Options{RecentEvents: 10})

	gen := &beatFakeGenerator{responses: []string{`not json`}}

	var stderr bytes.Buffer
	_, err := executeBeatPipeline(context.Background(), gen, world, bundle, nil, 0, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("director plan")) {
		t.Errorf("error should mention director plan: %v", err)
	}
}

func TestExecuteBeatPipelineApplyPersistsToWorld(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)

	world := model.World{
		ID: "w", Name: "W",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Find artifact", Status: model.ThreadStatusActive},
		},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	bundle := bridgenarrative.AdaptWorld(world, bridgenarrative.Options{RecentEvents: 10})
	gen := &beatFakeGenerator{responses: []string{
		`{"beat_id":"beat_apply","objective":"Test apply","target_node_id":"t1"}`,
		`{"id":"draft_1","beat_id":"beat_apply","title":"Applied Scene","kind":"scene","text":"Something happened."}`,
		`{"issues":[]}`,
		`{"events":[{"id":"ev_a","beat_id":"beat_apply","type":"scene","summary":"Scene played out."}],"memories":[{"id":"mem_a","type":"observation","subject":"world","text":"A change.","importance":5}]}`,
		`{"graph":{"current_node_id":"t1","nodes":[{"id":"t1","type":"quest","status":"completed","goal":"Find artifact"}]}}`,
	}}

	var stderr bytes.Buffer
	_, err := executeBeatPipeline(ctx, gen, world, bundle, st, 0, &stderr)
	if err != nil {
		t.Fatalf("error: %v, stderr=%s", err, stderr.String())
	}

	if !bytes.Contains(stderr.Bytes(), []byte("applied:")) {
		t.Error("stderr missing 'applied:' message")
	}

	updated, loadErr := st.LoadSnapshot(ctx, "w")
	if loadErr != nil {
		t.Fatalf("LoadSnapshot: %v", loadErr)
	}
	if updated.Clock.Sequence != 1 {
		t.Errorf("sequence = %d, want 1", updated.Clock.Sequence)
	}
	if len(updated.EventLog) < 2 {
		t.Fatalf("events = %d, want >= 2", len(updated.EventLog))
	}
	if len(updated.Memory) != 1 {
		t.Fatalf("memories = %d, want 1", len(updated.Memory))
	}
	if updated.Threads[0].Status != model.ThreadStatusResolved {
		t.Errorf("thread status = %q, want resolved", updated.Threads[0].Status)
	}
}

func TestExecuteBeatPipelineRewritesOnContinuityIssues(t *testing.T) {
	t.Parallel()

	world := inspectionWorld()
	bundle := bridgenarrative.AdaptWorld(world, bridgenarrative.Options{RecentEvents: 10})

	gen := &beatFakeGenerator{responses: []string{
		// 1: director plan
		`{"beat_id":"beat_rw","objective":"Test rewrite","target_node_id":"thread_open"}`,
		// 2: writer initial draft
		`{"id":"draft_v1","beat_id":"beat_rw","title":"Bad Scene","kind":"scene","text":"Bob in wrong place."}`,
		// 3: continuity check → finds issue
		`{"issues":[{"code":"LOCATION_MISMATCH","summary":"Bob should be elsewhere."}]}`,
		// 4: writer rewrite
		`{"id":"draft_v2","beat_id":"beat_rw","title":"Fixed Scene","kind":"scene","text":"Bob is at market."}`,
		// 5: continuity re-check → clean
		`{"issues":[]}`,
		// 6: memory extract
		`{"events":[{"id":"ev_rw","beat_id":"beat_rw","type":"scene","summary":"Scene."}],"memories":[]}`,
		// 7: state update
		`{"graph":{"current_node_id":"thread_open","nodes":[{"id":"thread_open","type":"mystery","status":"active","goal":"Open mystery"}]}}`,
	}}

	var stderr bytes.Buffer
	result, err := executeBeatPipeline(context.Background(), gen, world, bundle, nil, 2, &stderr)
	if err != nil {
		t.Fatalf("error: %v, stderr=%s", err, stderr.String())
	}

	if result.Output.Draft.Title != "Fixed Scene" {
		t.Errorf("draft title = %q, want Fixed Scene (rewritten)", result.Output.Draft.Title)
	}
	if len(result.Output.ContinuityIssues) != 0 {
		t.Errorf("final issues = %d, want 0", len(result.Output.ContinuityIssues))
	}

	stderrStr := stderr.String()
	if !bytes.Contains(stderr.Bytes(), []byte("rewriting (1/2)")) {
		t.Errorf("stderr missing rewrite progress: %s", stderrStr)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("continuity: clean")) {
		t.Errorf("stderr missing clean status: %s", stderrStr)
	}
}

func TestExecuteBeatPipelineRewriteExhausted(t *testing.T) {
	t.Parallel()

	world := inspectionWorld()
	bundle := bridgenarrative.AdaptWorld(world, bridgenarrative.Options{RecentEvents: 10})

	gen := &beatFakeGenerator{responses: []string{
		// 1: director plan
		`{"beat_id":"beat_ex","objective":"Exhausted","target_node_id":"thread_open"}`,
		// 2: writer draft
		`{"id":"draft_v1","beat_id":"beat_ex","title":"Scene","kind":"scene","text":"Text."}`,
		// 3: continuity → issue
		`{"issues":[{"code":"RULE_VIOLATION","summary":"Problem."}]}`,
		// 4: rewrite 1
		`{"id":"draft_v2","beat_id":"beat_ex","title":"Scene v2","kind":"scene","text":"Still bad."}`,
		// 5: continuity → still issue
		`{"issues":[{"code":"RULE_VIOLATION","summary":"Still a problem."}]}`,
		// 6: memory (proceeds despite issues)
		`{"events":[{"id":"ev","beat_id":"beat_ex","type":"scene","summary":"S."}],"memories":[]}`,
		// 7: state
		`{"graph":{"current_node_id":"thread_open","nodes":[{"id":"thread_open","type":"mystery","status":"active","goal":"g"}]}}`,
	}}

	var stderr bytes.Buffer
	result, err := executeBeatPipeline(context.Background(), gen, world, bundle, nil, 1, &stderr)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(result.Output.ContinuityIssues) != 1 {
		t.Errorf("issues = %d, want 1 (exhausted rewrites)", len(result.Output.ContinuityIssues))
	}
	if result.Output.Draft.Title != "Scene v2" {
		t.Errorf("draft = %q, want last rewrite", result.Output.Draft.Title)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("remaining (proceeding)")) {
		t.Errorf("stderr missing 'remaining' message: %s", stderr.String())
	}
}

func TestRunBeatMultiStepAccumulatesResults(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)

	world := model.World{
		ID: "w", Name: "W",
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusActive},
		},
	}
	if err := st.SaveSnapshot(ctx, world); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	beatResponses := func(beatID string) []string {
		return []string{
			fmt.Sprintf(`{"beat_id":"%s","objective":"Objective","target_node_id":"t1"}`, beatID),
			fmt.Sprintf(`{"id":"draft_%s","beat_id":"%s","title":"Scene %s","kind":"scene","text":"Text."}`, beatID, beatID, beatID),
			`{"issues":[]}`,
			fmt.Sprintf(`{"events":[{"id":"ev_%s","beat_id":"%s","type":"scene","summary":"Sum."}],"memories":[]}`, beatID, beatID),
			`{"graph":{"current_node_id":"t1","nodes":[{"id":"t1","type":"quest","status":"active","goal":"Quest"}]}}`,
		}
	}
	allResponses := append(beatResponses("b1"), beatResponses("b2")...)
	allResponses = append(allResponses, beatResponses("b3")...)

	gen := &beatFakeGenerator{responses: allResponses}
	genFactory := func(_, _ string) (engine.TextGenerator, error) { return gen, nil }

	_ = genFactory

	var stdout, stderr bytes.Buffer
	bundle := bridgenarrative.AdaptWorld(world, bridgenarrative.Options{RecentEvents: 10})

	results := make([]beatOutput, 0, 3)
	for step := 0; step < 3; step++ {
		w, err := st.LoadSnapshot(ctx, "w")
		if err != nil {
			t.Fatalf("LoadSnapshot step %d: %v", step, err)
		}
		bundle = bridgenarrative.AdaptWorld(w, bridgenarrative.Options{RecentEvents: 10})
		result, pErr := executeBeatPipeline(ctx, gen, w, bundle, st, 0, &stderr)
		if pErr != nil {
			t.Fatalf("step %d: %v", step, pErr)
		}
		results = append(results, result.Output)
	}

	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if results[0].Plan.BeatID != "b1" {
		t.Errorf("step 0 beat_id = %q", results[0].Plan.BeatID)
	}
	if results[2].Plan.BeatID != "b3" {
		t.Errorf("step 2 beat_id = %q", results[2].Plan.BeatID)
	}

	final, err := st.LoadSnapshot(ctx, "w")
	if err != nil {
		t.Fatalf("LoadSnapshot final: %v", err)
	}
	if final.Clock.Sequence != 3 {
		t.Errorf("final sequence = %d, want 3", final.Clock.Sequence)
	}
	if len(final.EventLog) < 6 {
		t.Errorf("final events = %d, want >= 6 (2 per beat × 3)", len(final.EventLog))
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	stdout.Write(data)
	var parsed []beatOutput
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal array: %v", err)
	}
	if len(parsed) != 3 {
		t.Fatalf("parsed = %d, want 3", len(parsed))
	}
}

func TestRunBeatStepsRequiresPositive(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"beat",
		"--workspace", t.TempDir(),
		"--world-id", "w",
		"--steps", "0",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

type beatFakeGenerator struct {
	responses []string
	callIdx   int
}

func (g *beatFakeGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	if g.callIdx >= len(g.responses) {
		return `{}`, nil
	}
	resp := g.responses[g.callIdx]
	g.callIdx++
	return resp, nil
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
