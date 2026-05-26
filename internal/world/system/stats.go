package system

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

type StatsSystem struct{}

func (StatsSystem) SetStatEvent(world model.World, eventID model.EventID, entityID model.EntityID, stat string, value model.Value) (model.WorldEvent, error) {
	stats, err := statsFor(world, entityID)
	if err != nil {
		return model.WorldEvent{}, err
	}
	values := cloneStatsValues(stats.Values)
	values[stat] = value
	return statsEvent(eventID, entityID, values)
}

func (StatsSystem) AdjustNumberStatEvent(world model.World, eventID model.EventID, entityID model.EntityID, stat string, delta float64) (model.WorldEvent, error) {
	stats, err := statsFor(world, entityID)
	if err != nil {
		return model.WorldEvent{}, err
	}
	current, ok := stats.Values[stat]
	if !ok {
		return model.WorldEvent{}, fmt.Errorf("stat %q not found", stat)
	}
	currentValue, ok := numberValue(current.Raw)
	if !ok || current.Kind != model.ValueKindNumber {
		return model.WorldEvent{}, fmt.Errorf("stat %q is not numeric", stat)
	}
	values := cloneStatsValues(stats.Values)
	current.Raw = currentValue + delta
	values[stat] = current
	return statsEvent(eventID, entityID, values)
}

func statsFor(world model.World, entityID model.EntityID) (model.StatsComponent, error) {
	entity, ok := world.Entities[entityID]
	if !ok {
		return model.StatsComponent{}, fmt.Errorf("entity %q not found", entityID)
	}
	stats, ok := entity.StatsComponent()
	if !ok {
		return model.StatsComponent{}, fmt.Errorf("entity %q has no stats component", entityID)
	}
	return stats, nil
}

func statsEvent(eventID model.EventID, entityID model.EntityID, values map[string]model.Value) (model.WorldEvent, error) {
	event := model.WorldEvent{
		ID:       eventID,
		Type:     model.EventTypeStatsChanged,
		Source:   model.EventSourceRuntime,
		ActorIDs: []model.EntityID{entityID},
		Effects: []model.Effect{{
			Kind:     model.EffectSetEntityComponent,
			TargetID: string(entityID),
			Payload: map[string]model.Value{
				"component": {Kind: model.ValueKindString, Raw: model.ComponentStats},
				"data":      {Kind: model.ValueKindObject, Raw: model.NewStatsComponent(values)},
			},
		}},
	}
	if err := event.Validate(); err != nil {
		return model.WorldEvent{}, err
	}
	return event, nil
}

func cloneStatsValues(values map[string]model.Value) map[string]model.Value {
	out := make(map[string]model.Value, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}
