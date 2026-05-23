package runtime

type RuntimeOption func(*Runtime)

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
