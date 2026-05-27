package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type LoadOptions struct {
	ConfigPath string
}

func LoadConfig(opts LoadOptions) (*Config, error) {
	cfg := DefaultConfig()
	path := resolveConfigPath(opts.ConfigPath)
	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
		const maxConfigSize = 1 << 20 // 1 MiB
		if len(raw) > maxConfigSize {
			return nil, fmt.Errorf("config file too large (%d bytes, max %d)", len(raw), maxConfigSize)
		}
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("parse config file: %w", err)
		}
		patchModelDefaults(cfg)
		if err := validateToolChoice(cfg); err != nil {
			return nil, err
		}
		if err := validateResponseFormat(cfg); err != nil {
			return nil, err
		}
	}
	cfg.applyCompatibilityMirror()
	return cfg, nil
}

// patchModelDefaults backfills model-level defaults when the YAML omits
// a field entirely.
func patchModelDefaults(cfg *Config) {
	if cfg.Model.ToolChoice == "" {
		cfg.Model.ToolChoice = "auto"
	}
	if cfg.Model.ResponseFormat == "" {
		cfg.Model.ResponseFormat = "text"
	}
}

func validateToolChoice(cfg *Config) error {
	switch cfg.Model.ToolChoice {
	case "auto", "forced", "forbidden":
		return nil
	default:
		return fmt.Errorf("model.tool_choice=%q is not supported (allowed: auto, forced, forbidden)", cfg.Model.ToolChoice)
	}
}

func validateResponseFormat(cfg *Config) error {
	switch cfg.Model.ResponseFormat {
	case "text", "json_object":
		return nil
	default:
		return fmt.Errorf("model.response_format=%q is not supported (allowed: text, json_object)", cfg.Model.ResponseFormat)
	}
}

type CLIOverrides struct {
	Model     string
	Workspace string
	BaseURL   string
}

func ApplyCLIOverrides(cfg *Config, o CLIOverrides) {
	if o.Model != "" {
		cfg.Model.Name = o.Model
	}
	if o.Workspace != "" {
		cfg.Runtime.Workspace = o.Workspace
	}
	if o.BaseURL != "" {
		cfg.Model.BaseURL = o.BaseURL
	}
	cfg.applyCompatibilityMirror()
}

func resolveConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if _, err := os.Stat("nobody.yaml"); err == nil {
		return "nobody.yaml"
	}
	return ""
}
