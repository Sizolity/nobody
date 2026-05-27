package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/fog"
	"github.com/sizolity/nobody/rpg/rule"
)

// Tool descriptions are shared between NewInvokableTools (full set) and
// NewDisclosedTools (progressive disclosure) so the two factories cannot drift.
const (
	descLookupRules      = "Retrieve detailed rules for a specific category. Use when you need mechanics before making decisions."
	descUpdateState      = "Apply a validated state change to an entity. Use for precise numeric changes or status transitions."
	descRoll             = "Roll dice for randomized outcomes. Returns the numeric result."
	descGetEntityState   = "Read-only inspection of an entity's current state."
	descExploreKnowledge = "Reveal a hidden entity or fact to make it available in future beats. Call when the player discovers something new through exploration, interaction, or study."
)

type LookupRulesParams struct {
	Category string   `json:"category" jsonschema:"description=Rule category to look up"`
	Tags     []string `json:"tags,omitempty" jsonschema:"description=Optional tag filter"`
}

type UpdateStateParams struct {
	EntityID string `json:"entity_id" jsonschema:"required,description=Target entity ID"`
	Key      string `json:"key" jsonschema:"required,description=State key to update"`
	Value    any    `json:"value" jsonschema:"required,description=New value (string or number or boolean)"`
}

type RollParams struct {
	Sides    int `json:"sides" jsonschema:"required,description=Number of sides (e.g. 20 for d20)"`
	Count    int `json:"count,omitempty" jsonschema:"description=Number of dice (default 1)"`
	Modifier int `json:"modifier,omitempty" jsonschema:"description=Flat modifier added to total (default 0)"`
}

type GetEntityStateParams struct {
	EntityID string `json:"entity_id" jsonschema:"required,description=Entity to inspect"`
}

type ExploreKnowledgeParams struct {
	TargetID string `json:"target_id" jsonschema:"required,description=Entity or fact ID to reveal"`
	Level    string `json:"level,omitempty" jsonschema:"description=Target visibility: known or explored (default: explored)"`
	Piece    string `json:"piece,omitempty" jsonschema:"description=Specific knowledge piece to unlock within an entity"`
}

// ToolContext holds mutable state for tool execution within a single beat.
// It is goroutine-safe for use within Eino's tool node.
type ToolContext struct {
	mu             sync.Mutex
	World          model.World
	PendingEffects []model.Effect
	Rng            *rand.Rand
	Disclosure     *fog.DisclosureState // nil = fog disabled
}

// GetPendingEffects returns a copy of accumulated effects (goroutine-safe).
func (tc *ToolContext) GetPendingEffects() []model.Effect {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	out := make([]model.Effect, len(tc.PendingEffects))
	copy(out, tc.PendingEffects)
	return out
}

func (tc *ToolContext) LookupRules(_ context.Context, params *LookupRulesParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	rpgRules := rule.FromWorldRules(tc.World.Rules)
	results := rule.Lookup(rpgRules, rule.LookupFilter{
		Category: params.Category,
		Tags:     params.Tags,
	})
	return rule.FormatRules(results), nil
}

