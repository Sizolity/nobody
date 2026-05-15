package llamacpp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/config"
)

// TestOpenAIRequestBody_ContractGuard exercises factory.CreateChatModel
// + Generate against a fake OpenAI server, captures the outgoing
// /v1/chat/completions request body, and asserts the JSON shape that
// llama-server is supposed to receive. This is the CI-level guard
// against silent regressions in eino-ext/components/model/openai when
// it changes how ExtraFields is spliced into the request body.
//
// If this test starts failing after an eino-ext bump, fix:
//  1. Re-read eino-ext/components/model/openai docs for ExtraFields /
//     ExtraBody semantics.
//  2. Adjust buildChatCfg in presets.go to match the new shape.
//  3. Update assertions below; do NOT silently downgrade them.
func TestOpenAIRequestBody_ContractGuard(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
		gotPath string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		mu.Unlock()
		// Minimal valid OpenAI completion response so eino doesn't error.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 0,
			"model": "qwen3.5-4b",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": "ok"}}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Model: config.ModelConfig{
			Provider: ProviderName,
			Name:     "qwen3.5-4b",
			BaseURL:  srv.URL + "/v1",
		},
	}
	preset := config.SamplingPreset{
		Temperature:       0.6,
		TopP:              0.9,
		TopK:              20,
		MinP:              0.05,
		PresencePenalty:   1.5,
		RepetitionPenalty: 1.05,
		NumPredict:        2048,
	}

	chatModel, err := factory{}.CreateChatModel(context.Background(), cfg, preset, true, 5*time.Second)
	require.NoError(t, err)

	_, err = chatModel.Generate(context.Background(), []*schema.Message{
		{Role: schema.User, Content: "ping"},
	})
	require.NoError(t, err, "fake server returned a valid completion; Generate must succeed")

	mu.Lock()
	body := append([]byte(nil), gotBody...)
	path := gotPath
	mu.Unlock()
	require.NotEmpty(t, body, "fake server must have received a request body")
	assert.Equal(t, "/v1/chat/completions", path, "eino openai client must hit /v1/chat/completions")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	// Top-level OpenAI-native fields populated from ChatModelConfig pointer fields.
	assert.Equal(t, "qwen3.5-4b", parsed["model"])
	if temp, ok := parsed["temperature"].(float64); ok {
		assert.InDelta(t, 0.6, temp, 1e-3)
	} else {
		t.Errorf("expected float temperature in body, got %T (%v)", parsed["temperature"], parsed["temperature"])
	}
	if topP, ok := parsed["top_p"].(float64); ok {
		assert.InDelta(t, 0.9, topP, 1e-3)
	} else {
		t.Errorf("expected float top_p in body, got %T", parsed["top_p"])
	}
	if pp, ok := parsed["presence_penalty"].(float64); ok {
		assert.InDelta(t, 1.5, pp, 1e-3)
	} else {
		t.Errorf("expected float presence_penalty in body, got %T", parsed["presence_penalty"])
	}
	assert.Equal(t, float64(2048), parsed["max_tokens"], "NumPredict must map to max_tokens")

	// llama.cpp-native fields go through ExtraFields and must land at
	// top level (not under "extra_body" or similar) for llama-server to
	// see them.
	assert.Equal(t, float64(20), parsed["top_k"], "TopK must reach top level via ExtraFields")
	if mp, ok := parsed["min_p"].(float64); ok {
		assert.InDelta(t, 0.05, mp, 1e-3, "MinP must reach top level via ExtraFields")
	} else {
		t.Errorf("expected float min_p in body, got %T (%v)", parsed["min_p"], parsed["min_p"])
	}
	if rp, ok := parsed["repetition_penalty"].(float64); ok {
		assert.InDelta(t, 1.05, rp, 1e-3, "RepetitionPenalty must reach top level via ExtraFields")
	} else {
		t.Errorf("expected float repetition_penalty in body, got %T", parsed["repetition_penalty"])
	}

	// chat_template_kwargs is the Qwen3 think toggle; must be a nested
	// object at the top level (not flattened by eino).
	ctk, ok := parsed["chat_template_kwargs"].(map[string]any)
	require.True(t, ok, "chat_template_kwargs must be a nested object at top level (got %T)", parsed["chat_template_kwargs"])
	assert.Equal(t, true, ctk["enable_thinking"], "think=true must propagate to chat_template_kwargs.enable_thinking")
}

