package model

import (
	"strings"
	"testing"
)

func TestConditionValidateRejectsUnsupportedOperator(t *testing.T) {
	t.Parallel()

	c := Condition{Kind: ConditionKindState, Path: "x", Operator: ">"}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() returned nil for unsupported operator")
	}
	if !strings.Contains(err.Error(), `unsupported condition operator ">"`) {
		t.Fatalf("Validate() error = %q, want unsupported-operator", err.Error())
	}
}

func TestConditionValidateAllowsAllSupportedOperators(t *testing.T) {
	t.Parallel()

	operators := []string{
		ConditionOperatorEqual,
		ConditionOperatorNotEqual,
		ConditionOperatorExists,
		ConditionOperatorNotExists,
	}
	for _, op := range operators {
		c := Condition{Kind: ConditionKindState, Path: "x", Operator: op}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate(operator=%q) = %v, want nil", op, err)
		}
	}
}

func TestConditionValidateAllowsEmptyOperator(t *testing.T) {
	t.Parallel()

	c := Condition{Kind: ConditionKindFact, Path: "x"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for empty operator (narrative-doc conditions)", err)
	}
}

func TestIsSupportedConditionOperator(t *testing.T) {
	t.Parallel()

	cases := []struct {
		operator string
		want     bool
	}{
		{ConditionOperatorEqual, true},
		{ConditionOperatorNotEqual, true},
		{ConditionOperatorExists, true},
		{ConditionOperatorNotExists, true},
		{"", false},
		{">", false},
		{"contains", false},
	}
	for _, tc := range cases {
		got := IsSupportedConditionOperator(tc.operator)
		if got != tc.want {
			t.Errorf("IsSupportedConditionOperator(%q) = %v, want %v", tc.operator, got, tc.want)
		}
	}
}
