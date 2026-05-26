package bridge

import (
	"strings"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
)

func TestAdaptWorldMapsCanonToNarrativeWorld(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "The Fall of Kingdoms",
		Canon: model.Canon{
			Genre:      []string{"fantasy", "mystery"},
			Tone:       []string{"dark", "suspenseful"},
			Premise:    "A kingdom torn apart by a hidden conspiracy.",
			Laws:       []string{"Magic requires sacrifice", "The dead stay dead"},
			Boundaries: []string{"No modern technology"},
			StyleGuide: []string{"Use short sentences in combat scenes"},
		},
	}

	got := AdaptWorld(w, Options{})
	if got.World.ID != "test_world" {
		t.Fatalf("World.ID = %q", got.World.ID)
	}
	if got.World.Title != "The Fall of Kingdoms" {
		t.Fatalf("World.Title = %q", got.World.Title)
	}
	if got.World.Genre != "fantasy, mystery" {
		t.Fatalf("World.Genre = %q", got.World.Genre)
	}
	if got.World.Tone != "dark, suspenseful" {
		t.Fatalf("World.Tone = %q", got.World.Tone)
	}
	if len(got.World.Rules) != 2 || got.World.Rules[0] != "Magic requires sacrifice" {
		t.Fatalf("World.Rules = %#v", got.World.Rules)
	}
	if len(got.World.Boundaries) != 1 {
		t.Fatalf("World.Boundaries = %#v", got.World.Boundaries)
	}
	if got.World.StyleGuide != "Use short sentences in combat scenes" {
		t.Fatalf("World.StyleGuide = %q", got.World.StyleGuide)
	}
}

func TestAdaptWorldExtractsCharactersFromEntities(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": {
				ID:   "char_alice",
				Type: "character",
				Name: "Alice",
				Tags: []string{"brave", "curious"},
				Components: map[string]any{
					model.ComponentActor: model.NewActorComponent(true, []string{"find the truth"}),
				},
			},
			"sword_1": {
				ID:   "sword_1",
				Type: "item",
				Name: "Magic Sword",
			},
		},
	}

	got := AdaptWorld(w, Options{})
	if len(got.Characters) != 1 {
		t.Fatalf("Characters count = %d, want 1", len(got.Characters))
	}
	ch := got.Characters[0]
	if ch.ID != "char_alice" || ch.Name != "Alice" {
		t.Fatalf("character mismatch: %#v", ch)
	}
	if ch.Role != "character" {
		t.Fatalf("Role = %q", ch.Role)
	}
	if len(ch.Traits) != 2 || ch.Traits[0] != "brave" {
		t.Fatalf("Traits = %#v", ch.Traits)
	}
	if len(ch.Goals) != 1 || ch.Goals[0] != "find the truth" {
		t.Fatalf("Goals = %#v", ch.Goals)
	}
}

func TestAdaptWorldExtractsLocationsFromEntities(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"tavern": {
				ID:          "tavern",
				Type:        "location",
				Name:        "The Golden Harp",
				Description: "A cozy tavern at the crossroads.",
				Tags:        []string{"social", "safe"},
			},
			"char_1": {
				ID:   "char_1",
				Type: "character",
				Name: "Bob",
			},
		},
	}

	got := AdaptWorld(w, Options{})
	if len(got.Locations) != 1 {
		t.Fatalf("Locations count = %d, want 1", len(got.Locations))
	}
	loc := got.Locations[0]
	if loc.ID != "tavern" || loc.Name != "The Golden Harp" {
		t.Fatalf("location mismatch: %#v", loc)
	}
	if loc.Description != "A cozy tavern at the crossroads." {
		t.Fatalf("Description = %q", loc.Description)
	}
}

func TestAdaptWorldMapsEventsToNarrativeEvents(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		EventLog: []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeMove, Source: model.EventSourceRuntime, Description: "Alice moved to the tower.", ActorIDs: []model.EntityID{"char_alice"}, TargetIDs: []model.EntityID{"tower"}},
			{ID: "event_2", Type: model.EventTypeNote, Source: model.EventSourceDirector, Intent: "Introduce tension"},
		},
	}

	got := AdaptWorld(w, Options{RecentEvents: 10})
	if len(got.Events) != 2 {
		t.Fatalf("Events count = %d, want 2", len(got.Events))
	}
	if got.Events[0].ID != "event_1" || got.Events[0].Type != model.EventTypeMove {
		t.Fatalf("event 0 mismatch: %#v", got.Events[0])
	}
	if got.Events[0].Summary != "Alice moved to the tower." {
		t.Fatalf("Summary = %q", got.Events[0].Summary)
	}
	if len(got.Events[0].ParticipantIDs) != 2 {
		t.Fatalf("ParticipantIDs = %#v", got.Events[0].ParticipantIDs)
	}
	if got.Events[1].Summary != "Introduce tension" {
		t.Fatalf("event 1 Summary should fallback to Intent: %q", got.Events[1].Summary)
	}
}

