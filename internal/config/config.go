package config

import (
	"time"
)

type SamplingPreset struct {
	Temperature       float32 `yaml:"temperature"`
	TopP              float32 `yaml:"top_p"`
	TopK              int     `yaml:"top_k"`
	MinP              float32 `yaml:"min_p"`
	PresencePenalty   float32 `yaml:"presence_penalty"`
	RepetitionPenalty float32 `yaml:"repetition_penalty"`
	NumPredict        int     `yaml:"num_predict"`
}

type PresetsConfig struct {
	Default  SamplingPreset `yaml:"default"`
	Coding   SamplingPreset `yaml:"coding"`
	Thinking SamplingPreset `yaml:"thinking"`
}

type Config struct {
	Model   ModelConfig   `yaml:"model"`
	Runtime RuntimeConfig `yaml:"runtime"`

	// Backward-compatible aliases used by existing call sites.
	ModelLegacy   string        `yaml:"-"`
	Workspace     string        `yaml:"-"`
	MaxIterations int           `yaml:"-"`
	Temperature   float32       `yaml:"-"`
	NumCtx        int           `yaml:"-"`
	Timeout       time.Duration `yaml:"-"`
}

type ModelConfig struct {
	Provider string        `yaml:"provider"`
	Name     string        `yaml:"name"`
	BaseURL  string        `yaml:"base_url"`
	Timeout  time.Duration `yaml:"timeout"`
	Think    string        `yaml:"think"` // auto | true | false
	Presets  PresetsConfig `yaml:"presets"`

	// ToolChoice controls the default tool-calling policy. Values:
	// "auto" (default), "forced", "forbidden". Interpretation is
	// provider-specific; this field only carries the intent.
	ToolChoice string `yaml:"tool_choice"`

	// ResponseFormat controls the response format hint. Values:
	// "text" (default), "json_object". Interpretation is
	// provider-specific.
	ResponseFormat string `yaml:"response_format"`

	// ProviderOpts carries provider-specific knobs. The outer key is
	// the provider name; the inner map holds arbitrary key/value pairs
	// that only the matching provider implementation understands.
	ProviderOpts map[string]map[string]any `yaml:"provider_opts"`
}

type RuntimeConfig struct {
	Workspace      string        `yaml:"workspace"`
	PromptsDir     string        `yaml:"prompts_dir"`
	MaxIterations  int           `yaml:"max_iterations"`
	Temperature    float32       `yaml:"temperature"`
	NumCtx         int           `yaml:"num_ctx"`
	EstimatedSpeed float64       `yaml:"estimated_speed"`
	ShellTimeout   time.Duration `yaml:"shell_timeout"`
}

func DefaultConfig() *Config {
	cfg := &Config{
		Model: ModelConfig{
			Name:           "",
			BaseURL:        "http://localhost:8080/v1",
			Timeout:        180 * time.Second,
			Think:          "false",
			ToolChoice:     "auto",
			ResponseFormat: "text",
			Presets: PresetsConfig{
				Default: SamplingPreset{
					Temperature:       0.7,
					TopP:              0.8,
					TopK:              20,
					MinP:              0.0,
					PresencePenalty:   1.5,
					RepetitionPenalty: 1.0,
					NumPredict:        8192,
				},
				Coding: SamplingPreset{
					Temperature:       0.6,
					TopP:              0.95,
					TopK:              20,
					MinP:              0.0,
					PresencePenalty:   0.0,
					RepetitionPenalty: 1.0,
					NumPredict:        16384,
				},
				Thinking: SamplingPreset{
					Temperature:       1.0,
					TopP:              0.95,
					TopK:              20,
					MinP:              0.0,
					PresencePenalty:   1.5,
					RepetitionPenalty: 1.0,
					NumPredict:        32768,
				},
			},
		},
		Runtime: RuntimeConfig{
			Workspace:      "./workspace",
			PromptsDir:     "prompts",
			MaxIterations:  15,
			Temperature:    0.7,
			NumCtx:         8192,
			EstimatedSpeed: 12,
			ShellTimeout:   60 * time.Second,
		},
	}
	cfg.applyCompatibilityMirror()
	return cfg
}

func (c *Config) applyCompatibilityMirror() {
	c.ModelLegacy = c.Model.Name
	c.Timeout = c.Model.Timeout
	c.Workspace = c.Runtime.Workspace
	c.MaxIterations = c.Runtime.MaxIterations
	c.Temperature = c.Runtime.Temperature
	c.NumCtx = c.Runtime.NumCtx
}
