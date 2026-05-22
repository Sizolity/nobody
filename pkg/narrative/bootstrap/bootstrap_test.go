package bootstrap_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/bootstrap"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

func TestCreateWorldWritesSeedData(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())

	err := bootstrap.CreateWorld(ctx, st, bootstrap.Seed{
		World: narrative.World{ID: "w1", Title: "Signal City", Genre: "modern fantasy"},
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
	})
	require.NoError(t, err)

	world, err := st.LoadWorld(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, "Signal City", world.Title)

	character, err := st.LoadCharacter(ctx, "w1", "c1")
	require.NoError(t, err)
	require.Equal(t, "Lin Xia", character.Name)

	location, err := st.LoadLocation(ctx, "w1", "l1")
	require.NoError(t, err)
	require.Equal(t, "Old Station", location.Name)

	graph, err := st.LoadStoryGraph(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, "intro", graph.CurrentNodeID)
	require.Equal(t, []narrative.StoryNode{{
		ID:           "intro",
		Type:         "scene",
		Status:       "ready",
		Goal:         "Find the first signal",
		CharacterIDs: []string{"c1"},
		LocationID:   "l1",
	}}, graph.Nodes)
}

func TestCreateWorldRejectsMissingReferencedCharacterBeforeWriting(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())

	err := bootstrap.CreateWorld(ctx, st, bootstrap.Seed{
		World: narrative.World{ID: "w1", Title: "Signal City"},
		InitialNode: narrative.StoryNode{
			ID:           "intro",
			Type:         "scene",
			Status:       "ready",
			Goal:         "Find the first signal",
			CharacterIDs: []string{"missing"},
		},
	})
	require.ErrorContains(t, err, `character_id "missing"`)

	_, loadErr := st.LoadWorld(ctx, "w1")
	require.Error(t, loadErr)
}

func TestCreateWorldRejectsUnsafeIDsBeforeWriting(t *testing.T) {
	ctx := context.Background()
	st := store.NewFileStore(t.TempDir())

	err := bootstrap.CreateWorld(ctx, st, bootstrap.Seed{
		World: narrative.World{ID: "w1", Title: "Bad"},
		Characters: []narrative.Character{
			{ID: "../escape", Name: "Bad"},
		},
		InitialNode: narrative.StoryNode{
			ID:     "intro",
			Type:   "scene",
			Status: "ready",
			Goal:   "Start",
		},
	})
	require.ErrorContains(t, err, "unsafe id")

	_, loadErr := st.LoadWorld(ctx, "w1")
	require.Error(t, loadErr)
}
