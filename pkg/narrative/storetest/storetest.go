// Package storetest provides conformance tests for implementations of the
// public narrative store.Store interface.
package storetest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

type Factory func(t testing.TB) store.Store

func RunStoreContract(t *testing.T, newStore Factory) {
	t.Helper()

	t.Run("world graph entities round trip", func(t *testing.T) {
		ctx := context.Background()
		st := newStore(t)

		world := narrative.World{ID: "w1", Title: "Signal City", Genre: "modern fantasy"}
		require.NoError(t, st.SaveWorld(ctx, world))
		gotWorld, err := st.LoadWorld(ctx, "w1")
		require.NoError(t, err)
		require.Equal(t, world, gotWorld)

		character := narrative.Character{ID: "c1", Name: "Lin Xia", Role: "protagonist"}
		require.NoError(t, st.SaveCharacter(ctx, "w1", character))
		gotCharacter, err := st.LoadCharacter(ctx, "w1", "c1")
		require.NoError(t, err)
		require.Equal(t, character, gotCharacter)

		location := narrative.Location{ID: "l1", Name: "Old Station"}
		require.NoError(t, st.SaveLocation(ctx, "w1", location))
		gotLocation, err := st.LoadLocation(ctx, "w1", "l1")
		require.NoError(t, err)
		require.Equal(t, location, gotLocation)

		graph := narrative.StoryGraph{
			CurrentNodeID: "intro",
			Nodes: []narrative.StoryNode{
				{ID: "intro", Type: "scene", Status: "ready", Goal: "Find the signal", CharacterIDs: []string{"c1"}, LocationID: "l1"},
			},
		}
		require.NoError(t, st.SaveStoryGraph(ctx, "w1", graph))
		gotGraph, err := st.LoadStoryGraph(ctx, "w1")
		require.NoError(t, err)
		require.Equal(t, graph, gotGraph)
	})

	t.Run("events memories and drafts round trip", func(t *testing.T) {
		ctx := context.Background()
		st := newStore(t)
		require.NoError(t, st.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))

		event := narrative.NarrativeEvent{ID: "e1", BeatID: "b1", Type: "scene_written", Summary: "Lin Xia finds the signal."}
		require.NoError(t, st.AppendEvent(ctx, "w1", event))
		events, err := st.ListEvents(ctx, "w1")
		require.NoError(t, err)
		require.Equal(t, []narrative.NarrativeEvent{event}, events)

		memory := narrative.Memory{ID: "m1", Type: "fact", Subject: "Signal", Text: "The signal is below the station.", Importance: 4}
		require.NoError(t, st.AppendMemory(ctx, "w1", memory))
		memories, err := st.ListMemories(ctx, "w1")
		require.NoError(t, err)
		require.Equal(t, []narrative.Memory{memory}, memories)

		draft := narrative.Draft{ID: "d1", BeatID: "b1", Title: "Opening", Kind: "scene", Text: "The station hummed."}
		require.NoError(t, st.SaveDraft(ctx, "w1", draft))
		gotDraft, err := st.LoadDraft(ctx, "w1", "d1")
		require.NoError(t, err)
		require.Equal(t, draft, gotDraft)
	})

	t.Run("empty logs list as nil", func(t *testing.T) {
		ctx := context.Background()
		st := newStore(t)
		require.NoError(t, st.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))

		events, err := st.ListEvents(ctx, "w1")
		require.NoError(t, err)
		require.Nil(t, events)

		memories, err := st.ListMemories(ctx, "w1")
		require.NoError(t, err)
		require.Nil(t, memories)
	})

	t.Run("invalid writes fail", func(t *testing.T) {
		ctx := context.Background()
		st := newStore(t)
		require.ErrorContains(t, st.SaveWorld(ctx, narrative.World{ID: "w1"}), "world.title")
		require.NoError(t, st.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))
		require.ErrorContains(t, st.SaveCharacter(ctx, "w1", narrative.Character{ID: "c1"}), "character.name")
		require.ErrorContains(t, st.AppendEvent(ctx, "w1", narrative.NarrativeEvent{ID: "e1"}), "event.beat_id")
		require.ErrorContains(t, st.SaveDraft(ctx, "w1", narrative.Draft{ID: "d1"}), "draft.beat_id")
	})
}
