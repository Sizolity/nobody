package runtime

import (
	"fmt"
	"strings"

	"github.com/sizolity/nobody/internal/world/model"
)

func evaluateEventPreconditions(world model.World, event model.WorldEvent) error {
	for i, condition := range event.Preconditions {
		ok, err := evaluatePrecondition(world, event, condition)
		if err != nil {
			return fmt.Errorf("precondition %d: %w", i, err)
		}
		if !ok {
			return fmt.Errorf("precondition %d failed: %s %s", i, condition.Path, condition.Operator)
		}
	}
	return nil
}

func evaluatePrecondition(world model.World, event model.WorldEvent, condition model.Condition) (bool, error) {
	switch condition.Kind {
	case model.ConditionKindState:
		return evaluateStatePrecondition(world, event, condition)
	case model.ConditionKindFact:
		return evaluateIDExistencePrecondition(condition, factExists(world, model.FactID(condition.Path)), "fact")
	case model.ConditionKindRelation:
		return evaluateIDExistencePrecondition(condition, relationExists(world, model.RelationID(condition.Path)), "relation")
	case model.ConditionKindMemory:
		return evaluateMemoryPrecondition(world, condition)
	case model.ConditionKindStat:
		return evaluateStatPrecondition(world, event, condition)
	default:
		return false, fmt.Errorf("unsupported precondition kind %q", condition.Kind)
	}
}

func evaluateStatePrecondition(world model.World, event model.WorldEvent, condition model.Condition) (bool, error) {
	value, exists, err := stateValueForPath(world, event, condition.Path)
	if err != nil {
		return false, err
	}

	switch condition.Operator {
	case model.ConditionOperatorExists:
		return exists, nil
	case model.ConditionOperatorNotExists:
		return !exists, nil
	case model.ConditionOperatorEqual:
		return exists && valuesEqual(value.Raw, condition.Value.Raw), nil
	case model.ConditionOperatorNotEqual:
		return exists && !valuesEqual(value.Raw, condition.Value.Raw), nil
	default:
		return false, fmt.Errorf("unsupported state precondition operator %q", condition.Operator)
	}
}

func stateValueForPath(world model.World, event model.WorldEvent, path string) (model.Value, bool, error) {
	parts := strings.Split(path, ".")
	switch {
	case len(parts) == 4 && parts[0] == "entity" && parts[2] == "state":
		value, exists := entityStateValue(world, model.EntityID(parts[1]), parts[3])
		return value, exists, nil
	case len(parts) == 3 && parts[0] == "actor" && parts[1] == "state":
		if len(event.ActorIDs) == 0 {
			return model.Value{}, false, fmt.Errorf("actor.state precondition requires at least one actor")
		}
		value, exists := entityStateValue(world, event.ActorIDs[0], parts[2])
		return value, exists, nil
	default:
		return model.Value{}, false, fmt.Errorf("unsupported state precondition path %q", path)
	}
}

func entityStateValue(world model.World, entityID model.EntityID, key string) (model.Value, bool) {
	entity, ok := world.Entities[entityID]
	if !ok || entity.State == nil {
		return model.Value{}, false
	}
	value, ok := entity.State[key]
	return value, ok
}

func evaluateIDExistencePrecondition(condition model.Condition, exists bool, kind string) (bool, error) {
	switch condition.Operator {
	case model.ConditionOperatorExists:
		return exists, nil
	case model.ConditionOperatorNotExists:
		return !exists, nil
	default:
		return false, fmt.Errorf("unsupported %s precondition operator %q", kind, condition.Operator)
	}
}

// evaluateMemoryPrecondition reports whether a memory matching c.Path
// exists. Two path forms are supported:
//
//   - "<memory_id>": direct match on MemoryRecord.ID.
//   - "owner.<owner_kind>[.<owner_id>]": match any memory whose owner
//     matches. owner_id may be omitted for owner_kind=world (the only
//     kind allowed to have an empty owner ID per MemoryRecord.Validate),
//     in which case all memories with that kind match.
//
// Equality on memory content is intentionally out of scope: use
// ConditionKindFact for structured equality assertions.
func evaluateMemoryPrecondition(world model.World, c model.Condition) (bool, error) {
	switch c.Operator {
	case model.ConditionOperatorExists:
		return memoryExistsForPath(world, c.Path), nil
	case model.ConditionOperatorNotExists:
		return !memoryExistsForPath(world, c.Path), nil
	default:
		return false, fmt.Errorf("unsupported memory precondition operator %q", c.Operator)
	}
}

