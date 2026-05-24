package runner

import (
	"context"
	"testing"

	"github.com/sizolity/nobody/internal/world/director"
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

func TestRunnerStepAppliesDirectorProposalsAndSavesSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{ID: "test_world", Name: "Test World"}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	r := New(
		st,
		worldruntime.WithoutRules(),
		worldruntime.WithDirectors(director.NewScriptDirector("script_1", []model.WorldEvent{{
			ID:     "event_1",
			Type:   model.EventTypeWorldFactChanged,
			Source: model.EventSourceDirector,
			Effects: []model.Effect{{
				Kind:     model.EffectSetFact,
				TargetID: "fact_1",
				Payload: map[string]model.Value{
					"subject_id": {Kind: model.ValueKindEntityRef, Raw: "tower"},
					"predicate":  {Kind: model.ValueKindString, Raw: "status"},
					"value":      {Kind: model.ValueKindString, Raw: "sealed"},
				},
			}},
		}})),
	)

	got, err := r.Step(ctx, "test_world")
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_1" {
		t.Fatalf("AppliedEvents mismatch: %#v", got.AppliedEvents)
	}

	saved, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(saved.Facts) != 1 || saved.Facts[0].ID != "fact_1" {
		t.Fatalf("fact not persisted: %#v", saved.Facts)
	}
	if len(saved.EventLog) != 1 || saved.EventLog[0].ID != "event_1" {
		t.Fatalf("event log not persisted: %#v", saved.EventLog)
	}
}

func TestRunnerStepDoesNotSaveWhenStepFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{ID: "test_world", Name: "Test World"}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	r := New(
		st,
		worldruntime.WithoutRules(),
		worldruntime.WithDirectors(director.NewScriptDirector("script_1", []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
			{ID: "event_2"},
		})),
	)

	if _, err := r.Step(ctx, "test_world"); err == nil {
		t.Fatal("Step returned nil for invalid proposal")
	}

	saved, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(saved.EventLog) != 0 {
		t.Fatalf("failed step was persisted: %#v", saved.EventLog)
	}
}

func TestRunnerStepConsumesQueueAndSavesRemainingQueue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{
		ID:   "test_world",
		Name: "Test World",
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "event_queued_1", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
			{Event: model.WorldEvent{ID: "event_queued_2", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
		},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	r := New(st, worldruntime.WithoutRules(), worldruntime.WithEventQueueLimit(1))

	got, err := r.Step(ctx, "test_world")
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_queued_1" {
		t.Fatalf("AppliedEvents mismatch: %#v", got.AppliedEvents)
	}

	saved, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(saved.EventLog) != 1 || saved.EventLog[0].ID != "event_queued_1" {
		t.Fatalf("event log not persisted: %#v", saved.EventLog)
	}
	if len(saved.EventQueue) != 1 || saved.EventQueue[0].Event.ID != "event_queued_2" {
		t.Fatalf("remaining queue not persisted: %#v", saved.EventQueue)
	}
}
