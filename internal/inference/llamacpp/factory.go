package llamacpp

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"github.com/sizolity/nobody/internal/config"
	"github.com/sizolity/nobody/internal/inference"
	"github.com/sizolity/nobody/internal/skills"
)

// ProviderName is the registry key used in cfg.Model.Provider and
// cfg.Model.ProviderOpts. Exported so tests and future callers can
// reference the same literal.
const ProviderName = "llamacpp"

// modeOpenAICompat is the only accepted value of
// provider_opts.llamacpp.mode in v1. Any other value is rejected at
// NewHealthChecker time so YAML typos fail loudly. A future "native"
// mode is planned for direct /completion + GBNF grammar once tool-call
// reliability on small models demands it.
const modeOpenAICompat = "openai_compat"

// factory keeps the chat/embedder/health constructor methods grouped for the
// llama.cpp Runtime.
type factory struct{}

func (factory) CreateChatModel(
	ctx context.Context,
	cfg *config.Config,
	preset config.SamplingPreset,
	think bool,
	timeout time.Duration,
) (model.ToolCallingChatModel, error) {
	if err := validateProviderOpts(cfg); err != nil {
		return nil, err
	}
	chatCfg := buildChatCfg(cfg, preset, think, timeout)
	if rf := buildResponseFormat(cfg); rf != nil {
		// Phase 2a T2: response_format is a first-class openai.ChatModelConfig
		// field, set once at construction time per spec ADR-2 (no per-call
		// RunOption wrapper needed for this knob).
		chatCfg.ResponseFormat = rf
	}
	inner, err := openai.NewChatModel(ctx, chatCfg)
	if err != nil {
		return nil, err
	}
	defaults := inference.DefaultsFromConfig(cfg)
	// Phase 2a T3: append the (0 or 1) grammar option built from
	// provider_opts.llamacpp.grammar so every Generate / Stream call
	// runs through the openai.WithRequestPayloadModifier hook that
	// injects the GBNF grammar into the outbound body. grammar=off
	// (the Phase 1 default) returns nil so the wrapper-vs-bare check
	// in WithDefaultOptions stays symmetric and operators who never
	// set the new yaml key see no behaviour change.
	defaults = append(defaults, buildGrammarOption(cfg)...)
	return inference.WithDefaultOptions(inner, defaults...), nil
}

func (factory) CreateEmbedder(
	ctx context.Context,
	cfg *config.Config,
	timeout time.Duration,
) (skills.Embedder, error) {
	if err := validateProviderOpts(cfg); err != nil {
		return nil, err
	}
	return createEmbedder(ctx, cfg, timeout)
}

// NewHealthChecker constructs the llamacpp health checker with defaults
// pulled from cfg.Model.ProviderOpts["llamacpp"] and performs the
// initial inference_check emission before returning. An invalid mode
// value is reported via a single inference_check/warn event; the returned
// checker then fails EnsureReady so harness.New rejects the run before the
// first call.
func (factory) NewHealthChecker(cfg *config.Config, emit inference.EventEmitter) inference.HealthChecker {
	if err := validateProviderOpts(cfg); err != nil {
		emitCheck(emit, inference.EventInferenceCheck, "error", map[string]any{
			inference.PayloadProviderKey: ProviderName,
			"event":                      "query_failed",
			"model":                      cfg.Model.Name,
			"error":                      err.Error(),
		})
		return newFailingChecker(err)
	}
	po := providerOpts(cfg)
	h := newHealthChecker(healthCheckerConfig{
		BaseURL:       cfg.Model.BaseURL,
		ModelName:     cfg.Model.Name,
		ProbePath:     optString(po, "probe_path", "/health"),
		MaxReconnect:  optInt(po, "reconnect_max", 5),
		ReconnectBase: optDuration(po, "reconnect_base", 1*time.Second),
		Emit:          emit,
	})
	LogCheck(cfg.Model.BaseURL, cfg.Model.Name, emit)
	return h
}

// validateProviderOpts enforces the v1 mode whitelist. Any unexpected
// value is a hard error at startup rather than a silent fallback:
// llama-server's semantics and llama.cpp's native endpoint diverge
// enough that we prefer to refuse to run over running against the
// wrong shape.
func validateProviderOpts(cfg *config.Config) error {
	po := providerOpts(cfg)
	mode := optString(po, "mode", modeOpenAICompat)
	if mode != modeOpenAICompat {
		return fmt.Errorf("provider_opts.llamacpp.mode=%q is not supported (v1 accepts only %q); the \"native\" mode for /completion + GBNF grammar is reserved for a future release", mode, modeOpenAICompat)
	}
	return nil
}

// providerOpts returns the llamacpp sub-map of cfg.Model.ProviderOpts,
// treating a missing cfg or missing block as the empty map so opt*
// helpers below fall back to their hard-coded defaults rather than
// panicking on nil indexing.
func providerOpts(cfg *config.Config) map[string]any {
	if cfg == nil || cfg.Model.ProviderOpts == nil {
		return nil
	}
	return cfg.Model.ProviderOpts[ProviderName]
}

// providerAPIKey reads provider_opts.llamacpp.api_key; most llama-server
// deployments leave auth off so the default is the empty string. Kept
// as a separate helper so the chat and embedder paths share one lookup.
func providerAPIKey(cfg *config.Config) string {
	return optString(providerOpts(cfg), "api_key", "")
}

func optString(m map[string]any, k, def string) string {
	if v, ok := m[k].(string); ok && v != "" {
		return v
	}
	return def
}

func optInt(m map[string]any, k string, def int) int {
	if v, ok := m[k].(int); ok && v > 0 {
		return v
	}
	return def
}

func optDuration(m map[string]any, k string, def time.Duration) time.Duration {
	if v, ok := m[k].(time.Duration); ok && v > 0 {
		return v
	}
	if s, ok := m[k].(string); ok {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return def
}

func embeddingOpts(cfg *config.Config) map[string]any {
	po := providerOpts(cfg)
	if po == nil {
		return nil
	}
	if v, ok := po["embedding"].(map[string]any); ok {
		return v
	}
	return nil
}
