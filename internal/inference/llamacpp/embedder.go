package llamacpp

import (
	"context"
	"time"

	embopenai "github.com/cloudwego/eino-ext/components/embedding/openai"

	"github.com/sizolity/nobody/internal/config"
	"github.com/sizolity/nobody/internal/skills"
)

// embedAdapter drops the variadic embedding.Option argument eino uses
// internally, because internal/skills.Embedder is the narrower consumer
// interface our retrieval path depends on (keeping eino-ext out of the
// skills package's import surface).
type embedAdapter struct {
	inner *embopenai.Embedder
}

func (a *embedAdapter) EmbedStrings(ctx context.Context, texts []string) ([][]float64, error) {
	return a.inner.EmbedStrings(ctx, texts)
}

// createEmbedder wires the eino OpenAI embedder to whichever llama-server
// instance is hosting the embedding model. llama-server has to be launched
// with --embedding and typically with a dedicated embedding checkpoint
// (e.g. nomic-embed-text-v1.5.Q8_0.gguf); so the embedding endpoint is
// usually a *different port* from the chat endpoint. Resolution order:
//
//  1. provider_opts.llamacpp.embedding.base_url / .name (when either set)
//  2. cfg.Model.BaseURL / cfg.Model.Name (shared with chat)
//
// Callers that run a single llama-server with both --embedding and a
// chat model should omit the embedding block and let fallback (2) do
// its thing.
func createEmbedder(ctx context.Context, cfg *config.Config, timeout time.Duration) (skills.Embedder, error) {
	baseURL, modelName := resolveEmbeddingTarget(cfg)
	emb, err := embopenai.NewEmbedder(ctx, &embopenai.EmbeddingConfig{
		BaseURL: baseURL,
		Model:   modelName,
		Timeout: timeout,
		APIKey:  providerAPIKey(cfg),
	})
	if err != nil {
		return nil, err
	}
	return &embedAdapter{inner: emb}, nil
}

func resolveEmbeddingTarget(cfg *config.Config) (string, string) {
	baseURL := cfg.Model.BaseURL
	modelName := cfg.Model.Name
	if opts := embeddingOpts(cfg); opts != nil {
		if v, ok := opts["base_url"].(string); ok && v != "" {
			baseURL = v
		}
		if v, ok := opts["name"].(string); ok && v != "" {
			modelName = v
		}
	}
	return baseURL, modelName
}
