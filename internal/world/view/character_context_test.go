package view

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestCharacterContextViewRequiresPerspective(t *testing.T) {
	world := model.World{ID: "test_world", Name: "Test World"}
	_, err := CharacterContextView{}.Render(world, CharacterContextRequest{})
	if err == nil {
		t.Fatal("Render returned nil without perspective")
	}
}

func TestCharacterContextViewRequiresExistingEntity(t *testing.T) {
	world := model.World{
		ID:       "test_world",
		Name:     "Test World",
		Entities: map[model.EntityID]model.Entity{},
	}
	_, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_b"})
	if err == nil {
		t.Fatal("Render returned nil for missing perspective entity")
	}
}

func TestCharacterContextViewIncludesOwnAndPublicWorldMemories(t *testing.T) {
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_b": {ID: "char_b", Type: "character", Name: "B"},
		},
		Memory: []model.MemoryRecord{
			{
				ID:          "memory_world_public",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "The king is dead.",
				TruthStatus: model.TruthStatusTrue,
				Confidence:  1.0,
				Importance:  0.8,
			},
			{
				ID:          "memory_b_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_b"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "A killed the king.",
				TruthStatus: model.TruthStatusUnknown,
				Confidence:  0.8,
				Importance:  0.7,
			},
		},
	}

	got, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if got.Perspective.ID != "char_b" {
		t.Fatalf("perspective mismatch: %#v", got.Perspective)
	}
	if len(got.Memories) != 2 {
		t.Fatalf("Memories length = %d, want 2: %#v", len(got.Memories), got.Memories)
	}
}

func TestCharacterContextViewDoesNotLeakOtherOrSecretMemories(t *testing.T) {
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_b": {ID: "char_b", Type: "character", Name: "B"},
		},
		Memory: []model.MemoryRecord{
			{
				ID:          "memory_world_secret",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Scope:       model.MemoryScopeFactual,
				Kind:        model.MemoryKindObservation,
				Content:     "D killed the king.",
				TruthStatus: model.TruthStatusSecret,
				Confidence:  1.0,
				Importance:  1.0,
			},
			{
				ID:          "memory_c_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "A may have been framed.",
				TruthStatus: model.TruthStatusUnknown,
				Confidence:  0.5,
				Importance:  0.5,
			},
			{
				ID:          "memory_b_private",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_b"},
				Scope:       model.MemoryScopeSubjective,
				Kind:        model.MemoryKindBelief,
				Content:     "A killed the king.",
				TruthStatus: model.TruthStatusUnknown,
				Confidence:  0.8,
				Importance:  0.7,
			},
		},
	}

	got, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if len(got.Memories) != 1 {
		t.Fatalf("Memories length = %d, want 1: %#v", len(got.Memories), got.Memories)
	}
	if got.Memories[0].ID != "memory_b_private" {
		t.Fatalf("unexpected visible memory: %#v", got.Memories)
	}
}

func TestCharacterContextViewHidesWorldMemoriesWithUnknownTruthStatus(t *testing.T) {
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_b": {ID: "char_b", Type: "character", Name: "B"},
		},
		Memory: []model.MemoryRecord{{
			ID:          "memory_world_invalid",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
			Content:     "Hidden by malformed status.",
			TruthStatus: "hidden",
		}},
	}

	got, err := CharacterContextView{}.Render(world, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if len(got.Memories) != 0 {
		t.Fatalf("malformed world memory was visible: %#v", got.Memories)
	}
}

// --- M5: Visibility-aware relations, facts, threads, events, nearby entities ---