func memoryExistsForPath(world model.World, path string) bool {
	if !strings.HasPrefix(path, "owner.") {
		for _, m := range world.Memory {
			if string(m.ID) == path {
				return true
			}
		}
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(path, "owner."), ".", 2)
	kind := parts[0]
	id := ""
	if len(parts) == 2 {
		id = parts[1]
	}
	for _, m := range world.Memory {
		if m.Owner.Kind != kind {
			continue
		}
		if id == "" || m.Owner.ID == id {
			return true
		}
	}
	return false
}

// evaluateStatPrecondition reads a numeric/typed stat value off an
// entity's StatsComponent. Path forms:
//
//   - "<entity_id>.<stat_key>": drill into world.Entities[entity_id]
//     .StatsComponent().Values[stat_key].
//   - "actor.<stat_key>": shorthand for the first ActorIDs entry.
//
// Operators: ==, !=, exists, not_exists. If the entity or its
// StatsComponent is missing, the stat is treated as missing (exists=false
// — note that not_exists therefore returns true for a missing entity,
// which mirrors state-precondition semantics).
func evaluateStatPrecondition(world model.World, event model.WorldEvent, c model.Condition) (bool, error) {
	value, exists, err := statValueForPath(world, event, c.Path)
	if err != nil {
		return false, err
	}
	switch c.Operator {
	case model.ConditionOperatorExists:
		return exists, nil
	case model.ConditionOperatorNotExists:
		return !exists, nil
	case model.ConditionOperatorEqual:
		return exists && valuesEqual(value.Raw, c.Value.Raw), nil
	case model.ConditionOperatorNotEqual:
		return exists && !valuesEqual(value.Raw, c.Value.Raw), nil
	default:
		return false, fmt.Errorf("unsupported stat precondition operator %q", c.Operator)
	}
}

func statValueForPath(world model.World, event model.WorldEvent, path string) (model.Value, bool, error) {
	parts := strings.SplitN(path, ".", 2)
	if len(parts) != 2 {
		return model.Value{}, false, fmt.Errorf("stat path %q must be %q or %q", path, "<entity_id>.<stat_key>", "actor.<stat_key>")
	}
	entityID := model.EntityID(parts[0])
	statKey := parts[1]
	if parts[0] == "actor" {
		if len(event.ActorIDs) == 0 {
			return model.Value{}, false, fmt.Errorf("actor.<stat_key> precondition requires at least one actor")
		}
		entityID = event.ActorIDs[0]
	}
	entity, ok := world.Entities[entityID]
	if !ok {
		return model.Value{}, false, nil
	}
	stats, ok := entity.StatsComponent()
	if !ok {
		return model.Value{}, false, nil
	}
	value, ok := stats.Values[statKey]
	return value, ok, nil
}

func factExists(world model.World, id model.FactID) bool {
	for _, fact := range world.Facts {
		if fact.ID == id {
			return true
		}
	}
	return false
}

func relationExists(world model.World, id model.RelationID) bool {
	for _, relation := range world.Relations {
		if relation.ID == id {
			return true
		}
	}
	return false
}

func valuesEqual(left, right any) bool {
	if leftString, ok := left.(string); ok {
		rightString, ok := right.(string)
		return ok && leftString == rightString
	}
	if leftBool, ok := left.(bool); ok {
		rightBool, ok := right.(bool)
		return ok && leftBool == rightBool
	}
	if leftNumber, ok := numberValue(left); ok {
		rightNumber, ok := numberValue(right)
		return ok && leftNumber == rightNumber
	}
	return left == nil && right == nil
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}
