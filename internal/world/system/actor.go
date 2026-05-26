package system

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

type ActorSystem struct{}

func (ActorSystem) CanAct(world model.World, entityID model.EntityID) (bool, error) {
	actor, err := actorFor(world, entityID)
	if err != nil {
		return false, err
	}
	return actor.CanAct, nil
}

func (ActorSystem) SetCanActEvent(world model.World, eventID model.EventID, entityID model.EntityID, canAct bool) (model.WorldEvent, error) {
	actor, err := actorFor(world, entityID)
	if err != nil {
		return model.WorldEvent{}, err
	}
	return actorEvent(eventID, entityID, canAct, actor.Goals)
}

func (ActorSystem) SetGoalsEvent(world model.World, eventID model.EventID, entityID model.EntityID, goals []string) (model.WorldEvent, error) {
	actor, err := actorFor(world, entityID)
	if err != nil {
		return model.WorldEvent{}, err
	}
	return actorEvent(eventID, entityID, actor.CanAct, append([]string(nil), goals...))
}

func actorFor(world model.World, entityID model.EntityID) (model.ActorComponent, error) {
	entity, ok := world.Entities[entityID]
	if !ok {
		return model.ActorComponent{}, fmt.Errorf("entity %q not found", entityID)
	}
	actor, ok := entity.ActorComponent()
	if !ok {
		return model.ActorComponent{}, fmt.Errorf("entity %q has no actor component", entityID)
	}
	return actor, nil
}

func actorEvent(eventID model.EventID, entityID model.EntityID, canAct bool, goals []string) (model.WorldEvent, error) {
	event := model.WorldEvent{
		ID:       eventID,
		Type:     model.EventTypeActorChanged,
		Source:   model.EventSourceRuntime,
		ActorIDs: []model.EntityID{entityID},
		Effects: []model.Effect{{
			Kind:     model.EffectSetEntityComponent,
			TargetID: string(entityID),
			Payload: map[string]model.Value{
				"component": {Kind: model.ValueKindString, Raw: model.ComponentActor},
				"data":      {Kind: model.ValueKindObject, Raw: model.NewActorComponent(canAct, goals)},
			},
		}},
	}
	if err := event.Validate(); err != nil {
		return model.WorldEvent{}, err
	}
	return event, nil
}
