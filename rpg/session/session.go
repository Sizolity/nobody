// Package session orchestrates the RPG beat pipeline using Eino's ReAct agent.
// The GM role (injected via role.GM) controls prompt generation, tool selection,
// and action suggestion. See rpg/gm/ for concrete GM implementations.
//
// Session itself is pure orchestration — no LLM logic, no prompt construction,
// no effect application. Each beat:
//
//  1. Loads the world (+ disclosure if fog enabled)
//  2. Asks the GM for the disclosed toolset and the system prompt
//  3. Runs the Eino ReAct agent with the resulting tools + prompt
//  4. Applies any pending effects via internal/world/runtime.ApplyEvent
//  5. Persists the updated world + disclosure
//  6. Asks the GM to suggest next-step ActionChoices for the PL
package session

import (
	"context"
	"fmt"
	"math/rand/v2"
	"path/filepath"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	worldmodel "github.com/sizolity/nobody/internal/world/model"
	worldruntime "github.com/sizolity/nobody/internal/world/runtime"
	"github.com/sizolity/nobody/internal/world/store"
	"github.com/sizolity/nobody/internal/world/view"
	"github.com/sizolity/nobody/rpg/fog"
	"github.com/sizolity/nobody/rpg/role"
	"github.com/sizolity/nobody/rpg/story"
	"github.com/sizolity/nobody/rpg/tools"
)

// Session manages a single RPG game session tied to one world.
type Session struct {
	gm         role.GM
	players    []role.Player
	store      *store.FileStore
	fogStore   *fog.Store
	storyStore *story.Store
	runtime    worldruntime.Runtime
	chatModel  model.ToolCallingChatModel
	rng        *rand.Rand
	maxStep    int
	fogEnabled bool
}

// Config holds parameters for creating a new Session.
type Config struct {
	GM            role.GM
	Players       []role.Player
	WorkspacePath string // root for all data; worlds, fog, and worldlines colocated under {WorkspacePath}/worlds/{worldID}/
	ChatModel     model.ToolCallingChatModel
	Rng           *rand.Rand
	MaxStep       int  // max tool-calling iterations per beat (default 10)
	FogEnabled    bool // enable progressive world disclosure (fog of war)
	// StoryEnabled toggles the WorldLine scheduler. When true, the session
	// loads worldlines.json at beat start, ticks them after player effects
	// apply, applies emitted events, and persists updated lines. Default off
	// keeps existing sessions unchanged.
	StoryEnabled bool
}

func New(cfg Config) (*Session, error) {
	if cfg.GM == nil {
		return nil, fmt.Errorf("GM is required")
	}
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
	worldsDir := filepath.Join(cfg.WorkspacePath, "worlds")
	sess := &Session{
		gm:         cfg.GM,
		players:    cfg.Players,
		store:      store.NewFileStore(cfg.WorkspacePath),
		fogStore:   fog.NewStore(worldsDir),
		runtime:    worldruntime.NewRuntime(),
		chatModel:  cfg.ChatModel,
		rng:        rng,
		maxStep:    maxStep,
		fogEnabled: cfg.FogEnabled,
	}
	if cfg.StoryEnabled {
		sess.storyStore = story.NewStore(worldsDir)
	}
	return sess, nil
}

// BeatInput is the user-facing input for running a beat.
type BeatInput struct {
	WorldID      string
	Action       role.PlayerAction
	RecentEvents int
}

// BeatOutput contains the results of running a beat.
type BeatOutput struct {
	World       worldmodel.World
	Narrative   string
	ToolEffects []worldmodel.Effect
	Choices     role.ActionChoices
}

