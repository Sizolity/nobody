package director

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestNarrativeDirector_EmptyWorldReturnsNoProposals(t *testing.T) {
	t.Parallel()

	d := NewNarrativeDirector("narr_1", NarrativeDirectorConfig{})
	got, err := d.Propose(Context{World: model.World{ID: "w", Name: "Empty"}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if got == nil {
		t.Fatal("events is nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestNarrativeDirector_HighTensionTriggersAdvancement(t *testing.T) {
	t.Parallel()

	d := NewNarrativeDirector("narr_1", NarrativeDirectorConfig{})
	w := model.World{
		ID: "w", Name: "World",
		Threads: []model.WorldThread{{
			ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest",
			Status: model.ThreadStatusActive, Tension: 0.8,
		}},
	}

	got, err := d.Propose(Context{World: w})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1: %#v", len(got), got)
	}
	e := got[0]
	if e.ID != "narr_advance_t1" {
		t.Fatalf("event ID = %q, want narr_advance_t1", e.ID)
	}
	if e.Type != model.EventTypeThreadChanged {
		t.Fatalf("event Type = %q, want %q", e.Type, model.EventTypeThreadChanged)
	}
	if e.Source != model.EventSourceDirector {
		t.Fatalf("event Source = %q, want %q", e.Source, model.EventSourceDirector)
	}
	if len(e.Effects) != 1 {
		t.Fatalf("effects count = %d, want 1", len(e.Effects))
	}
	eff := e.Effects[0]
	if eff.Kind != model.EffectUpdateThread {
		t.Fatalf("effect Kind = %q, want %q", eff.Kind, model.EffectUpdateThread)
	}
	if eff.TargetID != "t1" {
		t.Fatalf("effect TargetID = %q, want t1", eff.TargetID)
	}
	tensionVal, ok := eff.Payload["tension"]
	if !ok {
		t.Fatal("tension not in payload")
	}
	if tensionVal.Raw != 0.9 {
		t.Fatalf("tension = %v, want 0.9", tensionVal.Raw)
	}
}

func TestNarrativeDirector_LowTensionNoAdvancement(t *testing.T) {
	t.Parallel()

	d := NewNarrativeDirector("narr_1", NarrativeDirectorConfig{})
	w := model.World{
		ID: "w", Name: "World",
		Threads: []model.WorldThread{{
			ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest",
			Status: model.ThreadStatusActive, Tension: 0.3,
		}},
	}

	got, err := d.Propose(Context{World: w})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestNarrativeDirector_ConsecutiveSameTypeTriggersSceneVariety(t *testing.T) {
	t.Parallel()

	d := NewNarrativeDirector("narr_1", NarrativeDirectorConfig{})
	w := model.World{
		ID: "w", Name: "World",
		EventLog: []model.WorldEvent{
			{ID: "e1", Type: model.EventTypeNote, Source: model.EventSourceTest},
			{ID: "e2", Type: model.EventTypeNote, Source: model.EventSourceTest},
			{ID: "e3", Type: model.EventTypeNote, Source: model.EventSourceTest},
		},
	}

	got, err := d.Propose(Context{World: w})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1: %#v", len(got), got)
	}
	e := got[0]
	if e.ID != "narr_scene_shift" {
		t.Fatalf("event ID = %q, want narr_scene_shift", e.ID)
	}
	if e.Type != model.EventTypeNote {
		t.Fatalf("event Type = %q, want %q", e.Type, model.EventTypeNote)
	}
	if e.Source != model.EventSourceDirector {
		t.Fatalf("event Source = %q, want %q", e.Source, model.EventSourceDirector)
	}
	if e.Description != "The pace shifts..." {
		t.Fatalf("event Description = %q, want %q", e.Description, "The pace shifts...")
	}
}

func TestNarrativeDirector_DormantThreadRevivalAfterThreshold(t *testing.T) {
	t.Parallel()

	d := NewNarrativeDirector("narr_1", NarrativeDirectorConfig{
		DormantThreadReviveAfter: 3,
	})
	w := model.World{
		ID: "w", Name: "World",
		Threads: []model.WorldThread{{
			ID: "t1", Kind: model.ThreadKindQuest, Title: "Old Quest",
			Status: model.ThreadStatusDormant, Tension: 0.3,
		}},
		EventLog: []model.WorldEvent{
			{ID: "e1", Type: model.EventTypeNote, Source: model.EventSourceTest},
			{ID: "e2", Type: model.EventTypeMove, Source: model.EventSourceTest},
			{ID: "e3", Type: model.EventTypeNote, Source: model.EventSourceTest},
		},
	}

	got, err := d.Propose(Context{World: w})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}

	var revival *model.WorldEvent
	for i := range got {
		if got[i].ID == "narr_revive_t1" {
			revival = &got[i]
			break
		}
	}
	if revival == nil {
		t.Fatalf("expected narr_revive_t1 event, got %#v", got)
	}
	if revival.Type != model.EventTypeThreadChanged {
		t.Fatalf("event Type = %q, want %q", revival.Type, model.EventTypeThreadChanged)
	}
	if len(revival.Effects) != 1 {
		t.Fatalf("effects count = %d, want 1", len(revival.Effects))
	}
	eff := revival.Effects[0]
	if eff.Kind != model.EffectUpdateThread {
		t.Fatalf("effect Kind = %q, want %q", eff.Kind, model.EffectUpdateThread)
	}
	statusVal, ok := eff.Payload["status"]
	if !ok {
		t.Fatal("status not in payload")
	}
	if statusVal.Raw != model.ThreadStatusActive {
		t.Fatalf("status = %v, want %q", statusVal.Raw, model.ThreadStatusActive)
	}
}

func TestNarrativeDirector_CustomRuleFiresWhenConditionMet(t *testing.T) {
	t.Parallel()

	rule := NarrativeRule{
		ID:        "custom_1",
		Condition: func(w model.World) bool { return len(w.Threads) > 0 },
		Propose: func(w model.World) []model.WorldEvent {
			return []model.WorldEvent{{
				ID: "custom_event", Type: model.EventTypeNote,
				Source: model.EventSourceDirector, Description: "custom rule fired",
			}}
		},
	}
	d := NewNarrativeDirector("narr_1", NarrativeDirectorConfig{Rules: []NarrativeRule{rule}})
	w := model.World{
		ID: "w", Name: "World",
		Threads: []model.WorldThread{{
			ID: "t1", Kind: model.ThreadKindQuest, Title: "Quest",
			Status: model.ThreadStatusOpen, Tension: 0.1,
		}},
	}

	got, err := d.Propose(Context{World: w})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("events count = %d, want 1: %#v", len(got), got)
	}
	if got[0].ID != "custom_event" {
		t.Fatalf("event ID = %q, want custom_event", got[0].ID)
	}
	if got[0].Description != "custom rule fired" {
		t.Fatalf("Description = %q, want %q", got[0].Description, "custom rule fired")
	}
}

func TestNarrativeDirector_CustomRuleSkippedWhenConditionFalse(t *testing.T) {
	t.Parallel()

	rule := NarrativeRule{
		ID:        "custom_1",
		Condition: func(w model.World) bool { return false },
		Propose: func(w model.World) []model.WorldEvent {
			return []model.WorldEvent{{
				ID: "should_not_appear", Type: model.EventTypeNote,
				Source: model.EventSourceDirector,
			}}
		},
	}
	d := NewNarrativeDirector("narr_1", NarrativeDirectorConfig{Rules: []NarrativeRule{rule}})

	got, err := d.Propose(Context{World: model.World{ID: "w", Name: "World"}})
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("events count = %d, want 0: %#v", len(got), got)
	}
}

func TestNarrativeDirector_DefaultConfigValues(t *testing.T) {
	t.Parallel()

	d := NewNarrativeDirector("narr_1", NarrativeDirectorConfig{})
	if d.config.MinTensionForAdvance != 0.7 {
		t.Fatalf("MinTensionForAdvance = %v, want 0.7", d.config.MinTensionForAdvance)
	}
	if d.config.MaxConsecutiveSameType != 3 {
		t.Fatalf("MaxConsecutiveSameType = %d, want 3", d.config.MaxConsecutiveSameType)
	}
	if d.config.DormantThreadReviveAfter != 5 {
		t.Fatalf("DormantThreadReviveAfter = %d, want 5", d.config.DormantThreadReviveAfter)
	}
}
