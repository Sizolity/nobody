// Package llamacpp talks to a llama.cpp llama-server instance exposing the
// OpenAI-compatible HTTP API (/v1/chat/completions, /v1/embeddings).
//
// The v1 of this package deliberately stays on the OpenAI-compatible
// path so it can reuse eino-ext/components/model/openai without any
// transport rework. provider_opts.llamacpp.mode is reserved as the
// upgrade point to a future native /completion + GBNF grammar path
// when that becomes necessary for tool-call reliability on small
// models. Any other mode value is rejected at factory construction
// time so YAML typos fail loudly rather than silently running the
// wrong backend.
package llamacpp

import (
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/sizolity/nobody/internal/config"
)

// buildChatCfg translates a nobody SamplingPreset (+ global timeout /
// base URL / model name) into an eino openai.ChatModelConfig.
//
// llama-server accepts the standard OpenAI sampling knobs plus a
// handful of llama.cpp-native ones (top_k / min_p / repetition_penalty)
// and Qwen's chat_template_kwargs.enable_thinking. The native ones have
// no first-class field on ChatModelConfig, so we pack them into
// ExtraFields which eino splices into every outbound request body.
//
// think corresponds to the Qwen3 chat_template toggle: when false the
// template skips the <think>...</think> block entirely. At think=auto
// the caller (agent.ThinkAwareChatModel) builds two chat models and
// switches between them per request, so the flag is fixed at build
// time here, not at send time.
func buildChatCfg(cfg *config.Config, preset config.SamplingPreset, think bool, timeout time.Duration) *openai.ChatModelConfig {
	chat := &openai.ChatModelConfig{
		BaseURL: cfg.Model.BaseURL,
		Model:   cfg.Model.Name,
		Timeout: timeout,
		APIKey:  providerAPIKey(cfg),
	}

	if preset.Temperature != 0 {
		t := preset.Temperature
		chat.Temperature = &t
	}
	if preset.TopP > 0 {
		p := preset.TopP
		chat.TopP = &p
	}
	if preset.PresencePenalty != 0 {
		pp := preset.PresencePenalty
		chat.PresencePenalty = &pp
	}
	if preset.NumPredict > 0 {
		n := preset.NumPredict
		chat.MaxTokens = &n
	}

	extra := map[string]any{}
	if preset.TopK > 0 {
		extra["top_k"] = preset.TopK
	}
	if preset.MinP > 0 {
		extra["min_p"] = preset.MinP
	}
	if preset.RepetitionPenalty > 0 {
		// llama-server accepts repetition_penalty natively; we pass it
		// through rather than re-mapping onto OpenAI's frequency_penalty
		// because the two have different value ranges and semantics
		// (repetition_penalty is multiplicative; frequency_penalty is
		// additive and bounded to [-2, 2]).
		extra["repetition_penalty"] = preset.RepetitionPenalty
	}
	extra["chat_template_kwargs"] = map[string]any{
		"enable_thinking": think,
	}
	chat.ExtraFields = extra

	return chat
}

// buildResponseFormat translates cfg.Model.ResponseFormat (validated by
// loader at LoadConfig time, defaulted to "text" by patchModelDefaults)
// into an openai.ChatCompletionResponseFormat pointer suitable for
// assignment to ChatModelConfig.ResponseFormat. Returns nil for "text"
// (and the empty string, defensive) so the eino openai client omits
// the response_format field from outbound /v1/chat/completions bodies
// — that preserves Phase 1 behaviour bit-for-bit when operators leave
// the new yaml key at its default.
//
// Per ADR-2 in the Phase 2a spec we set this on ChatModelConfig at
// construction time rather than via a per-call RunOption: Eino exposes
// ResponseFormat as a first-class config field so we use it natively
// rather than replaying the same value through WithRequestPayloadModifier
// on every Generate / Stream call.
//
// Phase 2a only exposes "text" / "json_object" through nobody yaml; the
// json_schema mode that Eino also supports is intentionally NOT exposed
// here until a real agent task drives the schema design (spec §1.2 + §13 Q3).
func buildResponseFormat(cfg *config.Config) *openai.ChatCompletionResponseFormat {
	if cfg == nil {
		return nil
	}
	switch cfg.Model.ResponseFormat {
	case "json_object":
		return &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	case "text", "":
		return nil
	default:
		// Loader's validateResponseFormat rejects anything else; this
		// branch is defensive against programmatic Config construction
		// that bypasses LoadConfig (e.g. internal tests).
		return nil
	}
}
