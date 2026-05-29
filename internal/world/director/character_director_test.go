package director

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

type mockGenerator struct {
	response string
	err      error
}

func (m mockGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	return m.response, m.err
}

type capturingCharacterGenerator struct {
	lastSystem string
	lastUser   string
	response   string
}

func (g *capturingCharacterGenerator) Generate(_ context.Context, system, user string) (string, error) {
	g.lastSystem = system
	g.lastUser = user
	return g.response, nil
}

func actorEntity(id model.EntityID, name string, goals []string) model.Entity {
	return model.Entity{
		ID:   id,
		Type: "character",
		Name: name,
		Components: map[string]any{
			"actor": model.NewActorComponent(true, goals),
		},
	}
}

func nonActorEntity(id model.EntityID, name string) model.Entity {
	return model.Entity{
		ID:   id,
		Type: "character",
		Name: name,
		Components: map[string]any{
			"actor": model.NewActorComponent(false, nil),
		},
	}
}

func validEventJSON(events ...model.WorldEvent) string {
	data, _ := json.Marshal(events)
	return string(data)
}

func TestCharacterDirectorID(t *testing.T) {
	t.Parallel()
	d := NewCharacterDirector("char_dir_1", CharacterDirectorConfig{})
	if d.ID() != "char_dir_1" {
		t.Fatalf("ID() = %q, want %q", d.ID(), "char_dir_1")
	}
}

func TestCharacterDirectorProposeNoActors(t *testing.T) {
	t.Parallel()

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator: mockGenerator{response: "[]"},
	})

	got, err := d.Propose(Context{
		Ctx: context.Background(),
		World: model.World{
			ID:   "test_world",
			Name: "Test",
			Entities: map[model.EntityID]model.Entity{
				"loc_tavern": {ID: "loc_tavern", Type: "location", Name: "Tavern"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty proposals, got %d events", len(got))
	}
}

func TestCharacterDirectorProposeOneActor(t *testing.T) {
	t.Parallel()

	events := []model.WorldEvent{{
		ID:          "event_alice_looks",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceDirector,
		Description: "Alice looks around the tavern.",
		ActorIDs:    []model.EntityID{"char_alice"},
	}}

	gen := &capturingCharacterGenerator{
		response: validEventJSON(events...),
	}

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator: gen,
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": actorEntity("char_alice", "Alice", []string{"find treasure"}),
		},
		Memory: []model.MemoryRecord{{
			ID:          "mem_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_alice"},
			Content:     "I saw a map in the library.",
			TruthStatus: model.TruthStatusTrue,
		}},
	}

	got, err := d.Propose(Context{Ctx: context.Background(), World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].ID != "event_alice_looks" {
		t.Errorf("event ID = %q", got[0].ID)
	}

	if gen.lastUser == "" {
		t.Fatal("user prompt is empty")
	}
	if !strings.Contains(gen.lastUser, "char_alice") {
		t.Error("user prompt does not contain character ID")
	}
	if !strings.Contains(gen.lastUser, "Alice") {
		t.Error("user prompt does not contain character name")
	}
	if !strings.Contains(gen.lastUser, "find treasure") {
		t.Error("user prompt does not contain character goal")
	}
	if !strings.Contains(gen.lastUser, "I saw a map") {
		t.Error("user prompt does not contain character memory")
	}
}

func TestCharacterDirectorFilterSkipsExcluded(t *testing.T) {
	t.Parallel()

	gen := &capturingCharacterGenerator{
		response: validEventJSON(model.WorldEvent{
			ID:          "event_bob_acts",
			Type:        model.EventTypeNote,
			Source:      model.EventSourceDirector,
			Description: "Bob acts.",
			ActorIDs:    []model.EntityID{"char_bob"},
		}),
	}

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator: gen,
		Filter: func(entity model.Entity, _ model.World) bool {
			return entity.ID == "char_bob"
		},
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": actorEntity("char_alice", "Alice", nil),
			"char_bob":   actorEntity("char_bob", "Bob", nil),
		},
	}

	got, err := d.Propose(Context{Ctx: context.Background(), World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event (only Bob), got %d", len(got))
	}

	if !strings.Contains(gen.lastUser, "char_bob") {
		t.Error("prompt should be for Bob, got prompt for someone else")
	}
}

