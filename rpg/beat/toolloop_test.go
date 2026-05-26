package beat

import (
	"context"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/rule"
	"github.com/sizolity/nobody/rpg/tools"
)

func toolTestWorld() model.World {
	combatRule := rule.Rule{
		ID: "rule-combat-01", Category: "combat", Level: 1,
		Content: "Attack rolls use d20 + modifier", Source: rule.SourceSystem,
		Enabled: true, Tags: []string{"melee"},
	}
	return model.World{
		ID:   "world-test",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"hero-01": {
				ID: "hero-01", Type: "character", Name: "Arin",
				Tags: []string{"player"},
				State: map[string]model.Value{
					"hp": {Kind: model.ValueKindNumber, Raw: float64(20)},
				},
			},
		},
		Rules: []model.Rule{
			rule.ToModelRule(combatRule),
		},
	}
}

func TestExecuteToolCallsLookupRules(t *testing.T) {
	tc := &tools.ToolContext{World: toolTestWorld()}
	calls := []ToolCall{{
		ID:        "call-1",
		Name:      "lookup_rules",
		Arguments: `{"category":"combat"}`,
	}}

	results := ExecuteToolCalls(context.Background(), tc, calls)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.IsError {
		t.Fatalf("unexpected error: %s", r.Content)
	}
	if r.ToolCallID != "call-1" {
		t.Errorf("expected tool_call_id %q, got %q", "call-1", r.ToolCallID)
	}
	if !strings.Contains(r.Content, "Attack rolls") {
		t.Errorf("expected content to contain combat rule, got: %s", r.Content)
	}
}

func TestExecuteToolCallsUpdateState(t *testing.T) {
	tc := &tools.ToolContext{World: toolTestWorld()}
	calls := []ToolCall{{
		ID:        "call-2",
		Name:      "update_state",
		Arguments: `{"entity_id":"hero-01","key":"hp","value":15}`,
	}}

	results := ExecuteToolCalls(context.Background(), tc, calls)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.IsError {
		t.Fatalf("unexpected error: %s", r.Content)
	}

	effects := PendingEffects(tc)
	if len(effects) != 1 {
		t.Fatalf("expected 1 pending effect, got %d", len(effects))
	}
	if effects[0].Kind != model.EffectUpdateEntityState {
		t.Errorf("expected effect kind %q, got %q", model.EffectUpdateEntityState, effects[0].Kind)
	}
	if effects[0].TargetID != "hero-01" {
		t.Errorf("expected target_id %q, got %q", "hero-01", effects[0].TargetID)
	}
}

func TestExecuteToolCallsRoll(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 99))
	tc := &tools.ToolContext{World: toolTestWorld(), Rng: rng}
	calls := []ToolCall{{
		ID:        "call-3",
		Name:      "roll",
		Arguments: `{"sides":20,"count":2,"modifier":3}`,
	}}

	results := ExecuteToolCalls(context.Background(), tc, calls)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.IsError {
		t.Fatalf("unexpected error: %s", r.Content)
	}
	if !strings.Contains(r.Content, "total") {
		t.Errorf("expected result to contain 'total', got: %s", r.Content)
	}
}

func TestExecuteToolCallsUnknownTool(t *testing.T) {
	tc := &tools.ToolContext{World: toolTestWorld()}
	calls := []ToolCall{{
		ID:        "call-4",
		Name:      "nonexistent_tool",
		Arguments: `{}`,
	}}

	results := ExecuteToolCalls(context.Background(), tc, calls)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !r.IsError {
		t.Fatal("expected IsError=true for unknown tool")
	}
	if !strings.Contains(r.Content, "unknown tool") {
		t.Errorf("expected 'unknown tool' in error, got: %s", r.Content)
	}
}

func TestExecuteToolCallsBadJSON(t *testing.T) {
	tc := &tools.ToolContext{World: toolTestWorld()}
	calls := []ToolCall{{
		ID:        "call-5",
		Name:      "roll",
		Arguments: `{not valid json`,
	}}

	results := ExecuteToolCalls(context.Background(), tc, calls)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if !r.IsError {
		t.Fatal("expected IsError=true for bad JSON")
	}
	if !strings.Contains(r.Content, "parse roll args") {
		t.Errorf("expected parse error in content, got: %s", r.Content)
	}
}

func TestExecuteToolCallsMultiple(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	tc := &tools.ToolContext{World: toolTestWorld(), Rng: rng}
	calls := []ToolCall{
		{ID: "a", Name: "lookup_rules", Arguments: `{"category":"combat"}`},
		{ID: "b", Name: "roll", Arguments: `{"sides":6}`},
		{ID: "c", Name: "get_entity_state", Arguments: `{"entity_id":"hero-01"}`},
	}

	results := ExecuteToolCalls(context.Background(), tc, calls)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.IsError {
			t.Errorf("result[%d] unexpected error: %s", i, r.Content)
		}
		if r.ToolCallID != calls[i].ID {
			t.Errorf("result[%d] expected tool_call_id %q, got %q", i, calls[i].ID, r.ToolCallID)
		}
	}
}
