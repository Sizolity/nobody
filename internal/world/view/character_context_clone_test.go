package view

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

// Stage 6B — CharacterContextView.Render must never let product layers
// drift the runtime World by mutating returned data. The test below
// builds a deliberately "rich" world (every cloneable sub-structure is
// populated), invokes Render, then mutates every reachable slice / map /
// pointer field of the returned context. The source World must survive
// untouched.

func richVisibilityWorld() model.World {
	return model.World{
		ID:   "world_rich",
		Name: "Rich World",
		Canon: model.Canon{
			Genre: []string{"mystery"},
		},
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeCalendar, Calendar: map[string]int{"year": 1}},
		},
		Entities: map[model.EntityID]model.Entity{
			"char_a": {
				ID: "char_a", Type: "character", Name: "Alice",
				Aliases: []string{"Ali"},
				Components: map[string]any{
					"spatial": model.NewSpatialComponent("loc_tavern"),
					"profile": map[string]any{"title": "knight"},
				},
				State: map[string]model.Value{
					"mood": {Kind: model.ValueKindString, Raw: "calm"},
				},
				Tags: []string{"hero"},
			},
			"char_b": {
				ID: "char_b", Type: "character", Name: "Bob",
				Components: map[string]any{
					"spatial": model.NewSpatialComponent("loc_tavern"),
				},
			},
			"loc_tavern": {ID: "loc_tavern", Type: "location", Name: "Tavern"},
		},
		Relations: []model.Relation{
			{
				ID: "rel_1", Type: "ally", SourceID: "char_a", TargetID: "char_b",
				Visibility: &model.Visibility{Mode: model.VisibilityPrivate, EntityIDs: []model.EntityID{"char_a"}},
			},
		},
		Facts: []model.Fact{
			{
				ID: "fact_1", SubjectID: "char_a", Predicate: "has_inventory",
				Value: model.Value{Kind: model.ValueKindObject, Raw: map[string]any{
					"items": []any{"sword", "shield"},
				}},
				Visibility: &model.Visibility{Mode: model.VisibilityPrivate, EntityIDs: []model.EntityID{"char_a"}},
			},
		},
		Threads: []model.WorldThread{
			{
				ID: "thread_1", Kind: model.ThreadKindMystery, Title: "Find the killer",
				Status:         model.ThreadStatusOpen,
				ParticipantIDs: []model.EntityID{"char_a"},
				UpdatedBy:      []model.EventID{"evt_1"},
				Goals: []model.ThreadGoal{{
					ID:          "goal_1",
					Description: "find the killer",
					DesiredState: []model.Condition{{
						Kind:     model.ConditionKindMemory,
						Path:     "mem_1",
						Operator: "exists",
						Value:    model.Value{Kind: model.ValueKindObject, Raw: map[string]any{"threshold": float64(0.5)}},
						Owner:    &model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_a"},
					}},
				}},
				Clues: []model.ThreadClue{{
					ID:       "clue_1",
					Content:  "muddy boot print",
					KnownBy:  []model.EntityID{"char_a"},
					PointsTo: []model.EntityID{"char_b"},
				}},
				Branches: []model.ThreadBranch{{
					TriggerCondition: []model.Condition{{
						Kind:     model.ConditionKindState,
						Path:     "actor.state.alive",
						Operator: "==",
						Value:    model.Value{Kind: model.ValueKindObject, Raw: map[string]any{"value": true}},
					}},
					ResultHint: "witness flees",
					Weight:     0.7,
				}},
				Deadline: &model.WorldTime{Kind: model.WorldTimeCalendar, Calendar: map[string]int{"year": 7}},
			},
		},
		EventLog: []model.WorldEvent{
			{
				ID: "evt_1", Type: model.EventTypeNote, Source: model.EventSourceRuntime,
				ActorIDs: []model.EntityID{"char_a"},
				Preconditions: []model.Condition{{
					Kind:     model.ConditionKindMemory,
					Path:     "mem_1",
					Operator: "exists",
					Value:    model.Value{Kind: model.ValueKindObject, Raw: map[string]any{"score": float64(1)}},
					Owner:    &model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_a"},
				}},
				Effects: []model.Effect{{
					Kind:     model.EffectUpdateEntityState,
					TargetID: "char_a",
					Payload: map[string]model.Value{
						"mood": {Kind: model.ValueKindString, Raw: "calm"},
					},
				}},
				OccurredAt: model.WorldTime{Kind: model.WorldTimeCalendar, Calendar: map[string]int{"year": 1}},
				Metadata:   map[string]any{"trace_id": "t1"},
			},
		},
		Memory: []model.MemoryRecord{
			{
				ID:         "mem_1",
				Owner:      model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_a"},
				Content:    "I saw something.",
				SubjectIDs: []model.EntityID{"char_b"},
				EventIDs:   []model.EventID{"evt_1"},
				Emotion:    map[string]float64{"fear": 0.4, "joy": 0.1},
				Decay:      &model.MemoryDecay{Mode: model.MemoryDecayFadeConfidence, HalfLife: "1d"},
				Visibility: &model.Visibility{Mode: model.VisibilityPrivate, EntityIDs: []model.EntityID{"char_a"}},
				CreatedAt:  model.WorldTime{Kind: model.WorldTimeCalendar, Calendar: map[string]int{"year": 1}},
				UpdatedAt:  model.WorldTime{Kind: model.WorldTimeCalendar, Calendar: map[string]int{"year": 2}},
				LastAccess: model.WorldTime{Kind: model.WorldTimeCalendar, Calendar: map[string]int{"year": 3}},
			},
		},
	}
}

