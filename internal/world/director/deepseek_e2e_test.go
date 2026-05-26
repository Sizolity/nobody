package director

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestE2E_LLMDirector_DeepSeek(t *testing.T) {
	if os.Getenv("NOBODY_E2E_DEEPSEEK") == "" {
		t.Skip("set NOBODY_E2E_DEEPSEEK=1 and DEEPSEEK_API_KEY to run")
	}
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		t.Fatal("DEEPSEEK_API_KEY is required for E2E test")
	}

	gen := NewDeepSeekGenerator(DeepSeekGeneratorConfig{
		APIKey: apiKey,
	})

	systemPrompt := `You are a world director for a narrative simulation engine.
Given the current world state as JSON, propose exactly ONE world event as a JSON array.

Each event MUST have these fields:
- "id": a unique snake_case string (e.g. "event_merchant_arrives")
- "type": must be "note"
- "source": must be "director"
- "description": a one-sentence narrative description

Return ONLY a JSON array with one event object. No markdown, no explanation.
Example: [{"id":"event_dawn_breaks","type":"note","source":"director","description":"Dawn breaks over the village."}]`

	d := NewLLMDirector("e2e_deepseek", LLMDirectorConfig{
		SystemPrompt: systemPrompt,
		Generator:    gen,
	})

	world := model.World{
		ID:   "e2e_world",
		Name: "The Kingdom of Shadows",
		Entities: map[model.EntityID]model.Entity{
			"char_elara": {ID: "char_elara", Type: "character", Name: "Elara the Scholar"},
			"tavern":     {ID: "tavern", Type: "location", Name: "The Dusty Lantern"},
		},
		Threads: []model.WorldThread{
			{ID: "thread_missing_tome", Kind: model.ThreadKindMystery, Title: "The Missing Tome", Status: model.ThreadStatusActive},
		},
		Clock: model.WorldClock{Sequence: 1},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	events, err := d.Propose(Context{Ctx: ctx, World: world})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("Propose returned 0 events")
	}

	for i, e := range events {
		if e.ID == "" {
			t.Fatalf("event[%d].id is empty", i)
		}
		if e.Type == "" {
			t.Fatalf("event[%d].type is empty", i)
		}
		if e.Source == "" {
			t.Fatalf("event[%d].source is empty", i)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("event[%d] validation failed: %v", i, err)
		}
		t.Logf("event[%d]: id=%s type=%s desc=%q", i, e.ID, e.Type, e.Description)
	}
}
