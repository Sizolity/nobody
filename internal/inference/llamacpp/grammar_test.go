package llamacpp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/config"
)

// TestBuildGrammarOption_Off pins the Phase 1 default behaviour: when
// grammar=off (the value DefaultConfig seeds), buildGrammarOption emits
// zero options so the outer WithDefaultOptions wrapper degenerates to
// the bare openai ChatModel — operators upgrading from Phase 1 must see
// no on-the-wire change unless they explicitly opt in.
func TestBuildGrammarOption_Off(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	require.Equal(t, "off", cfg.Model.ProviderOpts[ProviderName]["grammar"], "DefaultConfig must seed grammar=off")
	assert.Empty(t, buildGrammarOption(cfg))
}

// TestBuildGrammarOption_MissingProviderOpts covers the defensive path
// when programmatic Config construction omits the ProviderOpts map
// entirely (e.g. tests that bypass LoadConfig). readGrammar must fall
// back to "off" rather than panic, and buildGrammarOption then emits
// zero options.
func TestBuildGrammarOption_MissingProviderOpts(t *testing.T) {
	cfg := &config.Config{Model: config.ModelConfig{Provider: ProviderName}}
	assert.Empty(t, buildGrammarOption(cfg))
}

// TestBuildGrammarOption_Auto verifies the +grammar tier of the Phase
// 2a benchmark matrix: when grammar=auto, exactly one option is emitted
// (the WithRequestPayloadModifier carrying jsonGrammar). We assert
// length only here — the contract test in openai_request_test.go
// asserts the on-the-wire shape via httptest.
func TestBuildGrammarOption_Auto(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.ProviderOpts[ProviderName]["grammar"] = "auto"
	opts := buildGrammarOption(cfg)
	require.Len(t, opts, 1, "auto must emit exactly one RequestPayloadModifier option")
}

// TestBuildGrammarOption_Custom verifies the operator-supplied GBNF
// passthrough path (spec ADR-5: nobody does not parse GBNF; llama-server
// is the source of truth). One option emitted, content irrelevant at
// this layer.
func TestBuildGrammarOption_Custom(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.ProviderOpts[ProviderName]["grammar"] = `root ::= "yes" | "no"`
	opts := buildGrammarOption(cfg)
	require.Len(t, opts, 1, "custom GBNF must emit exactly one RequestPayloadModifier option")
}

// TestInjectGrammar_AddsField is the unit-level guard for the modifier
// itself: given a minimal serialized OpenAI body, the returned function
// must add the grammar field at the top level and preserve every other
// field already present. This isolates the modifier's correctness from
// Eino's request pipeline, which the contract test exercises end-to-end.
func TestInjectGrammar_AddsField(t *testing.T) {
	mod := injectGrammar(`root ::= "a"`)
	in := []byte(`{"model":"x","messages":[]}`)
	out, err := mod(context.Background(), []*schema.Message{}, in)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(out, &parsed))
	assert.Equal(t, `root ::= "a"`, parsed["grammar"], "grammar field must hold the verbatim source string")
	assert.Equal(t, "x", parsed["model"], "must preserve other fields")
	_, ok := parsed["messages"].([]any)
	assert.True(t, ok, "messages array must survive the round-trip")
}

// TestInjectGrammar_PreservesPhase1Fields locks in compatibility with
// the Phase 1 contract (`top_k`, `chat_template_kwargs.enable_thinking`
// etc.). If a future refactor ever switches the modifier from a map
// round-trip to byte-level surgery, this test keeps the Phase 1 fields
// intact: any divergence is a CI red, not a silent regression.
func TestInjectGrammar_PreservesPhase1Fields(t *testing.T) {
	mod := injectGrammar(jsonGrammar)
	in := []byte(`{"model":"x","top_k":40,"chat_template_kwargs":{"enable_thinking":false}}`)
	out, err := mod(context.Background(), []*schema.Message{}, in)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(out, &parsed))
	assert.Equal(t, jsonGrammar, parsed["grammar"])
	assert.Equal(t, float64(40), parsed["top_k"], "top_k must survive (json.Number-less round-trip)")
	ctk, ok := parsed["chat_template_kwargs"].(map[string]any)
	require.True(t, ok, "chat_template_kwargs must survive as a nested object")
	assert.Equal(t, false, ctk["enable_thinking"])
}

// TestInjectGrammar_BadJSON verifies the failure path: a malformed body
// from Eino's serializer is a hard contract violation, not something to
// paper over. The modifier must surface a wrapped error so the request
// fails loudly rather than silently sending a corrupt body.
func TestInjectGrammar_BadJSON(t *testing.T) {
	mod := injectGrammar("g")
	_, err := mod(context.Background(), []*schema.Message{}, []byte("not-json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal", "error must identify the unmarshal step so debuggers can locate the failure")
	assert.Contains(t, err.Error(), "llamacpp: cannot inject grammar", "error must carry the package context prefix")
}
