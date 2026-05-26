package config

import (
	"encoding/json"
	"testing"

	"github.com/sizolity/nobody/internal/world/director"
	"github.com/sizolity/nobody/internal/world/model"
)

func TestLoadDirectorsBuildsScriptAndReconcileDirectors(t *testing.T) {
	t.Parallel()

	const data = `{
  "directors": [
    {
      "id": "script_1",
      "kind": "script",
      "events": [
        {"id": "event_script_1", "type": "note", "source": "director"}
      ]
    },
    {
      "id": "reconcile_1",
      "kind": "reconcile",
      "cases": [
        {
          "event_id": "event_reconcile_1",
          "target_memory_id": "memory_1",
          "when_truth_status": "unknown",
          "truth_status": "disputed",
          "confidence_delta": -0.5
        }
      ]
    }
  ]
}`

	directors, err := LoadDirectors([]byte(data))
	if err != nil {
		t.Fatalf("LoadDirectors returned error: %v", err)
	}
	if len(directors) != 2 {
		t.Fatalf("directors count = %d, want 2", len(directors))
	}
	if directors[0].ID() != "script_1" || directors[1].ID() != "reconcile_1" {
		t.Fatalf("director ids mismatch: %q %q", directors[0].ID(), directors[1].ID())
	}

	scriptEvents, err := directors[0].Propose(director.Context{})
	if err != nil {
		t.Fatalf("script Propose returned error: %v", err)
	}
	if len(scriptEvents) != 1 || scriptEvents[0].ID != "event_script_1" {
		t.Fatalf("script events mismatch: %#v", scriptEvents)
	}

	reconcileEvents, err := directors[1].Propose(director.Context{World: model.World{
		Memory: []model.MemoryRecord{{
			ID:          "memory_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
			Scope:       model.MemoryScopeSubjective,
			Kind:        model.MemoryKindBelief,
			Content:     "A killed the king.",
			TruthStatus: model.TruthStatusUnknown,
			Confidence:  0.8,
		}},
	}})
	if err != nil {
		t.Fatalf("reconcile Propose returned error: %v", err)
	}
	if len(reconcileEvents) != 1 || reconcileEvents[0].ID != "event_reconcile_1" {
		t.Fatalf("reconcile events mismatch: %#v", reconcileEvents)
	}
}

func TestLoadDirectorsRejectsUnsupportedKind(t *testing.T) {
	t.Parallel()

	_, err := LoadDirectors([]byte(`{"directors":[{"id":"random_1","kind":"random"}]}`))
	if err == nil {
		t.Fatal("LoadDirectors returned nil for unsupported kind")
	}
}

func TestLoadDirectorsRejectsInvalidDirectorConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data string
	}{
		{
			name: "missing id",
			data: `{"directors":[{"kind":"script","events":[]}]}`,
		},
		{
			name: "missing kind",
			data: `{"directors":[{"id":"script_1","events":[]}]}`,
		},
		{
			name: "invalid event",
			data: `{"directors":[{"id":"script_1","kind":"script","events":[{"id":"event_1"}]}]}`,
		},
		{
			name: "invalid reconcile case",
			data: `{"directors":[{"id":"reconcile_1","kind":"reconcile","cases":[{"target_memory_id":"memory_1"}]}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LoadDirectors([]byte(tc.data)); err == nil {
				t.Fatal("LoadDirectors returned nil")
			}
		})
	}
}

func TestDirectorConfigJSONRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := File{
		Directors: []DirectorConfig{{
			ID:   "script_1",
			Kind: DirectorKindScript,
			Events: []model.WorldEvent{{
				ID:     "event_1",
				Type:   model.EventTypeNote,
				Source: model.EventSourceDirector,
			}},
		}},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var got File
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if len(got.Directors) != 1 || got.Directors[0].Kind != DirectorKindScript {
		t.Fatalf("config mismatch: %#v", got)
	}
}
