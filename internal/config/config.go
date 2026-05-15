package config

import (
	"fmt"
	"time"
)

type BudgetConfig struct {
	SystemPct      float64 `yaml:"system_pct"`
	ProjectPct     float64 `yaml:"project_pct"`
	MemoryPct      float64 `yaml:"memory_pct"`
	SkillsPct      float64 `yaml:"skills_pct"`
	InferenceFloor float64 `yaml:"inference_floor"`

	// AlwaysOnTokenCap is the warn threshold for the combined always-on
	// prompt surfaces (system prompt + AGENT.md + triggered skill
	// fragments + handoff Key Context) measured via the coarse
	// "4 chars ≈ 1 token" heuristic in
	// internal/harness/budget_allocator.go. When a per-role audit
	// reports TotalTokens exceeds this cap, the harness emits a
	// single `runtime/budget_exceeded/warn` event and otherwise
	// continues unmodified (audit-only; no prompt trimming and no Run
	// blocking). 0 disables the audit entirely.
	//
	// Default 4000 is sized to flag prompts that would leave less
	// than ~28k of the standard 32k context window for the inference
	// slot — enough headroom to prompt a curator pass without firing
	// spuriously on legitimate multi-skill retrievals.
	AlwaysOnTokenCap int `yaml:"always_on_token_cap"`
}

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

type TracingConfig struct {
	Enabled       bool `yaml:"enabled"`
	MaxInputChars int  `yaml:"max_input_chars"`
	AsyncBuffer   int  `yaml:"async_buffer"`
	// NOTE: `dir` and `max_traces` were removed in the 2026-04-21
	// log-architecture refactor. The trace path is now derived from
	// the run-id (<workspace>/runs/<run-id>/traces.jsonl) and retention
	// is governed by RuntimeConfig.RunsRetentionMax / RunsRetentionDays.
	// yaml.v3 silently ignores unknown keys so pre-upgrade nobody.yaml
	// files still load.
}

type SandboxConfig struct {
	Mode            string   `yaml:"mode"`
	WorkspaceOnly   bool     `yaml:"workspace_only"` // reserved; SafeResolvePath enforces workspace-only
	EnvPassthrough  []string `yaml:"env_passthrough"`
	BwrapExtraRO    []string `yaml:"bwrap_extra_ro"`
	AuditAgentMD    bool     `yaml:"audit_agent_md"`
	MaxAgentMDBytes int      `yaml:"max_agent_md_bytes"`
}

// MemoryConfig is retained as migration material for future narrative memory
// storage.
type MemoryConfig struct {
	Enabled bool   `yaml:"enabled"`
	DBPath  string `yaml:"db_path"`
}

type OrchestratorCfg struct {
	MaxRetries     int    `yaml:"max_retries"`
	Planner        string `yaml:"planner"`
	MaxTransitions int    `yaml:"max_transitions"`
	MandatoryAudit bool   `yaml:"mandatory_audit"`
}

type Config struct {
	Model        ModelConfig     `yaml:"model"`
	Runtime      RuntimeConfig   `yaml:"runtime"`
	Confirm      ConfirmConfig   `yaml:"confirm"`
	Skills       SkillsConfig    `yaml:"skills"`
	Tracing      TracingConfig   `yaml:"tracing"`
	Sandbox      SandboxConfig   `yaml:"sandbox"`
	Memory       MemoryConfig    `yaml:"memory"`
	Orchestrator OrchestratorCfg `yaml:"orchestrator"`
	Context      ContextConfig   `yaml:"context"`

	// Backward-compatible aliases used by existing call sites.
	ModelLegacy   string        `yaml:"-"`
	Workspace     string        `yaml:"-"`
	MaxIterations int           `yaml:"-"`
	Temperature   float32       `yaml:"-"`
	NumCtx        int           `yaml:"-"`
	Timeout       time.Duration `yaml:"-"`
}