func testWorldWithVisibility() model.World {
	return model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {
				ID: "char_a", Type: "character", Name: "Alice",
				Components: map[string]any{
					"spatial": model.NewSpatialComponent("loc_tavern"),
					"faction": model.NewFactionComponent([]model.EntityID{"faction_guild"}, "", 0.9),
				},
			},
			"char_b": {
				ID: "char_b", Type: "character", Name: "Bob",
				Components: map[string]any{
					"spatial": model.NewSpatialComponent("loc_tavern"),
				},
			},
			"char_c": {
				ID: "char_c", Type: "character", Name: "Carol",
				Components: map[string]any{
					"spatial": model.NewSpatialComponent("loc_forest"),
				},
			},
			"loc_tavern": {ID: "loc_tavern", Type: "location", Name: "Tavern"},
			"loc_forest": {ID: "loc_forest", Type: "location", Name: "Forest"},
		},
		Relations: []model.Relation{
			{ID: "rel_public", Type: "ally", SourceID: "char_a", TargetID: "char_b"},
			{ID: "rel_secret", Type: "spy", SourceID: "char_a", TargetID: "char_c",
				Visibility: &model.Visibility{Mode: model.VisibilitySecret}},
			{ID: "rel_private", Type: "debt", SourceID: "char_c", TargetID: "char_a",
				Visibility: &model.Visibility{Mode: model.VisibilityPrivate, EntityIDs: []model.EntityID{"char_a"}}},
		},
		Facts: []model.Fact{
			{ID: "fact_public", SubjectID: "char_a", Predicate: "is_alive",
				Value: model.Value{Kind: "bool", Raw: true}},
			{ID: "fact_gm", SubjectID: "char_a", Predicate: "true_identity",
				Value:      model.Value{Kind: "string", Raw: "The Phantom"},
				Visibility: &model.Visibility{Mode: model.VisibilityGMOnly}},
			{ID: "fact_private", SubjectID: "char_b", Predicate: "secret_stash",
				Value:      model.Value{Kind: "string", Raw: "gold"},
				Visibility: &model.Visibility{Mode: model.VisibilityPrivate, EntityIDs: []model.EntityID{"char_a"}}},
		},
		Threads: []model.WorldThread{
			{ID: "thread_public", Kind: model.ThreadKindQuest, Title: "Slay Dragon",
				Status: model.ThreadStatusActive, ParticipantIDs: []model.EntityID{"char_b"}},
			{ID: "thread_participant", Kind: model.ThreadKindMystery, Title: "Who Stole the Crown",
				Status: model.ThreadStatusOpen, ParticipantIDs: []model.EntityID{"char_a", "char_b"}},
			{ID: "thread_secret", Kind: model.ThreadKindConflict, Title: "Shadow War",
				Status:     model.ThreadStatusActive,
				Visibility: &model.Visibility{Mode: model.VisibilitySecret}},
			{ID: "thread_secret_but_participant", Kind: model.ThreadKindPersonal, Title: "My Secret Quest",
				Status:         model.ThreadStatusOpen,
				ParticipantIDs: []model.EntityID{"char_a"},
				Visibility:     &model.Visibility{Mode: model.VisibilitySecret}},
		},
		EventLog: []model.WorldEvent{
			{ID: "evt_1", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Description: "Public event"},
			{ID: "evt_2", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Description: "Secret event",
				Visibility: &model.Visibility{Mode: model.VisibilitySecret}},
			{ID: "evt_3", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Description: "Actor event",
				ActorIDs:   []model.EntityID{"char_a"},
				Visibility: &model.Visibility{Mode: model.VisibilitySecret}},
			{ID: "evt_4", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Description: "Target event",
				TargetIDs:  []model.EntityID{"char_a"},
				Visibility: &model.Visibility{Mode: model.VisibilityGMOnly}},
			{ID: "evt_5", Type: model.EventTypeNote, Source: model.EventSourceRuntime, Description: "Private to A",
				Visibility: &model.Visibility{Mode: model.VisibilityPrivate, EntityIDs: []model.EntityID{"char_a"}}},
		},
	}
}

func TestVisibleRelationsIncludesPublicAndPrivateToSelf(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	ids := make(map[model.RelationID]bool)
	for _, r := range got.Relations {
		ids[r.ID] = true
	}
	if !ids["rel_public"] {
		t.Error("expected rel_public to be visible")
	}
	if !ids["rel_private"] {
		t.Error("expected rel_private to be visible (char_a is in EntityIDs)")
	}
	if ids["rel_secret"] {
		t.Error("expected rel_secret to be hidden")
	}
}

