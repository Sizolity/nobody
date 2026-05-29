package runtime

import (
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

func TestRuleDecisionValidateModifyRequiresEvent(t *testing.T) {
	t.Parallel()
	d := RuleDecision{Status: RuleDecisionModify}
	if err := d.Validate(); err == nil {
		t.Fatal("Validate returned nil for modify without ModifiedEvent")
	}
}

func TestRuleDecisionValidateAddEffectRequiresEffects(t *testing.T) {
	t.Parallel()
	d := RuleDecision{Status: RuleDecisionAddEffect}
	if err := d.Validate(); err == nil {
		t.Fatal("Validate returned nil for add_effect without AddedEffects")
	}
}

func TestRuleDecisionValidateEnqueueRequiresEvents(t *testing.T) {
	t.Parallel()
	d := RuleDecision{Status: RuleDecisionEnqueue}
	if err := d.Validate(); err == nil {
		t.Fatal("Validate returned nil for enqueue without EnqueuedEvents")
	}
}

func TestRuleDecisionValidateAcceptsValidDecisions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		decision RuleDecision
	}{
		{"allow", RuleDecision{Status: RuleDecisionAllow}},
		{"reject", RuleDecision{Status: RuleDecisionReject, Reason: "nope"}},
		{"modify", RuleDecision{
			Status:        RuleDecisionModify,
			ModifiedEvent: &model.WorldEvent{ID: "e", Type: model.EventTypeNote, Source: model.EventSourceTest},
		}},
		{"add_effect", RuleDecision{
			Status:       RuleDecisionAddEffect,
			AddedEffects: []model.Effect{{Kind: model.EffectSetFact, TargetID: "f"}},
		}},
		{"enqueue", RuleDecision{
			Status:         RuleDecisionEnqueue,
			EnqueuedEvents: []model.WorldEvent{{ID: "q", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.decision.Validate(); err != nil {
				t.Fatalf("Validate returned error for valid %s decision: %v", tc.name, err)
			}
		})
	}
}

type dynamicRule struct {
	id model.RuleID
	fn func(RuleContext, model.WorldEvent) RuleDecision
}

func (r dynamicRule) ID() model.RuleID { return r.id }
func (r dynamicRule) Evaluate(ctx RuleContext, event model.WorldEvent) RuleDecision {
	return r.fn(ctx, event)
}

func TestModifyDecisionReplacesEvent(t *testing.T) {
	t.Parallel()

	modified := model.WorldEvent{
		ID:          "event_modified",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceRuntime,
		Description: "modified description",
	}
	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_modify",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{Status: RuleDecisionModify, ModifiedEvent: &modified}
			},
		}},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:          "event_original",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "original",
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventLog) != 1 {
		t.Fatalf("EventLog length = %d, want 1", len(got.EventLog))
	}
	if got.EventLog[0].ID != "event_modified" {
		t.Fatalf("logged event ID = %q, want event_modified", got.EventLog[0].ID)
	}
	if got.EventLog[0].Description != "modified description" {
		t.Fatalf("logged event description = %q, want modified description", got.EventLog[0].Description)
	}
}

func TestAddEffectDecisionAppendsEffects(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_add_effect",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{
					Status: RuleDecisionAddEffect,
					AddedEffects: []model.Effect{{
						Kind:     model.EffectSetFact,
						TargetID: "fact_injected",
						Payload: map[string]model.Value{
							"subject_id": {Kind: model.ValueKindEntityRef, Raw: "entity_a"},
							"predicate":  {Kind: model.ValueKindString, Raw: "injected"},
							"value":      {Kind: model.ValueKindBoolean, Raw: true},
						},
					}},
				}
			},
		}},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Facts) != 1 {
		t.Fatalf("Facts length = %d, want 1", len(got.Facts))
	}
	if got.Facts[0].ID != "fact_injected" {
		t.Fatalf("fact ID = %q, want fact_injected", got.Facts[0].ID)
	}
}

