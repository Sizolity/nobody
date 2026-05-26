package tools

import (
	"encoding/json"
	"math/rand/v2"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/rpg/rule"
)

func testWorld() model.World {
	combatRule := rule.Rule{
		ID: "rule-combat-01", Category: "combat", Level: 1,
		Content: "Attack rolls use d20 + modifier", Source: rule.SourceSystem,
		Enabled: true, Tags: []string{"melee"},
	}
	socialRule := rule.Rule{
		ID: "rule-social-01", Category: "social", Level: 1,
		Content: "Persuasion checks use d20 + charisma", Source: rule.SourceSystem,
		Enabled: true, Tags: []string{"dialogue"},
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
			rule.ToModelRule(socialRule),
		},
	}
}

func TestLookupRules_FiltersByCategory(t *testing.T) {
	tc := &ToolContext{World: testWorld()}

	result, err := tc.LookupRules(LookupRulesParams{Category: "combat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result for combat rules")
	}
	if !contains(result, "Attack rolls") {
		t.Errorf("expected combat rule content, got: %s", result)
	}
	if contains(result, "Persuasion") {
		t.Errorf("should not contain social rule, got: %s", result)
	}
}

func TestUpdateState_ValidEntity(t *testing.T) {
	tc := &ToolContext{World: testWorld()}

	result, err := tc.UpdateState(UpdateStateParams{
		EntityID: "hero-01", Key: "hp", Value: float64(15),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if len(tc.PendingEffects) != 1 {
		t.Fatalf("expected 1 pending effect, got %d", len(tc.PendingEffects))
	}
	effect := tc.PendingEffects[0]
	if effect.Kind != model.EffectUpdateEntityState {
		t.Errorf("expected kind %q, got %q", model.EffectUpdateEntityState, effect.Kind)
	}
	if effect.TargetID != "hero-01" {
		t.Errorf("expected target_id %q, got %q", "hero-01", effect.TargetID)
	}
	val, ok := effect.Payload["hp"]
	if !ok {
		t.Fatal("expected payload to contain 'hp' key")
	}
	if val.Kind != model.ValueKindNumber {
		t.Errorf("expected value kind %q, got %q", model.ValueKindNumber, val.Kind)
	}
}

func TestUpdateState_UnknownEntity(t *testing.T) {
	tc := &ToolContext{World: testWorld()}

	_, err := tc.UpdateState(UpdateStateParams{
		EntityID: "nonexistent", Key: "hp", Value: float64(10),
	})
	if err == nil {
		t.Fatal("expected error for unknown entity")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestRoll_ValidRoll(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 99))
	tc := &ToolContext{World: testWorld(), Rng: rng}

	result, err := tc.Roll(RollParams{Sides: 20, Count: 1, Modifier: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := parsed["total"]; !ok {
		t.Error("expected 'total' in result")
	}
	if _, ok := parsed["rolls"]; !ok {
		t.Error("expected 'rolls' in result")
	}
	if _, ok := parsed["modifier"]; !ok {
		t.Error("expected 'modifier' in result")
	}
	total := parsed["total"].(float64)
	if total < 4 || total > 23 {
		t.Errorf("total %v out of range [4,23] for 1d20+3", total)
	}
}

func TestRoll_InvalidSides(t *testing.T) {
	tc := &ToolContext{World: testWorld()}

	_, err := tc.Roll(RollParams{Sides: 0, Count: 1})
	if err == nil {
		t.Fatal("expected error for sides < 1")
	}
	if !contains(err.Error(), "sides must be >= 1") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetEntityState_ValidEntity(t *testing.T) {
	tc := &ToolContext{World: testWorld()}

	result, err := tc.GetEntityState(GetEntityStateParams{EntityID: "hero-01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if parsed["id"] != "hero-01" {
		t.Errorf("expected id 'hero-01', got %v", parsed["id"])
	}
	if parsed["name"] != "Arin" {
		t.Errorf("expected name 'Arin', got %v", parsed["name"])
	}
	if parsed["type"] != "character" {
		t.Errorf("expected type 'character', got %v", parsed["type"])
	}
	if _, ok := parsed["state"]; !ok {
		t.Error("expected 'state' in result")
	}
}

func TestGetEntityState_UnknownEntity(t *testing.T) {
	tc := &ToolContext{World: testWorld()}

	_, err := tc.GetEntityState(GetEntityStateParams{EntityID: "ghost-99"})
	if err == nil {
		t.Fatal("expected error for unknown entity")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
