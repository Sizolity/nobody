package runtime

import (
	"fmt"
	"strings"

	"github.com/sizolity/nobody/internal/world/model"
)

const (
	preconditionOperatorEqual     = "=="
	preconditionOperatorNotEqual  = "!="
	preconditionOperatorExists    = "exists"
	preconditionOperatorNotExists = "not_exists"
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
	case preconditionOperatorExists:
		return exists, nil
	case preconditionOperatorNotExists:
		return !exists, nil
	case preconditionOperatorEqual:
		return exists && valuesEqual(value.Raw, condition.Value.Raw), nil
	case preconditionOperatorNotEqual:
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
	case preconditionOperatorExists:
		return exists, nil
	case preconditionOperatorNotExists:
		return !exists, nil
	default:
		return false, fmt.Errorf("unsupported %s precondition operator %q", kind, condition.Operator)
	}
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
