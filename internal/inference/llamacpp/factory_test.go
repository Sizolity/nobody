package llamacpp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/config"
	"github.com/sizolity/nobody/internal/inference"
)

func TestValidateProviderOpts_DefaultMode(t *testing.T) {
	cfg := &config.Config{Model: config.ModelConfig{Provider: ProviderName}}
	assert.NoError(t, validateProviderOpts(cfg))
}

func TestValidateProviderOpts_ExplicitOpenAICompat(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			ProviderOpts: map[string]map[string]any{
				ProviderName: {"mode": "openai_compat"},
			},
		},
	}
	assert.NoError(t, validateProviderOpts(cfg))
}

func TestValidateProviderOpts_RejectsUnknownMode(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			ProviderOpts: map[string]map[string]any{
				ProviderName: {"mode": "native"},
			},
		},
	}
	err := validateProviderOpts(cfg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), `provider_opts.llamacpp.mode="native"`)
	}
}

func TestNewHealthChecker_InvalidModeReturnsFailingChecker(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			Name:     "qwen3.5:latest",
			BaseURL:  "http://localhost:8080/v1",
			ProviderOpts: map[string]map[string]any{
				ProviderName: {"mode": "grammar_native"},
			},
		},
	}
	var events []map[string]any
	emit := func(eventName, severity string, payload map[string]any) {
		events = append(events, map[string]any{
			"eventName": eventName,
			"severity":  severity,
			"payload":   payload,
		})
	}
	checker := factory{}.NewHealthChecker(cfg, emit)
	assert.NotNil(t, checker)
	err := checker.EnsureReady(context.Background())
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "grammar_native")
	}
	assert.Equal(t, inference.StateDisconnected, checker.State())

	// One warn-or-error inference_check event is expected so the runtime
	// log records exactly what the operator misconfigured.
	assert.NotEmpty(t, events)
	assert.Equal(t, inference.EventInferenceCheck, events[0]["eventName"])
}

func TestCreateChatModel_InvalidModePropagates(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			Name:     "qwen3.5:latest",
			BaseURL:  "http://localhost:8080/v1",
			ProviderOpts: map[string]map[string]any{
				ProviderName: {"mode": "native"},
			},
		},
	}
	_, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, time.Second)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "mode")
	}
}

func TestResolveEmbeddingTarget_FallsBackToChatTarget(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			Name:     "qwen3.5:latest",
			BaseURL:  "http://localhost:8080/v1",
		},
	}
	url, name := resolveEmbeddingTarget(cfg)
	assert.Equal(t, "http://localhost:8080/v1", url)
	assert.Equal(t, "qwen3.5:latest", name)
}

func TestResolveEmbeddingTarget_OverrideWins(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			Name:     "qwen3.5:latest",
			BaseURL:  "http://localhost:8080/v1",
			ProviderOpts: map[string]map[string]any{
				ProviderName: {
					"embedding": map[string]any{
						"base_url": "http://localhost:8081/v1",
						"name":     "nomic-embed-text",
					},
				},
			},
		},
	}
	url, name := resolveEmbeddingTarget(cfg)
	assert.Equal(t, "http://localhost:8081/v1", url)
	assert.Equal(t, "nomic-embed-text", name)
}

func TestResolveEmbeddingTarget_PartialOverride(t *testing.T) {
	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			Name:     "qwen3.5:latest",
			BaseURL:  "http://localhost:8080/v1",
			ProviderOpts: map[string]map[string]any{
				ProviderName: {
					"embedding": map[string]any{
						"name": "bge-m3",
					},
				},
			},
		},
	}
	url, name := resolveEmbeddingTarget(cfg)
	assert.Equal(t, "http://localhost:8080/v1", url, "base_url should fall back when only name is overridden")
	assert.Equal(t, "bge-m3", name)
}

func TestOptHelpers(t *testing.T) {
	m := map[string]any{
		"str":     "hello",
		"empty":   "",
		"int":     7,
		"zero":    0,
		"dur_s":   "250ms",
		"dur_raw": 3 * time.Second,
		"bad_dur": "not-a-duration",
	}
	assert.Equal(t, "hello", optString(m, "str", "def"))
	assert.Equal(t, "def", optString(m, "empty", "def"))
	assert.Equal(t, "def", optString(m, "missing", "def"))
	assert.Equal(t, 7, optInt(m, "int", 1))
	assert.Equal(t, 1, optInt(m, "zero", 1))
	assert.Equal(t, 1, optInt(m, "missing", 1))
	assert.Equal(t, 250*time.Millisecond, optDuration(m, "dur_s", time.Minute))
	assert.Equal(t, 3*time.Second, optDuration(m, "dur_raw", time.Minute))
	assert.Equal(t, time.Minute, optDuration(m, "bad_dur", time.Minute))
	assert.Equal(t, time.Minute, optDuration(m, "missing", time.Minute))
}

func TestValidateProviderOpts_ErrorMentionsSupportedValue(t *testing.T) {
	// Sanity-check that the error text is grep-friendly for docs / support.
	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			ProviderOpts: map[string]map[string]any{
				ProviderName: {"mode": "custom"},
			},
		},
	}
	err := validateProviderOpts(cfg)
	if assert.Error(t, err) {
		assert.True(t, strings.Contains(err.Error(), modeOpenAICompat),
			"error must mention the only supported mode so operators can self-correct: %v", err)
	}
}

// TestFactory_CreateChatModel_WrapsWithDefaultOptions_WhenForced locks
// the Phase 2a T1 contract: when cfg.Model.ToolChoice is non-default
// ("forced"), CreateChatModel must wrap the bare openai.ChatModel with
// inference.WithDefaultOptions so every Generate / Stream call gets
// model.WithToolChoice(schema.ToolChoiceForced) prepended.
func TestFactory_CreateChatModel_WrapsWithDefaultOptions_WhenForced(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = "http://localhost:8080/v1"
	cfg.Model.Name = "test-model"
	cfg.Model.ToolChoice = "forced"

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, time.Second)
	require.NoError(t, err)
	require.NotNil(t, cm)
	// Wrapper must NOT be the bare *openai.ChatModel — that would mean
	// the defaults slice was empty and WithDefaultOptions returned the
	// inner model unchanged, which would silently disable tool_choice.
	_, isBareOpenAI := cm.(*openai.ChatModel)
	assert.False(t, isBareOpenAI, "expected WithDefaultOptions wrapper, got bare *openai.ChatModel")
}

// TestFactory_CreateChatModel_NoWrap_WhenAuto locks the inverse: when
// no Phase 2a defaults apply (tool_choice=auto, the DefaultConfig
// value), WithDefaultOptions must return the inner model unchanged so
// Phase 1 callers see no behaviour change.
func TestFactory_CreateChatModel_NoWrap_WhenAuto(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = "http://localhost:8080/v1"
	cfg.Model.Name = "test-model"
	// ToolChoice already "auto" via DefaultConfig — no other defaults to
	// emit until T3 grammar lands.

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, time.Second)
	require.NoError(t, err)
	require.NotNil(t, cm)
	_, isBareOpenAI := cm.(*openai.ChatModel)
	assert.True(t, isBareOpenAI, "expected bare *openai.ChatModel when no defaults apply, got wrapper")
}
