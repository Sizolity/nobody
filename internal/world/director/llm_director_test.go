package director

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestLLMDirectorParsesGeneratedEvents(t *testing.T) {
	t.Parallel()

	events := []model.WorldEvent{{
		ID:          "event_llm_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceDirector,
		Description: "A merchant arrives at the tavern.",
	}}
	responseJSON, _ := json.Marshal(events)

	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		SystemPrompt: "You are a world director.",
		Generator: fakeGenerator{
			response: string(responseJSON),
		},
	})

	got, err := d.Propose(Context{
		Ctx:   context.Background(),
		World: model.World{ID: "test_world", Name: "Test"},
	})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_llm_1" {
		t.Fatalf("proposal mismatch: %#v", got)
	}
	if got[0].Description != "A merchant arrives at the tavern." {
		t.Fatalf("description = %q", got[0].Description)
	}
}

func TestLLMDirectorReturnsEmptyOnEmptyResponse(t *testing.T) {
	t.Parallel()

	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator: fakeGenerator{response: "[]"},
	})

	got, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty proposals: %#v", got)
	}
}

func TestLLMDirectorReturnsErrorOnInvalidJSON(t *testing.T) {
	t.Parallel()

	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator: fakeGenerator{response: "not json at all"},
	})

	_, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err == nil {
		t.Fatal("Propose returned nil for invalid JSON")
	}
}

func TestLLMDirectorReturnsErrorOnGeneratorFailure(t *testing.T) {
	t.Parallel()

	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator: fakeGenerator{err: context.DeadlineExceeded},
	})

	_, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err == nil {
		t.Fatal("Propose returned nil for generator error")
	}
}

func TestLLMDirectorValidatesProposedEvents(t *testing.T) {
	t.Parallel()

	badEvents := []model.WorldEvent{{ID: "event_1"}}
	responseJSON, _ := json.Marshal(badEvents)

	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		Generator: fakeGenerator{response: string(responseJSON)},
	})

	_, err := d.Propose(Context{Ctx: context.Background(), World: model.World{ID: "w", Name: "W"}})
	if err == nil {
		t.Fatal("Propose returned nil for invalid event (missing type/source)")
	}
}

func TestLLMDirectorPromptIncludesWorldContext(t *testing.T) {
	t.Parallel()

	gen := &capturingGenerator{}
	d := NewLLMDirector("llm_1", LLMDirectorConfig{
		SystemPrompt: "You are the narrator.",
		Generator:    gen,
	})

	d.Propose(Context{
		Ctx: context.Background(),
		World: model.World{
			ID:   "test_world",
			Name: "Kingdom of Shadows",
		},
	})

	if gen.lastSystem != "You are the narrator." {
		t.Fatalf("system prompt = %q", gen.lastSystem)
	}
	if gen.lastUser == "" {
		t.Fatal("user prompt is empty")
	}
}

type fakeGenerator struct {
	response string
	err      error
}

func (g fakeGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	return g.response, g.err
}

type capturingGenerator struct {
	lastSystem string
	lastUser   string
}

func (g *capturingGenerator) Generate(_ context.Context, system, user string) (string, error) {
	g.lastSystem = system
	g.lastUser = user
	return "[]", nil
}
