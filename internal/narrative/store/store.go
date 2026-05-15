package store

import (
	"context"

	"github.com/sizolity/nobody/internal/narrative"
)

type Store interface {
	SaveWorld(ctx context.Context, world narrative.World) error
	LoadWorld(ctx context.Context, worldID string) (narrative.World, error)
	SaveCharacter(ctx context.Context, worldID string, character narrative.Character) error
	LoadCharacter(ctx context.Context, worldID, characterID string) (narrative.Character, error)
	SaveLocation(ctx context.Context, worldID string, location narrative.Location) error
	LoadLocation(ctx context.Context, worldID, locationID string) (narrative.Location, error)
	SaveStoryGraph(ctx context.Context, worldID string, graph narrative.StoryGraph) error
	LoadStoryGraph(ctx context.Context, worldID string) (narrative.StoryGraph, error)
	AppendEvent(ctx context.Context, worldID string, event narrative.NarrativeEvent) error
	ListEvents(ctx context.Context, worldID string) ([]narrative.NarrativeEvent, error)
	AppendMemory(ctx context.Context, worldID string, memory narrative.Memory) error
	ListMemories(ctx context.Context, worldID string) ([]narrative.Memory, error)
	SaveDraft(ctx context.Context, worldID string, draft narrative.Draft) error
	LoadDraft(ctx context.Context, worldID, draftID string) (narrative.Draft, error)
}