// RunBeat executes a single narrative beat via the Eino ReAct agent.
func (s *Session) RunBeat(ctx context.Context, input BeatInput) (BeatOutput, error) {
	world, err := s.store.LoadSnapshot(ctx, input.WorldID)
	if err != nil {
		return BeatOutput{}, fmt.Errorf("load world: %w", err)
	}

	var disclosure fog.DisclosureState
	if s.fogEnabled {
		disclosure, err = s.fogStore.Load(input.WorldID)
		if err != nil {
			return BeatOutput{}, fmt.Errorf("load disclosure: %w", err)
		}
	}

	tc := &tools.ToolContext{World: world, Rng: s.rng}
	if s.fogEnabled {
		tc.Disclosure = &disclosure
	}
	invokableTools, err := s.gm.Tools(tc)
	if err != nil {
		return BeatOutput{}, fmt.Errorf("create tools: %w", err)
	}

	baseTools := make([]tool.BaseTool, len(invokableTools))
	for i, t := range invokableTools {
		baseTools[i] = t
	}

	// Fog filter is applied to the PL-facing world view; the GM still sees the
	// full world internally for narration consistency post-disclosure.
	visibleWorld := world
	if s.fogEnabled {
		visibleWorld = fog.FilterWorld(world, disclosure)
	}

	// Pre-render the three projections the GM consumes — keeps the GM free of
	// raw model.World iteration.
	worldCtx := view.WorldDebugView{}.Render(visibleWorld)
	narrativeCtx := view.NarrativeView{}.Render(visibleWorld, view.NarrativeContextRequest{
		RecentEventLimit: input.RecentEvents,
	})
	var charCtxs []view.CharacterContext
	for _, p := range s.players {
		if p.CharacterID == "" {
			continue
		}
		if cc, err := (view.CharacterContextView{}).Render(visibleWorld, view.CharacterContextRequest{
			PerspectiveID: p.CharacterID,
		}); err == nil {
			charCtxs = append(charCtxs, cc)
		}
	}

	systemPrompt := s.gm.SystemPrompt(s.players, role.PromptOptions{
		WorldCtx:     worldCtx,
		NarrativeCtx: narrativeCtx,
		CharacterCtx: charCtxs,
		FogEnabled:   s.fogEnabled,
	})

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: s.chatModel,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: baseTools},
		MaxStep:          s.maxStep,
	})
	if err != nil {
		return BeatOutput{}, fmt.Errorf("create agent: %w", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt),
		schema.UserMessage(input.Action.Content),
	}

	response, err := agent.Generate(ctx, messages)
	if err != nil {
		return BeatOutput{}, fmt.Errorf("agent generate: %w", err)
	}

	effects := tc.GetPendingEffects()

	// Apply effects via runtime.ApplyEvent (covers all 17 effect kinds + rule
	// validation). The synthesized event captures the beat as a "note" sourced
	// from user input — concrete EventTypes like Move/StatsChanged are emitted
	// by world/system builders in future GMs.
	if len(effects) > 0 {
		toolEvent := worldmodel.WorldEvent{
			ID:          worldmodel.EventID(fmt.Sprintf("beat_%s_%d", input.WorldID, world.Clock.Sequence)),
			Type:        worldmodel.EventTypeNote,
			Source:      worldmodel.EventSourceUser,
			Description: "RPG beat tool effects",
			Effects:     effects,
		}
		world, err = s.runtime.ApplyEvent(world, toolEvent)
		if err != nil {
			return BeatOutput{}, fmt.Errorf("apply effects: %w", err)
		}
	}

	// Sequence increments once per beat regardless of effects, mirroring the
	// pre-refactor behavior (one tick == one PL action).
	world.Clock.Sequence++

	// === WorldLine scheduler ===
	// Runs after player tool-effects are applied and clock advances, so it
	// sees the post-action world. Emitted events flow through the same
	// runtime as player effects and persist with the same SaveSnapshot below.
	if s.storyStore != nil {
		lines, err := s.storyStore.Load(input.WorldID)
		if err != nil {
			return BeatOutput{}, fmt.Errorf("load worldlines: %w", err)
		}
		if len(lines) > 0 {
			tickOut, err := story.Tick(story.TickInput{
				World:     world,
				Lines:     lines,
				TimeScale: world.Clock.Current.Kind,
			}, s.rng)
			if err != nil {
				return BeatOutput{}, fmt.Errorf("worldline tick: %w", err)
			}
			for _, ev := range tickOut.Events {
				world, err = s.runtime.ApplyEvent(world, ev)
				if err != nil {
					return BeatOutput{}, fmt.Errorf("apply worldline event: %w", err)
				}
			}
			if err := s.storyStore.Save(input.WorldID, tickOut.UpdatedLines); err != nil {
				return BeatOutput{}, fmt.Errorf("save worldlines: %w", err)
			}
		}
	}

	if err := s.store.SaveSnapshot(ctx, world); err != nil {
		return BeatOutput{}, fmt.Errorf("save world: %w", err)
	}

	if s.fogEnabled {
		if err := s.fogStore.Save(input.WorldID, disclosure); err != nil {
			return BeatOutput{}, fmt.Errorf("save disclosure: %w", err)
		}
	}

	// Re-filter post-effect world so SuggestActions only sees what the PL can.
	visibleAfter := world
	if s.fogEnabled {
		visibleAfter = fog.FilterWorld(world, disclosure)
	}
	choices, err := s.gm.SuggestActions(ctx, visibleAfter, s.players, response.Content)
	if err != nil {
		return BeatOutput{}, fmt.Errorf("suggest actions: %w", err)
	}

	return BeatOutput{
		World:       world,
		Narrative:   response.Content,
		ToolEffects: effects,
		Choices:     choices,
	}, nil
}

// LoadWorld loads a world snapshot by ID.
func (s *Session) LoadWorld(ctx context.Context, worldID string) (worldmodel.World, error) {
	return s.store.LoadSnapshot(ctx, worldID)
}