func TestAdaptWorldMapsMemories(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		Memory: []model.MemoryRecord{
			{
				ID:          "memory_1",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_b"},
				Kind:        model.MemoryKindBelief,
				Content:     "A killed the king.",
				TruthStatus: model.TruthStatusUnknown,
				Importance:  0.7,
			},
		},
	}

	got := AdaptWorld(w, Options{})
	if len(got.Memories) != 1 {
		t.Fatalf("Memories count = %d, want 1", len(got.Memories))
	}
	m := got.Memories[0]
	if m.ID != "memory_1" || m.Text != "A killed the king." {
		t.Fatalf("memory mismatch: %#v", m)
	}
	if m.Type != model.MemoryKindBelief {
		t.Fatalf("Type = %q", m.Type)
	}
	if m.Subject != "char_b" {
		t.Fatalf("Subject = %q", m.Subject)
	}
	if m.Importance != 7 {
		t.Fatalf("Importance = %d, want 7 (0.7 * 10)", m.Importance)
	}
}

func TestAdaptWorldMapsThreadsToStoryGraph(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		Threads: []model.WorldThread{
			{ID: "thread_active", Kind: model.ThreadKindMystery, Title: "Find the killer", Status: model.ThreadStatusActive},
			{ID: "thread_open", Kind: model.ThreadKindQuest, Title: "Explore the ruins", Status: model.ThreadStatusOpen},
			{ID: "thread_done", Kind: model.ThreadKindPersonal, Title: "Old quest", Status: model.ThreadStatusResolved},
		},
	}

	got := AdaptWorld(w, Options{})
	if got.Graph.CurrentNodeID != "thread_active" {
		t.Fatalf("CurrentNodeID = %q, want thread_active", got.Graph.CurrentNodeID)
	}
	if len(got.Graph.Nodes) != 2 {
		t.Fatalf("Nodes count = %d, want 2 (only non-terminal threads)", len(got.Graph.Nodes))
	}
}

func TestAdaptWorldSetsUserInput(t *testing.T) {
	t.Parallel()

	w := model.World{ID: "test_world", Name: "Test"}
	got := AdaptWorld(w, Options{UserInput: "I search the room."})
	if got.Input != "I search the room." {
		t.Fatalf("Input = %q", got.Input)
	}
}

func TestAdaptWorldRecentEventsLimitsEvents(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		EventLog: []model.WorldEvent{
			{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest, Description: "a"},
			{ID: "event_2", Type: model.EventTypeNote, Source: model.EventSourceTest, Description: "b"},
			{ID: "event_3", Type: model.EventTypeNote, Source: model.EventSourceTest, Description: "c"},
		},
	}

	got := AdaptWorld(w, Options{RecentEvents: 2})
	if len(got.Events) != 2 {
		t.Fatalf("Events count = %d, want 2", len(got.Events))
	}
	if got.Events[0].ID != "event_2" || got.Events[1].ID != "event_3" {
		t.Fatalf("should take last 2 events: %#v", got.Events)
	}
}

func TestAdaptWorldEmptyWorldProducesValidBundle(t *testing.T) {
	t.Parallel()

	w := model.World{ID: "test_world", Name: "Empty"}
	got := AdaptWorld(w, Options{})

	if got.World.ID != "test_world" || got.World.Title != "Empty" {
		t.Fatalf("world mismatch: %#v", got.World)
	}
	if got.Characters == nil || got.Locations == nil || got.Events == nil || got.Memories == nil {
		t.Fatal("slices should be non-nil")
	}
	if len(got.Graph.Nodes) != 0 {
		t.Fatalf("Nodes should be empty: %#v", got.Graph.Nodes)
	}

	_ = strings.TrimSpace(got.Input)
}

func TestAdaptWorldMemoryFilterCapsMemories(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		Memory: []model.MemoryRecord{
			{ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "High.", Importance: 0.9, TruthStatus: model.TruthStatusTrue},
			{ID: "m2", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "Med.", Importance: 0.5, TruthStatus: model.TruthStatusTrue},
			{ID: "m3", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "Low.", Importance: 0.1, TruthStatus: model.TruthStatusTrue},
		},
	}

	got := AdaptWorld(w, Options{
		MemoryFilter: &store.MemoryFilter{MaxCount: 2},
	})
	if len(got.Memories) != 2 {
		t.Fatalf("Memories = %d, want 2", len(got.Memories))
	}
	if got.Memories[0].ID != "m1" {
		t.Errorf("first memory should be highest importance, got %s", got.Memories[0].ID)
	}
}

