package inference

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/config"
)

// captureModel records the opts received on each Generate / Stream call
// so tests can assert prepend ordering without standing up an HTTP server.
type captureModel struct {
	lastOpts []model.Option
	tools    []*schema.ToolInfo
}

func (c *captureModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	c.lastOpts = opts
	return &schema.Message{Role: schema.Assistant, Content: "ok"}, nil
}

func (c *captureModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	c.lastOpts = opts
	sr, sw := schema.Pipe[*schema.Message](1)
	sw.Close()
	return sr, nil
}

func (c *captureModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	clone := &captureModel{tools: tools}
	return clone, nil
}

func TestDefaultsFromConfig_Auto(t *testing.T) {
	cfg := &config.Config{}
	cfg.Model.ToolChoice = "auto"
	assert.Empty(t, DefaultsFromConfig(cfg))
}

func TestDefaultsFromConfig_Empty(t *testing.T) {
	cfg := &config.Config{}
	assert.Empty(t, DefaultsFromConfig(cfg))
}

func TestDefaultsFromConfig_Forced(t *testing.T) {
	cfg := &config.Config{}
	cfg.Model.ToolChoice = "forced"
	opts := DefaultsFromConfig(cfg)
	require.Len(t, opts, 1)
	o := model.GetCommonOptions(&model.Options{}, opts...)
	require.NotNil(t, o.ToolChoice)
	assert.Equal(t, schema.ToolChoiceForced, *o.ToolChoice)
}

func TestDefaultsFromConfig_Forbidden(t *testing.T) {
	cfg := &config.Config{}
	cfg.Model.ToolChoice = "forbidden"
	opts := DefaultsFromConfig(cfg)
	require.Len(t, opts, 1)
	o := model.GetCommonOptions(&model.Options{}, opts...)
	require.NotNil(t, o.ToolChoice)
	assert.Equal(t, schema.ToolChoiceForbidden, *o.ToolChoice)
}

func TestWithDefaultOptions_NoDefaults_ReturnsInner(t *testing.T) {
	inner := &captureModel{}
	got := WithDefaultOptions(inner)
	assert.Same(t, model.ToolCallingChatModel(inner), got)
}

func TestWithDefaultOptions_PrependsDefaults(t *testing.T) {
	inner := &captureModel{}
	def := model.WithToolChoice(schema.ToolChoiceForced)
	wrapped := WithDefaultOptions(inner, def)

	callerOpt := model.WithToolChoice(schema.ToolChoiceForbidden)
	_, err := wrapped.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}}, callerOpt)
	require.NoError(t, err)

	require.Len(t, inner.lastOpts, 2)
	// caller opt is last → wins
	o := model.GetCommonOptions(&model.Options{}, inner.lastOpts...)
	require.NotNil(t, o.ToolChoice)
	assert.Equal(t, schema.ToolChoiceForbidden, *o.ToolChoice, "caller opt must override default")
}

func TestWithDefaultOptions_WithTools_ReWraps(t *testing.T) {
	inner := &captureModel{}
	def := model.WithToolChoice(schema.ToolChoiceForced)
	wrapped := WithDefaultOptions(inner, def)

	withTools, err := wrapped.WithTools([]*schema.ToolInfo{{Name: "echo"}})
	require.NoError(t, err)
	// Must still be a wrapper so defaults persist on tool-bound model
	_, ok := withTools.(*defaultOptionsModel)
	require.True(t, ok, "WithTools must re-wrap so defaults persist")

	_, err = withTools.Generate(context.Background(), []*schema.Message{{Role: schema.User}})
	require.NoError(t, err)
	// Inner-most captureModel of the re-wrapped instance is a *new* clone (per WithTools impl);
	// the wrapper's defaults are still applied to that clone.
	dom := withTools.(*defaultOptionsModel)
	clone := dom.inner.(*captureModel)
	require.Len(t, clone.lastOpts, 1, "default opt must be passed to re-wrapped inner")
}

func TestWithDefaultOptions_Stream_Prepends(t *testing.T) {
	inner := &captureModel{}
	def := model.WithToolChoice(schema.ToolChoiceForced)
	wrapped := WithDefaultOptions(inner, def)

	sr, err := wrapped.Stream(context.Background(), []*schema.Message{{Role: schema.User}})
	require.NoError(t, err)
	defer sr.Close()
	require.Len(t, inner.lastOpts, 1)
}