type SkillsConfig struct {
	Enabled             bool          `yaml:"enabled"`
	AgentMDPath         string        `yaml:"agent_md_path"`
	SkillsDir           string        `yaml:"skills_dir"`
	RetrievalTopK       int           `yaml:"retrieval_top_k"`
	RetrievalTimeout    time.Duration `yaml:"retrieval_timeout"`
	MaxRisk             string        `yaml:"max_risk"`
	MaxSkills           int           `yaml:"max_skills"`
	MaxTotalChars       int           `yaml:"max_total_chars"`
	MaxLoadsPerTurn     int           `yaml:"max_loads_per_turn"`
	AutoBudgetEnabled   *bool         `yaml:"auto_budget_enabled"`
	AutoBudgetPercent   float64       `yaml:"auto_budget_percent"`
	StrictWorkspaceRoot *bool         `yaml:"strict_workspace_root"`
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

	// StallThreshold is the maximum wall-clock gap between successive
	// RecordToken stamps before the per-Run stallWatchdog fires the
	// Run-local context.CancelFunc. Re-introduced 2026-04-20 after the
	// 2026-Q2 cleanup removed the original readers; see
	// internal/harness/timeout.go#stallWatchdog for enforcement and
	// internal/harness/harness.go#runWithStallGuard for wiring.
	// Default 180s; 0 disables detection entirely; negative values are
	// treated as 0 (disabled) by the watchdog's `threshold <= 0` guard
	// rather than a loader patcher — this keeps the "zero means off"
	// semantics local to the enforcement site.
	StallThreshold time.Duration `yaml:"stall_threshold"`

	// IterationTimeout was removed in the 2026-Q2 code-cleanup when the
	// TimeoutManager readers that consumed it turned out to never run in
	// production. YAML files that still carry `iteration_timeout` parse
	// fine — gopkg.in/yaml.v3 silently ignores unknown struct fields —
	// so no downgrade migration is needed.
	ProgressIdleThreshold   time.Duration `yaml:"progress_idle_threshold"`
	ShowProgress            bool          `yaml:"show_progress"`
	StreamOutput            bool          `yaml:"stream_output"`
	VerboseLogs             bool          `yaml:"verbose_logs"`
	EnableDiagnose          bool          `yaml:"enable_diagnose"`
	EnableAsk               bool          `yaml:"enable_ask"`
	Budget                  BudgetConfig  `yaml:"budget"`
	StateDir                string        `yaml:"state_dir"`
	StateTTLDays            int           `yaml:"state_ttl_days"`
	StateArchiveKeep        int           `yaml:"state_archive_keep"`
	GracefulShutdownTimeout time.Duration `yaml:"graceful_shutdown_timeout"`

	// MaxAttempts is the upper bound of the harness outer retry loop:
	// Harness.Run wraps a single attempt (runOnce) in
	// `for attempt := 1..MaxAttempts` and carries the previous
	// attempt's `.handoff/current.md` into the next attempt's Resume
	// Context. Clamped to [1, 5] at Run time (>5 is capped; ≤0 falls
	// back to 1 == legacy single-attempt behaviour). Default 3.
	//
	// Orthogonal to OrchestratorConfig.MaxRetries: that governs intra-
	// attempt self-healing retries (no handoff replay, no context
	// reset), while MaxAttempts is the outer GAN-style retry with full
	// handoff carry-over. Worst-case agent invocations per Run are
	// roughly MaxAttempts × Orchestrator.MaxRetries; operators who
	// raise MaxAttempts should consider tuning Orchestrator.MaxRetries
	// down to keep total wall-clock bounded.
	//
	// Retained as migration material for future narrative run loops.
	MaxAttempts int `yaml:"max_attempts"`

	// Tool output spill: when a shell/run invocation produces more than
	// ToolOutputSpillBytes bytes of combined stdout/stderr, the tool writes
	// the full output into ToolOutputDir and returns a head+tail summary with
	// a marker pointing the agent at the spill file (read via `fs action=read`).
	// ToolOutputDir is interpreted relative to the workspace. Setting
	// ToolOutputSpillBytes to 0 disables spilling entirely.
	ToolOutputSpillBytes int    `yaml:"tool_output_spill_bytes"`
	ToolOutputDir        string `yaml:"tool_output_dir"`

	// ---- Retention (Project C log-architecture refactor) ----
	// Tenant layout (<workspace>/runs/<run-id>/) collapses retention to a
	// single directory-level sweep. Semantics for every *Retention* field:
	//   - positive value: apply retention policy as documented.
	//   - 0:              disable retention (the directory grows unbounded;
	//                     sweeper is a no-op for that target).
	//   - negative value: treated as 0 by the loader patcher.
	//
	// RunsRetentionMax caps the number of <workspace>/runs/<run-id>/
	// directories. When exceeded, the oldest (by lexicographic run-id,
	// which equals chronological order for the run-YYYYMMDD-* naming
	// scheme) are os.RemoveAll'd. 0 disables.
	RunsRetentionMax int `yaml:"runs_retention_max"`
	// RunsRetentionDays removes run directories whose mtime is older
	// than now-N*24h. OR-semantics with RunsRetentionMax (a dir that
	// violates either bound is deleted). 0 disables the age cut.
	RunsRetentionDays int `yaml:"runs_retention_days"`
	// RunLogRotateMaxBytes: when <workspace>/logs/run.log.md exceeds this
	// size, it is renamed to run.log.md.1 (overwriting any previous .1)
	// and a fresh run.log.md is started. Single-level rotation — we do
	// not keep a chain of .1 .2 .3 because the full trace of any given
	// run is reconstructable from runs/<run-id>/traces.jsonl anyway.
	RunLogRotateMaxBytes int `yaml:"run_log_rotate_max_bytes"`

	// NOTE: TracesRetentionMaxFiles / TracesRetentionDays /
	// HandoffSessionsRetentionMax / ToolOutputRetentionHours were retired
	// in the 2026-04-21 log-architecture refactor (tenant layout
	// subsumes the three sub-directories). Pre-upgrade YAML files that
	// still carry those keys parse fine — yaml.v3 silently ignores
	// unknown struct fields.
}

type ConfirmConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Shell     ShellRule     `yaml:"shell"`
	FileWrite FileWriteRule `yaml:"file_write"`
}

type ShellRule struct {
	Mode              string   `yaml:"mode"` // always|dangerous_only|never
	DangerousPatterns []string `yaml:"dangerous_patterns"`
}

type FileWriteRule struct {
	Mode string `yaml:"mode"` // always|new_file_only|never
}

// ContextConfig is retained as migration material for future narrative context
// management.
type ContextConfig struct {
	Strategy       string               `yaml:"strategy"`
	HandoffDir     string               `yaml:"handoff_dir"`
	ResetOnly      ResetOnlyParams      `yaml:"reset_only"`
	Hybrid         HybridParams         `yaml:"hybrid"`
	CompactionOnly CompactionOnlyParams `yaml:"compaction_only"`
}

type ResetOnlyParams struct {
	// TriggerPct is the fraction of the 'inference' budget slot that triggers
	// a reset. Default 0.85 via DefaultConfig.
	TriggerPct float64 `yaml:"trigger_pct"`
	// TokenThreshold is DEPRECATED: non-zero overrides TriggerPct with a
	// warning log. Kept for backward compatibility with existing YAML.
	TokenThreshold int `yaml:"token_threshold"`
	MaxIterations  int `yaml:"max_iterations_per_session"`
	MaxAttempts    int `yaml:"max_attempts_per_task"`
}

type HybridParams struct {
	// CompactionPct triggers middleware summarisation; default 0.70.
	CompactionPct float64 `yaml:"compaction_pct"`
	// ResetPct triggers session end + handoff; default 0.90.
	ResetPct float64 `yaml:"reset_pct"`
	// TokenThreshold is DEPRECATED: non-zero overrides both pcts with a warning.
	TokenThreshold int             `yaml:"token_threshold"`
	MaxIterations  int             `yaml:"max_iterations_per_session"`
	Reduction      ReductionParams `yaml:"reduction"`
}

// ReductionParams mirrors the YAML-serialisable subset of reduction.Config.
// Backend/RootDir/TokenCounter are not in YAML — they are injected by the
// Harness at strategy construction time (see internal/harness/harness.go).
type ReductionParams struct {
	Enabled           bool  `yaml:"enabled"`
	MaxLengthForTrunc int   `yaml:"max_length_for_trunc"`
	MaxTokensForClear int64 `yaml:"max_tokens_for_clear"`
}

type CompactionOnlyParams struct {
	// CompactionPct triggers middleware summarisation; default 0.70.
	CompactionPct float64             `yaml:"compaction_pct"`
	MaxIterations int                 `yaml:"max_iterations_per_session"`
	Summarization SummarizationParams `yaml:"summarization"`
}

// SummarizationParams mirrors the YAML-serialisable subset of
// summarization.Config. Model (a model.BaseChatModel) and TranscriptFilePath
// are injected by the Harness at strategy construction time.
type SummarizationParams struct {
	Enabled                  bool `yaml:"enabled"`
	TriggerContextTokens     int  `yaml:"trigger_context_tokens"`
	TriggerContextMessages   int  `yaml:"trigger_context_messages"`
	PreserveRecentUserTokens int  `yaml:"preserve_recent_user_tokens"`
}

