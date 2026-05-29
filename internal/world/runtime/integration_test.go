package runtime

import (
	"context"
	"testing"

	"github.com/sizolity/nobody/internal/world/director"
	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/view"
)

func setupMurderMysteryWorld() model.World {
	return model.World{
		ID:   "mystery_world",
		Name: "Murder Mystery",
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeTick, Tick: 0},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_detective": {
				ID:   "char_detective",
				Type: "character",
				Name: "Detective",
				Components: map[string]any{
					model.ComponentActor:   model.NewActorComponent(true, []string{"solve the case"}),
					model.ComponentSpatial: model.NewSpatialComponent("crime_scene"),
				},
			},
			"char_witness": {
				ID:   "char_witness",
				Type: "character",
				Name: "Witness",
				Components: map[string]any{
					model.ComponentActor:   model.NewActorComponent(true, nil),
					model.ComponentSpatial: model.NewSpatialComponent("crime_scene"),
				},
			},
			"char_suspect": {
				ID:   "char_suspect",
				Type: "character",
				Name: "Suspect",
				Components: map[string]any{
					model.ComponentActor:   model.NewActorComponent(true, nil),
					model.ComponentSpatial: model.NewSpatialComponent("crime_scene"),
				},
			},
			"crime_scene": {
				ID:   "crime_scene",
				Type: "location",
				Name: "Crime Scene",
			},
		},
		Threads: []model.WorldThread{{
			ID:             "murder_case",
			Kind:           model.ThreadKindMystery,
			Title:          "Murder Case",
			Status:         model.ThreadStatusActive,
			Tension:        0.5,
			ParticipantIDs: []model.EntityID{"char_detective", "char_witness", "char_suspect"},
		}},
		Memory: []model.MemoryRecord{
			{
				ID:          "mem_victim_found",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "victim was found at crime_scene",
				TruthStatus: model.TruthStatusTrue,
				Confidence:  1.0,
				Importance:  0.8,
			},
			{
				ID:          "mem_suspect_fleeing",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "suspect was seen fleeing",
				TruthStatus: model.TruthStatusTrue,
				Confidence:  1.0,
				Importance:  0.9,
				Visibility:  &model.Visibility{Mode: model.VisibilitySecret},
			},
			{
				ID:          "mem_witness_saw",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_witness"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindObservation,
				Content:     "I saw someone run away",
				TruthStatus: model.TruthStatusTrue,
				Confidence:  0.8,
				Importance:  0.7,
			},
		},
		Relations: []model.Relation{{
			ID:         "rel_suspect_suspects",
			Type:       "suspects",
			SourceID:   "char_suspect",
			TargetID:   "char_detective",
			Visibility: &model.Visibility{Mode: model.VisibilityPrivate, EntityIDs: []model.EntityID{"char_suspect"}},
		}},
	}
}

