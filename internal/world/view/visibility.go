package view

import "github.com/sizolity/nobody/internal/world/model"

// IsVisibleTo reports whether a piece of world state with the given Visibility
// is perceivable by entityID within the supplied world.
// A nil visibility is treated as public (visible to everyone).
func IsVisibleTo(vis *model.Visibility, entityID model.EntityID, world model.World) bool {
	if vis == nil {
		return true
	}
	switch vis.Mode {
	case "", model.VisibilityPublic:
		return true
	case model.VisibilitySecret, model.VisibilityGMOnly, model.VisibilityNarratorOnly:
		return false
	case model.VisibilityPrivate, model.VisibilityOwnerOnly, model.VisibilityParticipantsOnly:
		return containsEntityID(vis.EntityIDs, entityID)
	case model.VisibilityFactionOnly:
		return entityInFactions(entityID, vis.FactionIDs, world)
	case model.VisibilityLocationOnly:
		return entityAtSameLocation(entityID, vis.EntityIDs, world)
	default:
		return false
	}
}

func containsEntityID(ids []model.EntityID, target model.EntityID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func entityInFactions(entityID model.EntityID, factionIDs []model.EntityID, world model.World) bool {
	entity, ok := world.Entities[entityID]
	if !ok {
		return false
	}
	fc, ok := entity.FactionComponent()
	if !ok {
		return false
	}
	for _, entityFaction := range fc.FactionIDs {
		for _, visFaction := range factionIDs {
			if entityFaction == visFaction {
				return true
			}
		}
	}
	return false
}

func entityAtSameLocation(entityID model.EntityID, referenceIDs []model.EntityID, world model.World) bool {
	entity, ok := world.Entities[entityID]
	if !ok {
		return false
	}
	sc, ok := entity.SpatialComponent()
	if !ok || sc.LocationID == "" {
		return false
	}
	for _, refID := range referenceIDs {
		ref, ok := world.Entities[refID]
		if !ok {
			continue
		}
		refSC, ok := ref.SpatialComponent()
		if !ok {
			continue
		}
		if refSC.LocationID == sc.LocationID {
			return true
		}
	}
	return false
}
