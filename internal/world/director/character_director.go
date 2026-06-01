package director

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/view"
)

// DefaultCharacterSystemPrompt instructs the LLM to act as a specific
// character and propose actions consistent with that character's knowledge.
const DefaultCharacterSystemPrompt = `You are acting as a specific character in a narrative simulation.
Based on the character's goals, personality, memories, and current situation, propose actions the character would take.

Propose 0-3 events as a JSON array. Each event MUST have:
- "id": unique snake_case identifier (e.g. "event_alice_explores_cave")
- "type": one of "note", "world_fact_changed", "remember", "move", "thread_changed"
- "source": always "director"
- "actor_ids": array containing the character's entity ID
- "description": one-sentence narrative description of the action

Events MAY include "effects" to mutate world state (same format as the world director).

Consider ONLY what the character knows from their own memories and observations.
Do not use information the character would not have access to.
If the character has no reason to act, return an empty array: []

Return ONLY a valid JSON array. No markdown, no explanation.`

// CharacterDirectorConfig configures a CharacterDirector.
type CharacterDirectorConfig struct {
	// Generator is the LLM backend used to propose character actions.
	Generator TextGenerator

	// SystemPrompt overrides the default character prompt template.
	// If empty, DefaultCharacterSystemPrompt is used.
	SystemPrompt string

	// MaxActorsPerStep limits how many characters act in one runtime step.
	// 0 means no limit.
	MaxActorsPerStep int

	// Filter optionally selects which actors should act this step.
	// If nil, all actors with CanAct=true are considered.
	Filter func(entity model.Entity, world model.World) bool
}

// CharacterDirector is a per-character event proposer. It examines each
// entity that has an ActorComponent with CanAct=true, renders the
// visibility-aware CharacterContext via CharacterContextView, and uses
// a TextGenerator to propose actions from that character's perspective.
// Routing through the view guarantees the LLM only sees state the
// character can actually perceive (memories, relations, threads,
// nearby entities).
type CharacterDirector struct {
	id     string
	config CharacterDirectorConfig
	view   view.CharacterContextView
}

// NewCharacterDirector creates a CharacterDirector with the given id and config.
func NewCharacterDirector(id string, config CharacterDirectorConfig) CharacterDirector {
	return CharacterDirector{id: id, config: config}
}

func (d CharacterDirector) ID() string { return d.id }

func (d CharacterDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	actors := d.selectActors(ctx.World)
	if len(actors) == 0 {
		return []model.WorldEvent{}, nil
	}

	systemPrompt := d.config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultCharacterSystemPrompt
	}

	var allEvents []model.WorldEvent
	for _, actor := range actors {
		cc, err := d.view.Render(ctx.World, view.CharacterContextRequest{PerspectiveID: actor.ID})
		if err != nil {
			return nil, fmt.Errorf("character %s: render context: %w", actor.ID, err)
		}

		userPrompt := buildCharacterPrompt(cc)

		response, err := d.config.Generator.Generate(ctx.Ctx, systemPrompt, userPrompt)
		if err != nil {
			return nil, fmt.Errorf("character %s: llm generate: %w", actor.ID, err)
		}

		events, err := parseEventResponse(response)
		if err != nil {
			return nil, fmt.Errorf("character %s: %w", actor.ID, err)
		}

		for i := range events {
			events[i].Source = model.EventSourceDirector
			events[i].ActorIDs = prependActorID(actor.ID, events[i].ActorIDs)
		}

		allEvents = append(allEvents, events...)
	}

	if allEvents == nil {
		return []model.WorldEvent{}, nil
	}
	return allEvents, nil
}

