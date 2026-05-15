package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/internal/narrative"
)

func TestFileStoreRoundTripWorldGraphAndEntities(t *testing.T) {
	ctx := context.Background()
	s := NewFileStore(t.TempDir())

	world := narrative.World{ID: "w1", Title: "Signal City", Genre: "modern fantasy"}
	require.NoError(t, s.SaveWorld(ctx, world))

	gotWorld, err := s.LoadWorld(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, world, gotWorld)

	character := narrative.Character{ID: "c1", Name: "Lin Xia", Role: "protagonist"}
	require.NoError(t, s.SaveCharacter(ctx, "w1", character))

	gotCharacter, err := s.LoadCharacter(ctx, "w1", "c1")
	require.NoError(t, err)
	require.Equal(t, character, gotCharacter)

	location := narrative.Location{ID: "l1", Name: "Old Station", Description: "A sealed metro station."}
	require.NoError(t, s.SaveLocation(ctx, "w1", location))

	gotLocation, err := s.LoadLocation(ctx, "w1", "l1")
	require.NoError(t, err)
	require.Equal(t, location, gotLocation)

	graph := narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Find the first signal", CharacterIDs: []string{"c1"}, LocationID: "l1"},
		},
	}
	require.NoError(t, s.SaveStoryGraph(ctx, "w1", graph))

	gotGraph, err := s.LoadStoryGraph(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, graph, gotGraph)
}

func TestFileStoreAppendsEventsAndMemories(t *testing.T) {
	ctx := context.Background()
	s := NewFileStore(t.TempDir())
	world := narrative.World{ID: "w1", Title: "Signal City"}
	require.NoError(t, s.SaveWorld(ctx, world))

	event := narrative.NarrativeEvent{ID: "e1", BeatID: "b1", Type: "scene_written", Summary: "Lin Xia finds the signal."}
	require.NoError(t, s.AppendEvent(ctx, "w1", event))

	events, err := s.ListEvents(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, []narrative.NarrativeEvent{event}, events)

	memory := narrative.Memory{ID: "m1", Type: "fact", Subject: "Lin Xia", Text: "Lin Xia knows the signal exists.", Importance: 3}
	require.NoError(t, s.AppendMemory(ctx, "w1", memory))

	memories, err := s.ListMemories(ctx, "w1")
	require.NoError(t, err)
	require.Equal(t, []narrative.Memory{memory}, memories)
}

func TestFileStoreSavesDraftMarkdown(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStore(dir)
	world := narrative.World{ID: "w1", Title: "Signal City"}
	require.NoError(t, s.SaveWorld(ctx, world))

	draft := narrative.Draft{ID: "d1", BeatID: "b1", Title: "Opening Beat", Kind: "scene", Text: "The station hummed under the city."}
	require.NoError(t, s.SaveDraft(ctx, "w1", draft))

	raw, err := os.ReadFile(filepath.Join(dir, "narrative", "worlds", "w1", "drafts", "d1.md"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(raw), "---\n"))
	require.Contains(t, string(raw), "# Opening Beat")
	require.Contains(t, string(raw), "The station hummed under the city.")

	got, err := s.LoadDraft(ctx, "w1", "d1")
	require.NoError(t, err)
	require.Equal(t, draft, got)
}

func TestFileStoreRejectsUnsafePathIDs(t *testing.T) {
	ctx := context.Background()
	s := NewFileStore(t.TempDir())

	require.ErrorContains(t, s.SaveWorld(ctx, narrative.World{ID: "../escape", Title: "Bad"}), "unsafe id")
	require.NoError(t, s.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))
	require.ErrorContains(t, s.SaveCharacter(ctx, "w1", narrative.Character{ID: "../escape", Name: "Bad"}), "unsafe id")
	_, err := s.LoadDraft(ctx, "w1", "../escape")
	require.ErrorContains(t, err, "unsafe id")
}
