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

// Exported operator constants. Callers building Condition values should
// reference these instead of hard-coding "==" / "!=" / "exists" /
// "not_exists" strings. The runtime evaluators in
// internal/world/runtime/preconditions.go switch on these constants.
//
// Operator support differs per kind:
//   - state, stat:               ==, !=, exists, not_exists
//   - fact, relation, memory:    exists, not_exists  (equality is out of
//                                scope — use ConditionKindFact for
//                                structured equality assertions instead)
const (
	ConditionOperatorEqual     = "=="
	ConditionOperatorNotEqual  = "!="
	ConditionOperatorExists    = "exists"
	ConditionOperatorNotExists = "not_exists"
)

// IsSupportedConditionOperator reports whether op is one of the standard
// operators the runtime can evaluate. Note: this does not check whether
// op is meaningful for a particular ConditionKind — narrower per-kind
// validation happens inside the runtime evaluator (e.g. memory rejects
// equality).
func IsSupportedConditionOperator(op string) bool {
	switch op {
	case ConditionOperatorEqual, ConditionOperatorNotEqual,
		ConditionOperatorExists, ConditionOperatorNotExists:
		return true
	default:
		return false
	}
}

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
	// Operator is optional (some narrative-doc conditions may be
	// aspirational rather than evaluable). When non-empty, it must be
	// one of the runtime-supported operators.
	if c.Operator != "" && !IsSupportedConditionOperator(c.Operator) {
		return fmt.Errorf("unsupported condition operator %q", c.Operator)
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
