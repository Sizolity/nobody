package runtime

import (
	"errors"
	"testing"

	"github.com/sizolity/nobody/internal/world/director"
	"github.com/sizolity/nobody/internal/world/model"
)

func TestRuntimeStepWithNoDirectorsReturnsNonNilEmptyResult(t *testing.T) {
	t.Parallel()

	world := model.World{ID: "world_1", Name: "World"}
	got, err := NewRuntime(WithoutRules()).Step(world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if got.World.ID != "world_1" {
		t.Fatalf("world mismatch: %#v", got.World)
	}
	if got.Proposals == nil {
		t.Fatal("Proposals is nil, want non-nil empty slice")
	}
	if got.AppliedEvents == nil {
		t.Fatal("AppliedEvents is nil, want non-nil empty slice")
	}
	if len(got.Proposals) != 0 || len(got.AppliedEvents) != 0 {
		t.Fatalf("unexpected step result: %#v", got)
	}
}

func TestRuntimeStepAppliesDirectorProposals(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewScriptDirector("script_1", []model.WorldEvent{{
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
	got, err := rt.Step(model.World{ID: "world_1", Name: "World"})
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.Proposals) != 1 || got.Proposals[0].ID != "event_1" {
		t.Fatalf("Proposals mismatch: %#v", got.Proposals)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_1" {
		t.Fatalf("AppliedEvents mismatch: %#v", got.AppliedEvents)
	}
	if len(got.World.Facts) != 1 || got.World.Facts[0].ID != "fact_1" {
		t.Fatalf("fact was not applied: %#v", got.World.Facts)
	}
	if len(got.World.EventLog) != 1 || got.World.EventLog[0].ID != "event_1" {
		t.Fatalf("event was not logged: %#v", got.World.EventLog)
	}
}

func TestRuntimeStepAppliesReconcileDirectorProposals(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewReconcileDirector("reconcile_1", []director.ReconcileCase{{
			EventID:          "event_reconcile_1",
			TargetMemoryID:   "memory_1",
			WhenTruthStatus:  model.TruthStatusUnknown,
			TruthStatus:      model.TruthStatusDisputed,
			ConfidenceDelta:  -0.5,
			Summary:          "New evidence disputes this belief.",
			AddMemoryID:      "memory_2",
			AddMemoryContent: "I may have been wrong.",
		}})),
	)
	world := model.World{
		ID:   "world_1",
		Name: "World",
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

	got, err := rt.Step(world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.Proposals) != 1 || got.Proposals[0].ID != "event_reconcile_1" {
		t.Fatalf("Proposals mismatch: %#v", got.Proposals)
	}
	if got.World.Memory[0].TruthStatus != model.TruthStatusDisputed {
		t.Fatalf("memory was not reconciled: %#v", got.World.Memory[0])
	}
	if len(got.World.Memory) != 2 || got.World.Memory[1].ID != "memory_2" {
		t.Fatalf("follow-up memory missing: %#v", got.World.Memory)
	}
}

func TestRuntimeStepReturnsDirectorErrorsWithoutMutatingWorld(t *testing.T) {
	t.Parallel()

	world := model.World{ID: "world_1", Name: "World"}
	rt := NewRuntime(WithoutRules(), WithDirectors(errorDirector{err: errors.New("boom")}))

	got, err := rt.Step(world)
	if err == nil {
		t.Fatal("Step returned nil error")
	}
	if got.World.ID != "world_1" || len(got.World.EventLog) != 0 {
		t.Fatalf("world was mutated after director error: %#v", got.World)
	}
}

func TestRuntimeStepDoesNotLetDirectorsMutateWorldThroughContext(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		Entities: map[model.EntityID]model.Entity{
			"actor_1": {
				ID:   "actor_1",
				Type: "character",
				Name: "Actor",
				State: map[string]model.Value{
					"mood": {
						Kind: model.ValueKindObject,
						Raw:  map[string]any{"label": "calm"},
					},
				},
			},
		},
		Facts: []model.Fact{{
			ID:        "fact_1",
			SubjectID: "actor_1",
			Predicate: "status",
			Value:     model.Value{Kind: model.ValueKindObject, Raw: map[string]any{"label": "safe"}},
		}},
	}
	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(mutatingDirector{}),
	)

	got, err := rt.Step(world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if got.World.Entities["actor_1"].State["mood"].Raw.(map[string]any)["label"] != "calm" {
		t.Fatalf("director mutated result world entity state: %#v", got.World.Entities["actor_1"].State)
	}
	if got.World.Facts[0].Value.Raw.(map[string]any)["label"] != "safe" {
		t.Fatalf("director mutated result world facts: %#v", got.World.Facts)
	}
	if world.Entities["actor_1"].State["mood"].Raw.(map[string]any)["label"] != "calm" {
		t.Fatalf("director mutated input world entity state: %#v", world.Entities["actor_1"].State)
	}
	if world.Facts[0].Value.Raw.(map[string]any)["label"] != "safe" {
		t.Fatalf("director mutated input world facts: %#v", world.Facts)
	}
}

func TestRuntimeStepReturnsApplyErrorsWithPriorAppliedEvents(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewScriptDirector("script_1", []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceDirector},
			{ID: "event_2"},
		})),
	)

	got, err := rt.Step(model.World{ID: "world_1", Name: "World"})
	if err == nil {
		t.Fatal("Step returned nil error")
	}
	if len(got.Proposals) != 2 {
		t.Fatalf("Proposals count = %d, want 2: %#v", len(got.Proposals), got.Proposals)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_1" {
		t.Fatalf("AppliedEvents mismatch: %#v", got.AppliedEvents)
	}
	if len(got.World.EventLog) != 1 || got.World.EventLog[0].ID != "event_1" {
		t.Fatalf("world should include only prior applied event: %#v", got.World.EventLog)
	}
}

type errorDirector struct {
	err error
}

func (d errorDirector) ID() string {
	return "error_director"
}

func (d errorDirector) Propose(director.Context) ([]model.WorldEvent, error) {
	return nil, d.err
}

type mutatingDirector struct{}

func (d mutatingDirector) ID() string {
	return "mutating_director"
}

func (d mutatingDirector) Propose(ctx director.Context) ([]model.WorldEvent, error) {
	ctx.World.Entities["actor_1"].State["mood"].Raw.(map[string]any)["label"] = "angry"
	ctx.World.Facts[0].Value.Raw.(map[string]any)["label"] = "corrupted"
	return []model.WorldEvent{}, nil
}
