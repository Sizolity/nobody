package inference

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/nobody/internal/config"
)

// DefaultsFromConfig builds the provider-neutral default RunOptions
// implied by cfg.Model. Currently emits at most one option:
//
//   - model.WithToolChoice(...) when cfg.Model.ToolChoice is "forced"
//     or "forbidden". The "auto" case (and unset) emit nothing because
//     "no option" is semantically equivalent to schema.ToolChoiceAllowed
//     in Eino's openai client (which defaults to OpenAI tool_choice=auto).
//
// Returns an empty slice when no defaults apply so callers can splat
// without nil-checks.
//
// response_format does NOT appear here — it is set on
// openai.ChatModelConfig.ResponseFormat at construction time, not as a
// per-call RunOption.
func DefaultsFromConfig(cfg *config.Config) []model.Option {
	if cfg == nil {
		return nil
	}
	var opts []model.Option
	switch cfg.Model.ToolChoice {
	case "forced":
		opts = append(opts, model.WithToolChoice(schema.ToolChoiceForced))
	case "forbidden":
		opts = append(opts, model.WithToolChoice(schema.ToolChoiceForbidden))
	case "auto", "":
		// no-op: equivalent to ToolChoiceAllowed which is Eino's default
	}
	return opts
}

// WithDefaultOptions wraps a ToolCallingChatModel so every Generate /
// Stream call gets `defaults` prepended to the caller-supplied opts.
//
// Eino RunOption semantics: later options override earlier ones, so
// caller-supplied opts retain priority and can override the defaults
// per call (e.g. agent code can temporarily flip tool_choice).
//
// WithTools is forwarded to the inner model and the result is re-wrapped
// with the same defaults so derived tool-bound models retain the
// default behavior.
//
// When defaults is empty the inner model is returned unchanged to
// avoid a wrapper indirection that would only forward calls.
func WithDefaultOptions(inner model.ToolCallingChatModel, defaults ...model.Option) model.ToolCallingChatModel {
	if len(defaults) == 0 {
		return inner
	}
	return &defaultOptionsModel{inner: inner, defaults: defaults}
}

// defaultOptionsModel is the unexported ToolCallingChatModel returned
// by WithDefaultOptions. It only forwards Generate / Stream / WithTools
// to inner and prepends the captured defaults via merge(). It owns no
// mutable state of its own (defaults is read-only after construction)
// and is safe for concurrent use as long as inner is.
type defaultOptionsModel struct {
	inner    model.ToolCallingChatModel
	defaults []model.Option
}

func (m *defaultOptionsModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	merged := m.merge(opts)
	return m.inner.Generate(ctx, in, merged...)
}

func (m *defaultOptionsModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	merged := m.merge(opts)
	return m.inner.Stream(ctx, in, merged...)
}

func (m *defaultOptionsModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	withTools, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &defaultOptionsModel{inner: withTools, defaults: m.defaults}, nil
}

// merge prepends defaults to caller opts. We allocate a fresh slice
// so concurrent callers don't share the merged slice (and the
// defaults slice itself is never mutated).
func (m *defaultOptionsModel) merge(callerOpts []model.Option) []model.Option {
	merged := make([]model.Option, 0, len(m.defaults)+len(callerOpts))
	merged = append(merged, m.defaults...)
	merged = append(merged, callerOpts...)
	return merged
}
