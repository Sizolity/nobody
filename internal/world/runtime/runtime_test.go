package runtime

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestRuntimeRejectsEmptyEvent(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	_, err := rt.ApplyEvent(world, model.WorldEvent{})
	if err == nil {
		t.Fatal("ApplyEvent returned nil for empty event")
	}
}

func TestRuntimeAppliesEventToEventLog(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "test event",
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 || got.EventLog[0].ID != event.ID {
		t.Fatalf("EventLog mismatch: %#v", got.EventLog)
	}
}

func TestRuntimeAppliesSetFactEffect(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeWorldFactChanged,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectSetFact,
			TargetID: "fact_1",
			Payload: map[string]model.Value{
				"subject_id": {Kind: model.ValueKindEntityRef, Raw: "door_1"},
				"predicate":  {Kind: model.ValueKindString, Raw: "locked"},
				"value":      {Kind: model.ValueKindBoolean, Raw: true},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Facts) != 1 {
		t.Fatalf("Facts length = %d, want 1", len(got.Facts))
	}
	if got.Facts[0].ID != "fact_1" || got.Facts[0].Predicate != "locked" {
		t.Fatalf("unexpected fact: %#v", got.Facts[0])
	}
}

func TestRuntimeAppliesUpdateEntityStateEffect(t *testing.T) {
	rt := Runtime{}
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"door_1": {ID: "door_1", Type: "door", Name: "Door"},
		},
	}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateEntityState,
			TargetID: "door_1",
			Payload: map[string]model.Value{
				"locked": {Kind: model.ValueKindBoolean, Raw: true},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if got.Entities["door_1"].State["locked"].Raw != true {
		t.Fatalf("entity state not updated: %#v", got.Entities["door_1"].State)
	}
}

func TestRuntimeAppliesAddRelationEffect(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeRelationshipChanged,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectAddRelation,
			TargetID: "relation_1",
			Payload: map[string]model.Value{
				"type":      {Kind: model.ValueKindString, Raw: "owns"},
				"source_id": {Kind: model.ValueKindEntityRef, Raw: "hero"},
				"target_id": {Kind: model.ValueKindEntityRef, Raw: "sword"},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Relations) != 1 || got.Relations[0].Type != "owns" {
		t.Fatalf("unexpected relations: %#v", got.Relations)
	}
}

func TestRuntimeAppliesAddMemoryEffect(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeRemember,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectAddMemory,
			TargetID: "memory_1",
			Payload: map[string]model.Value{
				"owner_kind":   {Kind: model.ValueKindString, Raw: model.MemoryOwnerKindCharacter},
				"owner_id":     {Kind: model.ValueKindEntityRef, Raw: "char_b"},
				"scope":        {Kind: model.ValueKindString, Raw: model.MemoryScopeSubjective},
				"kind":         {Kind: model.ValueKindString, Raw: model.MemoryKindBelief},
				"content":      {Kind: model.ValueKindString, Raw: "A killed the king."},
				"truth_status": {Kind: model.ValueKindString, Raw: model.TruthStatusUnknown},
				"confidence":   {Kind: model.ValueKindNumber, Raw: 0.8},
				"importance":   {Kind: model.ValueKindNumber, Raw: 0.7},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Memory) != 1 {
		t.Fatalf("Memory length = %d, want 1", len(got.Memory))
	}
	if got.Memory[0].Owner.Kind != model.MemoryOwnerKindCharacter || got.Memory[0].Owner.ID != "char_b" {
		t.Fatalf("unexpected memory owner: %#v", got.Memory[0].Owner)
	}
	if got.Memory[0].Confidence != 0.8 {
		t.Fatalf("unexpected confidence: %#v", got.Memory[0])
	}
}

func TestRuntimeAppliesReviseMemoryEffect(t *testing.T) {
	rt := Runtime{}
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Memory: []model.MemoryRecord{{
			ID:          "memory_1",
			Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_c"},
			Scope:       model.MemoryScopeSubjective,
			Kind:        model.MemoryKindBelief,
			Content:     "A killed the king.",
			TruthStatus: model.TruthStatusUnknown,
			Confidence:  0.8,
			Importance:  0.7,
		}},
	}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeRemember,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectReviseMemory,
			TargetID: "memory_1",
			Payload: map[string]model.Value{
				"content":      {Kind: model.ValueKindString, Raw: "A may have been framed."},
				"truth_status": {Kind: model.ValueKindString, Raw: model.TruthStatusDisputed},
				"confidence":   {Kind: model.ValueKindNumber, Raw: 0.4},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if got.Memory[0].Content != "A may have been framed." {
		t.Fatalf("content not revised: %#v", got.Memory[0])
	}
	if got.Memory[0].TruthStatus != model.TruthStatusDisputed || got.Memory[0].Confidence != 0.4 {
		t.Fatalf("memory not revised: %#v", got.Memory[0])
	}
}

func TestRuntimeRejectsReviseMissingMemory(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeRemember,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectReviseMemory,
			TargetID: "missing_memory",
			Payload: map[string]model.Value{
				"confidence": {Kind: model.ValueKindNumber, Raw: 0.4},
			},
		}},
	}

	if _, err := rt.ApplyEvent(world, event); err == nil {
		t.Fatal("ApplyEvent returned nil for missing memory")
	}
}

