// Package config exposes shared runtime/model configuration for downstream
// product repositories.
package config

import internal "github.com/sizolity/nobody/internal/config"

type SamplingPreset = internal.SamplingPreset
type PresetsConfig = internal.PresetsConfig
type Config = internal.Config
type ModelConfig = internal.ModelConfig
type RuntimeConfig = internal.RuntimeConfig
type ManagedConfig = internal.ManagedConfig
type EmbeddingManagedConfig = internal.EmbeddingManagedConfig
type LoadOptions = internal.LoadOptions
type CLIOverrides = internal.CLIOverrides

func DefaultConfig() *Config {
	return internal.DefaultConfig()
}

func LoadConfig(opts LoadOptions) (*Config, error) {
	return internal.LoadConfig(opts)
}

func ApplyCLIOverrides(cfg *Config, opts CLIOverrides) {
	internal.ApplyCLIOverrides(cfg, opts)
}

func ParseManagedConfig(cfg *Config) (ManagedConfig, error) {
	return internal.ParseManagedConfig(cfg)
}

func ParseEmbeddingManagedConfig(cfg *Config) (EmbeddingManagedConfig, error) {
	return internal.ParseEmbeddingManagedConfig(cfg)
}
