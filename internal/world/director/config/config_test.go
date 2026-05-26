package config

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestLoadDirectorsBuildsEventTableDirector(t *testing.T) {
	t.Parallel()

	const data = `{
  "directors": [
    {
      "id": "table_1",
      "kind": "event_table",
      "entries": [
        {
          "weight": 1,
          "event": {"id": "event_1", "type": "note", "source": "director"}
        },
        {
          "weight": 3,
          "event": {"id": "event_2", "type": "note", "source": "director"}
        }
      ]
    }
  ]
}`

	directors, err := LoadDirectors([]byte(data))
	if err != nil {
		t.Fatalf("LoadDirectors returned error: %v", err)
	}
	if len(directors) != 1 || directors[0].ID() != "table_1" {
		t.Fatalf("directors mismatch: %#v", directors)
	}
	got, err := directors[0].Propose(director.Context{World: model.World{Clock: model.WorldClock{Sequence: 1}}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_2" {
		t.Fatalf("event table proposal mismatch: %#v", got)
	}
}

func TestLoadDirectorsBuildsRandomDirector(t *testing.T) {
	t.Parallel()

	const data = `{
  "directors": [
    {
      "id": "random_1",
      "kind": "random",
      "seed": 42,
      "entries": [
        {
          "weight": 1,
          "event": {"id": "event_1", "type": "note", "source": "director"}
        },
        {
          "weight": 1,
          "event": {"id": "event_2", "type": "note", "source": "director"}
        }
      ]
    }
  ]
}`

	directors, err := LoadDirectors([]byte(data))
	if err != nil {
		t.Fatalf("LoadDirectors returned error: %v", err)
	}
	if len(directors) != 1 || directors[0].ID() != "random_1" {
		t.Fatalf("directors mismatch: %#v", directors)
	}
	got, err := directors[0].Propose(director.Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("proposal length = %d, want 1", len(got))
	}
}

func TestLoadDirectorsBuildsRandomDirectorWithoutSeed(t *testing.T) {
	t.Parallel()

	const data = `{
  "directors": [
    {
      "id": "random_1",
      "kind": "random",
      "entries": [
        {
          "weight": 1,
          "event": {"id": "event_1", "type": "note", "source": "director"}
        }
      ]
    }
  ]
}`

	directors, err := LoadDirectors([]byte(data))
	if err != nil {
		t.Fatalf("LoadDirectors returned error: %v", err)
	}
	got, err := directors[0].Propose(director.Context{})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "event_1" {
		t.Fatalf("random proposal mismatch: %#v", got)
	}
}

func TestLoadDirectorsRejectsUnsupportedKind(t *testing.T) {
	t.Parallel()

	_, err := LoadDirectors([]byte(`{"directors":[{"id":"unknown_1","kind":"teleport"}]}`))
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
		{
			name: "invalid event table event",
			data: `{"directors":[{"id":"table_1","kind":"event_table","entries":[{"weight":1,"event":{"id":"event_1"}}]}]}`,
		},
		{
			name: "invalid event table weight",
			data: `{"directors":[{"id":"table_1","kind":"event_table","entries":[{"weight":0,"event":{"id":"event_1","type":"note","source":"director"}}]}]}`,
		},
		{
			name: "invalid random entry weight",
			data: `{"directors":[{"id":"random_1","kind":"random","entries":[{"weight":-1,"event":{"id":"event_1","type":"note","source":"director"}}]}]}`,
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

	seed := int64(42)
	cfg := File{
		Directors: []DirectorConfig{{
			ID:   "script_1",
			Kind: DirectorKindScript,
			Events: []model.WorldEvent{{
				ID:     "event_1",
				Type:   model.EventTypeNote,
				Source: model.EventSourceDirector,
			}},
		}, {
			ID:   "table_1",
			Kind: DirectorKindEventTable,
			Entries: []director.EventTableEntry{{
				Weight: 1,
				Event: model.WorldEvent{
					ID:     "event_2",
					Type:   model.EventTypeNote,
					Source: model.EventSourceDirector,
				},
			}},
		}, {
			ID:   "random_1",
			Kind: DirectorKindRandom,
			Seed: &seed,
			Entries: []director.EventTableEntry{{
				Weight: 1,
				Event: model.WorldEvent{
					ID:     "event_3",
					Type:   model.EventTypeNote,
					Source: model.EventSourceDirector,
				},
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
	if len(got.Directors) != 3 {
		t.Fatalf("directors count = %d, want 3", len(got.Directors))
	}
	if got.Directors[2].Kind != DirectorKindRandom || got.Directors[2].Seed == nil || *got.Directors[2].Seed != 42 {
		t.Fatalf("random config mismatch: %#v", got.Directors[2])
	}
}

func TestLoadDirectorsBuildsLLMDirector(t *testing.T) {
	t.Parallel()

	const data = `{
  "directors": [
    {
      "id": "llm_1",
      "kind": "llm",
      "provider": "deepseek",
      "model": "deepseek-v4-pro",
      "system_prompt": "You are a world director."
    }
  ]
}`

	factory := func(provider, model string) (director.TextGenerator, error) {
		if provider != "deepseek" {
			t.Fatalf("provider = %q, want deepseek", provider)
		}
		if model != "deepseek-v4-pro" {
			t.Fatalf("model = %q, want deepseek-v4-pro", model)
		}
		return &stubGenerator{response: "[]"}, nil
	}

	directors, err := LoadDirectors([]byte(data), LoadOptions{GeneratorFactory: factory})
	if err != nil {
		t.Fatalf("LoadDirectors returned error: %v", err)
	}
	if len(directors) != 1 || directors[0].ID() != "llm_1" {
		t.Fatalf("directors mismatch: %#v", directors)
	}
	got, err := directors[0].Propose(director.Context{
		Ctx:   context.Background(),
		World: model.World{ID: "w", Name: "W"},
	})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty proposals from stub: %#v", got)
	}
}

func TestLoadDirectorsLLMDefaultsProviderToDeepseek(t *testing.T) {
	t.Parallel()

	const data = `{
  "directors": [{"id": "llm_1", "kind": "llm"}]
}`

	var gotProvider string
	factory := func(provider, model string) (director.TextGenerator, error) {
		gotProvider = provider
		return &stubGenerator{response: "[]"}, nil
	}

	_, err := LoadDirectors([]byte(data), LoadOptions{GeneratorFactory: factory})
	if err != nil {
		t.Fatalf("LoadDirectors returned error: %v", err)
	}
	if gotProvider != "deepseek" {
		t.Fatalf("default provider = %q, want deepseek", gotProvider)
	}
}

func TestLoadDirectorsLLMRejectsWithoutFactory(t *testing.T) {
	t.Parallel()

	const data = `{
  "directors": [{"id": "llm_1", "kind": "llm"}]
}`

	_, err := LoadDirectors([]byte(data))
	if err == nil {
		t.Fatal("expected error when llm kind used without GeneratorFactory")
	}
}

func TestLoadDirectorsLLMRejectsFactoryError(t *testing.T) {
	t.Parallel()

	const data = `{
  "directors": [{"id": "llm_1", "kind": "llm", "provider": "unknown"}]
}`

	factory := func(provider, model string) (director.TextGenerator, error) {
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}

	_, err := LoadDirectors([]byte(data), LoadOptions{GeneratorFactory: factory})
	if err == nil {
		t.Fatal("expected error from factory")
	}
}

func TestDirectorConfigLLMJSONRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := File{
		Directors: []DirectorConfig{{
			ID:           "llm_1",
			Kind:         DirectorKindLLM,
			Provider:     "deepseek",
			Model:        "deepseek-v4-pro",
			SystemPrompt: "You are a narrator.",
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
	if len(got.Directors) != 1 {
		t.Fatalf("directors count = %d, want 1", len(got.Directors))
	}
	d := got.Directors[0]
	if d.Kind != DirectorKindLLM || d.Provider != "deepseek" || d.Model != "deepseek-v4-pro" || d.SystemPrompt != "You are a narrator." {
		t.Fatalf("llm config mismatch: %#v", d)
	}
}

type stubGenerator struct {
	response string
}

func (g *stubGenerator) Generate(_ context.Context, _, _ string) (string, error) {
	return g.response, nil
}