func TestCharacterContextRenderReturnsDeepCopy(t *testing.T) {
	t.Parallel()

	world := richVisibilityWorld()

	ctx, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	// --- Perspective entity ---
	ctx.Perspective.Aliases[0] = "MUTATED"
	ctx.Perspective.Tags[0] = "MUTATED"
	ctx.Perspective.Components["profile"].(map[string]any)["title"] = "MUTATED"
	ctx.Perspective.State["mood"] = model.Value{Kind: model.ValueKindString, Raw: "MUTATED"}

	// --- Memories ---
	if len(ctx.Memories) == 0 {
		t.Fatal("expected at least one visible memory")
	}
	mem := &ctx.Memories[0]
	mem.SubjectIDs[0] = "MUTATED"
	mem.EventIDs[0] = "MUTATED"
	mem.Emotion["fear"] = 0.99
	mem.Emotion["new"] = 0.5
	mem.Visibility.EntityIDs[0] = "MUTATED"
	*mem.Decay = model.MemoryDecay{Mode: model.MemoryDecayArchiveAfter}
	mem.CreatedAt.Calendar["year"] = 999
	mem.UpdatedAt.Calendar["year"] = 999
	mem.LastAccess.Calendar["year"] = 999

	// --- Relations ---
	if len(ctx.Relations) == 0 {
		t.Fatal("expected at least one visible relation")
	}
	ctx.Relations[0].Visibility.EntityIDs[0] = "MUTATED"

	// --- Facts ---
	if len(ctx.Facts) == 0 {
		t.Fatal("expected at least one visible fact")
	}
	ctx.Facts[0].Value.Raw.(map[string]any)["items"].([]any)[0] = "MUTATED"
	ctx.Facts[0].Visibility.EntityIDs[0] = "MUTATED"

	// --- Threads (including sub-types) ---
	if len(ctx.Threads) == 0 {
		t.Fatal("expected at least one visible thread")
	}
	th := &ctx.Threads[0]
	th.ParticipantIDs[0] = "MUTATED"
	th.UpdatedBy[0] = "MUTATED"
	th.Goals[0].DesiredState[0].Value.Raw.(map[string]any)["threshold"] = float64(0.99)
	th.Goals[0].DesiredState[0].Owner.ID = "MUTATED"
	th.Clues[0].KnownBy[0] = "MUTATED"
	th.Clues[0].PointsTo[0] = "MUTATED"
	th.Branches[0].TriggerCondition[0].Value.Raw.(map[string]any)["value"] = false
	th.Deadline.Calendar["year"] = 999

	// --- NearbyEntities ---
	if len(ctx.NearbyEntities) == 0 {
		t.Fatal("expected at least one nearby entity")
	}
	// char_b is the only nearby entity in this world (same loc_tavern as char_a).
	nb := &ctx.NearbyEntities[0]
	nb.Components["spatial"].(map[string]any)["location_id"] = "MUTATED"

	// --- RecentEvents ---
	if len(ctx.RecentEvents) == 0 {
		t.Fatal("expected at least one recent event")
	}
	ev := &ctx.RecentEvents[0]
	ev.ActorIDs[0] = "MUTATED"
	ev.Preconditions[0].Value.Raw.(map[string]any)["score"] = float64(99)
	ev.Preconditions[0].Owner.ID = "MUTATED"
	ev.Effects[0].Payload["mood"] = model.Value{Kind: model.ValueKindString, Raw: "MUTATED"}
	ev.OccurredAt.Calendar["year"] = 999
	ev.Metadata["trace_id"] = "MUTATED"

	// --- Assert: source world is unchanged ---
	srcAlice := world.Entities["char_a"]
	if srcAlice.Aliases[0] != "Ali" {
		t.Fatalf("source Perspective.Aliases mutated: %v", srcAlice.Aliases)
	}
	if srcAlice.Tags[0] != "hero" {
		t.Fatalf("source Perspective.Tags mutated: %v", srcAlice.Tags)
	}
	if srcAlice.Components["profile"].(map[string]any)["title"] != "knight" {
		t.Fatalf("source Perspective.Components mutated: %#v", srcAlice.Components)
	}
	if srcAlice.State["mood"].Raw != "calm" {
		t.Fatalf("source Perspective.State mutated: %#v", srcAlice.State)
	}

	srcMem := world.Memory[0]
	if srcMem.SubjectIDs[0] != "char_b" {
		t.Fatalf("source Memory.SubjectIDs mutated: %v", srcMem.SubjectIDs)
	}
	if srcMem.EventIDs[0] != "evt_1" {
		t.Fatalf("source Memory.EventIDs mutated: %v", srcMem.EventIDs)
	}
	if srcMem.Emotion["fear"] != 0.4 {
		t.Fatalf("source Memory.Emotion mutated: %v", srcMem.Emotion)
	}
	if _, ok := srcMem.Emotion["new"]; ok {
		t.Fatalf("source Memory.Emotion grew an extra key: %v", srcMem.Emotion)
	}
	if srcMem.Visibility.EntityIDs[0] != "char_a" {
		t.Fatalf("source Memory.Visibility mutated: %v", srcMem.Visibility.EntityIDs)
	}
	if srcMem.Decay.Mode != model.MemoryDecayFadeConfidence {
		t.Fatalf("source Memory.Decay mutated: %#v", srcMem.Decay)
	}
	if srcMem.CreatedAt.Calendar["year"] != 1 {
		t.Fatalf("source Memory.CreatedAt mutated: %v", srcMem.CreatedAt.Calendar)
	}
	if srcMem.UpdatedAt.Calendar["year"] != 2 {
		t.Fatalf("source Memory.UpdatedAt mutated: %v", srcMem.UpdatedAt.Calendar)
	}
	if srcMem.LastAccess.Calendar["year"] != 3 {
		t.Fatalf("source Memory.LastAccess mutated: %v", srcMem.LastAccess.Calendar)
	}

	if world.Relations[0].Visibility.EntityIDs[0] != "char_a" {
		t.Fatalf("source Relation.Visibility mutated: %v", world.Relations[0].Visibility.EntityIDs)
	}

	if world.Facts[0].Value.Raw.(map[string]any)["items"].([]any)[0] != "sword" {
		t.Fatalf("source Fact.Value mutated: %#v", world.Facts[0].Value.Raw)
	}
	if world.Facts[0].Visibility.EntityIDs[0] != "char_a" {
		t.Fatalf("source Fact.Visibility mutated: %v", world.Facts[0].Visibility.EntityIDs)
	}

	srcThread := world.Threads[0]
	if srcThread.ParticipantIDs[0] != "char_a" {
		t.Fatalf("source Thread.ParticipantIDs mutated: %v", srcThread.ParticipantIDs)
	}
	if srcThread.UpdatedBy[0] != "evt_1" {
		t.Fatalf("source Thread.UpdatedBy mutated: %v", srcThread.UpdatedBy)
	}
	if srcThread.Goals[0].DesiredState[0].Value.Raw.(map[string]any)["threshold"] != float64(0.5) {
		t.Fatalf("source Thread.Goals[0].DesiredState mutated: %#v",
			srcThread.Goals[0].DesiredState[0].Value.Raw)
	}
	if srcThread.Goals[0].DesiredState[0].Owner.ID != "char_a" {
		t.Fatalf("source Thread.Goals[0].DesiredState[0].Owner mutated: %#v",
			srcThread.Goals[0].DesiredState[0].Owner)
	}
	if srcThread.Clues[0].KnownBy[0] != "char_a" {
		t.Fatalf("source Thread.Clues[0].KnownBy mutated: %v", srcThread.Clues[0].KnownBy)
	}
	if srcThread.Clues[0].PointsTo[0] != "char_b" {
		t.Fatalf("source Thread.Clues[0].PointsTo mutated: %v", srcThread.Clues[0].PointsTo)
	}
	if srcThread.Branches[0].TriggerCondition[0].Value.Raw.(map[string]any)["value"] != true {
		t.Fatalf("source Thread.Branches[0].TriggerCondition mutated: %#v",
			srcThread.Branches[0].TriggerCondition[0].Value.Raw)
	}
	if srcThread.Deadline.Calendar["year"] != 7 {
		t.Fatalf("source Thread.Deadline mutated: %v", srcThread.Deadline.Calendar)
	}

	srcCharB := world.Entities["char_b"]
	if srcCharB.Components["spatial"].(map[string]any)["location_id"] != "loc_tavern" {
		t.Fatalf("source NearbyEntity[char_b].Components mutated: %#v", srcCharB.Components)
	}

	srcEvent := world.EventLog[0]
	if srcEvent.ActorIDs[0] != "char_a" {
		t.Fatalf("source EventLog ActorIDs mutated: %v", srcEvent.ActorIDs)
	}
	if srcEvent.Preconditions[0].Value.Raw.(map[string]any)["score"] != float64(1) {
		t.Fatalf("source EventLog Preconditions Value mutated: %#v",
			srcEvent.Preconditions[0].Value.Raw)
	}
	if srcEvent.Preconditions[0].Owner.ID != "char_a" {
		t.Fatalf("source EventLog Preconditions Owner mutated: %#v",
			srcEvent.Preconditions[0].Owner)
	}
	if srcEvent.Effects[0].Payload["mood"].Raw != "calm" {
		t.Fatalf("source EventLog Effects Payload mutated: %#v", srcEvent.Effects[0].Payload)
	}
	if srcEvent.OccurredAt.Calendar["year"] != 1 {
		t.Fatalf("source EventLog OccurredAt mutated: %v", srcEvent.OccurredAt.Calendar)
	}
	if srcEvent.Metadata["trace_id"] != "t1" {
		t.Fatalf("source EventLog Metadata mutated: %v", srcEvent.Metadata)
	}
}

func TestCharacterContextRenderPerspectiveIsClone(t *testing.T) {
	t.Parallel()

	world := richVisibilityWorld()
	ctx, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	// Mutating the perspective entity returned by the view must not affect
	// the source world map's stored entity.
	ctx.Perspective.Name = "MUTATED"
	if world.Entities["char_a"].Name != "Alice" {
		t.Fatalf("world entity Name was mutated through Perspective: %q",
			world.Entities["char_a"].Name)
	}
}
