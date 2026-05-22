package config

import (
	"fmt"
	"net/url"
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
		patchLlamacppProviderOpts(cfg)
		patchModelDefaults(cfg)
		if err := validateProvider(cfg); err != nil {
			return nil, err
		}
		if err := validateToolChoice(cfg); err != nil {
			return nil, err
		}
		if err := validateResponseFormat(cfg); err != nil {
			return nil, err
		}
		if err := validateLlamacppGrammar(cfg); err != nil {
			return nil, err
		}
		if err := validateLlamacppLifecycle(cfg); err != nil {
			return nil, err
		}
		if err := reconcileManagedBaseURL(cfg); err != nil {
			return nil, err
		}
		if err := reconcileEmbeddingManagedBaseURL(cfg); err != nil {
			return nil, err
		}
	}
	cfg.applyCompatibilityMirror()
	return cfg, nil
}

// patchLlamacppProviderOpts backfills the llamacpp-specific knobs under
// cfg.Model.ProviderOpts["llamacpp"] when a YAML file omits the nested
// block or any individual key. It sources defaults from DefaultConfig so a
// loose subset of overrides keeps its siblings intact.
func patchLlamacppProviderOpts(cfg *Config) {
	def := DefaultConfig().Model.ProviderOpts["llamacpp"]
	if cfg.Model.ProviderOpts == nil {
		cfg.Model.ProviderOpts = map[string]map[string]any{}
	}
	cur := cfg.Model.ProviderOpts["llamacpp"]
	if cur == nil {
		cur = map[string]any{}
	}
	for _, key := range []string{"mode", "lifecycle", "probe_path", "reconnect_max", "reconnect_base", "api_key", "grammar"} {
		if _, ok := cur[key]; !ok {
			cur[key] = def[key]
		}
	}
	if _, ok := cur["embedding"]; !ok {
		cur["embedding"] = def["embedding"]
	}
	cfg.Model.ProviderOpts["llamacpp"] = cur
}

// patchModelDefaults backfills model-level defaults when the YAML omits
// a field entirely. Only handles Phase 2a additions (ToolChoice /
// ResponseFormat); other model fields are handled by yaml.v3's natural
// zero-value semantics combined with the DefaultConfig overlay LoadConfig
// already does.
func patchModelDefaults(cfg *Config) {
	if cfg.Model.ToolChoice == "" {
		cfg.Model.ToolChoice = "auto"
	}
	if cfg.Model.ResponseFormat == "" {
		cfg.Model.ResponseFormat = "text"
	}
}

func validateProvider(cfg *Config) error {
	switch cfg.Model.Provider {
	case "", "llamacpp":
		cfg.Model.Provider = "llamacpp"
		return nil
	case "ollama":
		return fmt.Errorf("model.provider=ollama is no longer supported; use llama.cpp via model.base_url=http://localhost:8080/v1")
	default:
		return fmt.Errorf("model.provider=%q is not supported (allowed: llamacpp)", cfg.Model.Provider)
	}
}

// validateToolChoice rejects unknown cfg.Model.ToolChoice values at
// load time. Empty string falls back to "auto" via patchModelDefaults
// which runs before this validator, so an empty string reaching here
// would only occur if a future caller bypasses patchModelDefaults.
func validateToolChoice(cfg *Config) error {
	switch cfg.Model.ToolChoice {
	case "auto", "forced", "forbidden":
		return nil
	default:
		return fmt.Errorf("model.tool_choice=%q is not supported (allowed: auto, forced, forbidden)", cfg.Model.ToolChoice)
	}
}

// validateResponseFormat rejects unknown cfg.Model.ResponseFormat values
// at load time. Same fallback rule as validateToolChoice.
func validateResponseFormat(cfg *Config) error {
	switch cfg.Model.ResponseFormat {
	case "text", "json_object":
		return nil
	default:
		return fmt.Errorf("model.response_format=%q is not supported (allowed: text, json_object)", cfg.Model.ResponseFormat)
	}
}

// validateLlamacppLifecycle rejects unknown lifecycle values at load
// time. The "managed" branch activates the llama.cpp process manager;
// "external" preserves sidecar behaviour. patchLlamacppProviderOpts seeds "external" so an
// untouched yaml never reaches this check; an explicit empty string in
// yaml is rejected so typos fail loudly per the same convention as
// validateLlamacppGrammar.
func validateLlamacppLifecycle(cfg *Config) error {
	if cfg.Model.ProviderOpts == nil {
		return nil
	}
	po, ok := cfg.Model.ProviderOpts["llamacpp"]
	if !ok || po == nil {
		return nil
	}
	raw, present := po["lifecycle"]
	if !present {
		return nil
	}
	v, isStr := raw.(string)
	if !isStr {
		return fmt.Errorf("provider_opts.llamacpp.lifecycle must be a string, got %T", raw)
	}
	switch v {
	case "":
		return fmt.Errorf(`provider_opts.llamacpp.lifecycle cannot be empty string; use "external" to disable managed mode`)
	case "external", "managed":
		return nil
	default:
		return fmt.Errorf("provider_opts.llamacpp.lifecycle=%q is not supported (allowed: external, managed)", v)
	}
}

