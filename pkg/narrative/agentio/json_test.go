package agentio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/agentio"
	"github.com/sizolity/nobody/pkg/narrative/engine"
)

func TestDecodeJSONAcceptsPlainJSON(t *testing.T) {
	got, err := agentio.DecodeJSON[engine.BeatPlan](`{
		"beat_id": "beat-1",
		"objective": "Open the scene",
		"target_node_id": "intro"
	}`)
	require.NoError(t, err)
	require.Equal(t, engine.BeatPlan{BeatID: "beat-1", Objective: "Open the scene", TargetNodeID: "intro"}, got)
}

func TestDecodeJSONAcceptsFencedJSON(t *testing.T) {
	got, err := agentio.DecodeJSON[narrative.Draft]("```json\n{\"id\":\"draft-1\",\"beat_id\":\"beat-1\",\"title\":\"Opening\",\"kind\":\"scene\",\"text\":\"The station hummed.\"}\n```")
	require.NoError(t, err)
	require.Equal(t, "draft-1", got.ID)
	require.Equal(t, "The station hummed.", got.Text)
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	_, err := agentio.DecodeJSON[engine.BeatPlan](`{
		"beat_id": "beat-1",
		"objective": "Open the scene",
		"target_node_id": "intro",
		"unexpected": true
	}`)
	require.ErrorContains(t, err, "unknown field")
}

func TestDecodeJSONRejectsTrailingText(t *testing.T) {
	_, err := agentio.DecodeJSON[engine.BeatPlan](`{"beat_id":"beat-1","objective":"Open","target_node_id":"intro"} extra`)
	require.ErrorContains(t, err, "trailing")
}

func TestDecodeJSONValidatesKnownContracts(t *testing.T) {
	_, err := agentio.DecodeValidatedJSON[narrative.Draft](`{"id":"draft-1"}`)
	require.ErrorContains(t, err, "draft.beat_id")
}