// selectActors returns entities with CanAct=true, applying the optional
// filter and MaxActorsPerStep limit. Entities are sorted by ID for
// deterministic ordering.
func (d CharacterDirector) selectActors(w model.World) []model.Entity {
	var actors []model.Entity
	for _, entity := range w.Entities {
		ac, ok := entity.ActorComponent()
		if !ok || !ac.CanAct {
			continue
		}
		if d.config.Filter != nil && !d.config.Filter(entity, w) {
			continue
		}
		actors = append(actors, entity)
	}

	sort.Slice(actors, func(i, j int) bool {
		return actors[i].ID < actors[j].ID
	})

	if d.config.MaxActorsPerStep > 0 && len(actors) > d.config.MaxActorsPerStep {
		actors = actors[:d.config.MaxActorsPerStep]
	}
	return actors
}

// prependActorID ensures actorID is the first element in the IDs slice,
// avoiding duplicates.
func prependActorID(actorID model.EntityID, ids []model.EntityID) []model.EntityID {
	for _, id := range ids {
		if id == actorID {
			return ids
		}
	}
	return append([]model.EntityID{actorID}, ids...)
}

// characterPromptContext is the JSON structure sent as the user prompt
// for a single character's perspective.
type characterPromptContext struct {
	CharacterID   string              `json:"character_id"`
	Name          string              `json:"name"`
	Description   string              `json:"description,omitempty"`
	Goals         []string            `json:"goals,omitempty"`
	Location      string              `json:"location,omitempty"`
	NearbyEntities []characterNearby  `json:"nearby_entities,omitempty"`
	Memories      []characterMemory   `json:"memories,omitempty"`
	Relations     []characterRelation `json:"relations,omitempty"`
	ActiveThreads []characterThread   `json:"active_threads,omitempty"`
}

type characterNearby struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type characterMemory struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Kind    string `json:"kind,omitempty"`
	Scope   string `json:"scope,omitempty"`
}

type characterRelation struct {
	Type     string `json:"type"`
	TargetID string `json:"target_id"`
}

type characterThread struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// buildCharacterPrompt serializes a visibility-filtered CharacterContext
// (produced by view.CharacterContextView) into the JSON schema the LLM
// expects. The director MUST source all character-perceivable state from
// this projection; reading directly from the World would expose private
// memories, hidden relations, or unseen threads to the LLM.
func buildCharacterPrompt(cc view.CharacterContext) string {
	actor := cc.Perspective
	ctx := characterPromptContext{
		CharacterID: string(actor.ID),
		Name:        actor.Name,
		Description: actor.Description,
	}

	if ac, ok := actor.ActorComponent(); ok {
		ctx.Goals = ac.Goals
	}

	if cc.Location != nil {
		ctx.Location = cc.Location.Name
	} else if sc, ok := actor.SpatialComponent(); ok && sc.LocationID != "" {
		ctx.Location = string(sc.LocationID)
	}

	for _, nearby := range cc.NearbyEntities {
		ctx.NearbyEntities = append(ctx.NearbyEntities, characterNearby{
			ID:   string(nearby.ID),
			Type: nearby.Type,
			Name: nearby.Name,
		})
	}
	sort.Slice(ctx.NearbyEntities, func(i, j int) bool {
		return ctx.NearbyEntities[i].ID < ctx.NearbyEntities[j].ID
	})

	for _, mem := range cc.Memories {
		ctx.Memories = append(ctx.Memories, characterMemory{
			ID:      string(mem.ID),
			Content: mem.Content,
			Kind:    mem.Kind,
			Scope:   mem.Scope,
		})
	}

	for _, rel := range cc.Relations {
		target := rel.TargetID
		if rel.TargetID == actor.ID {
			target = rel.SourceID
		}
		ctx.Relations = append(ctx.Relations, characterRelation{
			Type:     rel.Type,
			TargetID: string(target),
		})
	}

	for _, th := range cc.Threads {
		if th.Status != model.ThreadStatusOpen && th.Status != model.ThreadStatusActive {
			continue
		}
		ctx.ActiveThreads = append(ctx.ActiveThreads, characterThread{
			ID:     string(th.ID),
			Title:  th.Title,
			Status: th.Status,
		})
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Sprintf(`{"character_id":%q,"name":%q}`, actor.ID, actor.Name)
	}
	return string(data)
}

