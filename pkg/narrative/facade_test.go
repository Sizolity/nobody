package narrative_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
)

func TestPublicNarrativeTypesValidate(t *testing.T) {
	world := narrative.World{ID: "w1", Title: "Signal City"}
	require.NoError(t, world.Validate())

	graph := narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Start the story"},
		},
	}
	require.NoError(t, graph.Validate())
}