func TestSecretRelationsExcluded(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, r := range got.Relations {
		if r.ID == "rel_secret" {
			t.Error("char_b should not see rel_secret")
		}
		if r.ID == "rel_private" {
			t.Error("char_b should not see rel_private (not in EntityIDs)")
		}
	}
}

func TestVisibleFactsIncludesPublicAndPrivateToSelf(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	ids := make(map[model.FactID]bool)
	for _, f := range got.Facts {
		ids[f.ID] = true
	}
	if !ids["fact_public"] {
		t.Error("expected fact_public to be visible")
	}
	if !ids["fact_private"] {
		t.Error("expected fact_private to be visible to char_a")
	}
	if ids["fact_gm"] {
		t.Error("expected fact_gm to be hidden from char_a")
	}
}

func TestGMOnlyFactsExcluded(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, f := range got.Facts {
		if f.ID == "fact_gm" {
			t.Error("char_b should not see fact_gm")
		}
		if f.ID == "fact_private" {
			t.Error("char_b should not see fact_private")
		}
	}
}

func TestThreadsWithParticipantAlwaysIncluded(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	ids := make(map[model.ThreadID]bool)
	for _, th := range got.Threads {
		ids[th.ID] = true
	}
	if !ids["thread_participant"] {
		t.Error("expected thread_participant (char_a is participant)")
	}
	if !ids["thread_secret_but_participant"] {
		t.Error("expected thread_secret_but_participant (char_a is participant despite secret visibility)")
	}
	if !ids["thread_public"] {
		t.Error("expected thread_public (nil visibility = public)")
	}
	if ids["thread_secret"] {
		t.Error("expected thread_secret to be hidden (char_a is not a participant)")
	}
}

func TestSecretThreadsWithoutParticipantExcluded(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_c"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, th := range got.Threads {
		if th.ID == "thread_secret" {
			t.Error("char_c should not see thread_secret")
		}
		if th.ID == "thread_secret_but_participant" {
			t.Error("char_c should not see thread_secret_but_participant")
		}
	}
}

func TestNearbyEntitiesAtSameLocation(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	nearbyIDs := make(map[model.EntityID]bool)
	for _, e := range got.NearbyEntities {
		nearbyIDs[e.ID] = true
	}
	if !nearbyIDs["char_b"] {
		t.Error("expected char_b nearby (same location: loc_tavern)")
	}
	if nearbyIDs["char_c"] {
		t.Error("char_c is at a different location, should not be nearby")
	}
	if nearbyIDs["char_a"] {
		t.Error("perspective entity should not appear in NearbyEntities")
	}
	if nearbyIDs["loc_tavern"] {
		t.Error("loc_tavern has no SpatialComponent, should not appear")
	}
}

func TestNearbyEntitiesEmptyWithoutSpatial(t *testing.T) {
	w := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_no_spatial": {ID: "char_no_spatial", Type: "character", Name: "Ghost"},
		},
	}
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_no_spatial"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(got.NearbyEntities) != 0 {
		t.Errorf("expected no nearby entities, got %d", len(got.NearbyEntities))
	}
}

func TestRecentEventsFiltersByVisibility(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	ids := make(map[model.EventID]bool)
	for _, e := range got.RecentEvents {
		ids[e.ID] = true
	}
	if !ids["evt_1"] {
		t.Error("expected evt_1 (nil visibility = public)")
	}
	if ids["evt_2"] {
		t.Error("expected evt_2 hidden (secret, char_a not actor/target)")
	}
	if !ids["evt_3"] {
		t.Error("expected evt_3 visible (char_a is actor)")
	}
	if !ids["evt_4"] {
		t.Error("expected evt_4 visible (char_a is target)")
	}
	if !ids["evt_5"] {
		t.Error("expected evt_5 visible (private to char_a)")
	}
}

func TestRecentEventsChronologicalOrder(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(got.RecentEvents) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(got.RecentEvents))
	}
	if got.RecentEvents[0].ID != "evt_1" {
		t.Errorf("expected first event to be evt_1, got %s", got.RecentEvents[0].ID)
	}
}

