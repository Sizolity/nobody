package llamacpp_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/config"
	"github.com/sizolity/nobody/pkg/inference/llamacpp"
	"github.com/sizolity/nobody/pkg/skills"
)

func TestPublicLlamacppRuntimeConstructsFromConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Model.ProviderOpts["llamacpp"]["lifecycle"] = "external"

	rt, err := llamacpp.NewRuntime(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, rt)
	require.NoError(t, rt.Close())
}

func TestPublicLlamacppProviderName(t *testing.T) {
	require.Equal(t, "llamacpp", llamacpp.ProviderName)
}

func TestPublicLlamacppRuntimeEmbedderSignatureUsesPublicSkillsPackage(t *testing.T) {
	var rt *llamacpp.Runtime
	var _ func(context.Context, time.Duration) (skills.Embedder, error) = rt.CreateEmbedder
}
