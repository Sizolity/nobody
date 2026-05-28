package runtime

import (
	"fmt"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestRegistryBuildCustomKind(t *testing.T) {
	t.Parallel()

	reg := NewRuleRegistry()
	reg.Register("custom", func(id model.RuleID, data any) (Rule, error) {
		return testRule{id: id, decision: RuleDecision{Status: RuleDecisionAllow}}, nil
	})

	rule, err := reg.Build(model.Rule{ID: "r1", Kind: "custom", Enabled: true})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if rule.ID() != "r1" {
		t.Fatalf("rule ID = %q, want r1", rule.ID())
	}
	decision := rule.Evaluate(RuleContext{}, model.WorldEvent{})
	if decision.Status != RuleDecisionAllow {
		t.Fatalf("decision = %q, want allow", decision.Status)
	}
}

func TestRegistryBuildUnknownKind(t *testing.T) {
	t.Parallel()

	reg := NewRuleRegistry()
	_, err := reg.Build(model.Rule{ID: "r1", Kind: "nonexistent", Enabled: true})
	if err == nil {
		t.Fatal("Build returned nil for unknown kind")
	}
}

func TestRegistryBuildAllSkipsDisabled(t *testing.T) {
	t.Parallel()

	reg := NewRuleRegistry()
	reg.Register("custom", func(id model.RuleID, _ any) (Rule, error) {
		return testRule{id: id, decision: RuleDecision{Status: RuleDecisionAllow}}, nil
	})

	rules, err := reg.BuildAll([]model.Rule{
		{ID: "r1", Kind: "custom", Enabled: true},
		{ID: "r2", Kind: "custom", Enabled: false},
		{ID: "r3", Kind: "custom", Enabled: true},
	})
	if err != nil {
		t.Fatalf("BuildAll returned error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("BuildAll returned %d rules, want 2", len(rules))
	}
	if rules[0].ID() != "r1" || rules[1].ID() != "r3" {
		t.Fatalf("unexpected rule IDs: %q, %q", rules[0].ID(), rules[1].ID())
	}
}

func TestRegistryBuildAllErrorsOnUnknownEnabled(t *testing.T) {
	t.Parallel()

	reg := NewRuleRegistry()
	_, err := reg.BuildAll([]model.Rule{
		{ID: "r1", Kind: "nonexistent", Enabled: true},
	})
	if err == nil {
		t.Fatal("BuildAll returned nil for unknown enabled kind")
	}
}

func TestRegistryBuildAllSkipsDisabledUnknownKind(t *testing.T) {
	t.Parallel()

	reg := NewRuleRegistry()
	rules, err := reg.BuildAll([]model.Rule{
		{ID: "r1", Kind: "nonexistent", Enabled: false},
	})
	if err != nil {
		t.Fatalf("BuildAll returned error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("BuildAll returned %d rules, want 0", len(rules))
	}
}

func TestRegistryFactoryErrorPropagates(t *testing.T) {
	t.Parallel()

	reg := NewRuleRegistry()
	reg.Register("broken", func(_ model.RuleID, _ any) (Rule, error) {
		return nil, fmt.Errorf("factory failed")
	})

	_, err := reg.Build(model.Rule{ID: "r1", Kind: "broken", Enabled: true})
	if err == nil {
		t.Fatal("Build returned nil for broken factory")
	}
}

func TestDefaultRegistryBuiltinEntityExists(t *testing.T) {
	t.Parallel()

	reg := DefaultRegistry()
	rule, err := reg.Build(model.Rule{ID: "r1", Kind: "entity_exists", Enabled: true})
	if err != nil {
		t.Fatalf("Build entity_exists returned error: %v", err)
	}
	if _, ok := rule.(EntityExistsRule); !ok {
		t.Fatalf("expected EntityExistsRule, got %T", rule)
	}
}

func TestDefaultRegistryBuiltinActorAlive(t *testing.T) {
	t.Parallel()

	reg := DefaultRegistry()
	rule, err := reg.Build(model.Rule{ID: "r1", Kind: "actor_alive", Enabled: true})
	if err != nil {
		t.Fatalf("Build actor_alive returned error: %v", err)
	}
	if _, ok := rule.(ActorAliveRule); !ok {
		t.Fatalf("expected ActorAliveRule, got %T", rule)
	}
}

func TestWithWorldRulesRejectsViaModelRule(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Rules: []model.Rule{
			{ID: "r1", Kind: "entity_exists", Enabled: true},
		},
	}
	rt := NewRuntime(WithoutRules(), WithWorldRules(DefaultRegistry()))
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"missing_actor"},
	}

	if _, err := rt.ApplyEvent(world, event); err == nil {
		t.Fatal("ApplyEvent returned nil for missing actor with world rule entity_exists")
	}
}

func TestWithWorldRulesAllowsWhenRuleDisabled(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Rules: []model.Rule{
			{ID: "r1", Kind: "entity_exists", Enabled: false},
		},
	}
	rt := NewRuntime(WithoutRules(), WithWorldRules(DefaultRegistry()))
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"missing_actor"},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("event was not logged: %#v", got.EventLog)
	}
}

func TestWithWorldRulesAdditiveWithExistingRules(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"actor_1": {
				ID:   "actor_1",
				Type: "character",
				Name: "Actor",
				State: map[string]model.Value{
					"alive": {Kind: model.ValueKindBoolean, Raw: false},
				},
			},
		},
		Rules: []model.Rule{
			{ID: "r1", Kind: "entity_exists", Enabled: true},
		},
	}
	rt := NewRuntime(WithRules(ActorAliveRule{}), WithWorldRules(DefaultRegistry()))
	event := model.WorldEvent{
		ID:       "event_1",
		Type:     model.EventTypeNote,
		Source:   model.EventSourceTest,
		ActorIDs: []model.EntityID{"actor_1"},
	}

	if _, err := rt.ApplyEvent(world, event); err == nil {
		t.Fatal("ApplyEvent returned nil — ActorAliveRule from WithRules should have rejected dead actor")
	}
}
