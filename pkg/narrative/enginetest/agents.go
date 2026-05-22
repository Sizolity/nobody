// Package enginetest provides deterministic test doubles for downstream
// products that need to exercise the narrative beat engine without an LLM.
package enginetest

import (
	"context"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/engine"
)

type Script struct {
	Plan             engine.BeatPlan
	Draft            narrative.Draft
	ContinuityIssues []engine.ContinuityIssue
	Events           []narrative.NarrativeEvent
	Memories         []narrative.Memory
	NextNode         *narrative.StoryNode
}

func DeterministicAgents(script Script) engine.Agents {
	return engine.Agents{
		Director:   director{script: script},
		Writer:     writer{script: script},
		Continuity: continuity{script: script},
		Memory:     memory{script: script},
		State:      state{script: script},
	}
}

type director struct {
	script Script
}

func (d director) PlanBeat(context.Context, engine.ContextBundle) (engine.BeatPlan, error) {
	return d.script.Plan, nil
}

type writer struct {
	script Script
}

func (w writer) WriteBeat(context.Context, engine.ContextBundle, engine.BeatPlan) (narrative.Draft, error) {
	return w.script.Draft, nil
}

type continuity struct {
	script Script
}

func (c continuity) Check(context.Context, engine.ContextBundle, narrative.Draft) (engine.ContinuityReport, error) {
	return engine.ContinuityReport{Issues: c.script.ContinuityIssues}, nil
}

type memory struct {
	script Script
}

func (m memory) Extract(context.Context, engine.ContextBundle, narrative.Draft) (engine.MemoryDelta, error) {
	return engine.MemoryDelta{
		Events:   m.script.Events,
		Memories: m.script.Memories,
	}, nil
}

type state struct {
	script Script
}

func (s state) Apply(_ context.Context, bundle engine.ContextBundle, _ engine.BeatPlan, _ engine.MemoryDelta) (engine.StateDelta, error) {
	graph := bundle.Graph
	if s.script.NextNode != nil {
		graph.CurrentNodeID = s.script.NextNode.ID
		graph.Nodes = append(graph.Nodes, *s.script.NextNode)
	}
	return engine.StateDelta{Graph: graph}, nil
}
