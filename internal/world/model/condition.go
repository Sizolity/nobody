package model

import "fmt"

type Condition struct {
	Kind     string       `json:"kind"`
	Path     string       `json:"path"`
	Operator string       `json:"operator,omitempty"`
	Value    Value        `json:"value,omitempty"`
	Owner    *MemoryOwner `json:"owner,omitempty"`
}

const (
	ConditionKindState    = "state"
	ConditionKindFact     = "fact"
	ConditionKindRelation = "relation"
	ConditionKindMemory   = "memory"
	ConditionKindStat     = "stat"
)

func (c Condition) Validate() error {
	if c.Kind == "" {
		return fmt.Errorf("condition.kind is required")
	}
	if !isSupportedConditionKind(c.Kind) {
		return fmt.Errorf("unsupported condition kind %q", c.Kind)
	}
	if c.Path == "" {
		return fmt.Errorf("condition.path is required")
	}
	return nil
}

func isSupportedConditionKind(kind string) bool {
	switch kind {
	case ConditionKindState, ConditionKindFact, ConditionKindRelation, ConditionKindMemory, ConditionKindStat:
		return true
	default:
		return false
	}
}