func TestAdaptWorldMemoryFilterExcludesSecret(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		Memory: []model.MemoryRecord{
			{ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "Public.", Importance: 0.5, TruthStatus: model.TruthStatusTrue},
			{ID: "m2", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "Secret.", Importance: 0.9, TruthStatus: model.TruthStatusSecret},
		},
	}

	got := AdaptWorld(w, Options{
		MemoryFilter: &store.MemoryFilter{ExcludeTruthStatus: []string{model.TruthStatusSecret}},
	})
	if len(got.Memories) != 1 {
		t.Fatalf("Memories = %d, want 1", len(got.Memories))
	}
	if got.Memories[0].ID != "m1" {
		t.Errorf("expected m1, got %s", got.Memories[0].ID)
	}
}

func TestAdaptWorldNilMemoryFilterIncludesAll(t *testing.T) {
	t.Parallel()

	w := model.World{
		ID:   "test_world",
		Name: "Test",
		Memory: []model.MemoryRecord{
			{ID: "m1", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "One.", TruthStatus: model.TruthStatusTrue},
			{ID: "m2", Owner: model.MemoryOwner{Kind: model.MemoryOwnerKindWorld}, Content: "Two.", TruthStatus: model.TruthStatusTrue},
		},
	}

	got := AdaptWorld(w, Options{})
	if len(got.Memories) != 2 {
		t.Fatalf("Memories = %d, want 2 (nil filter = include all)", len(got.Memories))
	}
}

func TestAdaptMemoriesPreservesKind(t *testing.T) {
	t.Parallel()

	memories := []model.MemoryRecord{
		{ID: "m1", Owner: model.MemoryOwner{Kind: "character", ID: "c1"}, Kind: "emotion", Content: "feels sad", Importance: 0.5},
		{ID: "m2", Owner: model.MemoryOwner{Kind: "character", ID: "c1"}, Kind: "belief", Content: "trusts nobody", Importance: 0.5},
		{ID: "m3", Owner: model.MemoryOwner{Kind: "character", ID: "c1"}, Kind: "secret", Content: "hidden power", Importance: 0.5},
		{ID: "m4", Owner: model.MemoryOwner{Kind: "character", ID: "c1"}, Kind: "observation", Content: "saw a bird", Importance: 0.5},
		{ID: "m5", Owner: model.MemoryOwner{Kind: "character", ID: "c1"}, Kind: "relationship", Content: "allied with Bob", Importance: 0.5},
	}
	result := adaptMemories(memories)
	expected := []string{"emotion", "belief", "secret", "observation", "relationship"}
	for i, m := range result {
		if m.Type != expected[i] {
			t.Errorf("memory %d: expected type %q, got %q", i, expected[i], m.Type)
		}
	}
}

func TestAdaptThreadsIncludesParticipantsAndLocation(t *testing.T) {
	t.Parallel()

	threads := []model.WorldThread{
		{
			ID:             "thread_1",
			Kind:           model.ThreadKindQuest,
			Title:          "Save the village",
			Status:         model.ThreadStatusActive,
			ParticipantIDs: []model.EntityID{"char_hero", "char_sage"},
			LocationID:     "loc_village",
		},
		{
			ID:     "thread_2",
			Kind:   model.ThreadKindMystery,
			Title:  "Who stole the gem",
			Status: model.ThreadStatusOpen,
		},
	}
	graph := adaptThreads(threads)
	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}

	node := graph.Nodes[0]
	if len(node.CharacterIDs) != 2 {
		t.Errorf("expected 2 character IDs, got %d", len(node.CharacterIDs))
	}
	if node.CharacterIDs[0] != "char_hero" || node.CharacterIDs[1] != "char_sage" {
		t.Errorf("unexpected CharacterIDs: %v", node.CharacterIDs)
	}
	if node.LocationID != "loc_village" {
		t.Errorf("expected location 'loc_village', got %q", node.LocationID)
	}

	node2 := graph.Nodes[1]
	if node2.CharacterIDs != nil {
		t.Errorf("expected nil CharacterIDs for thread without participants, got %v", node2.CharacterIDs)
	}
	if node2.LocationID != "" {
		t.Errorf("expected empty LocationID for thread without location, got %q", node2.LocationID)
	}
}

func TestAdaptEventsExtractsBeatID(t *testing.T) {
	t.Parallel()

	events := []model.WorldEvent{
		{ID: "beat_abc123", Type: "note", Source: model.EventSourceDirector, Description: "scene"},
		{ID: "event_xyz", Type: "note", Source: model.EventSourceDirector, Description: "action"},
		{ID: "manual_1", Type: "note", Source: model.EventSourceUser, Description: "user event"},
	}
	result := adaptEvents(events, 10)
	if result[0].BeatID != "beat_abc123" {
		t.Errorf("beat event: expected BeatID 'beat_abc123', got %q", result[0].BeatID)
	}
	if result[1].BeatID != "director" {
		t.Errorf("director event: expected BeatID 'director', got %q", result[1].BeatID)
	}
	if result[2].BeatID != "world" {
		t.Errorf("user event: expected BeatID 'world', got %q", result[2].BeatID)
	}
}