// reconcileManagedBaseURL ensures that when lifecycle=managed, the
// model.base_url port matches provider_opts.llamacpp.managed.port. If
// base_url was not explicitly set in YAML (still equals DefaultConfig()
// value), it is rewritten to point at managed.port so operators can flip
// lifecycle without also editing base_url. If base_url was set
// explicitly to a different port, it is a hard error: silently
// overriding the operator's URL is more dangerous than asking them to
// reconcile. Only runs when lifecycle=managed; external preserves
// Phase 1 behaviour entirely.
func reconcileManagedBaseURL(cfg *Config) error {
	po, ok := cfg.Model.ProviderOpts["llamacpp"]
	if !ok || po == nil {
		return nil
	}
	if lc, _ := po["lifecycle"].(string); lc != "managed" {
		return nil
	}
	mc, err := ParseManagedConfig(cfg)
	if err != nil {
		return err
	}
	defaultBaseURL := DefaultConfig().Model.BaseURL
	wantURL := fmt.Sprintf("http://%s:%d/v1", mc.Host, mc.Port)
	if cfg.Model.BaseURL == defaultBaseURL || cfg.Model.BaseURL == "" {
		cfg.Model.BaseURL = wantURL
		return nil
	}
	gotURL, err := url.Parse(cfg.Model.BaseURL)
	if err != nil {
		return fmt.Errorf("model.base_url is unparseable: %w", err)
	}
	gotPort := gotURL.Port()
	if gotPort == "" {
		return fmt.Errorf("model.base_url=%q has no port; managed mode requires explicit port matching managed.port=%d", cfg.Model.BaseURL, mc.Port)
	}
	wantPortStr := fmt.Sprintf("%d", mc.Port)
	if gotPort != wantPortStr {
		return fmt.Errorf("model.base_url port=%s does not match provider_opts.llamacpp.managed.port=%d; set base_url to http://<host>:%d/v1 or change managed.port", gotPort, mc.Port, mc.Port)
	}
	return nil
}

func reconcileEmbeddingManagedBaseURL(cfg *Config) error {
	ec, err := ParseEmbeddingManagedConfig(cfg)
	if err != nil {
		return err
	}
	if !ec.Enabled {
		return nil
	}
	po, ok := cfg.Model.ProviderOpts["llamacpp"]
	if !ok || po == nil {
		return nil
	}
	embedding, _ := po["embedding"].(map[string]any)
	if embedding == nil {
		embedding = map[string]any{}
		po["embedding"] = embedding
	}
	wantURL := fmt.Sprintf("http://%s:%d/v1", ec.Host, ec.Port)
	if raw, ok := embedding["base_url"].(string); ok && raw != "" {
		gotURL, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("provider_opts.llamacpp.embedding.base_url is unparseable: %w", err)
		}
		gotPort := gotURL.Port()
		if gotPort != fmt.Sprintf("%d", ec.Port) {
			return fmt.Errorf("provider_opts.llamacpp.embedding.base_url port=%s does not match embedding.managed.port=%d", gotPort, ec.Port)
		}
		return nil
	}
	embedding["base_url"] = wantURL
	return nil
}

// validateLlamacppGrammar rejects clearly malformed
// provider_opts.llamacpp.grammar values. "off"/"auto" are explicit
// known modes; any other non-empty string is treated as a custom
// GBNF and pushed to llama-server as source of truth (we don't parse
// GBNF here, per spec ADR-5).
//
// patchLlamacppProviderOpts runs before this validator and backfills
// the missing-key case to "off" (using the DefaultConfig value), so a
// missing key never reaches us. An explicit `grammar: ""` in YAML
// survives the patch and is rejected here per spec §4.4: empty string
// is ambiguous (off vs operator typo) and we follow Phase 1's
// fail-loudly convention rather than silently coerce. A non-string
// value (e.g. `grammar: 123`) is also rejected so YAML type errors
// surface at load time.
func validateLlamacppGrammar(cfg *Config) error {
	if cfg.Model.ProviderOpts == nil {
		return nil
	}
	po, ok := cfg.Model.ProviderOpts["llamacpp"]
	if !ok || po == nil {
		return nil
	}
	raw, present := po["grammar"]
	if !present {
		return nil
	}
	g, isStr := raw.(string)
	if !isStr {
		return fmt.Errorf("provider_opts.llamacpp.grammar must be a string, got %T", raw)
	}
	if g == "" {
		return fmt.Errorf(`provider_opts.llamacpp.grammar cannot be empty string; use "off" to disable`)
	}
	return nil
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