func TestCharacterDirectorMaxActorsPerStep(t *testing.T) {
	t.Parallel()

	callCount := 0
	gen := &countingMockGenerator{
		response: validEventJSON(model.WorldEvent{
			ID:          "event_act",
			Type:        model.EventTypeNote,
			Source:      model.EventSourceDirector,
			Description: "Acts.",
		}),
		callCount: &callCount,
	}

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator:        gen,
		MaxActorsPerStep: 2,
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_a": actorEntity("char_a", "A", nil),
			"char_b": actorEntity("char_b", "B", nil),
			"char_c": actorEntity("char_c", "C", nil),
			"char_d": actorEntity("char_d", "D", nil),
		},
	}

	got, err := d.Propose(Context{Ctx: context.Background(), World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 Generator calls, got %d", callCount)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
}

func TestCharacterDirectorGeneratorErrorPropagates(t *testing.T) {
	t.Parallel()

	genErr := errors.New("inference timeout")
	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator: mockGenerator{err: genErr},
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": actorEntity("char_alice", "Alice", nil),
		},
	}

	_, err := d.Propose(Context{Ctx: context.Background(), World: world})
	if err == nil {
		t.Fatal("expected error from generator, got nil")
	}
	if !strings.Contains(err.Error(), "inference timeout") {
		t.Fatalf("error should contain generator error, got: %v", err)
	}
}

func TestCharacterDirectorInvalidJSONReturnsError(t *testing.T) {
	t.Parallel()

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator: mockGenerator{response: "this is not json"},
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": actorEntity("char_alice", "Alice", nil),
		},
	}

	_, err := d.Propose(Context{Ctx: context.Background(), World: world})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestCharacterDirectorEventsGetCorrectSourceAndActorIDs(t *testing.T) {
	t.Parallel()

	events := []model.WorldEvent{{
		ID:          "event_alice_speaks",
		Type:        model.EventTypeNote,
		Source:      "wrong_source",
		Description: "Alice speaks.",
	}}

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator: mockGenerator{response: validEventJSON(events...)},
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": actorEntity("char_alice", "Alice", nil),
		},
	}

	got, err := d.Propose(Context{Ctx: context.Background(), World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}

	if got[0].Source != model.EventSourceDirector {
		t.Errorf("Source = %q, want %q", got[0].Source, model.EventSourceDirector)
	}
	if len(got[0].ActorIDs) == 0 || got[0].ActorIDs[0] != "char_alice" {
		t.Errorf("ActorIDs = %v, want [char_alice ...]", got[0].ActorIDs)
	}
}

func TestCharacterDirectorUsesDefaultSystemPrompt(t *testing.T) {
	t.Parallel()

	gen := &capturingCharacterGenerator{response: "[]"}

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator: gen,
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": actorEntity("char_alice", "Alice", nil),
		},
	}

	d.Propose(Context{Ctx: context.Background(), World: world})

	if gen.lastSystem != DefaultCharacterSystemPrompt {
		t.Fatalf("expected DefaultCharacterSystemPrompt, got %q", gen.lastSystem[:60])
	}
}

func TestCharacterDirectorUsesCustomSystemPrompt(t *testing.T) {
	t.Parallel()

	gen := &capturingCharacterGenerator{response: "[]"}

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator:    gen,
		SystemPrompt: "You are a custom narrator.",
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": actorEntity("char_alice", "Alice", nil),
		},
	}

	d.Propose(Context{Ctx: context.Background(), World: world})

	if gen.lastSystem != "You are a custom narrator." {
		t.Fatalf("expected custom prompt, got %q", gen.lastSystem)
	}
}

func TestCharacterDirectorNonActorEntitiesSkipped(t *testing.T) {
	t.Parallel()

	d := NewCharacterDirector("cd_1", CharacterDirectorConfig{
		Generator: mockGenerator{response: "[]"},
	})

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_idle":  nonActorEntity("char_idle", "Idle"),
			"loc_tavern": {ID: "loc_tavern", Type: "location", Name: "Tavern"},
		},
	}

	got, err := d.Propose(Context{Ctx: context.Background(), World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 events for non-actors, got %d", len(got))
	}
}

