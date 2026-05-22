package snapshot_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/bootstrap"
	"github.com/sizolity/nobody/pkg/narrative/snapshot"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

func TestLoadWorldSnapshotIncludesCurrentNodeEntitiesAndHistory(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	require.NoError(t, bootstrap.CreateWorld(ctx, st, bootstrap.Seed{
		World: narrative.World{ID: "w1", Title: "Signal City"},
		Characters: []narrative.Character{
			{ID: "c1", Name: "Lin Xia", Role: "protagonist"},
		},
		Locations: []narrative.Location{
			{ID: "l1", Name: "Old Station"},
		},
		InitialNode: narrative.StoryNode{
			ID:           "intro",
			Type:         "scene",
			Status:       "ready",
			Goal:         "Find the first signal",
			CharacterIDs: []string{"c1"},
			LocationID:   "l1",
		},
	}))
	require.NoError(t, st.AppendEvent(ctx, "w1", narrative.NarrativeEvent{
		ID: "e1", BeatID: "b1", Type: "scene_written", Summary: "Lin Xia finds the station.",
	}))
	require.NoError(t, st.AppendMemory(ctx, "w1", narrative.Memory{
		ID: "m1", Type: "fact", Subject: "Station", Text: "The station hums.", Importance: 4,
	}))

	got, err := snapshot.LoadWorld(ctx, st, "w1")
	require.NoError(t, err)
	require.Equal(t, "Signal City", got.World.Title)
	require.Equal(t, "intro", got.Graph.CurrentNodeID)
	require.Equal(t, narrative.StoryNode{
		ID:           "intro",
		Type:         "scene",
		Status:       "ready",
		Goal:         "Find the first signal",
		CharacterIDs: []string{"c1"},
		LocationID:   "l1",
	}, got.CurrentNode)
	require.Equal(t, []narrative.Character{{ID: "c1", Name: "Lin Xia", Role: "protagonist"}}, got.Characters)
	require.Equal(t, []narrative.Location{{ID: "l1", Name: "Old Station"}}, got.Locations)
	require.Len(t, got.Events, 1)
	require.Len(t, got.Memories, 1)
}

func TestLoadWorldSnapshotReturnsEntityLoadErrors(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())
	require.NoError(t, st.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))
	require.NoError(t, st.SaveStoryGraph(ctx, "w1", narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Find the first signal", CharacterIDs: []string{"missing"}},
		},
	}))

	_, err := snapshot.LoadWorld(ctx, st, "w1")
	require.ErrorContains(t, err, "load character")
	require.ErrorContains(t, err, "missing")
}
