package engine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/engine"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

func TestPublicEngineRunsBeat(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	require.NoError(t, st.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))
	require.NoError(t, st.SaveStoryGraph(ctx, "w1", narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Start the story"},
		},
	}))

	e := engine.New(st, engine.Agents{
		Director:   fixedDirector{},
		Writer:     fixedWriter{},
		Continuity: fixedContinuity{},
		Memory:     fixedMemory{},
		State:      fixedState{},
	})

	result, err := e.RunBeat(ctx, engine.RunBeatInput{WorldID: "w1", UserInput: "Begin"})
	require.NoError(t, err)
	require.Equal(t, "beat-1", result.BeatID)
	require.Equal(t, "draft-1", result.DraftID)
}

type fixedDirector struct{}

func (fixedDirector) PlanBeat(context.Context, engine.ContextBundle) (engine.BeatPlan, error) {
	return engine.BeatPlan{BeatID: "beat-1", Objective: "Open the scene", TargetNodeID: "intro"}, nil
}

type fixedWriter struct{}

func (fixedWriter) WriteBeat(_ context.Context, _ engine.ContextBundle, plan engine.BeatPlan) (narrative.Draft, error) {
	return narrative.Draft{ID: "draft-1", BeatID: plan.BeatID, Title: "Opening", Kind: "scene", Text: "The station hummed."}, nil
}

type fixedContinuity struct{}

func (fixedContinuity) Check(context.Context, engine.ContextBundle, narrative.Draft) (engine.ContinuityReport, error) {
	return engine.ContinuityReport{}, nil
}

type fixedMemory struct{}

func (fixedMemory) Extract(_ context.Context, _ engine.ContextBundle, draft narrative.Draft) (engine.MemoryDelta, error) {
	return engine.MemoryDelta{
		Events: []narrative.NarrativeEvent{
			{ID: "event-1", BeatID: draft.BeatID, Type: "scene_written", Summary: "Opening written."},
		},
		Memories: []narrative.Memory{
			{ID: "memory-1", Type: "fact", Subject: "Station", Text: "The station hums.", SourceEventID: "event-1"},
		},
	}, nil
}

type fixedState struct{}

func (fixedState) Apply(_ context.Context, bundle engine.ContextBundle, _ engine.BeatPlan, _ engine.MemoryDelta) (engine.StateDelta, error) {
	return engine.StateDelta{Graph: bundle.Graph}, nil
}
