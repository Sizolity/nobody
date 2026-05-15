package engine

import (
	"context"
	"fmt"

	"github.com/sizolity/nobody/internal/narrative"
	"github.com/sizolity/nobody/internal/narrative/store"
)

type Engine struct {
	store  store.Store
	agents Agents
}

func New(st store.Store, agents Agents) *Engine {
	return &Engine{store: st, agents: agents}
}

type RunBeatInput struct {
	WorldID   string
	UserInput string
}

type RunBeatResult struct {
	BeatID           string
	DraftID          string
	ContinuityIssues []ContinuityIssue
	EventIDs         []string
	MemoryIDs        []string
	CurrentNodeID    string
}

func (e *Engine) RunBeat(ctx context.Context, input RunBeatInput) (RunBeatResult, error) {
	if err := e.validateAgents(); err != nil {
		return RunBeatResult{}, err
	}
	bundle, err := e.loadContext(ctx, input)
	if err != nil {
		return RunBeatResult{}, err
	}
	plan, err := e.agents.Director.PlanBeat(ctx, bundle)
	if err != nil {
		return RunBeatResult{}, err
	}
	draft, err := e.agents.Writer.WriteBeat(ctx, bundle, plan)
	if err != nil {
		return RunBeatResult{}, err
	}
	report, err := e.agents.Continuity.Check(ctx, bundle, draft)
	if err != nil {
		return RunBeatResult{}, err
	}
	delta, err := e.agents.Memory.Extract(ctx, bundle, draft)
	if err != nil {
		return RunBeatResult{}, err
	}
	state, err := e.agents.State.Apply(ctx, bundle, plan, delta)
	if err != nil {
		return RunBeatResult{}, err
	}

	if err := e.store.SaveDraft(ctx, input.WorldID, draft); err != nil {
		return RunBeatResult{}, err
	}
	eventIDs := make([]string, 0, len(delta.Events))
	for _, event := range delta.Events {
		if err := e.store.AppendEvent(ctx, input.WorldID, event); err != nil {
			return RunBeatResult{}, err
		}
		eventIDs = append(eventIDs, event.ID)
	}
	memoryIDs := make([]string, 0, len(delta.Memories))
	for _, memory := range delta.Memories {
		if err := e.store.AppendMemory(ctx, input.WorldID, memory); err != nil {
			return RunBeatResult{}, err
		}
		memoryIDs = append(memoryIDs, memory.ID)
	}
	if err := e.store.SaveStoryGraph(ctx, input.WorldID, state.Graph); err != nil {
		return RunBeatResult{}, err
	}

	return RunBeatResult{
		BeatID:           plan.BeatID,
		DraftID:          draft.ID,
		ContinuityIssues: report.Issues,
		EventIDs:         eventIDs,
		MemoryIDs:        memoryIDs,
		CurrentNodeID:    state.Graph.CurrentNodeID,
	}, nil
}

func (e *Engine) loadContext(ctx context.Context, input RunBeatInput) (ContextBundle, error) {
	world, err := e.store.LoadWorld(ctx, input.WorldID)
	if err != nil {
		return ContextBundle{}, err
	}
	graph, err := e.store.LoadStoryGraph(ctx, input.WorldID)
	if err != nil {
		return ContextBundle{}, err
	}
	characters, locations, err := e.loadCurrentNodeEntities(ctx, input.WorldID, graph)
	if err != nil {
		return ContextBundle{}, err
	}
	events, err := e.store.ListEvents(ctx, input.WorldID)
	if err != nil {
		return ContextBundle{}, err
	}
	memories, err := e.store.ListMemories(ctx, input.WorldID)
	if err != nil {
		return ContextBundle{}, err
	}
	return ContextBundle{
		World:      world,
		Graph:      graph,
		Characters: characters,
		Locations:  locations,
		Events:     events,
		Memories:   memories,
		Input:      input.UserInput,
	}, nil
}

func (e *Engine) loadCurrentNodeEntities(ctx context.Context, worldID string, graph narrative.StoryGraph) ([]narrative.Character, []narrative.Location, error) {
	var current narrative.StoryNode
	for _, node := range graph.Nodes {
		if node.ID == graph.CurrentNodeID {
			current = node
			break
		}
	}
	characters := make([]narrative.Character, 0, len(current.CharacterIDs))
	for _, id := range current.CharacterIDs {
		character, err := e.store.LoadCharacter(ctx, worldID, id)
		if err != nil {
			return nil, nil, err
		}
		characters = append(characters, character)
	}
	var locations []narrative.Location
	if current.LocationID != "" {
		location, err := e.store.LoadLocation(ctx, worldID, current.LocationID)
		if err != nil {
			return nil, nil, err
		}
		locations = append(locations, location)
	}
	return characters, locations, nil
}

func (e *Engine) validateAgents() error {
	switch {
	case e.store == nil:
		return fmt.Errorf("narrative engine store is required")
	case e.agents.Director == nil:
		return fmt.Errorf("director agent is required")
	case e.agents.Writer == nil:
		return fmt.Errorf("scene writer agent is required")
	case e.agents.Continuity == nil:
		return fmt.Errorf("continuity agent is required")
	case e.agents.Memory == nil:
		return fmt.Errorf("memory agent is required")
	case e.agents.State == nil:
		return fmt.Errorf("state agent is required")
	default:
		return nil
	}
}
