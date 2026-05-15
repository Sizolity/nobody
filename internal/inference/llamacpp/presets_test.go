package llamacpp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/config"
)

func baseCfg() *config.Config {
	return &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			Name:     "qwen3.5:latest",
			BaseURL:  "http://localhost:8080/v1",
		},
	}
}

func TestBuildChatCfg_MapsStandardOpenAIFields(t *testing.T) {
	cfg := baseCfg()
	preset := config.SamplingPreset{
		Temperature:     0.6,
		TopP:            0.9,
		PresencePenalty: 1.5,
		NumPredict:      2048,
	}
	got := buildChatCfg(cfg, preset, false, 90*time.Second)

	assert.Equal(t, "http://localhost:8080/v1", got.BaseURL)
	assert.Equal(t, "qwen3.5:latest", got.Model)
	assert.Equal(t, 90*time.Second, got.Timeout)

	require.NotNil(t, got.Temperature)
	assert.InDelta(t, 0.6, float64(*got.Temperature), 1e-6)
	require.NotNil(t, got.TopP)
	assert.InDelta(t, 0.9, float64(*got.TopP), 1e-6)
	require.NotNil(t, got.PresencePenalty)
	assert.InDelta(t, 1.5, float64(*got.PresencePenalty), 1e-6)
	require.NotNil(t, got.MaxTokens)
	assert.Equal(t, 2048, *got.MaxTokens)
}

func TestBuildChatCfg_NativeFieldsGoThroughExtraFields(t *testing.T) {
	cfg := baseCfg()
	preset := config.SamplingPreset{
		TopK:              20,
		MinP:              0.05,
		RepetitionPenalty: 1.05,
	}
	got := buildChatCfg(cfg, preset, true, time.Second)

	require.NotNil(t, got.ExtraFields)
	assert.Equal(t, 20, got.ExtraFields["top_k"])
	assert.InDelta(t, 0.05, float64(got.ExtraFields["min_p"].(float32)), 1e-6)
	assert.InDelta(t, 1.05, float64(got.ExtraFields["repetition_penalty"].(float32)), 1e-6)

	ctk, ok := got.ExtraFields["chat_template_kwargs"].(map[string]any)
	require.True(t, ok, "chat_template_kwargs must be a map")
	assert.Equal(t, true, ctk["enable_thinking"])
}

func TestBuildChatCfg_ThinkFalsePropagates(t *testing.T) {
	got := buildChatCfg(baseCfg(), config.SamplingPreset{Temperature: 0.7}, false, time.Second)
	ctk := got.ExtraFields["chat_template_kwargs"].(map[string]any)
	assert.Equal(t, false, ctk["enable_thinking"])
}

func TestBuildChatCfg_OmitsUnsetNativeFields(t *testing.T) {
	// A zero preset should leave top_k / min_p / repetition_penalty unset
	// so llama-server uses its own defaults rather than receiving 0 —
	// repetition_penalty=0 in particular would suppress all repetition
	// logic in a confusing way.
	got := buildChatCfg(baseCfg(), config.SamplingPreset{}, true, time.Second)
	_, hasTopK := got.ExtraFields["top_k"]
	_, hasMinP := got.ExtraFields["min_p"]
	_, hasRepPen := got.ExtraFields["repetition_penalty"]
	assert.False(t, hasTopK)
	assert.False(t, hasMinP)
	assert.False(t, hasRepPen)
	// chat_template_kwargs is always set because Qwen3 templates always
	// want the flag (true is the model default).
	assert.Contains(t, got.ExtraFields, "chat_template_kwargs")
}

func TestBuildChatCfg_APIKeyFromProviderOpts(t *testing.T) {
	cfg := baseCfg()
	cfg.Model.ProviderOpts = map[string]map[string]any{
		ProviderName: {"api_key": "sk-test"},
	}
	got := buildChatCfg(cfg, config.SamplingPreset{}, false, time.Second)
	assert.Equal(t, "sk-test", got.APIKey)
}
