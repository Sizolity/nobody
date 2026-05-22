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

func TestFileStoreRejectsInvalidEventsAndMemories(t *testing.T) {
	ctx := context.Background()
	s := NewFileStore(t.TempDir())
	require.NoError(t, s.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))

	require.ErrorContains(t, s.AppendEvent(ctx, "w1", narrative.NarrativeEvent{ID: "e1"}), "event.beat_id")
	require.ErrorContains(t, s.AppendMemory(ctx, "w1", narrative.Memory{ID: "m1"}), "memory.type")
	require.ErrorContains(t, s.SaveDraft(ctx, "w1", narrative.Draft{ID: "d1"}), "draft.beat_id")
}

func TestFileStoreReturnsErrorForCorruptJSONL(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStore(dir)
	require.NoError(t, s.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))

	path := filepath.Join(dir, "narrative", "worlds", "w1", "events.jsonl")
	require.NoError(t, os.WriteFile(path, []byte("{bad-json}\n"), 0o644))

	_, err := s.ListEvents(ctx, "w1")
	require.Error(t, err)
}

func TestFileStoreValidatesLoadedDocuments(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStore(dir)
	worldDir := filepath.Join(dir, "narrative", "worlds", "w1")
	require.NoError(t, os.MkdirAll(filepath.Join(worldDir, "characters"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(worldDir, "locations"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(worldDir, "drafts"), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "world.json"), []byte(`{"id":"w1"}`), 0o644))
	_, err := s.LoadWorld(ctx, "w1")
	require.ErrorContains(t, err, "world.title")

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "characters", "c1.json"), []byte(`{"id":"c1"}`), 0o644))
	_, err = s.LoadCharacter(ctx, "w1", "c1")
	require.ErrorContains(t, err, "character.name")

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "locations", "l1.json"), []byte(`{"id":"l1"}`), 0o644))
	_, err = s.LoadLocation(ctx, "w1", "l1")
	require.ErrorContains(t, err, "location.name")

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "story_graph.json"), []byte(`{"current_node_id":"missing","nodes":[]}`), 0o644))
	_, err = s.LoadStoryGraph(ctx, "w1")
	require.ErrorContains(t, err, "current_node_id")
}

func TestFileStoreRejectsLoadedDocumentIDsThatDoNotMatchPath(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStore(dir)
	worldDir := filepath.Join(dir, "narrative", "worlds", "w1")
	require.NoError(t, os.MkdirAll(filepath.Join(worldDir, "characters"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(worldDir, "locations"), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "world.json"), []byte(`{"id":"w2","title":"Wrong World"}`), 0o644))
	_, err := s.LoadWorld(ctx, "w1")
	require.ErrorContains(t, err, "world id")

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "characters", "c1.json"), []byte(`{"id":"c2","name":"Wrong Character"}`), 0o644))
	_, err = s.LoadCharacter(ctx, "w1", "c1")
	require.ErrorContains(t, err, "character id")

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "locations", "l1.json"), []byte(`{"id":"l2","name":"Wrong Location"}`), 0o644))
	_, err = s.LoadLocation(ctx, "w1", "l1")
	require.ErrorContains(t, err, "location id")
}

func TestFileStoreValidatesLoadedEventsMemoriesAndDrafts(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStore(dir)
	require.NoError(t, s.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))
	worldDir := filepath.Join(dir, "narrative", "worlds", "w1")

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "events.jsonl"), []byte(`{"id":"e1"}`+"\n"), 0o644))
	_, err := s.ListEvents(ctx, "w1")
	require.ErrorContains(t, err, "event.beat_id")

	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "memories.jsonl"), []byte(`{"id":"m1"}`+"\n"), 0o644))
	_, err = s.ListMemories(ctx, "w1")
	require.ErrorContains(t, err, "memory.type")

	require.NoError(t, os.MkdirAll(filepath.Join(worldDir, "drafts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worldDir, "drafts", "d1.md"), []byte("---\n{\"id\":\"d1\"}\n---\n# Broken\n\ntext\n"), 0o644))
	_, err = s.LoadDraft(ctx, "w1", "d1")
	require.ErrorContains(t, err, "draft.beat_id")
}

func TestFileStoreRejectsDraftWhenFrontMatterIDDoesNotMatchPath(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s := NewFileStore(dir)
	require.NoError(t, s.SaveWorld(ctx, narrative.World{ID: "w1", Title: "Signal City"}))

	draftsDir := filepath.Join(dir, "narrative", "worlds", "w1", "drafts")
	require.NoError(t, os.MkdirAll(draftsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(draftsDir, "d1.md"), []byte("---\n{\"id\":\"d2\",\"beat_id\":\"b1\",\"title\":\"Wrong ID\",\"kind\":\"scene\"}\n---\n# Wrong ID\n\ntext\n"), 0o644))

	_, err := s.LoadDraft(ctx, "w1", "d1")
	require.ErrorContains(t, err, "draft id")
}
