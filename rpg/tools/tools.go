package tools

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/rule"
)

type LookupRulesParams struct {
	Category string   `json:"category" jsonschema:"description=Rule category to look up"`
	Tags     []string `json:"tags,omitempty" jsonschema:"description=Optional tag filter"`
}

type UpdateStateParams struct {
	EntityID string `json:"entity_id" jsonschema:"description=Target entity ID"`
	Key      string `json:"key" jsonschema:"description=State key to update"`
	Value    any    `json:"value" jsonschema:"description=New value (string or number or boolean)"`
}

type RollParams struct {
	Sides    int `json:"sides" jsonschema:"description=Number of sides (e.g. 20 for d20)"`
	Count    int `json:"count,omitempty" jsonschema:"description=Number of dice,default=1"`
	Modifier int `json:"modifier,omitempty" jsonschema:"description=Flat modifier added to total,default=0"`
}

type GetEntityStateParams struct {
	EntityID string `json:"entity_id" jsonschema:"description=Entity to inspect"`
}

// ToolContext holds state for tool execution within a single beat.
type ToolContext struct {
	World          model.World
	PendingEffects []model.Effect
	Rng            *rand.Rand
}

func (tc *ToolContext) LookupRules(params LookupRulesParams) (string, error) {
	rpgRules := rule.FromWorldRules(tc.World.Rules)
	results := rule.Lookup(rpgRules, rule.LookupFilter{
		Category: params.Category,
		Tags:     params.Tags,
	})
	return rule.FormatRules(results), nil
}

func (tc *ToolContext) UpdateState(params UpdateStateParams) (string, error) {
	entityID := model.EntityID(params.EntityID)
	if _, ok := tc.World.Entities[entityID]; !ok {
		return "", fmt.Errorf("entity %q not found", params.EntityID)
	}
	value := model.Value{Kind: inferValueKind(params.Value), Raw: params.Value}
	tc.PendingEffects = append(tc.PendingEffects, model.Effect{
		Kind:     model.EffectUpdateEntityState,
		TargetID: params.EntityID,
		Payload:  map[string]model.Value{params.Key: value},
	})
	return fmt.Sprintf("OK: %s.%s = %v", params.EntityID, params.Key, params.Value), nil
}

func (tc *ToolContext) Roll(params RollParams) (string, error) {
	count := params.Count
	if count < 1 {
		count = 1
	}
	sides := params.Sides
	if sides < 1 {
		return "", fmt.Errorf("sides must be >= 1")
	}
	rng := tc.Rng
	if rng == nil {
		rng = rand.New(rand.NewPCG(0, 0))
	}
	rolls := make([]int, count)
	total := params.Modifier
	for i := range rolls {
		rolls[i] = rng.IntN(sides) + 1
		total += rolls[i]
	}
	result, _ := json.Marshal(map[string]any{
		"rolls": rolls, "modifier": params.Modifier, "total": total,
	})
	return string(result), nil
}

func (tc *ToolContext) GetEntityState(params GetEntityStateParams) (string, error) {
	entity, ok := tc.World.Entities[model.EntityID(params.EntityID)]
	if !ok {
		return "", fmt.Errorf("entity %q not found", params.EntityID)
	}
	out := map[string]any{
		"id": entity.ID, "name": entity.Name, "type": entity.Type,
		"tags": entity.Tags, "state": entity.State,
	}
	data, _ := json.Marshal(out)
	return string(data), nil
}

func inferValueKind(v any) string {
	switch v.(type) {
	case float64, float32, int:
		return model.ValueKindNumber
	case bool:
		return model.ValueKindBoolean
	default:
		return model.ValueKindString
	}
}
