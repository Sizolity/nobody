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

func TestCharacterValidateRequiresIDAndName(t *testing.T) {
	require.ErrorContains(t, (Character{}).Validate(), "character.id")
	require.ErrorContains(t, (Character{ID: "c1"}).Validate(), "character.name")
	require.NoError(t, (Character{ID: "c1", Name: "Lin Xia"}).Validate())
}

func TestLocationValidateRequiresIDAndName(t *testing.T) {
	require.ErrorContains(t, (Location{}).Validate(), "location.id")
	require.ErrorContains(t, (Location{ID: "l1"}).Validate(), "location.name")
	require.NoError(t, (Location{ID: "l1", Name: "Old Station"}).Validate())
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

func TestStoryGraphValidateChecksNodeRequiredFieldsAndParents(t *testing.T) {
	graph := StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Open", ParentID: "missing"},
		},
	}
	require.ErrorContains(t, graph.Validate(), "parent_id")

	graph.Nodes[0].ParentID = ""
	graph.Nodes[0].Goal = ""
	require.ErrorContains(t, graph.Validate(), "story node goal")

	graph.Nodes[0].Goal = "Open"
	graph.Nodes[0].Status = ""
	require.ErrorContains(t, graph.Validate(), "story node status")

	graph.Nodes[0].Status = "ready"
	graph.Nodes[0].Type = ""
	require.ErrorContains(t, graph.Validate(), "story node type")
}

func TestDraftValidateRequiresBeatMetadataAndText(t *testing.T) {
	require.ErrorContains(t, (Draft{}).Validate(), "draft.id")
	require.ErrorContains(t, (Draft{ID: "d1"}).Validate(), "draft.beat_id")
	require.ErrorContains(t, (Draft{ID: "d1", BeatID: "b1"}).Validate(), "draft.title")
	require.ErrorContains(t, (Draft{ID: "d1", BeatID: "b1", Title: "Opening"}).Validate(), "draft.kind")
	require.ErrorContains(t, (Draft{ID: "d1", BeatID: "b1", Title: "Opening", Kind: "scene"}).Validate(), "draft.text")
	require.NoError(t, (Draft{ID: "d1", BeatID: "b1", Title: "Opening", Kind: "scene", Text: "The station hummed."}).Validate())
}

func TestNarrativeEventValidateRequiresBeatTypeAndSummary(t *testing.T) {
	require.ErrorContains(t, (NarrativeEvent{}).Validate(), "event.id")
	require.ErrorContains(t, (NarrativeEvent{ID: "e1"}).Validate(), "event.beat_id")
	require.ErrorContains(t, (NarrativeEvent{ID: "e1", BeatID: "b1"}).Validate(), "event.type")
	require.ErrorContains(t, (NarrativeEvent{ID: "e1", BeatID: "b1", Type: "scene_written"}).Validate(), "event.summary")
	require.NoError(t, (NarrativeEvent{ID: "e1", BeatID: "b1", Type: "scene_written", Summary: "Lin Xia finds the signal."}).Validate())
}

func TestMemoryValidateRequiresSubjectTextAndImportanceRange(t *testing.T) {
	require.ErrorContains(t, (Memory{}).Validate(), "memory.id")
	require.ErrorContains(t, (Memory{ID: "m1"}).Validate(), "memory.type")
	require.ErrorContains(t, (Memory{ID: "m1", Type: "fact"}).Validate(), "memory.subject")
	require.ErrorContains(t, (Memory{ID: "m1", Type: "fact", Subject: "Lin Xia"}).Validate(), "memory.text")
	require.ErrorContains(t, (Memory{ID: "m1", Type: "fact", Subject: "Lin Xia", Text: "Knows the signal.", Importance: 11}).Validate(), "memory.importance")
	require.NoError(t, (Memory{ID: "m1", Type: "fact", Subject: "Lin Xia", Text: "Knows the signal.", Importance: 5}).Validate())
}
