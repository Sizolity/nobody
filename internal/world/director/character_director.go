package director

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sizolity/nobody/internal/world/model"
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
// entity that has an ActorComponent with CanAct=true, gathers their
// personal context (goals, memories, spatial info), and uses a
// TextGenerator to propose actions from that character's perspective.
type CharacterDirector struct {
	id     string
	config CharacterDirectorConfig
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
		userPrompt := buildCharacterPrompt(actor, ctx.World)

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

func buildCharacterPrompt(actor model.Entity, w model.World) string {
	ctx := characterPromptContext{
		CharacterID: string(actor.ID),
		Name:        actor.Name,
		Description: actor.Description,
	}

	if ac, ok := actor.ActorComponent(); ok {
		ctx.Goals = ac.Goals
	}

	var locationID model.EntityID
	if sc, ok := actor.SpatialComponent(); ok {
		locationID = sc.LocationID
		if loc, exists := w.Entities[locationID]; exists {
			ctx.Location = loc.Name
		} else {
			ctx.Location = string(locationID)
		}
	}

	if locationID != "" {
		for _, entity := range w.Entities {
			if entity.ID == actor.ID {
				continue
			}
			if sc, ok := entity.SpatialComponent(); ok && sc.LocationID == locationID {
				ctx.NearbyEntities = append(ctx.NearbyEntities, characterNearby{
					ID:   string(entity.ID),
					Type: entity.Type,
					Name: entity.Name,
				})
			}
		}
		sort.Slice(ctx.NearbyEntities, func(i, j int) bool {
			return ctx.NearbyEntities[i].ID < ctx.NearbyEntities[j].ID
		})
	}

	actorIDStr := string(actor.ID)
	for _, mem := range w.Memory {
		if mem.Owner.Kind == model.MemoryOwnerKindCharacter && mem.Owner.ID == actorIDStr {
			ctx.Memories = append(ctx.Memories, characterMemory{
				ID:      string(mem.ID),
				Content: mem.Content,
				Kind:    mem.Kind,
				Scope:   mem.Scope,
			})
		}
	}

	for _, rel := range w.Relations {
		if rel.SourceID == actor.ID {
			ctx.Relations = append(ctx.Relations, characterRelation{
				Type:     rel.Type,
				TargetID: string(rel.TargetID),
			})
		} else if rel.TargetID == actor.ID {
			ctx.Relations = append(ctx.Relations, characterRelation{
				Type:     rel.Type,
				TargetID: string(rel.SourceID),
			})
		}
	}

	for _, th := range w.Threads {
		if th.Status != model.ThreadStatusOpen && th.Status != model.ThreadStatusActive {
			continue
		}
		for _, pid := range th.ParticipantIDs {
			if pid == actor.ID {
				ctx.ActiveThreads = append(ctx.ActiveThreads, characterThread{
					ID:     string(th.ID),
					Title:  th.Title,
					Status: th.Status,
				})
				break
			}
		}
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Sprintf(`{"character_id":%q,"name":%q}`, actor.ID, actor.Name)
	}
	return string(data)
}

// buildCharacterPromptReadable returns a human-readable version of the
// character prompt for debugging. Not used by Propose directly.
func buildCharacterPromptReadable(actor model.Entity, w model.World) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Character: %s (%s)\n", actor.Name, actor.ID)
	if actor.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", actor.Description)
	}

	if ac, ok := actor.ActorComponent(); ok && len(ac.Goals) > 0 {
		fmt.Fprintf(&b, "Goals: %s\n", strings.Join(ac.Goals, "; "))
	}

	if sc, ok := actor.SpatialComponent(); ok && sc.LocationID != "" {
		if loc, exists := w.Entities[sc.LocationID]; exists {
			fmt.Fprintf(&b, "Location: %s\n", loc.Name)
		}
	}

	return b.String()
}
