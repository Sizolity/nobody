package runner

import (
	"context"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
	"github.com/sizolity/nobody/internal/world/store"
)

func TestRunnerAppliesEventAndSavesSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"door_1": {ID: "door_1", Type: "door", Name: "Door"},
		},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	r := New(st, worldruntime.WithoutRules())
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateEntityState,
			TargetID: "door_1",
			Payload: map[string]model.Value{
				"locked": {Kind: model.ValueKindBoolean, Raw: true},
			},
		}},
	}

	got, err := r.ApplyEvent(ctx, "test_world", event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if got.Entities["door_1"].State["locked"].Raw != true {
		t.Fatalf("event was not applied: %#v", got.Entities["door_1"].State)
	}

	saved, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(saved.EventLog) != 1 || saved.EventLog[0].ID != "event_1" {
		t.Fatalf("event log not persisted: %#v", saved.EventLog)
	}
	if saved.Entities["door_1"].State["locked"].Raw != true {
		t.Fatalf("entity state not persisted: %#v", saved.Entities["door_1"].State)
	}
}

func TestRunnerDoesNotSaveRejectedEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"actor_1": {
				ID:   "actor_1",
				Type: "character",
				Name: "Actor",
				State: map[string]model.Value{
					"alive": {Kind: model.ValueKindBoolean, Raw: false},
				},
			},
			"door_1": {ID: "door_1", Type: "door", Name: "Door"},
		},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	r := New(st)
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"actor_1"},
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateEntityState,
			TargetID: "door_1",
			Payload: map[string]model.Value{
				"locked": {Kind: model.ValueKindBoolean, Raw: true},
			},
		}},
	}

	if _, err := r.ApplyEvent(ctx, "test_world", event); err == nil {
		t.Fatal("ApplyEvent returned nil for rejected event")
	}

	saved, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(saved.EventLog) != 0 {
		t.Fatalf("rejected event was persisted: %#v", saved.EventLog)
	}
	if saved.Entities["door_1"].State != nil {
		t.Fatalf("rejected event effects were persisted: %#v", saved.Entities["door_1"].State)
	}
}
