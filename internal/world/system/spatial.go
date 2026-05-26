package system

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

type SpatialSystem struct{}

func (SpatialSystem) MoveEvent(world model.World, eventID model.EventID, entityID, locationID model.EntityID) (model.WorldEvent, error) {
	entity, ok := world.Entities[entityID]
	if !ok {
		return model.WorldEvent{}, fmt.Errorf("entity %q not found", entityID)
	}
	if _, ok := entity.SpatialComponent(); !ok {
		return model.WorldEvent{}, fmt.Errorf("entity %q has no spatial component", entityID)
	}
	if _, ok := world.Entities[locationID]; !ok {
		return model.WorldEvent{}, fmt.Errorf("location %q not found", locationID)
	}
	event := model.WorldEvent{
		ID:         eventID,
		Type:       model.EventTypeMove,
		Source:     model.EventSourceRuntime,
		ActorIDs:   []model.EntityID{entityID},
		LocationID: locationID,
		Effects: []model.Effect{{
			Kind:     model.EffectSetEntityComponent,
			TargetID: string(entityID),
			Payload: map[string]model.Value{
				"component": {Kind: model.ValueKindString, Raw: model.ComponentSpatial},
				"data":      {Kind: model.ValueKindObject, Raw: model.NewSpatialComponent(locationID)},
			},
		}},
	}
	if err := event.Validate(); err != nil {
		return model.WorldEvent{}, err
	}
	return event, nil
}
