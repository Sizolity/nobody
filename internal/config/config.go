package config

import (
	"fmt"
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

	// ToolChoice controls the default tool-calling policy applied to every
	// Generate / Stream call. Values: "auto" (default; equivalent to
	// schema.ToolChoiceAllowed → OpenAI tool_choice=auto), "forced"
	// (schema.ToolChoiceForced → OpenAI tool_choice=required or specific
	// function name when the tool set is unambiguous), "forbidden"
	// (schema.ToolChoiceForbidden → OpenAI tool_choice=none). Per-call
	// overrides are still honored — defaults are prepended, callers' opts
	// take precedence.
	ToolChoice string `yaml:"tool_choice"`

	// ResponseFormat controls the OpenAI response_format field on every
	// Generate / Stream request. Values: "text" (default; no response_format
	// sent), "json_object" (sets response_format={type:"json_object"}, which
	// most modern chat models understand to constrain output to a syntactically
	// valid JSON value).
	ResponseFormat string `yaml:"response_format"`

	// ProviderOpts carries provider-specific knobs that the matching
	// inference sub-package (internal/inference/<provider>) is
	// responsible for parsing. Shape: map[provider-name]map[key]any.
	// The outer key is flat so every provider gets its own nested
	// block, and the inference layer remains free to add new options
	// without touching this struct. Keys not recognised by the active
	// provider are ignored.
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

// ManagedConfig carries the parsed provider_opts.llamacpp.managed sub-tree
// consumed by internal/inference/llamacpp/manager.go when
// provider_opts.llamacpp.lifecycle == "managed". The two pointer fields
// (NGL, Ctx) carry presence information so the runtime can apply
// Recommend() defaults for unset values without overwriting an explicit
// `0` or `8192` written by the operator. See spec §3.3 for the explicit-
// detection contract.
type ManagedConfig struct {
	Bin           string
	Model         string
	Port          int
	Host          string
	NGL           *int
	Ctx           *int
	Template      string
	ExtraFlags    []string
	HealthTimeout time.Duration
}

type EmbeddingManagedConfig struct {
	Enabled       bool
	Bin           string
	Model         string
	Port          int
	Host          string
	NGL           *int
	Ctx           *int
	ExtraFlags    []string
	HealthTimeout time.Duration
}

// ParseManagedConfig extracts the managed sub-tree from
// cfg.Model.ProviderOpts["llamacpp"]["managed"]. Returns the zero value
// (with all spec §3.2 defaults applied) when the block is missing.
// Errors only on type mismatches that yaml.v3 can produce when the
// operator writes the wrong shape (e.g. `port: "8080"` string instead
// of int).
//
// NGL and Ctx are returned as nil pointers when absent; callers distinguish
// "operator did not set" from "operator set 0" by checking the pointer.
func ParseManagedConfig(cfg *Config) (ManagedConfig, error) {
	def := ManagedConfig{
		Bin:           "llama-server",
		Model:         "~/models/qwen3.5-4b-q4_k_m.gguf",
		Port:          8080,
		Host:          "127.0.0.1",
		Template:      "",
		ExtraFlags:    []string{},
		HealthTimeout: 30 * time.Second,
	}
	if cfg == nil || cfg.Model.ProviderOpts == nil {
		return def, nil
	}
	po, ok := cfg.Model.ProviderOpts["llamacpp"]
	if !ok || po == nil {
		return def, nil
	}
	rawBlock, ok := po["managed"]
	if !ok || rawBlock == nil {
		return def, nil
	}
	block, ok := rawBlock.(map[string]any)
	if !ok {
		return def, fmt.Errorf("provider_opts.llamacpp.managed must be a map, got %T", rawBlock)
	}
	out := def
	if raw, ok := block["bin"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.bin must be a string, got %T", raw)
		}
		if s != "" {
			out.Bin = s
		}
	}
	if raw, ok := block["model"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.model must be a string, got %T", raw)
		}
		if s != "" {
			out.Model = s
		}
	}
	if raw, ok := block["port"]; ok && raw != nil {
		n, ok := raw.(int)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.port must be an int, got %T", raw)
		}
		if n <= 0 || n > 65535 {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.port=%d out of range (1-65535)", n)
		}
		out.Port = n
	}
	if raw, ok := block["host"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.host must be a string, got %T", raw)
		}
		if s != "" {
			out.Host = s
		}
	}
	if raw, ok := block["template"]; ok && raw != nil {
		// Empty string is an explicit "use jinja default" — preserve;
		// only the type assertion gates this branch.
		s, ok := raw.(string)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.template must be a string, got %T", raw)
		}
		out.Template = s
	}
	if raw, ok := block["ngl"]; ok && raw != nil {
		n, ok := raw.(int)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.ngl must be an int, got %T", raw)
		}
		out.NGL = &n
	}
	if raw, ok := block["ctx"]; ok && raw != nil {
		n, ok := raw.(int)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.ctx must be an int, got %T", raw)
		}
		out.Ctx = &n
	}
	if v, ok := block["extra_flags"].([]any); ok {
		flags := make([]string, 0, len(v))
		for i, raw := range v {
			s, ok := raw.(string)
			if !ok {
				return out, fmt.Errorf("provider_opts.llamacpp.managed.extra_flags[%d] must be a string, got %T", i, raw)
			}
			flags = append(flags, s)
		}
		out.ExtraFlags = flags
	}
	if v, ok := block["health_timeout"].(string); ok && v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return out, fmt.Errorf("provider_opts.llamacpp.managed.health_timeout: %w", err)
		}
		out.HealthTimeout = d
	}
	return out, nil
}

