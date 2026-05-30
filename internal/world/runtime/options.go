package runtime

import (
	"time"

	"github.com/sizolity/nobody/internal/world/director"
)

type RuntimeOption func(*Runtime)
type TimeNowFunc func() time.Time

func NewRuntime(options ...RuntimeOption) Runtime {
	rt := Runtime{Rules: DefaultRules()}
	for _, option := range options {
		option(&rt)
	}
	return rt
}

func DefaultRules() []Rule {
	return []Rule{
		EntityExistsRule{},
		ActorAliveRule{},
	}
}

func WithoutRules() RuntimeOption {
	return func(rt *Runtime) {
		rt.Rules = nil
	}
}

func WithRules(rules ...Rule) RuntimeOption {
	return func(rt *Runtime) {
		rt.Rules = append([]Rule(nil), rules...)
	}
}

func WithDirectors(directors ...director.Director) RuntimeOption {
	return func(rt *Runtime) {
		rt.Directors = append([]director.Director(nil), directors...)
	}
}

func WithWorldRules(registry *RuleRegistry) RuntimeOption {
	return func(rt *Runtime) {
		rt.worldRuleRegistry = registry
	}
}

func WithEventQueueLimit(limit int) RuntimeOption {
	return func(rt *Runtime) {
		rt.EventQueueLimit = limit
	}
}

func WithPostApplyHooks(hooks ...PostApplyHook) RuntimeOption {
	return func(rt *Runtime) {
		rt.postApplyHooks = append(rt.postApplyHooks, hooks...)
	}
}

func WithTimeNow(now TimeNowFunc) RuntimeOption {
	return func(rt *Runtime) {
		rt.timeNow = now
	}
}
