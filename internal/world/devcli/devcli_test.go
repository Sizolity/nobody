package devcli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// NOTE: world-template support (TestRunInitWithTemplate / TestRunInitUnknownTemplate)
// was moved out of this framework CLI when the RPG product was split into the
// Worldline repository. Product-specific world templates now live next to their
// products (e.g. github.com/sizolity/worldline/rpg/template/). devcli's `init`
// stays template-free on purpose.

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

func TestRunLineageRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"lineage"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunLineageUnknownQuery(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"lineage", "--workspace", workspace, "--world-id", "w", "--query", "bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunLineageAncestorsAndChildren(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)

	root := model.World{ID: "root", Name: "Root"}
	child := model.World{
		ID: "child_a", Name: "Child",
		Metadata: model.WorldMetadata{Fork: &model.ForkInfo{ParentWorldID: "root", ForkSequence: 1}},
	}
	if err := st.SaveSnapshot(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSnapshot(ctx, child); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"lineage", "--workspace", workspace, "--world-id", "child_a", "--query", "ancestors"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}

	var result struct {
		WorldID string `json:"world_id"`
		Query   string `json:"query"`
		Nodes   []struct {
			WorldID string `json:"world_id"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, stdout.String())
	}
	if result.Query != "ancestors" {
		t.Errorf("query = %q, want ancestors", result.Query)
	}
	if len(result.Nodes) != 1 || result.Nodes[0].WorldID != "root" {
		t.Errorf("nodes = %v, want [root]", result.Nodes)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{"lineage", "--workspace", workspace, "--world-id", "root", "--query", "children"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("children exit = %d", code)
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal children: %v", err)
	}
	if len(result.Nodes) != 1 || result.Nodes[0].WorldID != "child_a" {
		t.Errorf("children nodes = %v, want [child_a]", result.Nodes)
	}
}

func TestRunDiffRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"diff"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunDiffShowsChanges(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)

	a := model.World{
		ID: "wa", Name: "World A",
		Clock: model.WorldClock{Sequence: 3},
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusActive},
		},
	}
	b := model.World{
		ID: "wb", Name: "World B",
		Clock: model.WorldClock{Sequence: 7},
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice the Brave"},
			"e2": {ID: "e2", Type: "location", Name: "Market"},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest", Status: model.ThreadStatusResolved},
		},
	}
	if err := st.SaveSnapshot(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSnapshot(ctx, b); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"diff", "--workspace", workspace, "--world-a", "wa", "--world-b", "wb"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}

	var diff struct {
		WorldA   string `json:"world_a"`
		WorldB   string `json:"world_b"`
		ClockA   int64  `json:"clock_a"`
		ClockB   int64  `json:"clock_b"`
		Entities struct {
			Added   []string `json:"added"`
			Changed []struct {
				ID string `json:"id"`
			} `json:"changed"`
		} `json:"entities"`
		Threads struct {
			StatusChanged []struct {
				ID      string `json:"id"`
				StatusA string `json:"status_a"`
				StatusB string `json:"status_b"`
			} `json:"status_changed"`
		} `json:"threads"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &diff); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, stdout.String())
	}
	if diff.ClockA != 3 || diff.ClockB != 7 {
		t.Errorf("clocks = %d/%d, want 3/7", diff.ClockA, diff.ClockB)
	}
	if len(diff.Entities.Added) != 1 || diff.Entities.Added[0] != "e2" {
		t.Errorf("entities added = %v, want [e2]", diff.Entities.Added)
	}
	if len(diff.Entities.Changed) != 1 || diff.Entities.Changed[0].ID != "e1" {
		t.Errorf("entities changed = %v, want [e1]", diff.Entities.Changed)
	}
	if len(diff.Threads.StatusChanged) != 1 {
		t.Fatalf("status_changed = %d, want 1", len(diff.Threads.StatusChanged))
	}
	if diff.Threads.StatusChanged[0].StatusB != model.ThreadStatusResolved {
		t.Errorf("status_b = %q", diff.Threads.StatusChanged[0].StatusB)
	}
}

func TestRunDiffTextFormat(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)

	a := model.World{
		ID: "wa", Name: "World A",
		Clock:    model.WorldClock{Sequence: 3},
		Entities: map[model.EntityID]model.Entity{"e1": {ID: "e1", Type: "character", Name: "Alice"}},
	}
	b := model.World{
		ID: "wb", Name: "World B",
		Clock: model.WorldClock{Sequence: 5},
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice the Brave"},
			"e2": {ID: "e2", Type: "location", Name: "Market"},
		},
	}
	if err := st.SaveSnapshot(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSnapshot(ctx, b); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"diff", "--workspace", workspace, "--world-a", "wa", "--world-b", "wb", "--format", "text"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "diff wa → wb") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "clock: 3 → 5") {
		t.Errorf("missing clock:\n%s", out)
	}
	if !strings.Contains(out, "+ entity e2") {
		t.Errorf("missing added entity:\n%s", out)
	}
	if !strings.Contains(out, "~ entity e1") {
		t.Errorf("missing changed entity:\n%s", out)
	}
}

func TestRunMergeRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"merge"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRunMergeNoConflict(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)

	base := model.World{
		ID: "base", Name: "Base",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
	}
	source := model.World{
		ID: "source", Name: "Source",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
			"e2": {ID: "e2", Type: "location", Name: "Market"},
		},
	}
	target := model.World{
		ID: "target", Name: "Target",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
	}
	for _, w := range []model.World{base, source, target} {
		if err := st.SaveSnapshot(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"merge", "--workspace", workspace, "--base", "base", "--source", "source", "--target", "target"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}

	var result struct {
		Report struct {
			Conflicts     []any    `json:"conflicts"`
			EntitiesAdded []string `json:"entities_added"`
		} `json:"report"`
		Merged *struct {
			ID string `json:"id"`
		} `json:"merged"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, stdout.String())
	}
	if len(result.Report.Conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(result.Report.Conflicts))
	}
	if len(result.Report.EntitiesAdded) != 1 || result.Report.EntitiesAdded[0] != "e2" {
		t.Errorf("entities_added = %v, want [e2]", result.Report.EntitiesAdded)
	}
	if result.Merged == nil || result.Merged.ID != "target" {
		t.Error("merged world should be present without --apply")
	}
}

func TestRunMergeApplyBlockedByConflict(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)

	base := model.World{
		ID: "base", Name: "Base",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
	}
	source := model.World{
		ID: "source", Name: "Source",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice X"},
		},
	}
	target := model.World{
		ID: "target", Name: "Target",
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice Y"},
		},
	}
	for _, w := range []model.World{base, source, target} {
		if err := st.SaveSnapshot(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"merge", "--workspace", workspace, "--base", "base", "--source", "source", "--target", "target", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (outputs report)", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("refusing to apply")) {
		t.Errorf("stderr should mention refusing to apply: %s", stderr.String())
	}
}

func TestRunMergeApplySucceeds(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	st := store.NewFileStore(workspace)

	base := model.World{
		ID: "base", Name: "Base", Clock: model.WorldClock{Sequence: 3},
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
	}
	source := model.World{
		ID: "source", Name: "Source", Clock: model.WorldClock{Sequence: 6},
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
			"e2": {ID: "e2", Type: "location", Name: "Market"},
		},
	}
	target := model.World{
		ID: "target", Name: "Target", Clock: model.WorldClock{Sequence: 5},
		Entities: map[model.EntityID]model.Entity{
			"e1": {ID: "e1", Type: "character", Name: "Alice"},
		},
	}
	for _, w := range []model.World{base, source, target} {
		if err := st.SaveSnapshot(ctx, w); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"merge", "--workspace", workspace, "--base", "base", "--source", "source", "--target", "target", "--apply"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}

	updated, err := st.LoadSnapshot(ctx, "target")
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if _, ok := updated.Entities["e2"]; !ok {
		t.Error("e2 should have been merged into target")
	}
	if updated.Clock.Sequence != 6 {
		t.Errorf("clock = %d, want 6", updated.Clock.Sequence)
	}
}

func TestRunValidateRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runValidate(context.Background(), []string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunValidateCleanWorld(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Clean",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
		},
		Relations: []model.Relation{},
		Facts:     []model.Fact{{ID: "f1", SubjectID: "char_a", Predicate: "alive", Value: model.Value{Kind: model.ValueKindBoolean, Raw: true}}},
	}
	fs := store.NewFileStore(workspace)
	if err := fs.SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runValidate(context.Background(), []string{"--workspace", workspace, "--world-id", "w1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "clean") {
		t.Errorf("expected 'clean' in stderr, got: %s", stderr.String())
	}
}

func TestRunValidateBrokenRef(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Broken",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
		},
		Relations: []model.Relation{
			{ID: "rel_1", Type: "ally", SourceID: "char_a", TargetID: "nonexistent"},
		},
	}
	fs := store.NewFileStore(workspace)
	if err := fs.SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runValidate(context.Background(), []string{"--workspace", workspace, "--world-id", "w1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "error") {
		t.Errorf("expected 'error' in stderr, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "nonexistent") {
		t.Errorf("expected 'nonexistent' in output, got: %s", stdout.String())
	}
}

func TestRunExportRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runExport(context.Background(), []string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunExportImportRoundTrip(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()

	w := model.World{
		ID: "w1", Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "Alice"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "char_a", Predicate: "alive", Value: model.Value{Kind: model.ValueKindBoolean, Raw: true}},
		},
		Clock: model.WorldClock{Sequence: 3},
	}
	fs := store.NewFileStore(workspace)
	if err := fs.SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(t.TempDir(), "w1.tar.gz")

	var expOut, expErr bytes.Buffer
	code := runExport(ctx, []string{"--workspace", workspace, "--world-id", "w1", "--output", archivePath}, &expOut, &expErr)
	if code != 0 {
		t.Fatalf("export exit %d; stderr=%s", code, expErr.String())
	}
	if !strings.Contains(expErr.String(), "exported") {
		t.Errorf("expected 'exported' in stderr, got: %s", expErr.String())
	}

	var impOut, impErr bytes.Buffer
	code = runImport(ctx, []string{"--workspace", workspace, "--input", archivePath, "--new-id", "w1_copy"}, &impOut, &impErr)
	if code != 0 {
		t.Fatalf("import exit %d; stderr=%s", code, impErr.String())
	}
	if !strings.Contains(impErr.String(), "imported") {
		t.Errorf("expected 'imported' in stderr, got: %s", impErr.String())
	}
	if !strings.Contains(impOut.String(), "w1_copy") {
		t.Errorf("expected 'w1_copy' in output, got: %s", impOut.String())
	}

	loaded, err := fs.LoadSnapshot(ctx, "w1_copy")
	if err != nil {
		t.Fatalf("load imported: %v", err)
	}
	if loaded.Name != "Test World" {
		t.Errorf("name = %q", loaded.Name)
	}
	if len(loaded.Entities) != 1 {
		t.Errorf("entities = %d", len(loaded.Entities))
	}
}

func TestRunImportRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runImport(context.Background(), []string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunDrainQueueRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runDrainQueue(context.Background(), []string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunDrainQueueEmptyQueue(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runDrainQueue(ctx, []string{"--workspace", workspace, "--world-id", "w1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "empty") {
		t.Errorf("expected 'empty' in stderr, got: %s", stderr.String())
	}
}

func TestRunDrainQueueProcessesEvents(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{},
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "q1", Type: model.EventTypeNote, Source: model.EventSourceRuntime}, Priority: 10},
			{Event: model.WorldEvent{ID: "q2", Type: model.EventTypeNote, Source: model.EventSourceRuntime}, Priority: 5},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runDrainQueue(ctx, []string{"--workspace", workspace, "--world-id", "w1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "2 applied") {
		t.Errorf("expected '2 applied' in stderr, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "saved") {
		t.Errorf("expected 'saved' in stderr, got: %s", stderr.String())
	}

	loaded, err := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.EventQueue) != 0 {
		t.Errorf("queue should be empty, got %d", len(loaded.EventQueue))
	}
	if len(loaded.EventLog) != 2 {
		t.Errorf("event log = %d, want 2", len(loaded.EventLog))
	}
}

func TestRunDrainQueueWithLimit(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{},
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "q1", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
			{Event: model.WorldEvent{ID: "q2", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
			{Event: model.WorldEvent{ID: "q3", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runDrainQueue(ctx, []string{"--workspace", workspace, "--world-id", "w1", "--limit", "2"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "2 applied") {
		t.Errorf("expected '2 applied' in stderr, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "1 remaining") {
		t.Errorf("expected '1 remaining' in stderr, got: %s", stderr.String())
	}
}

func TestRunDrainQueueDryRun(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{},
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "q1", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runDrainQueue(ctx, []string{"--workspace", workspace, "--world-id", "w1", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "dry-run") {
		t.Errorf("expected 'dry-run' in stderr, got: %s", stderr.String())
	}

	loaded, err := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.EventQueue) != 1 {
		t.Errorf("queue should still have 1 item (dry-run), got %d", len(loaded.EventQueue))
	}
}

func TestRunHistoryRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runHistory(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunHistoryTextOutput(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "History",
		Entities: map[model.EntityID]model.Entity{
			"char1": {ID: "char1", Name: "Alice", Type: "character"},
		},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "Storm approached.", ActorIDs: []model.EntityID{"char1"}},
			{ID: "ev2", Type: model.EventTypeMove, Source: model.EventSourceRuntime, Description: "Alice moved.", LocationID: "char1"},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runHistory(ctx, []string{"--workspace", workspace, "--world-id", "w1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected resolved name 'Alice':\n%s", out)
	}
	if !strings.Contains(out, "Storm approached.") {
		t.Errorf("expected description:\n%s", out)
	}
	if !strings.Contains(out, "[move]") {
		t.Errorf("expected event type:\n%s", out)
	}
}

func TestRunHistoryJSONOutput(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "J",
		Entities: map[model.EntityID]model.Entity{},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceUser, Intent: "look around"},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runHistory(ctx, []string{"--workspace", workspace, "--world-id", "w1", "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"summary": "look around"`) {
		t.Errorf("missing JSON summary:\n%s", stdout.String())
	}
}