// TestOpenAIRequestBody_ContractGuard_ThinkOff is the negative half of
// the toggle: think=false must still emit chat_template_kwargs with
// enable_thinking=false (Qwen3 templates always look at the flag).
func TestOpenAIRequestBody_ContractGuard_ThinkOff(t *testing.T) {
	var (
		capMu        sync.Mutex
		capturedBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capMu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		capMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "x", "object": "chat.completion", "created": 0, "model": "x",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": ""}}],
			"usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
		}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Model: config.ModelConfig{Provider: ProviderName, Name: "x", BaseURL: srv.URL + "/v1"},
	}
	chatModel, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{Temperature: 0.7}, false, 5*time.Second)
	require.NoError(t, err)
	_, _ = chatModel.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "x"}})

	capMu.Lock()
	body := append([]byte(nil), capturedBody...)
	capMu.Unlock()
	require.NotEmpty(t, body, "fake server must have received a request body (think=false branch)")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	ctk, ok := parsed["chat_template_kwargs"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, ctk["enable_thinking"])
}

// TestResponseFormatShape_JSONObject is the Phase 2a T2 contract test:
// when cfg.Model.ResponseFormat == "json_object", the outbound
// /v1/chat/completions body must carry a top-level
// `response_format: {"type": "json_object"}` object. This pins the
// translation from nobody yaml → openai.ChatCompletionResponseFormat
// → eino-ext openai client serialization, so an upstream change in any
// of the three layers surfaces as a CI red rather than silent drift.
//
// We deliberately reuse the httptest pattern of the Phase 1 contract
// test above so the diff stays uniform; do NOT factor a shared helper
// until a third contract test (TestGrammarShape from Phase 2a T3)
// makes the duplication actually painful.
func TestResponseFormatShape_JSONObject(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 0, "model": "qwen3.5-4b",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": "{}"}}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer srv.Close()

	// DefaultConfig() seeds ResponseFormat="text" so we must override it
	// explicitly; using DefaultConfig (not a bare struct) keeps the test
	// honest about which knob actually flips the wire shape.
	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = srv.URL + "/v1"
	cfg.Model.Name = "qwen3.5-4b"
	cfg.Model.ResponseFormat = "json_object"

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, 5*time.Second)
	require.NoError(t, err)
	_, err = cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	require.NoError(t, err, "fake server returns a valid completion; Generate must succeed")

	mu.Lock()
	body := append([]byte(nil), gotBody...)
	mu.Unlock()
	require.NotEmpty(t, body, "fake server must have received a request body")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	rf, ok := parsed["response_format"].(map[string]any)
	require.True(t, ok, "expected top-level response_format object, got %T (%v)", parsed["response_format"], parsed["response_format"])
	assert.Equal(t, "json_object", rf["type"], "ResponseFormat=json_object must serialize as {type: json_object}")
	assert.Len(t, rf, 1, "json_object mode response_format object must contain only the type key (got %v)", rf)
}

// TestGrammarShape_Auto is the Phase 2a T3 contract test: when
// provider_opts.llamacpp.grammar="auto", the outbound
// /v1/chat/completions body must carry a top-level `grammar` string
// equal to the embedded jsonGrammar constant. This pins three
// independent layers in one assertion:
//
//  1. grammar.go::buildGrammarOption emits the WithRequestPayloadModifier
//     option (rather than skipping or returning nil).
//  2. Eino's openai client wires the modifier into its request pipeline
//     (chat_model.go:723 — RequestPayloadModifier is invoked once per
//     request, after standard fields are serialized).
//  3. The injectGrammar map round-trip preserves Phase 1 fields
//     (model / chat_template_kwargs) so this T3 layer composes with the
//     Phase 1 contract guarded by TestOpenAIRequestBody_ContractGuard.
//
// We assert structural shape (top-level field, string type, contains
// the canonical "root" rule marker) rather than full string equality:
// any future tweak to jsonGrammar's whitespace would otherwise force a
// test churn unrelated to behaviour.
func TestGrammarShape_Auto(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 0, "model": "qwen3.5-4b",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": "{}"}}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer srv.Close()

	// DefaultConfig() seeds grammar="off" and think=false; we must flip
	// grammar to "auto" explicitly. Reusing DefaultConfig (rather than a
	// bare ModelConfig) keeps this test honest about which yaml knob
	// flips the wire shape.
	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = srv.URL + "/v1"
	cfg.Model.Name = "qwen3.5-4b"
	cfg.Model.ProviderOpts[ProviderName]["grammar"] = "auto"

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, 5*time.Second)
	require.NoError(t, err)
	_, err = cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	require.NoError(t, err, "fake server returns a valid completion; Generate must succeed")

	mu.Lock()
	body := append([]byte(nil), gotBody...)
	mu.Unlock()
	require.NotEmpty(t, body, "fake server must have received a request body")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	g, ok := parsed["grammar"].(string)
	require.True(t, ok, "expected top-level grammar string field, got %T (%v)", parsed["grammar"], parsed["grammar"])
	assert.Equal(t, jsonGrammar, g, "grammar=auto must serialize the embedded jsonGrammar verbatim")
	assert.Contains(t, g, "root", "auto grammar must contain llama.cpp's built-in json grammar root rule marker")

	// Phase 1 contract co-verification: grammar injection must NOT
	// damage the fields TestOpenAIRequestBody_ContractGuard asserts on,
	// otherwise the wrapper-vs-bare composition is broken.
	assert.Equal(t, "qwen3.5-4b", parsed["model"], "model must survive the modifier round-trip")
	ctk, ok := parsed["chat_template_kwargs"].(map[string]any)
	require.True(t, ok, "chat_template_kwargs must survive the modifier round-trip (got %T)", parsed["chat_template_kwargs"])
	assert.Equal(t, false, ctk["enable_thinking"], "think=false branch of chat_template_kwargs must survive")
}

