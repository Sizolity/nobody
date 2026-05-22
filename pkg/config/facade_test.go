package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/config"
)

func TestPublicConfigLoadsSharedRuntimeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nobody.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
model:
  provider: llamacpp
  name: qwen3.5-public
  timeout: 45s
runtime:
  workspace: ./writer-workspace
`), 0o644))

	cfg, err := config.LoadConfig(config.LoadOptions{ConfigPath: path})
	require.NoError(t, err)
	require.Equal(t, "qwen3.5-public", cfg.Model.Name)
	require.Equal(t, 45*time.Second, cfg.Model.Timeout)
	require.Equal(t, "./writer-workspace", cfg.Runtime.Workspace)
}

func TestPublicConfigDefaultCanBeOverridden(t *testing.T) {
	cfg := config.DefaultConfig()
	config.ApplyCLIOverrides(cfg, config.CLIOverrides{
		Model:     "qwen3.5-writer",
		Workspace: "./workspace-writer",
		BaseURL:   "http://127.0.0.1:18080/v1",
	})

	require.Equal(t, "qwen3.5-writer", cfg.Model.Name)
	require.Equal(t, "qwen3.5-writer", cfg.ModelLegacy)
	require.Equal(t, "./workspace-writer", cfg.Runtime.Workspace)
	require.Equal(t, "./workspace-writer", cfg.Workspace)
	require.Equal(t, "http://127.0.0.1:18080/v1", cfg.Model.BaseURL)
}