func TestBuildCharacterPromptIncludesContext(t *testing.T) {
	t.Parallel()

	alice := model.Entity{
		ID:          "char_alice",
		Type:        "character",
		Name:        "Alice",
		Description: "A curious adventurer.",
		Components: map[string]any{
			"actor":   model.NewActorComponent(true, []string{"find treasure", "survive"}),
			"spatial": model.NewSpatialComponent("loc_tavern"),
		},
	}

	world := model.World{
		ID:   "test_world",
		Name: "Test",
		Entities: map[model.EntityID]model.Entity{
			"char_alice": alice,
			"char_bob": {
				ID: "char_bob", Type: "character", Name: "Bob",
				Components: map[string]any{
					"spatial": model.NewSpatialComponent("loc_tavern"),
				},
			},
			"loc_tavern": {ID: "loc_tavern", Type: "location", Name: "The Old Tavern"},
			"char_eve": {
				ID: "char_eve", Type: "character", Name: "Eve",
				Components: map[string]any{
					"spatial": model.NewSpatialComponent("loc_market"),
				},
			},
		},
		Memory: []model.MemoryRecord{
			{
				ID:          "mem_1",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_alice"},
				Content:     "I heard a rumor about a hidden cave.",
				TruthStatus: model.TruthStatusUnknown,
				Kind:        model.MemoryKindRumor,
			},
			{
				ID:          "mem_2",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
				Content:     "The kingdom is at war.",
				TruthStatus: model.TruthStatusTrue,
			},
			{
				ID:          "mem_3",
				Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_bob"},
				Content:     "Bob's private memory.",
				TruthStatus: model.TruthStatusTrue,
			},
		},
		Relations: []model.Relation{{
			ID: "rel_1", Type: "ally", SourceID: "char_alice", TargetID: "char_bob",
		}},
		Threads: []model.WorldThread{{
			ID: "thread_1", Title: "Find the treasure", Status: model.ThreadStatusOpen,
			Kind: model.ThreadKindQuest, ParticipantIDs: []model.EntityID{"char_alice"},
		}},
	}

	prompt := buildCharacterPrompt(alice, world)

	var parsed characterPromptContext
	if err := json.Unmarshal([]byte(prompt), &parsed); err != nil {
		t.Fatalf("prompt is not valid JSON: %v\nprompt: %s", err, prompt)
	}

	if parsed.CharacterID != "char_alice" {
		t.Errorf("character_id = %q", parsed.CharacterID)
	}
	if parsed.Name != "Alice" {
		t.Errorf("name = %q", parsed.Name)
	}
	if parsed.Description != "A curious adventurer." {
		t.Errorf("description = %q", parsed.Description)
	}
	if len(parsed.Goals) != 2 {
		t.Errorf("goals count = %d, want 2", len(parsed.Goals))
	}
	if parsed.Location != "The Old Tavern" {
		t.Errorf("location = %q, want %q", parsed.Location, "The Old Tavern")
	}
	if len(parsed.NearbyEntities) != 1 || parsed.NearbyEntities[0].Name != "Bob" {
		t.Errorf("nearby = %+v, want [Bob]", parsed.NearbyEntities)
	}
	if len(parsed.Memories) != 1 || !strings.Contains(parsed.Memories[0].Content, "hidden cave") {
		t.Errorf("memories = %+v, want Alice's memory only", parsed.Memories)
	}
	if len(parsed.Relations) != 1 || parsed.Relations[0].Type != "ally" {
		t.Errorf("relations = %+v", parsed.Relations)
	}
	if len(parsed.ActiveThreads) != 1 || parsed.ActiveThreads[0].Title != "Find the treasure" {
		t.Errorf("threads = %+v", parsed.ActiveThreads)
	}
}

func TestPrependActorID(t *testing.T) {
	t.Parallel()

	got := prependActorID("char_alice", nil)
	if len(got) != 1 || got[0] != "char_alice" {
		t.Fatalf("prepend to nil = %v", got)
	}

	got = prependActorID("char_alice", []model.EntityID{"char_bob"})
	if len(got) != 2 || got[0] != "char_alice" {
		t.Fatalf("prepend new = %v", got)
	}

	got = prependActorID("char_alice", []model.EntityID{"char_alice", "char_bob"})
	if len(got) != 2 {
		t.Fatalf("prepend existing = %v, should not duplicate", got)
	}
}

// --- test helper ---

type countingMockGenerator struct {
	response  string
	callCount *int
}

func (g *countingMockGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	*g.callCount++
	return g.response, nil
}