func (tc *ToolContext) UpdateState(_ context.Context, params *UpdateStateParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
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

func (tc *ToolContext) Roll(_ context.Context, params *RollParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	count := params.Count
	if count < 1 {
		count = 1
	}
	if params.Sides < 1 {
		return "", fmt.Errorf("sides must be >= 1")
	}
	rng := tc.Rng
	if rng == nil {
		rng = rand.New(rand.NewPCG(0, 0))
	}
	rolls := make([]int, count)
	total := params.Modifier
	for i := range rolls {
		rolls[i] = rng.IntN(params.Sides) + 1
		total += rolls[i]
	}
	result, _ := json.Marshal(map[string]any{
		"rolls": rolls, "modifier": params.Modifier, "total": total,
	})
	return string(result), nil
}

func (tc *ToolContext) GetEntityState(_ context.Context, params *GetEntityStateParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
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

// NewInvokableTools creates Eino InvokableTool instances bound to a ToolContext.
// These can be passed directly to react.AgentConfig.ToolsConfig.
func NewInvokableTools(tc *ToolContext) ([]tool.InvokableTool, error) {
	lookupRules, err := utils.InferTool("lookup_rules", descLookupRules, tc.LookupRules)
	if err != nil {
		return nil, fmt.Errorf("infer lookup_rules: %w", err)
	}

	updateState, err := utils.InferTool("update_state", descUpdateState, tc.UpdateState)
	if err != nil {
		return nil, fmt.Errorf("infer update_state: %w", err)
	}

	roll, err := utils.InferTool("roll", descRoll, tc.Roll)
	if err != nil {
		return nil, fmt.Errorf("infer roll: %w", err)
	}

	getEntityState, err := utils.InferTool("get_entity_state", descGetEntityState, tc.GetEntityState)
	if err != nil {
		return nil, fmt.Errorf("infer get_entity_state: %w", err)
	}

	exploreKnowledge, err := utils.InferTool("explore_knowledge", descExploreKnowledge, tc.ExploreKnowledge)
	if err != nil {
		return nil, fmt.Errorf("infer explore_knowledge: %w", err)
	}

	return []tool.InvokableTool{lookupRules, updateState, roll, getEntityState, exploreKnowledge}, nil
}

func (tc *ToolContext) ExploreKnowledge(_ context.Context, params *ExploreKnowledgeParams) (string, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.Disclosure == nil {
		return "fog disabled", nil
	}

	level := fog.Explored
	if params.Level == "known" {
		level = fog.Known
	}

	action := fog.RevealAction{ToLevel: level, Piece: params.Piece}

	entityID := model.EntityID(params.TargetID)
	if _, ok := tc.World.Entities[entityID]; ok {
		action.EntityID = entityID
	} else {
		factID := model.FactID(params.TargetID)
		foundFact := false
		for _, f := range tc.World.Facts {
			if f.ID == factID {
				foundFact = true
				break
			}
		}
		if foundFact {
			action.FactID = factID
		} else {
			return "", fmt.Errorf("target %q not found as entity or fact", params.TargetID)
		}
	}

	fog.Reveal(tc.Disclosure, action)

	if params.Piece != "" {
		return fmt.Sprintf("unlocked piece %q for %s", params.Piece, params.TargetID), nil
	}
	return fmt.Sprintf("revealed %s (level: %s)", params.TargetID, level), nil
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

// NewDisclosedTools returns the subset of tools appropriate for the current
// beat, given the world state and disclosure context. Progressive disclosure:
// the LLM never sees tools that have no useful effect right now.
//
// Always available: get_entity_state, roll.
// When tc.World.Rules is non-empty: + lookup_rules.
// When any entity has mutable state (State map non-empty OR StatsComponent present): + update_state.
// When tc.Disclosure != nil (fog enabled): + explore_knowledge.
//
// Tool descriptions are shared with NewInvokableTools via package-level
// const declarations to prevent drift.
func NewDisclosedTools(tc *ToolContext) ([]tool.InvokableTool, error) {
	out := make([]tool.InvokableTool, 0, 5)

	getState, err := utils.InferTool("get_entity_state", descGetEntityState, tc.GetEntityState)
	if err != nil {
		return nil, fmt.Errorf("infer get_entity_state: %w", err)
	}
	out = append(out, getState)

	rollTool, err := utils.InferTool("roll", descRoll, tc.Roll)
	if err != nil {
		return nil, fmt.Errorf("infer roll: %w", err)
	}
	out = append(out, rollTool)

	if len(tc.World.Rules) > 0 {
		lookupRules, err := utils.InferTool("lookup_rules", descLookupRules, tc.LookupRules)
		if err != nil {
			return nil, fmt.Errorf("infer lookup_rules: %w", err)
		}
		out = append(out, lookupRules)
	}

	if hasMutableEntities(tc.World) {
		updateState, err := utils.InferTool("update_state", descUpdateState, tc.UpdateState)
		if err != nil {
			return nil, fmt.Errorf("infer update_state: %w", err)
		}
		out = append(out, updateState)
	}

	if tc.Disclosure != nil {
		explore, err := utils.InferTool("explore_knowledge", descExploreKnowledge, tc.ExploreKnowledge)
		if err != nil {
			return nil, fmt.Errorf("infer explore_knowledge: %w", err)
		}
		out = append(out, explore)
	}

	return out, nil
}

// hasMutableEntities reports whether any entity carries mutable state
// (ad-hoc State map non-empty OR a Stats component present) that an
// update_state tool could affect.
func hasMutableEntities(w model.World) bool {
	for _, e := range w.Entities {
		if len(e.State) > 0 {
			return true
		}
		if _, ok := e.StatsComponent(); ok {
			return true
		}
	}
	return false
}
