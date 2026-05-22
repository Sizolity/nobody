package enginetest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/engine"
	"github.com/sizolity/nobody/pkg/narrative/enginetest"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

func TestDeterministicAgentsRunBeat(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	require.NoError(t, st.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))
	require.NoError(t, st.SaveStoryGraph(ctx, "w1", narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Open"},
		},
	}))

	agents := enginetest.DeterministicAgents(enginetest.Script{
		Plan: engine.BeatPlan{BeatID: "beat-1", Objective: "Open the scene", TargetNodeID: "intro"},
		Draft: narrative.Draft{
			ID: "draft-1", BeatID: "beat-1", Title: "Opening", Kind: "scene", Text: "The station hummed.",
		},
		Events: []narrative.NarrativeEvent{
			{ID: "event-1", BeatID: "beat-1", Type: "scene_written", Summary: "Opening written."},
		},
		Memories: []narrative.Memory{
			{ID: "memory-1", Type: "fact", Subject: "Station", Text: "The station hums.", SourceEventID: "event-1"},
		},
	})

	result, err := engine.New(st, agents).RunBeat(ctx, engine.RunBeatInput{WorldID: "w1"})
	require.NoError(t, err)
	require.Equal(t, "beat-1", result.BeatID)
	require.Equal(t, "draft-1", result.DraftID)
	require.Equal(t, []string{"event-1"}, result.EventIDs)
	require.Equal(t, []string{"memory-1"}, result.MemoryIDs)
}

func TestDeterministicAgentsCanAdvanceGraph(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	require.NoError(t, st.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))
	require.NoError(t, st.SaveStoryGraph(ctx, "w1", narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Open"},
		},
	}))

	agents := enginetest.DeterministicAgents(enginetest.Script{
		Plan: engine.BeatPlan{BeatID: "beat-1", Objective: "Open the scene", TargetNodeID: "intro"},
		Draft: narrative.Draft{
			ID: "draft-1", BeatID: "beat-1", Title: "Opening", Kind: "scene", Text: "The station hummed.",
		},
		NextNode: &narrative.StoryNode{ID: "after-intro", Type: "scene", Status: "ready", Goal: "Choose next move"},
	})

	result, err := engine.New(st, agents).RunBeat(ctx, engine.RunBeatInput{WorldID: "w1"})
	require.NoError(t, err)
	require.Equal(t, "after-intro", result.CurrentNodeID)
}
