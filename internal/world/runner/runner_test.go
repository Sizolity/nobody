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

func TestRunnerRunExecutesMultipleStepsAndSaves(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{
		ID:    "test_world",
		Name:  "Test World",
		Clock: model.WorldClock{Sequence: 0},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	r := New(
		st,
		worldruntime.WithoutRules(),
		worldruntime.WithDirectors(director.NewScriptDirector("script_1", []model.WorldEvent{{
			ID:     "event_1",
			Type:   model.EventTypeNote,
			Source: model.EventSourceDirector,
		}})),
	)

	got, err := r.Run(ctx, "test_world", 3)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got.StepsCompleted != 3 {
		t.Fatalf("StepsCompleted = %d, want 3", got.StepsCompleted)
	}
	if got.World.Clock.Sequence != 3 {
		t.Fatalf("Clock.Sequence = %d, want 3", got.World.Clock.Sequence)
	}

	saved, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if saved.Clock.Sequence != 3 {
		t.Fatalf("persisted Clock.Sequence = %d, want 3", saved.Clock.Sequence)
	}
	if len(saved.EventLog) != 3 {
		t.Fatalf("persisted EventLog length = %d, want 3", len(saved.EventLog))
	}
}

func TestRunnerRunDoesNotSaveOnFirstStepFailure(t *testing.T) {
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
			{ID: "event_1"},
		})),
	)

	if _, err := r.Run(ctx, "test_world", 5); err == nil {
		t.Fatal("Run returned nil error for invalid proposal")
	}

	saved, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(saved.EventLog) != 0 {
		t.Fatalf("failed run was persisted: %#v", saved.EventLog)
	}
}

func TestRunnerCheckpointAndRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{
		ID:    "test_world",
		Name:  "Test World",
		Clock: model.WorldClock{Sequence: 3},
		Entities: map[model.EntityID]model.Entity{
			"hero": {ID: "hero", Type: "character", Name: "Hero"},
		},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	r := New(st, worldruntime.WithoutRules())

	cp, err := r.Checkpoint(ctx, "test_world")
	if err != nil {
		t.Fatalf("Checkpoint returned error: %v", err)
	}
	if cp.Sequence != 3 {
		t.Fatalf("Checkpoint sequence = %d, want 3", cp.Sequence)
	}

	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}
	advanced, err := r.ApplyEvent(ctx, "test_world", event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if advanced.Clock.Sequence != 3 {
		t.Logf("Clock.Sequence after event = %d", advanced.Clock.Sequence)
	}
	if len(advanced.EventLog) != 1 {
		t.Fatalf("expected 1 event in log, got %d", len(advanced.EventLog))
	}

	rb, err := r.Rollback(ctx, "test_world", 3)
	if err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}
	if rb.RestoredSequence != 3 {
		t.Fatalf("Rollback restored sequence = %d, want 3", rb.RestoredSequence)
	}
	if len(rb.World.EventLog) != 0 {
		t.Fatalf("expected empty event log after rollback, got %d", len(rb.World.EventLog))
	}

	saved, err := st.LoadSnapshot(ctx, "test_world")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if len(saved.EventLog) != 0 {
		t.Fatalf("expected empty event log in store after rollback, got %d", len(saved.EventLog))
	}
}

func TestRunnerListCheckpoints(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{ID: "test_world", Name: "Test World", Clock: model.WorldClock{Sequence: 1}}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	r := New(st, worldruntime.WithoutRules())

	seqs, err := r.ListCheckpoints(ctx, "test_world")
	if err != nil {
		t.Fatalf("ListCheckpoints returned error: %v", err)
	}
	if len(seqs) != 0 {
		t.Fatalf("expected 0 checkpoints, got %d", len(seqs))
	}

	if _, err := r.Checkpoint(ctx, "test_world"); err != nil {
		t.Fatalf("Checkpoint returned error: %v", err)
	}

	seqs, err = r.ListCheckpoints(ctx, "test_world")
	if err != nil {
		t.Fatalf("ListCheckpoints returned error: %v", err)
	}
	if len(seqs) != 1 || seqs[0] != 1 {
		t.Fatalf("expected [1], got %v", seqs)
	}
}

func TestRunnerCheckpointRejectsNonCheckpointStore(t *testing.T) {
	t.Parallel()

	r := Runner{store: &nonCheckpointStore{}}
	if _, err := r.Checkpoint(context.Background(), "w"); err == nil {
		t.Fatal("expected error for non-checkpoint store")
	}
	if _, err := r.Rollback(context.Background(), "w", 0); err == nil {
		t.Fatal("expected error for non-checkpoint store")
	}
	if _, err := r.ListCheckpoints(context.Background(), "w"); err == nil {
		t.Fatal("expected error for non-checkpoint store")
	}
}

type nonCheckpointStore struct{}

func (s *nonCheckpointStore) LoadSnapshot(context.Context, string) (model.World, error) {
	return model.World{}, nil
}
func (s *nonCheckpointStore) SaveSnapshot(context.Context, model.World) error {
	return nil
}

func TestRunnerForkFromCurrentState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{
		ID:    "source",
		Name:  "Source World",
		Clock: model.WorldClock{Sequence: 5},
		Entities: map[model.EntityID]model.Entity{
			"hero": {ID: "hero", Type: "character", Name: "Hero"},
		},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	r := New(st, worldruntime.WithoutRules())
	result, err := r.Fork(ctx, "source", "branch_x", 0)
	if err != nil {
		t.Fatalf("Fork returned error: %v", err)
	}
	if string(result.World.ID) != "branch_x" {
		t.Fatalf("forked ID = %q", result.World.ID)
	}
	if result.ForkSequence != 5 {
		t.Fatalf("fork sequence = %d, want 5", result.ForkSequence)
	}

	loaded, err := st.LoadSnapshot(ctx, "branch_x")
	if err != nil {
		t.Fatalf("LoadSnapshot branch_x returned error: %v", err)
	}
	if len(loaded.Entities) != 1 {
		t.Fatalf("forked entities count = %d", len(loaded.Entities))
	}
}

func TestRunnerForkFromCheckpoint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	initial := model.World{
		ID:    "source",
		Name:  "At Seq 2",
		Clock: model.WorldClock{Sequence: 2},
	}
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}
	if _, err := st.SaveCheckpoint(ctx, "source"); err != nil {
		t.Fatalf("SaveCheckpoint returned error: %v", err)
	}

	initial.Name = "At Seq 8"
	initial.Clock.Sequence = 8
	if err := st.SaveSnapshot(ctx, initial); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	r := New(st, worldruntime.WithoutRules())
	result, err := r.Fork(ctx, "source", "branch_y", 2)
	if err != nil {
		t.Fatalf("Fork returned error: %v", err)
	}
	if result.World.Name != "At Seq 2" {
		t.Fatalf("forked name = %q, want 'At Seq 2'", result.World.Name)
	}
	if result.ForkSequence != 2 {
		t.Fatalf("fork sequence = %d, want 2", result.ForkSequence)
	}
}

func TestRunnerForkRejectsNonForkStore(t *testing.T) {
	t.Parallel()

	r := Runner{store: &nonCheckpointStore{}}
	if _, err := r.Fork(context.Background(), "a", "b", 0); err == nil {
		t.Fatal("expected error for non-fork store")
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