func TestEnqueueDecisionAddsToEventQueue(t *testing.T) {
	t.Parallel()

	enqueuedEvent := model.WorldEvent{
		ID:     "event_follow_up",
		Type:   model.EventTypeNote,
		Source: model.EventSourceRuntime,
	}
	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_enqueue",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{
					Status:         RuleDecisionEnqueue,
					EnqueuedEvents: []model.WorldEvent{enqueuedEvent},
				}
			},
		}},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.EventQueue) != 1 {
		t.Fatalf("EventQueue length = %d, want 1", len(got.EventQueue))
	}
	if got.EventQueue[0].Event.ID != "event_follow_up" {
		t.Fatalf("queued event ID = %q, want event_follow_up", got.EventQueue[0].Event.ID)
	}
	if len(got.EventLog) != 1 || got.EventLog[0].ID != "event_1" {
		t.Fatalf("original event not logged: %#v", got.EventLog)
	}
}

func TestMultipleRulesChainCorrectly(t *testing.T) {
	t.Parallel()

	modified := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
		Effects: []model.Effect{{
			Kind:     model.EffectSetFact,
			TargetID: "fact_from_modify",
			Payload: map[string]model.Value{
				"subject_id": {Kind: model.ValueKindEntityRef, Raw: "entity_a"},
				"predicate":  {Kind: model.ValueKindString, Raw: "modified"},
				"value":      {Kind: model.ValueKindBoolean, Raw: true},
			},
		}},
	}
	rt := Runtime{
		Rules: []Rule{
			dynamicRule{
				id: "rule_modify",
				fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
					return RuleDecision{Status: RuleDecisionModify, ModifiedEvent: &modified}
				},
			},
			dynamicRule{
				id: "rule_add_effect",
				fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
					return RuleDecision{
						Status: RuleDecisionAddEffect,
						AddedEffects: []model.Effect{{
							Kind:     model.EffectSetFact,
							TargetID: "fact_from_add",
							Payload: map[string]model.Value{
								"subject_id": {Kind: model.ValueKindEntityRef, Raw: "entity_b"},
								"predicate":  {Kind: model.ValueKindString, Raw: "added"},
								"value":      {Kind: model.ValueKindString, Raw: "yes"},
							},
						}},
					}
				},
			},
			dynamicRule{
				id: "rule_enqueue",
				fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
					return RuleDecision{
						Status:         RuleDecisionEnqueue,
						EnqueuedEvents: []model.WorldEvent{{ID: "event_queued", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
					}
				},
			},
		},
	}

	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}

	got, err := rt.ApplyEvent(world, event)
	if err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if len(got.Facts) != 2 {
		t.Fatalf("Facts length = %d, want 2", len(got.Facts))
	}
	if got.Facts[0].ID != "fact_from_modify" {
		t.Fatalf("first fact ID = %q, want fact_from_modify", got.Facts[0].ID)
	}
	if got.Facts[1].ID != "fact_from_add" {
		t.Fatalf("second fact ID = %q, want fact_from_add", got.Facts[1].ID)
	}
	if len(got.EventQueue) != 1 || got.EventQueue[0].Event.ID != "event_queued" {
		t.Fatalf("EventQueue mismatch: %#v", got.EventQueue)
	}
}

func TestRejectShortCircuitsAfterModify(t *testing.T) {
	t.Parallel()

	modified := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "modified",
	}
	rt := Runtime{
		Rules: []Rule{
			dynamicRule{
				id: "rule_modify",
				fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
					return RuleDecision{Status: RuleDecisionModify, ModifiedEvent: &modified}
				},
			},
			dynamicRule{
				id: "rule_reject",
				fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
					return RuleDecision{Status: RuleDecisionReject, Reason: "blocked after modify"}
				},
			},
		},
	}

	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}

	_, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for rejected event after modify")
	}
}

func TestModifyDecisionPassesModifiedEventToNextRule(t *testing.T) {
	t.Parallel()

	modified := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "was_modified",
	}
	var observedDescription string
	rt := Runtime{
		Rules: []Rule{
			dynamicRule{
				id: "rule_modify",
				fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
					return RuleDecision{Status: RuleDecisionModify, ModifiedEvent: &modified}
				},
			},
			dynamicRule{
				id: "rule_observe",
				fn: func(_ RuleContext, ev model.WorldEvent) RuleDecision {
					observedDescription = ev.Description
					return RuleDecision{Status: RuleDecisionAllow}
				},
			},
		},
	}

	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}

	if _, err := rt.ApplyEvent(world, event); err != nil {
		t.Fatalf("ApplyEvent returned error: %v", err)
	}
	if observedDescription != "was_modified" {
		t.Fatalf("next rule saw description = %q, want was_modified", observedDescription)
	}
}
