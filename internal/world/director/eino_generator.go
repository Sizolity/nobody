package director

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// EinoGeneratorConfig configures an EinoGenerator created via NewEinoChatGenerator.
type EinoGeneratorConfig struct {
	// BaseURL is the OpenAI-compatible API base (e.g. "https://api.deepseek.com/v1").
	BaseURL string
	// Model name (e.g. "deepseek-v4-pro").
	Model string
	// APIKey for authentication.
	APIKey string
	// JSONMode forces the model to return valid JSON (sets response_format to json_object).
	JSONMode bool
	// Stream enables streaming mode. When true, Generate and GenerateRepair
	// use BaseChatModel.Stream internally, accumulating chunks into the full
	// response. The TextGenerator/ConversationGenerator interface is unchanged.
	Stream bool
	// StreamWriter receives each chunk in real-time when streaming is enabled.
	// If nil, chunks are accumulated silently. Ignored when Stream is false.
	StreamWriter io.Writer
}

// NewEinoChatGenerator constructs an EinoGenerator from config, creating the
// underlying Eino ChatModel internally. This is the preferred constructor for
// production use — callers don't need to touch eino-ext/components/model/openai
// directly.
func NewEinoChatGenerator(ctx context.Context, cfg EinoGeneratorConfig) (*EinoGenerator, error) {
	chatCfg := &openai.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		APIKey:  cfg.APIKey,
	}
	if cfg.JSONMode {
		chatCfg.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}
	m, err := openai.NewChatModel(ctx, chatCfg)
	if err != nil {
		return nil, fmt.Errorf("create eino chat model: %w", err)
	}
	return &EinoGenerator{model: m, stream: cfg.Stream, streamWriter: cfg.StreamWriter}, nil
}

// TokenUsage mirrors schema.TokenUsage for use outside eino.
type TokenUsage = schema.TokenUsage

// EinoGenerator adapts an Eino BaseChatModel to the TextGenerator and
// ConversationGenerator interfaces. Any OpenAI-compatible provider (DeepSeek,
// local llama.cpp, etc.) can be used through eino-ext/components/model/openai.
type EinoGenerator struct {
	model        einomodel.BaseChatModel
	stream       bool
	streamWriter io.Writer
	lastUsage    *schema.TokenUsage
}

// NewEinoGenerator wraps an existing Eino BaseChatModel as a TextGenerator.
// For most cases prefer NewEinoChatGenerator which handles construction.
func NewEinoGenerator(m einomodel.BaseChatModel) *EinoGenerator {
	return &EinoGenerator{model: m}
}

// NewEinoStreamGenerator wraps an existing Eino BaseChatModel with streaming enabled.
func NewEinoStreamGenerator(m einomodel.BaseChatModel) *EinoGenerator {
	return &EinoGenerator{model: m, stream: true}
}

func (g *EinoGenerator) Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	msgs := make([]*schema.Message, 0, 2)
	if systemPrompt != "" {
		msgs = append(msgs, &schema.Message{Role: schema.System, Content: systemPrompt})
	}
	msgs = append(msgs, &schema.Message{Role: schema.User, Content: userPrompt})
	return g.doGenerate(ctx, msgs)
}

func (g *EinoGenerator) GenerateRepair(ctx context.Context, systemPrompt, originalUser, previousAssistant, repairUser string) (string, error) {
	msgs := make([]*schema.Message, 0, 4)
	if systemPrompt != "" {
		msgs = append(msgs, &schema.Message{Role: schema.System, Content: systemPrompt})
	}
	msgs = append(msgs,
		&schema.Message{Role: schema.User, Content: originalUser},
		&schema.Message{Role: schema.Assistant, Content: previousAssistant},
		&schema.Message{Role: schema.User, Content: repairUser},
	)
	return g.doGenerate(ctx, msgs)
}

func (g *EinoGenerator) doGenerate(ctx context.Context, msgs []*schema.Message) (string, error) {
	if g.stream {
		return g.doStream(ctx, msgs)
	}
	resp, err := g.model.Generate(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("eino generate: %w", err)
	}
	g.lastUsage = nil
	if resp.ResponseMeta != nil {
		g.lastUsage = resp.ResponseMeta.Usage
	}
	return resp.Content, nil
}

// SetStreamWriter sets or replaces the writer that receives live chunks.
func (g *EinoGenerator) SetStreamWriter(w io.Writer) {
	g.streamWriter = w
}

// LastUsage returns the token usage from the most recent Generate or
// GenerateRepair call. Returns nil when the provider did not report usage.
func (g *EinoGenerator) LastUsage() *schema.TokenUsage {
	return g.lastUsage
}

func (g *EinoGenerator) doStream(ctx context.Context, msgs []*schema.Message) (string, error) {
	reader, err := g.model.Stream(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("eino stream: %w", err)
	}
	defer reader.Close()

	g.lastUsage = nil
	var buf strings.Builder
	for {
		chunk, recvErr := reader.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				break
			}
			return "", fmt.Errorf("eino stream recv: %w", recvErr)
		}
		buf.WriteString(chunk.Content)
		if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
			g.lastUsage = chunk.ResponseMeta.Usage
		}
		if g.streamWriter != nil {
			_, _ = io.WriteString(g.streamWriter, chunk.Content)
		}
	}
	return buf.String(), nil
}

// ProviderDefaults maps known provider names to their default base URL and model.
var ProviderDefaults = map[string]EinoGeneratorConfig{
	"deepseek": {
		BaseURL:  "https://api.deepseek.com/v1",
		Model:    "deepseek-v4-pro",
		JSONMode: true,
	},
}

// ProviderGeneratorOption configures optional behavior for NewProviderGenerator.
type ProviderGeneratorOption func(*EinoGeneratorConfig)

// WithStreamWriter enables streaming and tees chunks to w in real-time.
func WithStreamWriter(w io.Writer) ProviderGeneratorOption {
	return func(cfg *EinoGeneratorConfig) {
		cfg.Stream = true
		cfg.StreamWriter = w
	}
}

// NewProviderGenerator creates an EinoGenerator for a named provider.
// It applies ProviderDefaults, overriding the model if modelName is non-empty,
// and reads the API key from the apiKey parameter. Returns an error if the
// provider is unknown or apiKey is empty.
func NewProviderGenerator(ctx context.Context, provider, modelName, apiKey string, opts ...ProviderGeneratorOption) (*EinoGenerator, error) {
	defaults, ok := ProviderDefaults[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported LLM provider %q", provider)
	}
	cfg := defaults
	cfg.APIKey = apiKey
	if modelName != "" {
		cfg.Model = modelName
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%s provider requires an API key", provider)
	}
	return NewEinoChatGenerator(ctx, cfg)
}
