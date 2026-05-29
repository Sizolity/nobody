package view

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

const defaultMaxEvents = 20

type CharacterContextRequest struct {
	PerspectiveID model.EntityID
	MaxEvents     int // limit recent visible events; 0 = default (20)
}

type CharacterContext struct {
	Perspective    model.Entity
	Memories       []model.MemoryRecord
	Relations      []model.Relation
	Facts          []model.Fact
	Threads        []model.WorldThread
	NearbyEntities []model.Entity
	RecentEvents   []model.WorldEvent
}

type CharacterContextView struct{}

func (v CharacterContextView) Render(world model.World, req CharacterContextRequest) (CharacterContext, error) {
	if req.PerspectiveID == "" {
		return CharacterContext{}, fmt.Errorf("perspective_id is required")
	}
	if err := model.ValidateID(string(req.PerspectiveID)); err != nil {
		return CharacterContext{}, fmt.Errorf("perspective_id: %w", err)
	}
	perspective, ok := world.Entities[req.PerspectiveID]
	if !ok {
		return CharacterContext{}, fmt.Errorf("perspective entity %q not found", req.PerspectiveID)
	}

	maxEvents := req.MaxEvents
	if maxEvents <= 0 {
		maxEvents = defaultMaxEvents
	}

	return CharacterContext{
		Perspective:    perspective,
		Memories:       visibleMemories(world, req.PerspectiveID),
		Relations:      visibleRelations(world, req.PerspectiveID),
		Facts:          visibleFacts(world, req.PerspectiveID),
		Threads:        visibleThreads(world, req.PerspectiveID),
		NearbyEntities: nearbyEntities(world, req.PerspectiveID),
		RecentEvents:   visibleRecentEvents(world, req.PerspectiveID, maxEvents),
	}, nil
}

// VisibleMemoriesForCharacter is the legacy API kept for backward compatibility.
// It does not consult the Visibility field; use visibleMemories for full filtering.
func VisibleMemoriesForCharacter(memories []model.MemoryRecord, characterID model.EntityID) []model.MemoryRecord {
	visible := make([]model.MemoryRecord, 0, len(memories))
	for _, memory := range memories {
		if isVisibleMemoryForCharacter(memory, characterID) {
			visible = append(visible, memory)
		}
	}
	return visible
}

func visibleMemories(world model.World, perspectiveID model.EntityID) []model.MemoryRecord {
	visible := make([]model.MemoryRecord, 0, len(world.Memory))
	for _, memory := range world.Memory {
		if memory.Visibility != nil {
			if IsVisibleTo(memory.Visibility, perspectiveID, world) {
				visible = append(visible, memory)
			}
			continue
		}
		if isVisibleMemoryForCharacter(memory, perspectiveID) {
			visible = append(visible, memory)
		}
	}
	return visible
}

func isVisibleMemoryForCharacter(memory model.MemoryRecord, characterID model.EntityID) bool {
	switch memory.Owner.Kind {
	case model.MemoryOwnerKindCharacter:
		return memory.Owner.ID == string(characterID)
	case model.MemoryOwnerKindWorld:
		return isPublicWorldTruthStatus(memory.TruthStatus)
	default:
		return false
	}
}

func isPublicWorldTruthStatus(status string) bool {
	switch status {
	case "", model.TruthStatusTrue, model.TruthStatusFalse, model.TruthStatusUnknown, model.TruthStatusDisputed, model.TruthStatusOutdated:
		return true
	default:
		return false
	}
}

func visibleRelations(world model.World, perspectiveID model.EntityID) []model.Relation {
	out := make([]model.Relation, 0, len(world.Relations))
	for _, rel := range world.Relations {
		if !IsVisibleTo(rel.Visibility, perspectiveID, world) {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func visibleFacts(world model.World, perspectiveID model.EntityID) []model.Fact {
	out := make([]model.Fact, 0, len(world.Facts))
	for _, fact := range world.Facts {
		if IsVisibleTo(fact.Visibility, perspectiveID, world) {
			out = append(out, fact)
		}
	}
	return out
}

func visibleThreads(world model.World, perspectiveID model.EntityID) []model.WorldThread {
	out := make([]model.WorldThread, 0, len(world.Threads))
	for _, thread := range world.Threads {
		if containsEntityID(thread.ParticipantIDs, perspectiveID) {
			out = append(out, thread)
			continue
		}
		if IsVisibleTo(thread.Visibility, perspectiveID, world) {
			out = append(out, thread)
		}
	}
	return out
}

func nearbyEntities(world model.World, perspectiveID model.EntityID) []model.Entity {
	perspective, ok := world.Entities[perspectiveID]
	if !ok {
		return []model.Entity{}
	}
	sc, ok := perspective.SpatialComponent()
	if !ok || sc.LocationID == "" {
		return []model.Entity{}
	}
	out := make([]model.Entity, 0)
	for id, entity := range world.Entities {
		if id == perspectiveID {
			continue
		}
		esc, ok := entity.SpatialComponent()
		if !ok {
			continue
		}
		if esc.LocationID == sc.LocationID {
			out = append(out, entity)
		}
	}
	return out
}

func visibleRecentEvents(world model.World, perspectiveID model.EntityID, limit int) []model.WorldEvent {
	out := make([]model.WorldEvent, 0, limit)
	for i := len(world.EventLog) - 1; i >= 0 && len(out) < limit; i-- {
		event := world.EventLog[i]
		if IsVisibleTo(event.Visibility, perspectiveID, world) {
			out = append(out, event)
			continue
		}
		if containsEntityID(event.ActorIDs, perspectiveID) || containsEntityID(event.TargetIDs, perspectiveID) {
			out = append(out, event)
		}
	}
	// Reverse to chronological order.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
