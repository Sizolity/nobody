package engine

import (
	"context"

	"github.com/sizolity/nobody/internal/narrative"
)

type Agents struct {
	Director   DirectorAgent
	Writer     SceneWriterAgent
	Continuity ContinuityAgent
	Memory     MemoryAgent
	State      StateAgent
}

type DirectorAgent interface {
	PlanBeat(context.Context, ContextBundle) (BeatPlan, error)
}

type SceneWriterAgent interface {
	WriteBeat(context.Context, ContextBundle, BeatPlan) (narrative.Draft, error)
}

type ContinuityAgent interface {
	Check(context.Context, ContextBundle, narrative.Draft) (ContinuityReport, error)
}

type MemoryAgent interface {
	Extract(context.Context, ContextBundle, narrative.Draft) (MemoryDelta, error)
}

type StateAgent interface {
	Apply(context.Context, ContextBundle, BeatPlan, MemoryDelta) (StateDelta, error)
}

type ContextBundle struct {
	World      narrative.World
	Graph      narrative.StoryGraph
	Characters []narrative.Character
	Locations  []narrative.Location
	Events     []narrative.NarrativeEvent
	Memories   []narrative.Memory
	Input      string
}

type BeatPlan struct {
	BeatID       string
	Objective    string
	TargetNodeID string
}

type ContinuityReport struct {
	Issues []ContinuityIssue
}

type ContinuityIssue struct {
	Code    string
	Summary string
}

type MemoryDelta struct {
	Events   []narrative.NarrativeEvent
	Memories []narrative.Memory
}

type StateDelta struct {
	Graph narrative.StoryGraph
}
