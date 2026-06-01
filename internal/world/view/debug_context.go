package view

import (
	"sort"

	"github.com/sizolity/nobody/internal/world/model"
)

// WorldDebugContext is a read-only GM/debug projection that exposes the
// complete world truth with no ownership or visibility filtering.
type WorldDebugContext struct {
	World      WorldSummary           `json:"world"`
	Entities   []model.Entity         `json:"entities"`
	Facts      []model.Fact           `json:"facts"`
	Relations  []model.Relation       `json:"relations"`
	Memories   []model.MemoryRecord   `json:"memories"`
	Threads    []model.WorldThread    `json:"threads"`
	Rules      []model.Rule           `json:"rules"`
	EventLog   []model.WorldEvent     `json:"event_log"`
	EventQueue []model.EventQueueItem `json:"event_queue"`
}

// WorldSummary captures top-level world metadata without the collection fields.
type WorldSummary struct {
	ID          model.WorldID       `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Canon       model.Canon         `json:"canon,omitempty"`
	Clock       model.WorldClock    `json:"clock,omitempty"`
	Metadata    model.WorldMetadata `json:"metadata,omitempty"`
}

// WorldDebugView renders the full world state for GM / debug inspection.
// Unlike CharacterContextView it applies no ownership or truth-status
// filtering — secrets, private memories, and hidden narrator knowledge
// are all included.
type WorldDebugView struct{}

// Render projects the world into a WorldDebugContext.  Entities are sorted
// by ID for deterministic output.  All slice fields are guaranteed non-nil
// and fully deep-cloned, so mutating the returned context never affects
// the source world.
func (v WorldDebugView) Render(w model.World) WorldDebugContext {
	return WorldDebugContext{
		World: WorldSummary{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			Canon:       w.Canon.Clone(),
			Clock:       w.Clock.Clone(),
			Metadata:    w.Metadata.Clone(),
		},
		Entities:   entitiesSorted(w.Entities),
		Facts:      cloneFactSlice(w.Facts),
		Relations:  cloneRelationSlice(w.Relations),
		Memories:   cloneMemorySlice(w.Memory),
		Threads:    cloneThreadSlice(w.Threads),
		Rules:      cloneRuleSlice(w.Rules),
		EventLog:   cloneEventSlice(w.EventLog),
		EventQueue: cloneEventQueueItemSlice(w.EventQueue),
	}
}

func entitiesSorted(m map[model.EntityID]model.Entity) []model.Entity {
	if len(m) == 0 {
		return []model.Entity{}
	}
	ids := make([]model.EntityID, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]model.Entity, len(ids))
	for i, id := range ids {
		out[i] = m[id].Clone()
	}
	return out
}
