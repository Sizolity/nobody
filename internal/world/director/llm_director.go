package director

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

// TextGenerator abstracts an LLM inference call. Implementations wrap
// provider-specific chat APIs (llama.cpp, OpenAI, etc.).
type TextGenerator interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

type LLMDirectorConfig struct {
	SystemPrompt string
	Generator    TextGenerator
}

type LLMDirector struct {
	id     string
	config LLMDirectorConfig
}

func NewLLMDirector(id string, config LLMDirectorConfig) LLMDirector {
	return LLMDirector{id: id, config: config}
}

func (d LLMDirector) ID() string {
	return d.id
}

func (d LLMDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	userPrompt := buildWorldPrompt(ctx.World)
	response, err := d.config.Generator.Generate(ctx.Ctx, d.config.SystemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}
	events, err := parseEventResponse(response)
	if err != nil {
		return nil, fmt.Errorf("llm parse: %w", err)
	}
	return events, nil
}

func buildWorldPrompt(w model.World) string {
	data, err := json.Marshal(worldPromptContext{
		WorldID:  string(w.ID),
		Name:     w.Name,
		Entities: entitySummaries(w.Entities),
		Threads:  threadSummaries(w.Threads),
		Clock:    w.Clock.Sequence,
	})
	if err != nil {
		return fmt.Sprintf(`{"world_id":%q,"name":%q}`, w.ID, w.Name)
	}
	return string(data)
}

type worldPromptContext struct {
	WorldID  string           `json:"world_id"`
	Name     string           `json:"name"`
	Entities []entitySummary  `json:"entities,omitempty"`
	Threads  []threadSummary  `json:"threads,omitempty"`
	Clock    int64            `json:"clock"`
}

type entitySummary struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type threadSummary struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func entitySummaries(entities map[model.EntityID]model.Entity) []entitySummary {
	out := make([]entitySummary, 0, len(entities))
	for _, e := range entities {
		out = append(out, entitySummary{
			ID:   string(e.ID),
			Type: e.Type,
			Name: e.Name,
		})
	}
	return out
}

func threadSummaries(threads []model.WorldThread) []threadSummary {
	out := make([]threadSummary, 0, len(threads))
	for _, th := range threads {
		out = append(out, threadSummary{
			ID:     string(th.ID),
			Title:  th.Title,
			Status: th.Status,
		})
	}
	return out
}

func parseEventResponse(response string) ([]model.WorldEvent, error) {
	var events []model.WorldEvent
	if err := json.Unmarshal([]byte(response), &events); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("event[%d]: %w", i, err)
		}
	}
	return cloneEvents(events), nil
}