func ParseEmbeddingManagedConfig(cfg *Config) (EmbeddingManagedConfig, error) {
	chatDef, _ := ParseManagedConfig(cfg)
	def := EmbeddingManagedConfig{
		Enabled:       false,
		Bin:           chatDef.Bin,
		Port:          8081,
		Host:          "127.0.0.1",
		ExtraFlags:    []string{"--embedding"},
		HealthTimeout: 30 * time.Second,
	}
	if cfg == nil || cfg.Model.ProviderOpts == nil {
		return def, nil
	}
	po, ok := cfg.Model.ProviderOpts["llamacpp"]
	if !ok || po == nil {
		return def, nil
	}
	embedding, ok := po["embedding"].(map[string]any)
	if !ok || embedding == nil {
		return def, nil
	}
	rawManaged, ok := embedding["managed"]
	if !ok || rawManaged == nil {
		return def, nil
	}
	block, ok := rawManaged.(map[string]any)
	if !ok {
		return def, fmt.Errorf("provider_opts.llamacpp.embedding.managed must be a map, got %T", rawManaged)
	}
	out := def
	if raw, ok := block["enabled"]; ok && raw != nil {
		v, ok := raw.(bool)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.enabled must be a bool, got %T", raw)
		}
		out.Enabled = v
	}
	if raw, ok := block["bin"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.bin must be a string, got %T", raw)
		}
		if s != "" {
			out.Bin = s
		}
	}
	if raw, ok := block["model"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.model must be a string, got %T", raw)
		}
		if s != "" {
			out.Model = s
		}
	}
	if raw, ok := block["port"]; ok && raw != nil {
		n, ok := raw.(int)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.port must be an int, got %T", raw)
		}
		if n <= 0 || n > 65535 {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.port=%d out of range (1-65535)", n)
		}
		out.Port = n
	}
	if raw, ok := block["host"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.host must be a string, got %T", raw)
		}
		if s != "" {
			out.Host = s
		}
	}
	if raw, ok := block["ngl"]; ok && raw != nil {
		n, ok := raw.(int)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.ngl must be an int, got %T", raw)
		}
		out.NGL = &n
	}
	if raw, ok := block["ctx"]; ok && raw != nil {
		n, ok := raw.(int)
		if !ok {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.ctx must be an int, got %T", raw)
		}
		out.Ctx = &n
	}
	if v, ok := block["extra_flags"].([]any); ok {
		flags := make([]string, 0, len(v)+1)
		for i, raw := range v {
			s, ok := raw.(string)
			if !ok {
				return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.extra_flags[%d] must be a string, got %T", i, raw)
			}
			flags = append(flags, s)
		}
		out.ExtraFlags = appendEmbeddingFlag(flags)
	}
	if v, ok := block["health_timeout"].(string); ok && v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.health_timeout: %w", err)
		}
		out.HealthTimeout = d
	}
	if v, ok := block["health_timeout"].(time.Duration); ok && v > 0 {
		out.HealthTimeout = v
	}
	if out.Enabled && out.Model == "" {
		return out, fmt.Errorf("provider_opts.llamacpp.embedding.managed.model is required when embedding managed is enabled")
	}
	out.ExtraFlags = appendEmbeddingFlag(out.ExtraFlags)
	return out, nil
}

func appendEmbeddingFlag(flags []string) []string {
	for _, flag := range flags {
		if flag == "--embedding" {
			return flags
		}
	}
	return append(flags, "--embedding")
}

func DefaultConfig() *Config {
	cfg := &Config{
		Model: ModelConfig{
			Provider:       "llamacpp",
			Name:           "qwen3.5-4b",
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
			ProviderOpts: map[string]map[string]any{
				"llamacpp": {
					"mode":           "openai_compat",
					"lifecycle":      "external",
					"probe_path":     "/health",
					"reconnect_max":  5,
					"reconnect_base": 1 * time.Second,
					"api_key":        "",
					"grammar":        "off",
					"embedding": map[string]any{
						"managed": map[string]any{
							"enabled":        false,
							"port":           8081,
							"host":           "127.0.0.1",
							"extra_flags":    []any{"--embedding"},
							"health_timeout": "30s",
						},
					},
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