// ManagedConfig carries the parsed provider_opts.llamacpp.managed sub-tree
// consumed by internal/inference/llamacpp/manager.go when
// provider_opts.llamacpp.lifecycle == "managed". The two pointer fields
// (NGL, Ctx) carry presence information so harness.New can apply
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
// NGL and Ctx are returned as nil pointers when absent; callers (harness)
// distinguish "operator did not set" from "operator set 0" by checking
// the pointer.
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

func boolPtr(b bool) *bool { return &b }

func BoolPtr(b bool) *bool { return &b }

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
			Workspace:             "./workspace",
			PromptsDir:            "prompts",
			MaxIterations:         15,
			Temperature:           0.7,
			NumCtx:                8192,
			EstimatedSpeed:        12,
			ShellTimeout:          60 * time.Second,
			StallThreshold:        180 * time.Second,
			ProgressIdleThreshold: 3 * time.Second,
			ShowProgress:          true,
			StreamOutput:          false,
			VerboseLogs:           true,
			EnableDiagnose:        false,
			EnableAsk:             false,
			Budget: BudgetConfig{
				SystemPct:        0.20,
				ProjectPct:       0.10,
				MemoryPct:        0.10,
				SkillsPct:        0.10,
				InferenceFloor:   0.50,
				AlwaysOnTokenCap: 4000,
			},
			StateDir:                ".state",
			StateTTLDays:            7,
			StateArchiveKeep:        5,
			GracefulShutdownTimeout: 30 * time.Second,
			MaxAttempts:             3,
			ToolOutputSpillBytes:    8 * 1024,
			ToolOutputDir:           ".nobody/tool_output",
			RunsRetentionMax:        200,
			RunsRetentionDays:       30,
			RunLogRotateMaxBytes:    5 * 1024 * 1024,
		},
		Confirm: ConfirmConfig{
			Enabled: false,
			Shell: ShellRule{
				Mode:              "dangerous_only",
				DangerousPatterns: []string{"rm -rf", "sudo", "dd ", "mkfs"},
			},
			FileWrite: FileWriteRule{
				Mode: "new_file_only",
			},
		},
		Skills: SkillsConfig{
			Enabled:             true,
			AgentMDPath:         "prompts/AGENT.md",
			SkillsDir:           "skills",
			RetrievalTopK:       5,
			RetrievalTimeout:    3 * time.Second,
			MaxRisk:             "medium",
			MaxSkills:           2,
			MaxTotalChars:       2400,
			MaxLoadsPerTurn:     3,
			AutoBudgetEnabled:   boolPtr(true),
			AutoBudgetPercent:   0.10,
			StrictWorkspaceRoot: boolPtr(true),
		},
		Tracing: TracingConfig{
			Enabled:       false,
			MaxInputChars: 4000,
			AsyncBuffer:   100,
		},
		Sandbox: SandboxConfig{
			Mode:            "auto",
			WorkspaceOnly:   true,
			EnvPassthrough:  []string{"PATH", "HOME", "TERM", "LANG", "GOPATH", "GOROOT", "GOBIN", "TMPDIR"},
			AuditAgentMD:    true,
			MaxAgentMDBytes: 10240,
		},
		Memory: MemoryConfig{
			Enabled: true,
			DBPath:  ".nobody/memory.db",
		},
		Orchestrator: OrchestratorCfg{
			MaxRetries:     3,
			Planner:        "auto",
			MaxTransitions: 10,
		},
		Context: ContextConfig{
			Strategy:   "reset_only",
			HandoffDir: ".handoff",
			ResetOnly: ResetOnlyParams{
				TriggerPct:     0.85,
				TokenThreshold: 12000,
				MaxIterations:  8,
				MaxAttempts:    3,
			},
			Hybrid: HybridParams{
				CompactionPct: 0.70,
				ResetPct:      0.90,
				MaxIterations: 20,
				Reduction: ReductionParams{
					Enabled:           false,
					MaxLengthForTrunc: 16000,
					MaxTokensForClear: 20000,
				},
			},
			CompactionOnly: CompactionOnlyParams{
				CompactionPct: 0.70,
				MaxIterations: 50,
				Summarization: SummarizationParams{
					Enabled:                  false,
					TriggerContextTokens:     80000,
					PreserveRecentUserTokens: 20000,
				},
			},
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

func BoolVal(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}