func TestRunHistoryLastNFilter(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "N",
		Entities: map[model.EntityID]model.Entity{},
		EventLog: []model.WorldEvent{
			{ID: "ev1", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "First."},
			{ID: "ev2", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "Second."},
			{ID: "ev3", Type: model.EventTypeNote, Source: model.EventSourceDirector, Description: "Third."},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runHistory(ctx, []string{"--workspace", workspace, "--world-id", "w1", "--last", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "First.") {
		t.Errorf("--last 1 should not show first event:\n%s", out)
	}
	if !strings.Contains(out, "Third.") {
		t.Errorf("--last 1 should show last event:\n%s", out)
	}
}

func TestRunSummarizeRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runSummarize(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunSummarizeOutput(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test World", Description: "A test.",
		Entities: map[model.EntityID]model.Entity{
			"char1": {ID: "char1", Name: "Alice", Type: "character"},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: "quest", Title: "Find treasure", Status: model.ThreadStatusActive},
		},
		Clock: model.WorldClock{Sequence: 5},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runSummarize(ctx, []string{"--workspace", workspace, "--world-id", "w1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "# Test World") {
		t.Errorf("missing title:\n%s", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("missing entity:\n%s", out)
	}
	if !strings.Contains(out, "Find treasure") {
		t.Errorf("missing thread:\n%s", out)
	}
}

func TestRunInspectEntityRequiresFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runInspectEntity(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunInspectEntityList(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"alice": {ID: "alice", Type: "character", Name: "Alice"},
			"tower": {ID: "tower", Type: "location", Name: "Tower"},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectEntity(ctx, []string{"--workspace", workspace, "--world-id", "w1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Tower") {
		t.Errorf("missing entities:\n%s", out)
	}
}

func TestRunInspectEntityDetail(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"alice": {ID: "alice", Type: "character", Name: "Alice", Description: "Brave."},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectEntity(ctx, []string{"--workspace", workspace, "--world-id", "w1", "--entity-id", "alice"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "# Alice") {
		t.Errorf("missing title:\n%s", out)
	}
	if !strings.Contains(out, "Brave.") {
		t.Errorf("missing description:\n%s", out)
	}
}

func TestRunInspectEntityNotFound(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectEntity(ctx, []string{"--workspace", workspace, "--world-id", "w1", "--entity-id", "missing"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found' in stderr: %s", stderr.String())
	}
}

func TestRunInspectEntityJSON(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"alice": {ID: "alice", Type: "character", Name: "Alice"},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectEntity(ctx, []string{"--workspace", workspace, "--world-id", "w1", "--entity-id", "alice", "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name": "Alice"`) {
		t.Errorf("missing JSON name:\n%s", stdout.String())
	}
}

func TestRunManageThreadNoSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runManageThread(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func threadTestWorld(t *testing.T) (string, model.World) {
	t.Helper()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Find treasure", Status: model.ThreadStatusActive},
			{ID: "t2", Kind: model.ThreadKindMystery, Title: "Who did it", Status: model.ThreadStatusOpen},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return workspace, w
}

func TestRunManageThreadListAll(t *testing.T) {
	t.Parallel()
	workspace, _ := threadTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageThread(context.Background(), []string{"list", "--workspace", workspace, "--world-id", "w1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Find treasure") || !strings.Contains(out, "Who did it") {
		t.Errorf("missing threads:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "2 thread(s)") {
		t.Errorf("missing count in stderr: %s", stderr.String())
	}
}

func TestRunManageThreadListFilterByStatus(t *testing.T) {
	t.Parallel()
	workspace, _ := threadTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageThread(context.Background(), []string{"list", "--workspace", workspace, "--world-id", "w1", "--status", "active"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Find treasure") {
		t.Errorf("missing active thread:\n%s", out)
	}
	if strings.Contains(out, "Who did it") {
		t.Errorf("open thread should be filtered out:\n%s", out)
	}
}

func TestRunManageThreadAdd(t *testing.T) {
	t.Parallel()
	workspace, _ := threadTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageThread(ctx, []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--thread-id", "t3", "--title", "New Quest", "--kind", "quest",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"title": "New Quest"`) {
		t.Errorf("missing JSON output:\n%s", stdout.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.Threads) != 3 {
		t.Fatalf("threads = %d, want 3", len(loaded.Threads))
	}
}

func TestRunManageThreadAddDuplicate(t *testing.T) {
	t.Parallel()
	workspace, _ := threadTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageThread(context.Background(), []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--thread-id", "t1", "--title", "Dup",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("expected 'already exists': %s", stderr.String())
	}
}

func TestRunManageThreadAddInvalid(t *testing.T) {
	t.Parallel()
	workspace, _ := threadTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageThread(context.Background(), []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--thread-id", "t3", "--title", "Bad", "--kind", "invalid_kind",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestRunManageThreadSetStatus(t *testing.T) {
	t.Parallel()
	workspace, _ := threadTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageThread(ctx, []string{"set-status",
		"--workspace", workspace, "--world-id", "w1",
		"--thread-id", "t1", "--status", "resolved",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "active → resolved") {
		t.Errorf("expected transition in stderr: %s", stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	for _, th := range loaded.Threads {
		if string(th.ID) == "t1" && th.Status != model.ThreadStatusResolved {
			t.Errorf("thread t1 status = %s, want resolved", th.Status)
		}
	}
}

func TestRunManageThreadSetStatusNotFound(t *testing.T) {
	t.Parallel()
	workspace, _ := threadTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageThread(context.Background(), []string{"set-status",
		"--workspace", workspace, "--world-id", "w1",
		"--thread-id", "missing", "--status", "resolved",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found': %s", stderr.String())
	}
}

func TestRunManageThreadSetStatusInvalid(t *testing.T) {
	t.Parallel()
	workspace, _ := threadTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageThread(context.Background(), []string{"set-status",
		"--workspace", workspace, "--world-id", "w1",
		"--thread-id", "t1", "--status", "bogus",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

// --- manage-memory tests ---

func memoryTestWorld(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{},
		Memory: []model.MemoryRecord{
			{
				ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Kind: model.MemoryKindObservation, Content: "The sun rose over the mountains",
				TruthStatus: model.TruthStatusTrue, Importance: 0.3, Confidence: 0.9,
			},
			{
				ID: "m2", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "alice"},
				Kind: model.MemoryKindBelief, Content: "I think there is treasure nearby",
				TruthStatus: model.TruthStatusUnknown, Importance: 0.8, Confidence: 0.5,
				SubjectIDs: []model.EntityID{"alice"},
			},
			{
				ID: "m3", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Kind: model.MemoryKindRumor, Summary: "A dragon was spotted",
				TruthStatus: model.TruthStatusSecret, Importance: 0.9, Confidence: 0.2,
			},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestRunManageMemoryNoSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunManageMemoryListAll(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "m1") || !strings.Contains(out, "m2") || !strings.Contains(out, "m3") {
		t.Errorf("missing memories:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "3 memory record(s)") {
		t.Errorf("missing count: %s", stderr.String())
	}
}

func TestRunManageMemoryListFilterOwnerKind(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
		"--owner-kind", "character",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "m2") {
		t.Errorf("missing character memory:\n%s", out)
	}
	if strings.Contains(out, "m1") || strings.Contains(out, "m3") {
		t.Errorf("world memories should be filtered out:\n%s", out)
	}
}

func TestRunManageMemoryListFilterKind(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
		"--kind", "rumor",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "m3") {
		t.Errorf("missing rumor memory:\n%s", out)
	}
	if strings.Contains(out, "m1") || strings.Contains(out, "m2") {
		t.Errorf("non-rumor memories should be filtered out:\n%s", out)
	}
}

func TestRunManageMemoryListJSON(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
		"--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "m1"`) {
		t.Errorf("missing JSON output:\n%s", stdout.String())
	}
}

func TestRunManageMemoryAdd(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageMemory(ctx, []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--memory-id", "m4", "--content", "A new discovery",
		"--owner-kind", "character", "--owner-id", "bob",
		"--importance", "0.7",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "m4"`) {
		t.Errorf("missing JSON output:\n%s", stdout.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.Memory) != 4 {
		t.Fatalf("memory count = %d, want 4", len(loaded.Memory))
	}
}

func TestRunManageMemoryAddDuplicate(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--memory-id", "m1", "--content", "dup",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("expected 'already exists': %s", stderr.String())
	}
}

func TestRunManageMemoryAddNoContent(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--memory-id", "m5",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunManageMemoryInspectText(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"inspect",
		"--workspace", workspace, "--world-id", "w1",
		"--memory-id", "m2",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "# Memory: m2") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "character") || !strings.Contains(out, "alice") {
		t.Errorf("missing owner info:\n%s", out)
	}
	if !strings.Contains(out, "treasure nearby") {
		t.Errorf("missing content:\n%s", out)
	}
	if !strings.Contains(out, "Subjects") {
		t.Errorf("missing subjects:\n%s", out)
	}
}

func TestRunManageMemoryInspectJSON(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"inspect",
		"--workspace", workspace, "--world-id", "w1",
		"--memory-id", "m1", "--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"content": "The sun rose`) {
		t.Errorf("missing JSON content:\n%s", stdout.String())
	}
}

func TestRunManageMemoryInspectNotFound(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageMemory(context.Background(), []string{"inspect",
		"--workspace", workspace, "--world-id", "w1",
		"--memory-id", "missing",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found': %s", stderr.String())
	}
}

// --- clock tests ---

func clockTestWorld(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Chrono",
		Entities: map[model.EntityID]model.Entity{},
		Clock: model.WorldClock{
			Current: model.WorldTime{
				Kind: model.WorldTimeTick, Tick: 10, Label: "Morning",
			},
			Calendar:  "standard",
			TimeScale: "1h/tick",
			Sequence:  5,
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestRunClockNoSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runClock(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunClockShowText(t *testing.T) {
	t.Parallel()
	workspace := clockTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runClock(context.Background(), []string{"show",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Sequence") || !strings.Contains(out, "5") {
		t.Errorf("missing sequence:\n%s", out)
	}
	if !strings.Contains(out, "Tick") || !strings.Contains(out, "10") {
		t.Errorf("missing tick:\n%s", out)
	}
	if !strings.Contains(out, "Morning") {
		t.Errorf("missing label:\n%s", out)
	}
	if !strings.Contains(out, "1h/tick") {
		t.Errorf("missing time scale:\n%s", out)
	}
}

func TestRunClockShowJSON(t *testing.T) {
	t.Parallel()
	workspace := clockTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runClock(context.Background(), []string{"show",
		"--workspace", workspace, "--world-id", "w1",
		"--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"sequence": 5`) {
		t.Errorf("missing JSON sequence:\n%s", stdout.String())
	}
}

func TestRunClockAdvance(t *testing.T) {
	t.Parallel()
	workspace := clockTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runClock(ctx, []string{"advance",
		"--workspace", workspace, "--world-id", "w1",
		"--ticks", "3", "--label", "Noon",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "10 → 13") {
		t.Errorf("expected tick transition: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "5 → 6") {
		t.Errorf("expected seq transition: %s", stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if loaded.Clock.Current.Tick != 13 {
		t.Errorf("tick = %d, want 13", loaded.Clock.Current.Tick)
	}
	if loaded.Clock.Current.Label != "Noon" {
		t.Errorf("label = %q, want Noon", loaded.Clock.Current.Label)
	}
	if loaded.Clock.Sequence != 6 {
		t.Errorf("sequence = %d, want 6", loaded.Clock.Sequence)
	}
}

func TestRunClockAdvanceDefaultTicks(t *testing.T) {
	t.Parallel()
	workspace := clockTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runClock(ctx, []string{"advance",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if loaded.Clock.Current.Tick != 11 {
		t.Errorf("tick = %d, want 11", loaded.Clock.Current.Tick)
	}
}

func TestRunClockAdvanceRejectsZero(t *testing.T) {
	t.Parallel()
	workspace := clockTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runClock(context.Background(), []string{"advance",
		"--workspace", workspace, "--world-id", "w1",
		"--ticks", "0",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunClockSet(t *testing.T) {
	t.Parallel()
	workspace := clockTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runClock(ctx, []string{"set",
		"--workspace", workspace, "--world-id", "w1",
		"--tick", "100", "--label", "Midnight", "--kind", "scene",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "3 field(s)") {
		t.Errorf("expected 3 fields changed: %s", stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if loaded.Clock.Current.Tick != 100 {
		t.Errorf("tick = %d, want 100", loaded.Clock.Current.Tick)
	}
	if loaded.Clock.Current.Label != "Midnight" {
		t.Errorf("label = %q, want Midnight", loaded.Clock.Current.Label)
	}
	if loaded.Clock.Current.Kind != model.WorldTimeScene {
		t.Errorf("kind = %q, want scene", loaded.Clock.Current.Kind)
	}
	if loaded.Clock.Sequence != 6 {
		t.Errorf("sequence = %d, want 6", loaded.Clock.Sequence)
	}
}

func TestRunClockSetNothingToSet(t *testing.T) {
	t.Parallel()
	workspace := clockTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runClock(context.Background(), []string{"set",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "nothing to set") {
		t.Errorf("expected 'nothing to set': %s", stderr.String())
	}
}

// --- manage-relation tests ---

func relationTestWorld(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"alice": {ID: "alice", Name: "Alice", Type: "character"},
			"bob":   {ID: "bob", Name: "Bob", Type: "character"},
			"carol": {ID: "carol", Name: "Carol", Type: "character"},
		},
		Relations: []model.Relation{
			{ID: "r1", Type: "ally", SourceID: "alice", TargetID: "bob"},
			{ID: "r2", Type: "enemy", SourceID: "bob", TargetID: "carol"},
			{ID: "r3", Type: "ally", SourceID: "alice", TargetID: "carol"},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestRunManageRelationNoSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runManageRelation(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunManageRelationListAll(t *testing.T) {
	t.Parallel()
	workspace := relationTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageRelation(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Bob") {
		t.Errorf("missing entity names:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "3 relation(s)") {
		t.Errorf("missing count: %s", stderr.String())
	}
}

func TestRunManageRelationListFilterEntity(t *testing.T) {
	t.Parallel()
	workspace := relationTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageRelation(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "bob",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "2 relation(s)") {
		t.Errorf("expected 2 relations for bob: %s", stderr.String())
	}
}

func TestRunManageRelationListFilterType(t *testing.T) {
	t.Parallel()
	workspace := relationTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageRelation(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
		"--type", "enemy",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "1 relation(s)") {
		t.Errorf("expected 1 enemy relation: %s", stderr.String())
	}
}

func TestRunManageRelationAdd(t *testing.T) {
	t.Parallel()
	workspace := relationTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageRelation(ctx, []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--relation-id", "r4", "--type", "mentor",
		"--source-id", "carol", "--target-id", "alice",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"type": "mentor"`) {
		t.Errorf("missing JSON:\n%s", stdout.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.Relations) != 4 {
		t.Fatalf("relations = %d, want 4", len(loaded.Relations))
	}
}

func TestRunManageRelationAddDuplicate(t *testing.T) {
	t.Parallel()
	workspace := relationTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageRelation(context.Background(), []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--relation-id", "r1", "--type", "x",
		"--source-id", "a", "--target-id", "b",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("expected 'already exists': %s", stderr.String())
	}
}

func TestRunManageRelationRemove(t *testing.T) {
	t.Parallel()
	workspace := relationTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageRelation(ctx, []string{"remove",
		"--workspace", workspace, "--world-id", "w1",
		"--relation-id", "r2",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.Relations) != 2 {
		t.Fatalf("relations = %d, want 2", len(loaded.Relations))
	}
	for _, r := range loaded.Relations {
		if string(r.ID) == "r2" {
			t.Error("r2 should have been removed")
		}
	}
}

func TestRunManageRelationRemoveNotFound(t *testing.T) {
	t.Parallel()
	workspace := relationTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageRelation(context.Background(), []string{"remove",
		"--workspace", workspace, "--world-id", "w1",
		"--relation-id", "missing",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found': %s", stderr.String())
	}
}

// --- manage-fact tests ---

func factTestWorld(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"alice": {ID: "alice", Name: "Alice", Type: "character"},
		},
		Facts: []model.Fact{
			{ID: "f1", SubjectID: "alice", Predicate: "age", Value: model.Value{Kind: model.ValueKindNumber, Raw: float64(25)}},
			{ID: "f2", SubjectID: "alice", Predicate: "occupation", Value: model.Value{Kind: model.ValueKindString, Raw: "warrior"}},
			{ID: "f3", SubjectID: "bob", Predicate: "alive", Value: model.Value{Kind: model.ValueKindBoolean, Raw: true}},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestRunManageFactNoSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runManageFact(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunManageFactListAll(t *testing.T) {
	t.Parallel()
	workspace := factTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageFact(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "age") || !strings.Contains(out, "occupation") || !strings.Contains(out, "alive") {
		t.Errorf("missing predicates:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "3 fact(s)") {
		t.Errorf("missing count: %s", stderr.String())
	}
}

func TestRunManageFactListFilterSubject(t *testing.T) {
	t.Parallel()
	workspace := factTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageFact(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
		"--subject-id", "alice",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "2 fact(s)") {
		t.Errorf("expected 2 facts for alice: %s", stderr.String())
	}
}

func TestRunManageFactListFilterPredicate(t *testing.T) {
	t.Parallel()
	workspace := factTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageFact(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
		"--predicate", "alive",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "1 fact(s)") {
		t.Errorf("expected 1 'alive' fact: %s", stderr.String())
	}
}

func TestRunManageFactAdd(t *testing.T) {
	t.Parallel()
	workspace := factTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageFact(ctx, []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--fact-id", "f4", "--subject-id", "alice",
		"--predicate", "hometown", "--value", "Riverwood",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"predicate": "hometown"`) {
		t.Errorf("missing JSON:\n%s", stdout.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.Facts) != 4 {
		t.Fatalf("facts = %d, want 4", len(loaded.Facts))
	}
}

func TestRunManageFactAddDuplicate(t *testing.T) {
	t.Parallel()
	workspace := factTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageFact(context.Background(), []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--fact-id", "f1", "--subject-id", "x",
		"--predicate", "y", "--value", "z",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("expected 'already exists': %s", stderr.String())
	}
}

func TestRunManageFactRemove(t *testing.T) {
	t.Parallel()
	workspace := factTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageFact(ctx, []string{"remove",
		"--workspace", workspace, "--world-id", "w1",
		"--fact-id", "f2",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.Facts) != 2 {
		t.Fatalf("facts = %d, want 2", len(loaded.Facts))
	}
	for _, f := range loaded.Facts {
		if string(f.ID) == "f2" {
			t.Error("f2 should have been removed")
		}
	}
}

func TestRunManageFactRemoveNotFound(t *testing.T) {
	t.Parallel()
	workspace := factTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageFact(context.Background(), []string{"remove",
		"--workspace", workspace, "--world-id", "w1",
		"--fact-id", "missing",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found': %s", stderr.String())
	}
}

// --- manage-queue tests ---

func queueTestWorld(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{},
		EventQueue: []model.EventQueueItem{
			{
				Event:       model.WorldEvent{ID: "q1", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Intent: "morning briefing"},
				Priority:    5,
				CreatedBy:   "system",
				ErrorPolicy: model.QueueErrorPolicySkip,
			},
			{
				Event:       model.WorldEvent{ID: "q2", Type: model.EventTypeMove, Source: model.EventSourceUser, Description: "Alice moves north"},
				Priority:    1,
				MaxAttempts: 3,
				ErrorPolicy: model.QueueErrorPolicyRetry,
			},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestRunManageQueueNoSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunManageQueueListAll(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "q1") || !strings.Contains(out, "q2") {
		t.Errorf("missing events:\n%s", out)
	}
	if !strings.Contains(out, "pri=5") {
		t.Errorf("missing priority:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "2 queued event(s)") {
		t.Errorf("missing count: %s", stderr.String())
	}
}

func TestRunManageQueueListJSON(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
		"--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "q1"`) {
		t.Errorf("missing JSON:\n%s", stdout.String())
	}
}

func TestRunManageQueueListEmpty(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	w := model.World{ID: "w1", Name: "E", Entities: map[model.EntityID]model.Entity{}}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "queue is empty") {
		t.Errorf("expected 'queue is empty': %s", stdout.String())
	}
}

func TestRunManageQueueInspectText(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"inspect",
		"--workspace", workspace, "--world-id", "w1",
		"--event-id", "q1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "# Queued Event: q1") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "note") {
		t.Errorf("missing type:\n%s", out)
	}
	if !strings.Contains(out, "morning briefing") {
		t.Errorf("missing intent:\n%s", out)
	}
	if !strings.Contains(out, "Priority") {
		t.Errorf("missing priority section:\n%s", out)
	}
}

func TestRunManageQueueInspectJSON(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"inspect",
		"--workspace", workspace, "--world-id", "w1",
		"--event-id", "q2", "--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "q2"`) {
		t.Errorf("missing JSON:\n%s", stdout.String())
	}
}

func TestRunManageQueueInspectNotFound(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"inspect",
		"--workspace", workspace, "--world-id", "w1",
		"--event-id", "missing",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found': %s", stderr.String())
	}
}

func TestRunManageQueueEnqueue(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageQueue(ctx, []string{"enqueue",
		"--workspace", workspace, "--world-id", "w1",
		"--event-id", "q3", "--type", "note",
		"--intent", "test event", "--priority", "10",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id": "q3"`) {
		t.Errorf("missing JSON:\n%s", stdout.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.EventQueue) != 3 {
		t.Fatalf("queue = %d, want 3", len(loaded.EventQueue))
	}
}

func TestRunManageQueueEnqueueDuplicate(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"enqueue",
		"--workspace", workspace, "--world-id", "w1",
		"--event-id", "q1", "--type", "note",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("expected 'already exists': %s", stderr.String())
	}
}

func TestRunManageQueueEnqueueInvalid(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"enqueue",
		"--workspace", workspace, "--world-id", "w1",
		"--event-id", "q3", "--type", "note",
		"--error-policy", "bad_policy",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestRunManageQueueRemove(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageQueue(ctx, []string{"remove",
		"--workspace", workspace, "--world-id", "w1",
		"--event-id", "q1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.EventQueue) != 1 {
		t.Fatalf("queue = %d, want 1", len(loaded.EventQueue))
	}
	if string(loaded.EventQueue[0].Event.ID) != "q2" {
		t.Errorf("wrong remaining event: %s", loaded.EventQueue[0].Event.ID)
	}
}

func TestRunManageQueueRemoveNotFound(t *testing.T) {
	t.Parallel()
	workspace := queueTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageQueue(context.Background(), []string{"remove",
		"--workspace", workspace, "--world-id", "w1",
		"--event-id", "missing",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found': %s", stderr.String())
	}
}

// --- stats CLI tests ---

func TestRunStatsText(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runStats(context.Background(), []string{
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "# Stats") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "Memories") {
		t.Errorf("missing memories row:\n%s", out)
	}
}

func TestRunStatsJSON(t *testing.T) {
	t.Parallel()
	workspace := memoryTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runStats(context.Background(), []string{
		"--workspace", workspace, "--world-id", "w1",
		"--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"memory_count"`) {
		t.Errorf("missing JSON field:\n%s", stdout.String())
	}
}

func TestRunStatsMissingFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runStats(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

// --- manage-entity tests ---

func entityTestWorld(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"alice": {ID: "alice", Type: "character", Name: "Alice", Tags: []string{"brave", "smart"}},
			"town":  {ID: "town", Type: "location", Name: "Town Square"},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestRunManageEntityNoSubcommand(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunManageEntityListAll(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "alice") || !strings.Contains(out, "town") {
		t.Errorf("missing entities:\n%s", out)
	}
	if !strings.Contains(stderr.String(), "2 entity(ies)") {
		t.Errorf("missing count: %s", stderr.String())
	}
}

func TestRunManageEntityListFilterType(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1", "--type", "character",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "alice") {
		t.Errorf("missing alice:\n%s", out)
	}
	if strings.Contains(out, "town") {
		t.Errorf("town should be filtered out:\n%s", out)
	}
}

func TestRunManageEntityListFilterTag(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1", "--tag", "brave",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "alice") {
		t.Errorf("missing alice (has brave tag):\n%s", out)
	}
	if strings.Contains(out, "town") {
		t.Errorf("town should be filtered (no brave tag):\n%s", out)
	}
}

func TestRunManageEntityListJSON(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"list",
		"--workspace", workspace, "--world-id", "w1", "--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name": "Alice"`) {
		t.Errorf("missing JSON:\n%s", stdout.String())
	}
}

func TestRunManageEntitySetStateString(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageEntity(ctx, []string{"set-state",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "alice", "--key", "mood", "--value", "happy",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	state := loaded.Entities["alice"].State
	if state == nil || state["mood"].Raw != "happy" {
		t.Errorf("state = %v, want mood=happy", state)
	}
}

func TestRunManageEntitySetStateNumeric(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageEntity(ctx, []string{"set-state",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "alice", "--key", "health", "--num-value", "85.5", "--numeric",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	v := loaded.Entities["alice"].State["health"]
	if v.Kind != model.ValueKindNumber || v.Raw != 85.5 {
		t.Errorf("state = %+v, want number 85.5", v)
	}
}

func TestRunManageEntitySetStateRemove(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()
	w := model.World{
		ID: "w1", Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"alice": {ID: "alice", Type: "character", Name: "Alice",
				State: map[string]model.Value{"mood": {Kind: model.ValueKindString, Raw: "sad"}}},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(ctx, w); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runManageEntity(ctx, []string{"set-state",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "alice", "--key", "mood", "--remove",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if _, exists := loaded.Entities["alice"].State["mood"]; exists {
		t.Error("mood should be removed")
	}
}

func TestRunManageEntitySetStateNotFound(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"set-state",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "missing", "--key", "x", "--value", "y",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestRunManageEntitySetStateMissingValue(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"set-state",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "alice", "--key", "mood",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunManageEntityAdd(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageEntity(ctx, []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "bob", "--type", "character", "--name", "Bob",
		"--description", "A merchant", "--tags", "friendly,wealthy",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name": "Bob"`) {
		t.Errorf("missing JSON:\n%s", stdout.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.Entities) != 3 {
		t.Fatalf("entities = %d, want 3", len(loaded.Entities))
	}
	bob := loaded.Entities["bob"]
	if bob.Description != "A merchant" {
		t.Errorf("description = %q", bob.Description)
	}
	if len(bob.Tags) != 2 || bob.Tags[0] != "friendly" {
		t.Errorf("tags = %v", bob.Tags)
	}
}

func TestRunManageEntityAddDuplicate(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "alice", "--type", "character", "--name", "Dup",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("expected 'already exists': %s", stderr.String())
	}
}

func TestRunManageEntityAddMissingType(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"add",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "x", "--name", "X",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunManageEntitySetTagAdd(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageEntity(ctx, []string{"set-tag",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "alice", "--add", "heroic,wise",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	tags := loaded.Entities["alice"].Tags
	if len(tags) != 4 {
		t.Fatalf("tags = %v, want 4 tags", tags)
	}
}

func TestRunManageEntitySetTagRemove(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageEntity(ctx, []string{"set-tag",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "alice", "--remove", "brave",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	tags := loaded.Entities["alice"].Tags
	if len(tags) != 1 || tags[0] != "smart" {
		t.Errorf("tags = %v, want [smart]", tags)
	}
}

func TestRunManageEntitySetTagNoop(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"set-tag",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "alice",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunManageEntitySetTagNotFound(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"set-tag",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "missing", "--add", "x",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
}

func TestRunManageEntityRemove(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := runManageEntity(ctx, []string{"remove",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "town",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}

	loaded, _ := store.NewFileStore(workspace).LoadSnapshot(ctx, "w1")
	if len(loaded.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(loaded.Entities))
	}
	if _, exists := loaded.Entities["town"]; exists {
		t.Error("town should be removed")
	}
}

func TestRunManageEntityRemoveNotFound(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runManageEntity(context.Background(), []string{"remove",
		"--workspace", workspace, "--world-id", "w1",
		"--entity-id", "missing",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("expected 'not found': %s", stderr.String())
	}
}

// --- budget tests ---

func TestRunBudgetText(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runBudget(context.Background(), []string{
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Context Budget Estimate") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "Entities") {
		t.Errorf("missing entities:\n%s", out)
	}
	if !strings.Contains(out, "**Total**") {
		t.Errorf("missing total:\n%s", out)
	}
}

func TestRunBudgetJSON(t *testing.T) {
	t.Parallel()
	workspace := entityTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runBudget(context.Background(), []string{
		"--workspace", workspace, "--world-id", "w1", "--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"total_tokens"`) {
		t.Errorf("missing total_tokens in JSON:\n%s", stdout.String())
	}
}

func TestRunBudgetMissingFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runBudget(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

// --- preflight tests ---

func preflightTestWorld(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	w := model.World{
		ID: "w1", Name: "Test World",
		Canon: model.Canon{Premise: "A fantasy realm", Genre: []string{"fantasy"}},
		Entities: map[model.EntityID]model.Entity{
			"hero": {
				ID: "hero", Type: "character", Name: "Hero",
				Components: map[string]any{
					"actor": map[string]any{"can_act": true, "goals": []any{"save the world"}},
				},
			},
		},
		Threads: []model.WorldThread{
			{ID: "t1", Kind: model.ThreadKindQuest, Title: "Main Quest", Status: model.ThreadStatusActive},
		},
	}
	if err := store.NewFileStore(workspace).SaveSnapshot(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func TestRunPreflightTextPass(t *testing.T) {
	t.Parallel()
	workspace := preflightTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runPreflight(context.Background(), []string{
		"--workspace", workspace, "--world-id", "w1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s\nstdout=%s", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "PASS") {
		t.Errorf("expected PASS:\n%s", out)
	}
}

func TestRunPreflightJSON(t *testing.T) {
	t.Parallel()
	workspace := preflightTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runPreflight(context.Background(), []string{
		"--workspace", workspace, "--world-id", "w1", "--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"pass": true`) {
		t.Errorf("expected pass=true in JSON:\n%s", stdout.String())
	}
}

func TestRunPreflightFailBudget(t *testing.T) {
	t.Parallel()
	workspace := preflightTestWorld(t)

	var stdout, stderr bytes.Buffer
	code := runPreflight(context.Background(), []string{
		"--workspace", workspace, "--world-id", "w1", "--max-tokens", "1",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 (fail), got %d", code)
	}
	if !strings.Contains(stdout.String(), "FAIL") {
		t.Errorf("expected FAIL:\n%s", stdout.String())
	}
}

func TestRunPreflightMissingFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runPreflight(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunIngestSource(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"init", "--workspace", workspace, "--world-id", "w1", "--name", "Test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init failed: %s", stderr.String())
	}

	srcFile := filepath.Join(workspace, "novel.txt")
	if err := os.WriteFile(srcFile, []byte("Chapter 1\n\nThe hero set out."), 0o644); err != nil {
		t.Fatal(err)
	}

	draftFile := filepath.Join(workspace, "draft.json")
	draftJSON := `{
		"entities": [
			{"id": "char_hero", "type": "character", "name": "Hero", "source_refs": ["ch1"]}
		],
		"facts": [
			{"id": "fact_origin", "subject_id": "char_hero", "predicate": "origin", "value": "unknown"}
		]
	}`
	if err := os.WriteFile(draftFile, []byte(draftJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{
		"ingest-source",
		"--workspace", workspace,
		"--world-id", "w1",
		"--file", srcFile,
		"--draft-file", draftFile,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ingest-source failed (code %d): %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "2 inserted") {
		t.Errorf("expected '2 inserted' (1 entity + 1 fact), got: %s", stdout.String())
	}

	st := store.NewFileStore(workspace)
	world, err := st.LoadSnapshot(ctx, "w1")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if _, ok := world.Entities["char_hero"]; !ok {
		t.Error("expected char_hero entity after ingest")
	}
	if len(world.Facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(world.Facts))
	}
}

func TestRunIngestSourceMissingFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := runIngestSource(context.Background(), nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
}

func TestRunIngestSourceMinConfidence(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"init", "--workspace", workspace, "--world-id", "w1", "--name", "Test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init failed: %s", stderr.String())
	}

	srcFile := filepath.Join(workspace, "novel.txt")
	if err := os.WriteFile(srcFile, []byte("Chapter 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	draftFile := filepath.Join(workspace, "draft.json")
	draftJSON := `{
		"entities": [
			{"id": "char_high", "type": "character", "name": "High", "confidence": 0.9},
			{"id": "char_low", "type": "character", "name": "Low", "confidence": 0.2}
		]
	}`
	if err := os.WriteFile(draftFile, []byte(draftJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{
		"ingest-source",
		"--workspace", workspace,
		"--world-id", "w1",
		"--file", srcFile,
		"--draft-file", draftFile,
		"--min-confidence", "0.5",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ingest-source failed (code %d): %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "1 inserted") {
		t.Errorf("expected '1 inserted', got: %s", out)
	}
	if !strings.Contains(out, "1 filtered") {
		t.Errorf("expected '1 filtered', got: %s", out)
	}

	st := store.NewFileStore(workspace)
	world, err := st.LoadSnapshot(ctx, "w1")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if _, ok := world.Entities["char_high"]; !ok {
		t.Error("expected char_high entity")
	}
	if _, ok := world.Entities["char_low"]; ok {
		t.Error("char_low should have been filtered out")
	}
}

func TestRunIngestSourceKindAndStrict(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"init", "--workspace", workspace, "--world-id", "w1", "--name", "Test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init failed: %s", stderr.String())
	}

	srcFile := filepath.Join(workspace, "novel.txt")
	if err := os.WriteFile(srcFile, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}

	// draft with a dangling fact subject -> warning
	draftFile := filepath.Join(workspace, "draft.json")
	draftJSON := `{
		"entities": [{"id": "char_hero", "type": "character", "name": "Hero"}],
		"facts": [{"id": "fact_ghost", "subject_id": "phantom", "predicate": "is", "value": "void"}]
	}`
	if err := os.WriteFile(draftFile, []byte(draftJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{
		"ingest-source",
		"--workspace", workspace,
		"--world-id", "w1",
		"--file", srcFile,
		"--kind", "novel",
		"--draft-file", draftFile,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("non-strict ingest should succeed despite warning: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), `kind="novel"`) {
		t.Errorf("expected kind in output, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning:") {
		t.Errorf("expected warning in stderr, got: %s", stderr.String())
	}

	// Same draft under --strict should fail
	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{
		"ingest-source",
		"--workspace", workspace,
		"--world-id", "w1",
		"--file", srcFile,
		"--kind", "novel",
		"--draft-file", draftFile,
		"--strict",
	}, &stdout, &stderr)
	if code == 0 {
		t.Errorf("strict mode should reject draft with warnings; stdout: %s", stdout.String())
	}
}

func TestRunIngestSourceRejectsInvalidThreadKind(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{
		"init", "--workspace", workspace, "--world-id", "w1", "--name", "Test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init failed: %s", stderr.String())
	}

	srcFile := filepath.Join(workspace, "src.txt")
	if err := os.WriteFile(srcFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	draftFile := filepath.Join(workspace, "draft.json")
	draftJSON := `{
		"threads": [
			{"id": "t_bad", "kind": "subplot", "title": "Bad", "status": "open"},
			{"id": "t_ok",  "kind": "quest",   "title": "Ok",  "status": "open"}
		]
	}`
	if err := os.WriteFile(draftFile, []byte(draftJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(ctx, []string{
		"ingest-source",
		"--workspace", workspace,
		"--world-id", "w1",
		"--file", srcFile,
		"--draft-file", draftFile,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ingest failed: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 rejected") {
		t.Errorf("expected '1 rejected', got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "thread \"t_bad\" rejected") {
		t.Errorf("expected rejection note for t_bad, stderr: %s", stderr.String())
	}
}

func TestRunIngestSourceNoDraftFile(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	Run(ctx, []string{
		"init", "--workspace", workspace, "--world-id", "w1", "--name", "Test",
	}, &stdout, &stderr)

	srcFile := filepath.Join(workspace, "story.txt")
	os.WriteFile(srcFile, []byte("text"), 0o644)

	stdout.Reset()
	stderr.Reset()
	code := Run(ctx, []string{
		"ingest-source",
		"--workspace", workspace,
		"--world-id", "w1",
		"--file", srcFile,
	}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected exit 2 without --draft-file, got %d", code)
	}
}