func TestMaxEventsLimitsRecentEvents(t *testing.T) {
	w := testWorldWithVisibility()
	got, err := CharacterContextView{}.Render(w, CharacterContextRequest{
		PerspectiveID: "char_a",
		MaxEvents:     2,
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(got.RecentEvents) > 2 {
		t.Errorf("expected at most 2 recent events, got %d", len(got.RecentEvents))
	}
}

func TestMemoryVisibilityFieldRespected(t *testing.T) {
	w := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {ID: "char_a", Type: "character", Name: "A"},
			"char_b": {ID: "char_b", Type: "character", Name: "B"},
		},
		Memory: []model.MemoryRecord{
			{
				ID:      "mem_vis_private",
				Owner:   model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Content: "Visible only to char_a via Visibility field.",
				Visibility: &model.Visibility{
					Mode:      model.VisibilityPrivate,
					EntityIDs: []model.EntityID{"char_a"},
				},
			},
			{
				ID:      "mem_vis_secret",
				Owner:   model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Content: "Secret via Visibility field.",
				Visibility: &model.Visibility{
					Mode: model.VisibilitySecret,
				},
			},
			{
				ID:          "mem_legacy_world",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Content:     "Legacy world memory with no Visibility field.",
				TruthStatus: model.TruthStatusTrue,
			},
		},
	}

	gotA, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render for char_a: %v", err)
	}
	aIDs := make(map[model.MemoryID]bool)
	for _, m := range gotA.Memories {
		aIDs[m.ID] = true
	}
	if !aIDs["mem_vis_private"] {
		t.Error("char_a should see mem_vis_private")
	}
	if aIDs["mem_vis_secret"] {
		t.Error("char_a should not see mem_vis_secret")
	}
	if !aIDs["mem_legacy_world"] {
		t.Error("char_a should see mem_legacy_world (nil Visibility, legacy fallback)")
	}

	gotB, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render for char_b: %v", err)
	}
	bIDs := make(map[model.MemoryID]bool)
	for _, m := range gotB.Memories {
		bIDs[m.ID] = true
	}
	if bIDs["mem_vis_private"] {
		t.Error("char_b should not see mem_vis_private")
	}
	if !bIDs["mem_legacy_world"] {
		t.Error("char_b should see mem_legacy_world")
	}
}

func TestFactionOnlyVisibility(t *testing.T) {
	w := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"char_a": {
				ID: "char_a", Type: "character", Name: "A",
				Components: map[string]any{
					"faction": model.NewFactionComponent([]model.EntityID{"faction_guild"}, "", 0.8),
				},
			},
			"char_b": {ID: "char_b", Type: "character", Name: "B"},
		},
		Facts: []model.Fact{
			{ID: "fact_faction", SubjectID: "char_a", Predicate: "guild_secret",
				Value:      model.Value{Kind: "string", Raw: "meeting at dawn"},
				Visibility: &model.Visibility{Mode: model.VisibilityFactionOnly, FactionIDs: []model.EntityID{"faction_guild"}}},
		},
	}
	gotA, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_a"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(gotA.Facts) != 1 || gotA.Facts[0].ID != "fact_faction" {
		t.Errorf("char_a (in faction_guild) should see faction-only fact, got %d facts", len(gotA.Facts))
	}

	gotB, err := CharacterContextView{}.Render(w, CharacterContextRequest{PerspectiveID: "char_b"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(gotB.Facts) != 0 {
		t.Errorf("char_b (no faction) should not see faction-only fact, got %d facts", len(gotB.Facts))
	}
}

func TestIsVisibleToNilVisibility(t *testing.T) {
	w := model.World{
		ID:       "test_world",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	if !IsVisibleTo(nil, "anyone", w) {
		t.Error("nil visibility should be visible to everyone")
	}
}

func TestIsVisibleToUnknownModeHidden(t *testing.T) {
	w := model.World{
		ID:       "test_world",
		Name:     "Test",
		Entities: map[model.EntityID]model.Entity{},
	}
	vis := &model.Visibility{Mode: "totally_made_up"}
	if IsVisibleTo(vis, "anyone", w) {
		t.Error("unknown visibility mode should default to hidden")
	}
}
