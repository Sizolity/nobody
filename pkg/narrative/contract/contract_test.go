package contract_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/contract"
	"github.com/sizolity/nobody/pkg/narrative/engine"
)

func TestValidateBeatPlanRequiresTargetNodeInGraph(t *testing.T) {
	graph := narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Open"},
		},
	}

	require.NoError(t, contract.ValidateBeatPlan(engine.BeatPlan{
		BeatID: "beat-1", Objective: "Open the scene", TargetNodeID: "intro",
	}, graph))

	err := contract.ValidateBeatPlan(engine.BeatPlan{
		BeatID: "beat-1", Objective: "Open the scene", TargetNodeID: "missing",
	}, graph)
	require.ErrorContains(t, err, "target_node_id")
}

func TestValidateDraftForPlanRequiresMatchingBeatID(t *testing.T) {
	plan := engine.BeatPlan{BeatID: "beat-1", Objective: "Open", TargetNodeID: "intro"}

	require.NoError(t, contract.ValidateDraftForPlan(narrative.Draft{
		ID: "draft-1", BeatID: "beat-1", Title: "Opening", Kind: "scene", Text: "The station hummed.",
	}, plan))

	err := contract.ValidateDraftForPlan(narrative.Draft{
		ID: "draft-1", BeatID: "other", Title: "Opening", Kind: "scene", Text: "The station hummed.",
	}, plan)
	require.ErrorContains(t, err, "does not match beat plan")
}

func TestValidateMemoryDeltaRequiresEventsAndMemoriesToMatchPlan(t *testing.T) {
	plan := engine.BeatPlan{BeatID: "beat-1", Objective: "Open", TargetNodeID: "intro"}

	require.NoError(t, contract.ValidateMemoryDelta(engine.MemoryDelta{
		Events: []narrative.NarrativeEvent{
			{ID: "event-1", BeatID: "beat-1", Type: "scene_written", Summary: "Opening written."},
		},
		Memories: []narrative.Memory{
			{ID: "memory-1", Type: "fact", Subject: "Station", Text: "The station hums.", SourceEventID: "event-1"},
		},
	}, plan))

	err := contract.ValidateMemoryDelta(engine.MemoryDelta{
		Events: []narrative.NarrativeEvent{
			{ID: "event-1", BeatID: "other", Type: "scene_written", Summary: "Opening written."},
		},
	}, plan)
	require.ErrorContains(t, err, "does not match beat plan")

	err = contract.ValidateMemoryDelta(engine.MemoryDelta{
		Memories: []narrative.Memory{
			{ID: "memory-1", Type: "fact", Subject: "Station", Text: "The station hums.", SourceEventID: "missing"},
		},
	}, plan)
	require.ErrorContains(t, err, "source_event_id")
}

func TestValidateStateDeltaRequiresValidGraph(t *testing.T) {
	require.NoError(t, contract.ValidateStateDelta(engine.StateDelta{
		Graph: narrative.StoryGraph{
			CurrentNodeID: "intro",
			Nodes: []narrative.StoryNode{
				{ID: "intro", Type: "scene", Status: "ready", Goal: "Open"},
			},
		},
	}))

	err := contract.ValidateStateDelta(engine.StateDelta{
		Graph: narrative.StoryGraph{CurrentNodeID: "missing"},
	})
	require.ErrorContains(t, err, "state graph")
}
