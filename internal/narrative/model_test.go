package narrative

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorldValidateRequiresIDAndTitle(t *testing.T) {
	require.ErrorContains(t, (World{}).Validate(), "world.id")
	require.ErrorContains(t, (World{ID: "w1"}).Validate(), "world.title")
	require.NoError(t, (World{ID: "w1", Title: "Signal City"}).Validate())
}

func TestStoryGraphValidateChecksCurrentNode(t *testing.T) {
	graph := StoryGraph{
		CurrentNodeID: "missing",
		Nodes: []StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Open the story"},
		},
	}

	require.ErrorContains(t, graph.Validate(), "current_node_id")

	graph.CurrentNodeID = "intro"
	require.NoError(t, graph.Validate())
}

func TestStoryGraphValidateRejectsDuplicateNodeIDs(t *testing.T) {
	graph := StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Open"},
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Repeat"},
		},
	}

	require.ErrorContains(t, graph.Validate(), "duplicate story node")
}
