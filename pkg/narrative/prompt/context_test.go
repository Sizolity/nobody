package prompt_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sizolity/nobody/pkg/narrative"
	"github.com/sizolity/nobody/pkg/narrative/engine"
	"github.com/sizolity/nobody/pkg/narrative/prompt"
)

func TestContextJSONRendersStableIndentedBundle(t *testing.T) {
	got, err := prompt.ContextJSON(engine.ContextBundle{
		World: narrative.World{ID: "w1", Title: "Signal City", Genre: "modern fantasy"},
		Graph: narrative.StoryGraph{
			CurrentNodeID: "intro",
			Nodes: []narrative.StoryNode{
				{ID: "intro", Type: "scene", Status: "ready", Goal: "Find the signal", CharacterIDs: []string{"c1"}, LocationID: "l1"},
			},
		},
		Characters: []narrative.Character{{ID: "c1", Name: "Lin Xia", Role: "protagonist"}},
		Locations:  []narrative.Location{{ID: "l1", Name: "Old Station"}},
		Events:     []narrative.NarrativeEvent{{ID: "event-0", BeatID: "beat-0", Type: "scene_written", Summary: "The first clue appeared."}},
		Memories:   []narrative.Memory{{ID: "memory-0", Type: "fact", Subject: "Signal", Text: "The signal appears near old transit lines.", Importance: 4}},
		Input:      "follow the hum",
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
	  "world": {"id": "w1", "title": "Signal City", "genre": "modern fantasy"},
	  "graph": {
	    "current_node_id": "intro",
	    "nodes": [{
	      "id": "intro",
	      "type": "scene",
	      "status": "ready",
	      "goal": "Find the signal",
	      "character_ids": ["c1"],
	      "location_id": "l1"
	    }]
	  },
	  "characters": [{"id": "c1", "name": "Lin Xia", "role": "protagonist"}],
	  "locations": [{"id": "l1", "name": "Old Station"}],
	  "events": [{"id": "event-0", "beat_id": "beat-0", "type": "scene_written", "summary": "The first clue appeared."}],
	  "memories": [{"id": "memory-0", "type": "fact", "subject": "Signal", "text": "The signal appears near old transit lines.", "importance": 4}],
	  "input": "follow the hum"
	}`, got)
	require.Contains(t, got, "\n  \"world\"")
}

func TestContextPromptWrapsJSONWithInstruction(t *testing.T) {
	got, err := prompt.ContextPrompt(engine.ContextBundle{World: narrative.World{ID: "w1", Title: "Signal City"}})
	require.NoError(t, err)
	require.Contains(t, got, "Use the following narrative context JSON.")
	require.Contains(t, got, "```json")
	require.Contains(t, got, "\"title\": \"Signal City\"")
}