func TestIntegrationMurderMystery(t *testing.T) {
	world := setupMurderMysteryWorld()

	t.Run("Step1_ExternalInput", func(t *testing.T) {
		ext := director.NewExternalDirector("ext_user", nil)
		ext.SubmitText("Detective examines the crime scene", "char_detective")

		rt := NewRuntime(
			WithDirectors(ext),
			WithWorldRules(DefaultRegistry()),
			WithPostApplyHooks(AutoExtractMemory(), AutoUpdateThread()),
		)

		result, err := rt.Step(context.Background(), world)
		if err != nil {
			t.Fatalf("Step error: %v", err)
		}
		if len(result.AppliedEvents) != 1 {
			t.Fatalf("applied events = %d, want 1", len(result.AppliedEvents))
		}
		applied := result.AppliedEvents[0]
		if applied.Type != model.EventTypeNote {
			t.Fatalf("event type = %q, want %q", applied.Type, model.EventTypeNote)
		}

		expectedMemID := model.MemoryID("mem_auto_" + string(applied.ID) + "_char_detective")
		found := false
		for _, mem := range result.World.Memory {
			if mem.ID == expectedMemID {
				found = true
				if mem.Owner.Kind != model.MemoryOwnerKindCharacter || mem.Owner.ID != "char_detective" {
					t.Fatalf("auto-extracted memory wrong owner: %+v", mem.Owner)
				}
				break
			}
		}
		if !found {
			t.Fatalf("AutoExtractMemory did not create %q", expectedMemID)
		}

		var murderCase *model.WorldThread
		for i := range result.World.Threads {
			if result.World.Threads[i].ID == "murder_case" {
				murderCase = &result.World.Threads[i]
				break
			}
		}
		if murderCase == nil {
			t.Fatal("murder_case thread not found")
		}
		hasEventID := false
		for _, eid := range murderCase.UpdatedBy {
			if eid == applied.ID {
				hasEventID = true
				break
			}
		}
		if !hasEventID {
			t.Fatalf("AutoUpdateThread did not append event to murder_case.UpdatedBy: %v", murderCase.UpdatedBy)
		}

		world = result.World
	})

	t.Run("Step2_RuleModification", func(t *testing.T) {
		registry := DefaultRegistry()
		registry.Register("add_clue_on_examine", func(id model.RuleID, _ any) (Rule, error) {
			return addClueOnExamineRule{id: id}, nil
		})

		world.Rules = append(world.Rules, model.Rule{
			ID:      "rule_add_clue",
			Kind:    "add_clue_on_examine",
			Enabled: true,
		})

		ext := director.NewExternalDirector("ext_user", nil)
		ext.SubmitEvents(model.WorldEvent{
			ID:          "event_examine_clue",
			Type:        model.EventTypeNote,
			Source:      model.EventSourceUser,
			ActorIDs:    []model.EntityID{"char_detective"},
			LocationID:  "crime_scene",
			Description: "detective examines clue",
		})

		rt := NewRuntime(
			WithDirectors(ext),
			WithWorldRules(registry),
			WithPostApplyHooks(AutoExtractMemory(), AutoUpdateThread()),
		)

		result, err := rt.Step(context.Background(), world)
		if err != nil {
			t.Fatalf("Step error: %v", err)
		}

		var clueFound bool
		for _, f := range result.World.Facts {
			if f.ID == "fact_clue_found" {
				clueFound = true
				if f.Predicate != "clue_discovered" {
					t.Fatalf("fact predicate = %q, want %q", f.Predicate, "clue_discovered")
				}
				break
			}
		}
		if !clueFound {
			t.Fatalf("rule did not create fact_clue_found; facts = %+v", result.World.Facts)
		}

		world = result.World
	})

	t.Run("Step3_VisibilityCheck", func(t *testing.T) {
		cv := view.CharacterContextView{}

		detectiveCtx, err := cv.Render(world, view.CharacterContextRequest{PerspectiveID: "char_detective"})
		if err != nil {
			t.Fatalf("Render detective: %v", err)
		}

		assertHasMemory(t, detectiveCtx.Memories, "mem_victim_found", "detective should see public memory")
		assertNoMemory(t, detectiveCtx.Memories, "mem_suspect_fleeing", "detective must not see secret memory")
		assertHasThread(t, detectiveCtx.Threads, "murder_case", "detective should see murder_case thread")
		if len(detectiveCtx.NearbyEntities) == 0 {
			t.Fatal("detective should see nearby entities at crime_scene")
		}
		assertNoRelation(t, detectiveCtx.Relations, "rel_suspect_suspects", "detective must not see private suspect->detective relation")

		witnessCtx, err := cv.Render(world, view.CharacterContextRequest{PerspectiveID: "char_witness"})
		if err != nil {
			t.Fatalf("Render witness: %v", err)
		}

		assertHasMemory(t, witnessCtx.Memories, "mem_witness_saw", "witness should see own memory")
		assertHasMemory(t, witnessCtx.Memories, "mem_victim_found", "witness should see public memory")
		assertHasThread(t, witnessCtx.Threads, "murder_case", "witness should see murder_case thread")
		assertNoRelation(t, witnessCtx.Relations, "rel_suspect_suspects", "witness must not see private suspect->detective relation")
	})

	t.Run("Step4_NarrativeAdvancement", func(t *testing.T) {
		for i := range world.Threads {
			if world.Threads[i].ID == "murder_case" {
				world.Threads[i].Tension = 0.8
				break
			}
		}

		rt := NewRuntime(
			WithDirectors(director.NewNarrativeDirector("narrative_1", director.NarrativeDirectorConfig{})),
		)

		result, err := rt.Step(context.Background(), world)
		if err != nil {
			t.Fatalf("Step error: %v", err)
		}

		found := false
		for _, p := range result.Proposals {
			if p.ID == "narr_advance_murder_case" {
				found = true
				break
			}
		}
		if !found {
			ids := make([]model.EventID, len(result.Proposals))
			for i, p := range result.Proposals {
				ids[i] = p.ID
			}
			t.Fatalf("NarrativeDirector did not propose thread advancement; proposals = %v", ids)
		}

		world = result.World
	})

	t.Run("Step5_SystemCleanup", func(t *testing.T) {
		world.Facts = append(world.Facts, model.Fact{
			ID:         "fact_stale",
			SubjectID:  "crime_scene",
			Predicate:  "rumor",
			Value:      model.Value{Kind: model.ValueKindString, Raw: "unreliable tip"},
			Confidence: 0.05,
		})

		rt := NewRuntime(
			WithoutRules(),
			WithDirectors(director.NewSystemDirector("system_1", director.SystemDirectorConfig{
				EnableConsistencyCheck: true,
			})),
		)

		result, err := rt.Step(context.Background(), world)
		if err != nil {
			t.Fatalf("Step error: %v", err)
		}

		found := false
		for _, p := range result.Proposals {
			if p.ID == "sys_clean_fact_stale" {
				found = true
				break
			}
		}
		if !found {
			ids := make([]model.EventID, len(result.Proposals))
			for i, p := range result.Proposals {
				ids[i] = p.ID
			}
			t.Fatalf("SystemDirector did not propose stale fact removal; proposals = %v", ids)
		}

		for _, f := range result.World.Facts {
			if f.ID == "fact_stale" {
				t.Fatal("stale fact should have been removed after apply")
			}
		}
	})
}

