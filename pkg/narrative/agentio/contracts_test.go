package agentio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/agentio"
	"github.com/sizolity/nobody/pkg/narrative/engine"
)

func TestDecodeBeatPlanValidatesAgainstGraph(t *testing.T) {
	graph := narrative.StoryGraph{
		CurrentNodeID: "intro",
		Nodes: []narrative.StoryNode{
			{ID: "intro", Type: "scene", Status: "ready", Goal: "Open"},
		},
	}

	got, err := agentio.DecodeBeatPlan(`{"beat_id":"beat-1","objective":"Open","target_node_id":"intro"}`, graph)
	require.NoError(t, err)
	require.Equal(t, engine.BeatPlan{BeatID: "beat-1", Objective: "Open", TargetNodeID: "intro"}, got)

	_, err = agentio.DecodeBeatPlan(`{"beat_id":"beat-1","objective":"Open","target_node_id":"missing"}`, graph)
	require.ErrorContains(t, err, "target_node_id")
}

func TestDecodeDraftValidatesAgainstPlan(t *testing.T) {
	plan := engine.BeatPlan{BeatID: "beat-1", Objective: "Open", TargetNodeID: "intro"}

	got, err := agentio.DecodeDraft("```json\n{\"id\":\"draft-1\",\"beat_id\":\"beat-1\",\"title\":\"Opening\",\"kind\":\"scene\",\"text\":\"The station hummed.\"}\n```", plan)
	require.NoError(t, err)
	require.Equal(t, "draft-1", got.ID)

	_, err = agentio.DecodeDraft(`{"id":"draft-1","beat_id":"other","title":"Opening","kind":"scene","text":"The station hummed."}`, plan)
	require.ErrorContains(t, err, "does not match beat plan")
}

func TestDecodeMemoryDeltaValidatesAgainstPlan(t *testing.T) {
	plan := engine.BeatPlan{BeatID: "beat-1", Objective: "Open", TargetNodeID: "intro"}

	got, err := agentio.DecodeMemoryDelta(`{
		"events": [{"id":"event-1","beat_id":"beat-1","type":"scene_written","summary":"Opening written."}],
		"memories": [{"id":"memory-1","type":"fact","subject":"Station","text":"The station hums.","source_event_id":"event-1"}]
	}`, plan)
	require.NoError(t, err)
	require.Len(t, got.Events, 1)
	require.Len(t, got.Memories, 1)

	_, err = agentio.DecodeMemoryDelta(`{
		"memories": [{"id":"memory-1","type":"fact","subject":"Station","text":"The station hums.","source_event_id":"missing"}]
	}`, plan)
	require.ErrorContains(t, err, "source_event_id")
}

func TestDecodeStateDeltaValidatesGraph(t *testing.T) {
	got, err := agentio.DecodeStateDelta(`{
		"graph": {
			"current_node_id": "intro",
			"nodes": [{"id":"intro","type":"scene","status":"ready","goal":"Open"}]
		}
	}`)
	require.NoError(t, err)
	require.Equal(t, "intro", got.Graph.CurrentNodeID)

	_, err = agentio.DecodeStateDelta(`{"graph":{"current_node_id":"missing","nodes":[]}}`)
	require.ErrorContains(t, err, "state graph")
}
