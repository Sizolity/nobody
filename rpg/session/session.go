// Package session orchestrates the RPG beat pipeline using Eino's ReAct agent.
// The LLM acts as a Dungeon Master: it receives world context, decides what
// happens narratively, and uses tools (roll dice, update state, lookup rules)
// to maintain deterministic game mechanics.
package session

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	worldmodel "github.com/sizolity/nobody/internal/world/model"
	"github.com/sizolity/nobody/internal/world/store"
	"github.com/sizolity/nobody/rpg/bridge"
	"github.com/sizolity/nobody/rpg/fog"
	"github.com/sizolity/nobody/rpg/tools"
)

// Session manages a single RPG game session tied to one world.
type Session struct {
	store     *store.FileStore
	fogStore  *fog.Store
	chatModel model.ToolCallingChatModel
	rng       *rand.Rand
	maxStep   int
	fogEnabled bool
}

// Config holds parameters for creating a new Session.
type Config struct {
	WorkspacePath string
	RPGDataDir    string // base dir for rpg-specific data (disclosure.json, etc); defaults to WorkspacePath + "/rpg"
	ChatModel     model.ToolCallingChatModel
	Rng           *rand.Rand
	MaxStep       int  // max tool-calling iterations per beat (default 10)
	FogEnabled    bool // enable progressive world disclosure (fog of war)
}

func New(cfg Config) (*Session, error) {
	if cfg.WorkspacePath == "" {
		return nil, fmt.Errorf("workspace path is required")
	}
	if cfg.ChatModel == nil {
		return nil, fmt.Errorf("chat model is required")
	}
	rng := cfg.Rng
	if rng == nil {
		rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}
	maxStep := cfg.MaxStep
	if maxStep <= 0 {
		maxStep = 10
	}
	rpgDataDir := cfg.RPGDataDir
	if rpgDataDir == "" {
		rpgDataDir = cfg.WorkspacePath + "/rpg"
	}
	return &Session{
		store:      store.NewFileStore(cfg.WorkspacePath),
		fogStore:   fog.NewStore(rpgDataDir),
		chatModel:  cfg.ChatModel,
		rng:        rng,
		maxStep:    maxStep,
		fogEnabled: cfg.FogEnabled,
	}, nil
}

// BeatInput is the user-facing input for running a beat.
type BeatInput struct {
	WorldID      string
	UserInput    string
	RecentEvents int
}

// BeatOutput contains the results of running a beat.
type BeatOutput struct {
	World       worldmodel.World
	Narrative   string
	ToolEffects []worldmodel.Effect
}

// RunBeat executes a single narrative beat via the Eino ReAct agent:
// 1. Load world from store
// 2. Adapt world → context (system prompt)
// 3. Create ReAct agent with RPG tools bound to world state
// 4. Agent generates narrative + executes tool calls automatically
// 5. Collect pending effects from tool executions
// 6. Apply effects to world state
// 7. Persist the updated world
func (s *Session) RunBeat(ctx context.Context, input BeatInput) (BeatOutput, error) {
	world, err := s.store.LoadSnapshot(ctx, input.WorldID)
	if err != nil {
		return BeatOutput{}, fmt.Errorf("load world: %w", err)
	}

	// Load disclosure state (fog of war)
	var disclosure fog.DisclosureState
	if s.fogEnabled {
		disclosure, err = s.fogStore.Load(input.WorldID)
		if err != nil {
			return BeatOutput{}, fmt.Errorf("load disclosure: %w", err)
		}
	}

	// Build tool context with optional disclosure reference
	tc := &tools.ToolContext{World: world, Rng: s.rng}
	if s.fogEnabled {
		tc.Disclosure = &disclosure
	}
	invokableTools, err := tools.NewInvokableTools(tc)
	if err != nil {
		return BeatOutput{}, fmt.Errorf("create tools: %w", err)
	}

	basTools := make([]tool.BaseTool, len(invokableTools))
	for i, t := range invokableTools {
		basTools[i] = t
	}

	// Apply fog filter: DM only sees revealed content
	visibleWorld := world
	if s.fogEnabled {
		visibleWorld = fog.FilterWorld(world, disclosure)
	}

	systemPrompt := buildSystemPrompt(visibleWorld, bridge.Options{
		UserInput:    input.UserInput,
		RecentEvents: input.RecentEvents,
	}, s.fogEnabled)

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: s.chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: basTools,
		},
		MaxStep: s.maxStep,
	})
	if err != nil {
		return BeatOutput{}, fmt.Errorf("create agent: %w", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(input.UserInput),
	}

	response, err := agent.Generate(ctx, messages)
	if err != nil {
		return BeatOutput{}, fmt.Errorf("agent generate: %w", err)
	}

	effects := tc.GetPendingEffects()
	world = applyEffects(world, effects)
	world.Clock.Sequence++

	if err := s.store.SaveSnapshot(ctx, world); err != nil {
		return BeatOutput{}, fmt.Errorf("save world: %w", err)
	}

	// Persist updated disclosure state (explore_knowledge tool may have mutated it)
	if s.fogEnabled {
		if err := s.fogStore.Save(input.WorldID, disclosure); err != nil {
			return BeatOutput{}, fmt.Errorf("save disclosure: %w", err)
		}
	}

	return BeatOutput{
		World:       world,
		Narrative:   response.Content,
		ToolEffects: effects,
	}, nil
}

// LoadWorld loads a world snapshot by ID.
func (s *Session) LoadWorld(ctx context.Context, worldID string) (worldmodel.World, error) {
	return s.store.LoadSnapshot(ctx, worldID)
}
