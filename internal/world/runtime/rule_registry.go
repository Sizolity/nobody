package runtime

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

// RuleFactory creates a runtime Rule from a model Rule's data.
type RuleFactory func(id model.RuleID, data any) (Rule, error)

// RuleRegistry maps model.Rule.Kind strings to factories.
type RuleRegistry struct {
	factories map[string]RuleFactory
}

func NewRuleRegistry() *RuleRegistry {
	return &RuleRegistry{factories: make(map[string]RuleFactory)}
}

// Register adds a factory for a rule kind. Overwrites if kind already registered.
func (r *RuleRegistry) Register(kind string, factory RuleFactory) {
	r.factories[kind] = factory
}

// Build creates a runtime Rule from a model Rule.
// Returns error if kind is unknown or factory fails.
func (r *RuleRegistry) Build(rule model.Rule) (Rule, error) {
	factory, ok := r.factories[rule.Kind]
	if !ok {
		return nil, fmt.Errorf("unknown rule kind %q", rule.Kind)
	}
	return factory(rule.ID, rule.Data)
}

// BuildAll creates runtime Rules from a slice of model Rules.
// Skips disabled rules (Enabled == false). Returns error on first failure.
func (r *RuleRegistry) BuildAll(rules []model.Rule) ([]Rule, error) {
	var out []Rule
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		built, err := r.Build(rule)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", rule.ID, err)
		}
		out = append(out, built)
	}
	return out, nil
}

// DefaultRegistry returns a RuleRegistry with the builtin rule kinds pre-registered.
func DefaultRegistry() *RuleRegistry {
	reg := NewRuleRegistry()
	reg.Register("entity_exists", func(id model.RuleID, _ any) (Rule, error) {
		return EntityExistsRule{}, nil
	})
	reg.Register("actor_alive", func(id model.RuleID, _ any) (Rule, error) {
		return ActorAliveRule{}, nil
	})
	return reg
}
