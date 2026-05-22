// Package snapshot provides read helpers for product UIs and CLIs that need a
// coherent view of a narrative world without running a beat.
package snapshot

import (
	"context"
	"fmt"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/store"
)

type WorldSnapshot struct {
	World       narrative.World
	Graph       narrative.StoryGraph
	CurrentNode narrative.StoryNode
	Characters  []narrative.Character
	Locations   []narrative.Location
	Events      []narrative.NarrativeEvent
	Memories    []narrative.Memory
}

func LoadWorld(ctx context.Context, st store.Store, worldID string) (WorldSnapshot, error) {
	if st == nil {
		return WorldSnapshot{}, fmt.Errorf("store is required")
	}
	world, err := st.LoadWorld(ctx, worldID)
	if err != nil {
		return WorldSnapshot{}, fmt.Errorf("load world: %w", err)
	}
	graph, err := st.LoadStoryGraph(ctx, worldID)
	if err != nil {
		return WorldSnapshot{}, fmt.Errorf("load story graph: %w", err)
	}
	current, err := currentNode(graph)
	if err != nil {
		return WorldSnapshot{}, err
	}
	characters := make([]narrative.Character, 0, len(current.CharacterIDs))
	for _, id := range current.CharacterIDs {
		character, err := st.LoadCharacter(ctx, worldID, id)
		if err != nil {
			return WorldSnapshot{}, fmt.Errorf("load character %q: %w", id, err)
		}
		characters = append(characters, character)
	}
	var locations []narrative.Location
	if current.LocationID != "" {
		location, err := st.LoadLocation(ctx, worldID, current.LocationID)
		if err != nil {
			return WorldSnapshot{}, fmt.Errorf("load location %q: %w", current.LocationID, err)
		}
		locations = append(locations, location)
	}
	events, err := st.ListEvents(ctx, worldID)
	if err != nil {
		return WorldSnapshot{}, fmt.Errorf("list events: %w", err)
	}
	memories, err := st.ListMemories(ctx, worldID)
	if err != nil {
		return WorldSnapshot{}, fmt.Errorf("list memories: %w", err)
	}
	return WorldSnapshot{
		World:       world,
		Graph:       graph,
		CurrentNode: current,
		Characters:  characters,
		Locations:   locations,
		Events:      events,
		Memories:    memories,
	}, nil
}

func currentNode(graph narrative.StoryGraph) (narrative.StoryNode, error) {
	for _, node := range graph.Nodes {
		if node.ID == graph.CurrentNodeID {
			return node, nil
		}
	}
	return narrative.StoryNode{}, fmt.Errorf("current node %q not found", graph.CurrentNodeID)
}