// TestGrammarShape_Auto_DroppedWhenToolsBound is the second half of
// the T3 contract, added 2026-04-28 after a GPU bench against
// llama.cpp v8895 surfaced a server-side hard mutex: requests that
// carry both a non-empty `tools` array AND a top-level `grammar`
// string get rejected with HTTP 400 "Cannot use custom grammar
// constraints with tools." before any sampling happens. injectGrammar
// degrades gracefully by dropping the grammar field whenever it sees
// tools on the outbound body — this test pins that contract so a
// future refactor cannot accidentally re-introduce the 400 path.
//
// Why pin it explicitly: a regression here is silent at the schema
// layer (no compile error) but turns every tool-using agent into a
// hard-fail loop on llama-server. The Phase 2a benchmark report
// 20260428T021413Z-llamacpp-bench.md captures the original 10/10
// failure rate that this fix addresses.
func TestGrammarShape_Auto_DroppedWhenToolsBound(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "x", "object": "chat.completion", "created": 0, "model": "x",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": "{}"}}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = srv.URL + "/v1"
	cfg.Model.Name = "qwen3.5-4b"
	cfg.Model.ProviderOpts[ProviderName]["grammar"] = "auto"

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, 5*time.Second)
	require.NoError(t, err)

	tool := &schema.ToolInfo{
		Name: "echo",
		Desc: "echoes",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"text": {Type: schema.String, Required: true, Desc: "what to echo"},
		}),
	}
	bound, err := cm.WithTools([]*schema.ToolInfo{tool})
	require.NoError(t, err)

	_, err = bound.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "echo hi"}})
	require.NoError(t, err, "fake server returns a valid completion; Generate must succeed")

	mu.Lock()
	body := append([]byte(nil), gotBody...)
	mu.Unlock()
	require.NotEmpty(t, body, "fake server must have received a request body")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	tools, ok := parsed["tools"].([]any)
	require.True(t, ok, "test harness bug: outbound body must carry tools[] (got %T)", parsed["tools"])
	require.NotEmpty(t, tools, "test harness bug: outbound tools[] must be non-empty")

	_, hasGrammar := parsed["grammar"]
	assert.False(t, hasGrammar,
		`grammar=auto + tools=[echo] must drop grammar from the outbound body to avoid llama-server's "Cannot use custom grammar constraints with tools." 400; got grammar=%v`,
		parsed["grammar"])
}

// TestGrammarShape_OffOmitsField is the negative half of the T3
// contract: the default ("off") MUST NOT emit a grammar field in the
// outbound body. Without this assertion, a buildGrammarOption bug
// returning a no-op modifier (instead of nil) would still inject
// `grammar: ""` and silently change the wire shape on Phase 1
// installations.
func TestGrammarShape_OffOmitsField(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "x", "object": "chat.completion", "created": 0, "model": "x",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": ""}}],
			"usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
		}`))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig() // grammar="off" by default
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = srv.URL + "/v1"
	cfg.Model.Name = "qwen3.5-4b"

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, 5*time.Second)
	require.NoError(t, err)
	_, err = cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	require.NoError(t, err)

	mu.Lock()
	body := append([]byte(nil), gotBody...)
	mu.Unlock()
	require.NotEmpty(t, body)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	_, present := parsed["grammar"]
	assert.False(t, present, `default grammar="off" must NOT serialize a grammar field on the wire (got: %v)`, parsed["grammar"])
}

// TestResponseFormatShape_TextOmitsField is the negative half of the
// T2 contract: the default ("text") MUST NOT emit a response_format
// field in the outbound body. This is what guarantees Phase 1
// behaviour is preserved bit-for-bit when an operator leaves the new
// yaml key at its default — without this assertion, a future bug
// where buildResponseFormat returns a {Type: "text"} object would
// silently change the wire shape.
func TestResponseFormatShape_TextOmitsField(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "x", "object": "chat.completion", "created": 0, "model": "x",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": ""}}],
			"usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
		}`))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig() // ResponseFormat="text" by default
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = srv.URL + "/v1"
	cfg.Model.Name = "qwen3.5-4b"

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, 5*time.Second)
	require.NoError(t, err)
	_, err = cm.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	require.NoError(t, err)

	mu.Lock()
	body := append([]byte(nil), gotBody...)
	mu.Unlock()
	require.NotEmpty(t, body)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	_, present := parsed["response_format"]
	assert.False(t, present, `default ResponseFormat="text" must NOT serialize a response_format field on the wire (got: %v)`, parsed["response_format"])
}

