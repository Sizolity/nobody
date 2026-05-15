package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/narrative"
	"github.com/sizolity/nobody/internal/narrative/store"
)

func TestEngineRunBeatPersistsDraftEventsMemoriesAndGraph(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	world := narrative.World{ID: "w1", Title: "Signal City", Genre: "modern fantasy"}
	require.NoError(t, st.SaveWorld(ctx, world))
	require.NoError(t, st.SaveCharacter(ctx, "w1", narrative.Character{ID: "c1", Name: "Lin Xia", Role: "protagonist"}))
	require.NoError(t, st.SaveLocation(ctx, "w1", narrative.Location{ID: "l1", Name: "Old Station"}))
	require.NoError(t, st.SaveStoryGraph(ctx, "w1", narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Find the signal", CharacterIDs: []string{"c1"}, LocationID: "l1"},
		},
	}))

	eng := New(st, Agents{
		Director:   contextCheckingDirector{t: t},
		Writer:     fakeWriter{},
		Continuity: fakeContinuity{},
		Memory:     fakeMemory{},
		State:      fakeState{},
	})

	result, err := eng.RunBeat(ctx, RunBeatInput{WorldID: "w1", UserInput: "start"})
	require.NoError(t, err)
	require.Equal(t, "beat-1", result.BeatID)
	require.Equal(t, "draft-1", result.DraftID)
	require.Empty(t, result.ContinuityIssues)
	require.Equal(t, []string{"event-1"}, result.EventIDs)
	require.Equal(t, []string{"memory-1"}, result.MemoryIDs)
	require.Equal(t, "after-intro", result.CurrentNodeID)

	draft, err := st.LoadDraft(ctx, "w1", "draft-1")
	require.NoError(t, err)
	require.Contains(t, draft.Text, "station hummed")

	events, err := st.ListEvents(ctx, "w1")
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "scene_written", events[0].Type)

	memories, err := st.ListMemories(ctx, "w1")
	require.NoError(t, err)
	require.Len(t, memories, 1)
	require.Equal(t, "Lin Xia", memories[0].Subject)

	graph, err := st.LoadStoryGraph(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, "after-intro", graph.CurrentNodeID)
}

type contextCheckingDirector struct {
	t *testing.T
}

func (d contextCheckingDirector) PlanBeat(_ context.Context, bundle ContextBundle) (BeatPlan, error) {
	require.Equal(d.t, []narrative.Character{{ID: "c1", Name: "Lin Xia", Role: "protagonist"}}, bundle.Characters)
	require.Equal(d.t, []narrative.Location{{ID: "l1", Name: "Old Station"}}, bundle.Locations)
	return BeatPlan{BeatID: "beat-1", Objective: "Reveal the signal", TargetNodeID: "intro"}, nil
}

type fakeWriter struct{}

func (fakeWriter) WriteBeat(_ context.Context, _ ContextBundle, plan BeatPlan) (narrative.Draft, error) {
	return narrative.Draft{ID: "draft-1", BeatID: plan.BeatID, Title: "Opening", Kind: "scene", Text: "The station hummed under the city."}, nil
}

type fakeContinuity struct{}

func (fakeContinuity) Check(context.Context, ContextBundle, narrative.Draft) (ContinuityReport, error) {
	return ContinuityReport{}, nil
}

type fakeMemory struct{}

func (fakeMemory) Extract(_ context.Context, _ ContextBundle, draft narrative.Draft) (MemoryDelta, error) {
	return MemoryDelta{
		Events: []narrative.NarrativeEvent{
			{ID: "event-1", BeatID: draft.BeatID, Type: "scene_written", Summary: "Lin Xia finds the signal."},
		},
		Memories: []narrative.Memory{
			{ID: "memory-1", Type: "fact", Subject: "Lin Xia", Text: "Lin Xia knows the signal exists.", SourceEventID: "event-1"},
		},
	}, nil
}

type fakeState struct{}

func (fakeState) Apply(_ context.Context, bundle ContextBundle, _ BeatPlan, _ MemoryDelta) (StateDelta, error) {
	graph := bundle.Graph
	graph.CurrentNodeID = "after-intro"
	graph.Nodes = append(graph.Nodes, narrative.StoryNode{ID: "after-intro", Type: "scene", Status: "ready", Goal: "Choose what to do next"})
	return StateDelta{Graph: graph}, nil
}
