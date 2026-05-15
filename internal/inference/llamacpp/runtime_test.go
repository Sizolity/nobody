package llamacpp

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/config"
)

func TestRuntime_NewSkipsEmbeddingManagerWhenDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.ProviderOpts["llamacpp"]["lifecycle"] = "external"

	rt, err := NewRuntime(cfg, func(string, string, map[string]any) {})
	require.NoError(t, err)
	require.Nil(t, rt.embeddingManagerForTest())
}

func TestRuntime_NewCreatesEmbeddingManagerWhenEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.ProviderOpts["llamacpp"]["embedding"] = map[string]any{
		"name": "nomic-embed-text-v1.5",
		"managed": map[string]any{
			"enabled": true,
			"model":   "/models/embed.gguf",
			"port":    18081,
		},
	}

	rt, err := NewRuntime(cfg, func(string, string, map[string]any) {})
	require.NoError(t, err)
	require.NotNil(t, rt.embeddingManagerForTest())
}