// TestToolChoiceShape_Forced_SingleTool is the Phase 2a T1 contract test
// for the single-tool branch of Eino acl's populateToolChoice mapping
// (eino-ext/libs/acl/openai/chat_model.go:1035-1058): when
// cfg.Model.ToolChoice="forced" and exactly one tool is bound, the
// outbound /v1/chat/completions body must carry a
//
//	"tool_choice": {"type":"function","function":{"name":"<tool>"}}
//
// object — NOT the bare string "required". This pins three layers in
// one assertion:
//
//  1. inference.DefaultsFromConfig translates "forced" into
//     model.WithToolChoice(schema.ToolChoiceForced) (options.go:32-33).
//  2. inference.WithDefaultOptions prepends the option on every Generate
//     call so the option survives WithTools wrapping.
//  3. eino-ext acl's populateToolChoice sees one entry in req.Tools and
//     follows the "single-tool short-circuit" branch — any future
//     refactor that loses that short-circuit will surface here as a
//     CI red rather than a silent semantics drift on llama-server.
func TestToolChoiceShape_Forced_SingleTool(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "x", "object": "chat.completion", "created": 0, "model": "x",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": ""}}],
			"usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
		}`))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = srv.URL + "/v1"
	cfg.Model.Name = "qwen3.5-4b"
	cfg.Model.ToolChoice = "forced"

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, 5*time.Second)
	require.NoError(t, err)

	bound, err := cm.WithTools([]*schema.ToolInfo{{
		Name:        "echo",
		Desc:        "Echo the input back.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}})
	require.NoError(t, err, "WithTools must accept a 1-tool list")

	_, err = bound.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	require.NoError(t, err)

	mu.Lock()
	body := append([]byte(nil), gotBody...)
	mu.Unlock()
	require.NotEmpty(t, body)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	tc, ok := parsed["tool_choice"].(map[string]any)
	require.True(t, ok, "expected tool_choice object for Forced+1tool, got %T (%v)", parsed["tool_choice"], parsed["tool_choice"])
	assert.Equal(t, "function", tc["type"], "Forced+1tool must serialize as type=function")
	fn, ok := tc["function"].(map[string]any)
	require.True(t, ok, "tool_choice.function must be an object, got %T (%v)", tc["function"], tc["function"])
	assert.Equal(t, "echo", fn["name"], "single-tool short-circuit must name the bound tool")
}

// TestToolChoiceShape_Forced_MultiTool is the multi-tool branch of the
// same T1 contract: when cfg.Model.ToolChoice="forced" and ≥2 tools are
// bound (no allowed_tool_names override), the outbound body must carry
// the bare string `"tool_choice": "required"` per Eino acl's
// populateToolChoice (chat_model.go:1067-1068). Without this assertion,
// a future Eino tweak that emits {"type":"required"} or
// {"type":"any"} would silently change wire semantics on
// llama-server.
func TestToolChoiceShape_Forced_MultiTool(t *testing.T) {
	var (
		mu      sync.Mutex
		gotBody []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "x", "object": "chat.completion", "created": 0, "model": "x",
			"choices": [{"index": 0, "finish_reason": "stop", "message": {"role": "assistant", "content": ""}}],
			"usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
		}`))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Model.Provider = ProviderName
	cfg.Model.BaseURL = srv.URL + "/v1"
	cfg.Model.Name = "qwen3.5-4b"
	cfg.Model.ToolChoice = "forced"

	cm, err := factory{}.CreateChatModel(context.Background(), cfg, config.SamplingPreset{}, false, 5*time.Second)
	require.NoError(t, err)

	bound, err := cm.WithTools([]*schema.ToolInfo{
		{Name: "echo", Desc: "Echo the input back.", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})},
		{Name: "ping", Desc: "Reply with pong.", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})},
	})
	require.NoError(t, err, "WithTools must accept a 2-tool list")

	_, err = bound.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "hi"}})
	require.NoError(t, err)

	mu.Lock()
	body := append([]byte(nil), gotBody...)
	mu.Unlock()
	require.NotEmpty(t, body)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	s, ok := parsed["tool_choice"].(string)
	require.True(t, ok, "expected bare-string tool_choice for Forced+multi-tool, got %T (%v)", parsed["tool_choice"], parsed["tool_choice"])
	assert.Equal(t, "required", s, "Forced+multi-tool must serialize as tool_choice=\"required\"")
}
