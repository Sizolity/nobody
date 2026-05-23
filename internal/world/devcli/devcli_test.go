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