func TestRuntimeAppliesOpenThreadEffect(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeThreadChanged,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectOpenThread,
			TargetID: "thread_1",
			Payload: map[string]model.Value{
				"kind":     {Kind: model.ValueKindString, Raw: model.ThreadKindMystery},
				"title":    {Kind: model.ValueKindString, Raw: "Find the killer"},
				"summary":  {Kind: model.ValueKindString, Raw: "C investigates the king murder case."},
				"status":   {Kind: model.ValueKindString, Raw: model.ThreadStatusOpen},
				"priority": {Kind: model.ValueKindNumber, Raw: 0.8},
				"tension":  {Kind: model.ValueKindNumber, Raw: 0.6},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Threads) != 1 || got.Threads[0].ID != "thread_1" {
		t.Fatalf("unexpected threads: %#v", got.Threads)
	}
	if got.Threads[0].Status != model.ThreadStatusOpen || got.Threads[0].Tension != 0.6 {
		t.Fatalf("unexpected thread: %#v", got.Threads[0])
	}
}

func TestRuntimeAppliesUpdateThreadEffect(t *testing.T) {
	rt := Runtime{}
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Threads: []model.WorldThread{{
			ID:       "thread_1",
			Kind:     model.ThreadKindMystery,
			Title:    "Find the killer",
			Status:   model.ThreadStatusOpen,
			Priority: 0.8,
			Tension:  0.6,
		}},
	}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeThreadChanged,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateThread,
			TargetID: "thread_1",
			Payload: map[string]model.Value{
				"summary": {Kind: model.ValueKindString, Raw: "C found a temple clue."},
				"status":  {Kind: model.ValueKindString, Raw: model.ThreadStatusActive},
				"tension": {Kind: model.ValueKindNumber, Raw: 0.9},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if got.Threads[0].Summary != "C found a temple clue." || got.Threads[0].Status != model.ThreadStatusActive {
		t.Fatalf("thread not updated: %#v", got.Threads[0])
	}
	if got.Threads[0].Tension != 0.9 {
		t.Fatalf("thread tension not updated: %#v", got.Threads[0])
	}
}

func TestRuntimeAppliesCloseThreadEffect(t *testing.T) {
	rt := Runtime{}
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Threads: []model.WorldThread{{
			ID:       "thread_1",
			Kind:     model.ThreadKindMystery,
			Title:    "Find the killer",
			Status:   model.ThreadStatusActive,
			Priority: 0.8,
			Tension:  0.9,
		}},
	}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeThreadChanged,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectCloseThread,
			TargetID: "thread_1",
			Payload: map[string]model.Value{
				"summary": {Kind: model.ValueKindString, Raw: "The killer was revealed."},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if got.Threads[0].Status != model.ThreadStatusResolved {
		t.Fatalf("thread not resolved: %#v", got.Threads[0])
	}
}

func TestRuntimeRejectsUpdateMissingThread(t *testing.T) {
	rt := Runtime{}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeThreadChanged,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateThread,
			TargetID: "missing_thread",
			Payload: map[string]model.Value{
				"tension": {Kind: model.ValueKindNumber, Raw: 0.9},
			},
		}},
	}

	if _, err := rt.ApplyEvent(world, event); err == nil {
		t.Fatal("ApplyEvent returned nil for missing thread")
	}
}

type testRule struct {
	id       model.RuleID
	decision RuleDecision
}

func (r testRule) ID() model.RuleID {
	return r.id
}

func (r testRule) Evaluate(_ RuleContext, _ model.WorldEvent) RuleDecision {
	return r.decision
}

func TestRuntimeRejectsEventWhenRuleRejects(t *testing.T) {
	rt := Runtime{
		Rules: []Rule{testRule{
			id: "rule_no_notes",
			decision: RuleDecision{
				Status: RuleDecisionReject,
				Reason: "notes are blocked",
			},
		}},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest}

	got, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for rejected event")
	}
	if len(got.EventLog) != 0 {
		t.Fatalf("rejected event was logged: %#v", got.EventLog)
	}
}

func TestRuntimeAppliesEventWhenRulesAllow(t *testing.T) {
	rt := Runtime{
		Rules: []Rule{testRule{
			id:       "rule_allow",
			decision: RuleDecision{Status: RuleDecisionAllow},
		}},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("event was not logged: %#v", got.EventLog)
	}
}

func TestRuntimeEvaluatesRulesBeforeEffects(t *testing.T) {
	rt := Runtime{
		Rules: []Rule{testRule{
			id: "rule_reject",
			decision: RuleDecision{
				Status: RuleDecisionReject,
				Reason: "blocked before effects",
			},
		}},
	}
	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"door_1": {ID: "door_1", Type: "door", Name: "Door"},
		},
	}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectUpdateEntityState,
			TargetID: "door_1",
			Payload: map[string]model.Value{
				"locked": {Kind: model.ValueKindBoolean, Raw: true},
			},
		}},
	}

	got, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for rejected event")
	}
	if got.Entities["door_1"].State != nil {
		t.Fatalf("effect was applied before rule rejection: %#v", got.Entities["door_1"].State)
	}
}

func TestRuntimeApplyEventDoesNotMutateInputWorldWhenLaterEffectFails(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "Test World",
		Entities: map[model.EntityID]model.Entity{
			"door_1": {
				ID:    "door_1",
				Type:  "item",
				Name:  "Door",
				State: map[string]model.Value{},
			},
		},
	}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
		Effects: []model.Effect{
			{
				Kind:     model.EffectUpdateEntityState,
				TargetID: "door_1",
				Payload: map[string]model.Value{
					"locked": {Kind: model.ValueKindBoolean, Raw: true},
				},
			},
			{
				Kind:     model.EffectReviseMemory,
				TargetID: "missing_memory",
			},
		},
	}

	if _, err := (Runtime{}).ApplyEvent(world, event); err == nil {
		t.Fatal("ApplyEvent returned nil for failing event")
	}
	if _, ok := world.Entities["door_1"].State["locked"]; ok {
		t.Fatalf("input world was mutated after failed event: %#v", world.Entities["door_1"].State)
	}
}
