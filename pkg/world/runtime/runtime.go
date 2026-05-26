// Package runtime exposes the world event application loop, rule engine,
// and multi-step execution for downstream repositories.
package runtime

import (
	internal "github.com/sizolity/nobody/internal/world/runtime"
	"github.com/sizolity/nobody/pkg/world/director"
)

type Runtime = internal.Runtime
type StepResult = internal.StepResult
type RunResult = internal.RunResult
type RuntimeOption = internal.RuntimeOption

type Rule = internal.Rule
type RuleContext = internal.RuleContext
type RuleDecision = internal.RuleDecision

type EntityExistsRule = internal.EntityExistsRule
type ActorAliveRule = internal.ActorAliveRule

const (
	RuleDecisionAllow  = internal.RuleDecisionAllow
	RuleDecisionReject = internal.RuleDecisionReject
)

func NewRuntime(options ...RuntimeOption) Runtime {
	return internal.NewRuntime(options...)
}

func DefaultRules() []Rule {
	return internal.DefaultRules()
}

func WithoutRules() RuntimeOption {
	return internal.WithoutRules()
}

func WithRules(rules ...Rule) RuntimeOption {
	return internal.WithRules(rules...)
}

func WithDirectors(directors ...director.Director) RuntimeOption {
	return internal.WithDirectors(directors...)
}

func WithEventQueueLimit(limit int) RuntimeOption {
	return internal.WithEventQueueLimit(limit)
}
