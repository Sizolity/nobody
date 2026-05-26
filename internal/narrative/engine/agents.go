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
	World      narrative.World            `json:"world"`
	Graph      narrative.StoryGraph       `json:"graph"`
	Characters []narrative.Character      `json:"characters"`
	Locations  []narrative.Location       `json:"locations"`
	Events     []narrative.NarrativeEvent `json:"events"`
	Memories   []narrative.Memory         `json:"memories"`
	Input      string                     `json:"input"`
}

type BeatPlan struct {
	BeatID       string `json:"beat_id"`
	Objective    string `json:"objective"`
	TargetNodeID string `json:"target_node_id"`
}

type ContinuityReport struct {
	Issues []ContinuityIssue
}

type ContinuityIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
}

const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// HasCritical returns true if any issue has critical severity.
func (r ContinuityReport) HasCritical() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

type MemoryDelta struct {
	Events   []narrative.NarrativeEvent
	Memories []narrative.Memory
}

type StateDelta struct {
	Graph narrative.StoryGraph
}