// addClueOnExamineRule adds a set_fact effect when the detective submits a note event.
type addClueOnExamineRule struct {
	id model.RuleID
}

func (r addClueOnExamineRule) ID() model.RuleID { return r.id }

func (r addClueOnExamineRule) Evaluate(_ RuleContext, event model.WorldEvent) RuleDecision {
	if event.Type != model.EventTypeNote {
		return RuleDecision{Status: RuleDecisionAllow}
	}
	for _, actor := range event.ActorIDs {
		if actor == "char_detective" {
			return RuleDecision{
				Status: RuleDecisionAddEffect,
				AddedEffects: []model.Effect{{
					Kind:     model.EffectSetFact,
					TargetID: "fact_clue_found",
					Payload: map[string]model.Value{
						"subject_id": {Kind: model.ValueKindEntityRef, Raw: "crime_scene"},
						"predicate":  {Kind: model.ValueKindString, Raw: "clue_discovered"},
						"value":      {Kind: model.ValueKindBoolean, Raw: true},
					},
				}},
			}
		}
	}
	return RuleDecision{Status: RuleDecisionAllow}
}

func assertHasMemory(t *testing.T, memories []model.MemoryRecord, id model.MemoryID, msg string) {
	t.Helper()
	for _, m := range memories {
		if m.ID == id {
			return
		}
	}
	t.Fatalf("%s: memory %q not found in %d memories", msg, id, len(memories))
}

func assertNoMemory(t *testing.T, memories []model.MemoryRecord, id model.MemoryID, msg string) {
	t.Helper()
	for _, m := range memories {
		if m.ID == id {
			t.Fatalf("%s: memory %q should not be visible", msg, id)
		}
	}
}

func assertHasThread(t *testing.T, threads []model.WorldThread, id model.ThreadID, msg string) {
	t.Helper()
	for _, th := range threads {
		if th.ID == id {
			return
		}
	}
	t.Fatalf("%s: thread %q not found in %d threads", msg, id, len(threads))
}

func assertNoRelation(t *testing.T, relations []model.Relation, id model.RelationID, msg string) {
	t.Helper()
	for _, r := range relations {
		if r.ID == id {
			t.Fatalf("%s: relation %q should not be visible", msg, id)
		}
	}
}
