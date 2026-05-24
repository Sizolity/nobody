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
