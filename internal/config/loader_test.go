package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigKeepsSharedRuntimeAndIgnoresLegacyProductKeys(t *testing.T) {
	path := writeConfig(t, `
model:
  name: qwen3.5-custom
  base_url: http://127.0.0.1:18080/v1
  timeout: 45s
runtime:
  workspace: ./narrative-workspace
  prompts_dir: narrative-prompts
  max_iterations: 7
  temperature: 0.4
  num_ctx: 16384
  estimated_speed: 24
  shell_timeout: 15s
orchestrator:
  max_retries: 99
  planner: old-agent-loop
sandbox:
  mode: old-shell-tools
skills:
  agent_md_path: prompts/AGENT.md
context:
  handoff_dir: .handoff
memory:
  db_path: .nobody/memory.db
`)

	cfg, err := LoadConfig(LoadOptions{ConfigPath: path})
	require.NoError(t, err)

	require.Equal(t, "qwen3.5-custom", cfg.Model.Name)
	require.Equal(t, "http://127.0.0.1:18080/v1", cfg.Model.BaseURL)
	require.Equal(t, 45*time.Second, cfg.Model.Timeout)
	require.Equal(t, "./narrative-workspace", cfg.Runtime.Workspace)
	require.Equal(t, "narrative-prompts", cfg.Runtime.PromptsDir)
	require.Equal(t, 7, cfg.Runtime.MaxIterations)
	require.Equal(t, float32(0.4), cfg.Runtime.Temperature)
	require.Equal(t, 16384, cfg.Runtime.NumCtx)
	require.Equal(t, 24.0, cfg.Runtime.EstimatedSpeed)
	require.Equal(t, 15*time.Second, cfg.Runtime.ShellTimeout)

	require.Equal(t, cfg.Model.Name, cfg.ModelLegacy)
	require.Equal(t, cfg.Runtime.Workspace, cfg.Workspace)
	require.Equal(t, cfg.Runtime.MaxIterations, cfg.MaxIterations)
	require.Equal(t, cfg.Runtime.Temperature, cfg.Temperature)
	require.Equal(t, cfg.Runtime.NumCtx, cfg.NumCtx)
	require.Equal(t, cfg.Model.Timeout, cfg.Timeout)
}

func TestLoadConfigPatchesModelDefaults(t *testing.T) {
	path := writeConfig(t, `
model:
  name: test-model
`)
	cfg, err := LoadConfig(LoadOptions{ConfigPath: path})
	require.NoError(t, err)
	require.Equal(t, "auto", cfg.Model.ToolChoice)
	require.Equal(t, "text", cfg.Model.ResponseFormat)
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nobody.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}
