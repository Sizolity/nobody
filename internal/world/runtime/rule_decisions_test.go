package runtime

import (
	"errors"
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
		{"require_check", RuleDecision{
			Status:           RuleDecisionRequireCheck,
			CheckDescription: "verify something",
		}},
		{"raise_conflict", RuleDecision{
			Status: RuleDecisionRaiseConflict,
			ConflictDetails: &RuleConflict{
				Kind:        "schedule",
				Description: "overlapping events",
			},
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

// --- S5B: require_check / raise_conflict ---

func TestRuleDecisionValidateAcceptsRequireCheckWithDescription(t *testing.T) {
	t.Parallel()
	d := RuleDecision{Status: RuleDecisionRequireCheck, CheckDescription: "verify inventory"}
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate returned error for valid require_check: %v", err)
	}
}

func TestRuleDecisionValidateRejectsRequireCheckWithoutDescription(t *testing.T) {
	t.Parallel()
	d := RuleDecision{Status: RuleDecisionRequireCheck}
	if err := d.Validate(); err == nil {
		t.Fatal("Validate returned nil for require_check without CheckDescription")
	}
}

func TestRuleDecisionValidateAcceptsRaiseConflictWithDetails(t *testing.T) {
	t.Parallel()
	d := RuleDecision{
		Status: RuleDecisionRaiseConflict,
		ConflictDetails: &RuleConflict{
			Kind:        "schedule",
			Description: "overlapping events",
		},
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate returned error for valid raise_conflict: %v", err)
	}
}

func TestRuleDecisionValidateRejectsRaiseConflictWithoutDetails(t *testing.T) {
	t.Parallel()
	d := RuleDecision{Status: RuleDecisionRaiseConflict}
	if err := d.Validate(); err == nil {
		t.Fatal("Validate returned nil for raise_conflict without ConflictDetails")
	}
}

func TestRuleDecisionValidateRejectsRaiseConflictEmptyDescription(t *testing.T) {
	t.Parallel()
	d := RuleDecision{
		Status:          RuleDecisionRaiseConflict,
		ConflictDetails: &RuleConflict{Kind: "schedule"},
	}
	if err := d.Validate(); err == nil {
		t.Fatal("Validate returned nil for raise_conflict with empty Description")
	}
}

func TestApplyEventRequireCheckReturnsTypedError(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_check",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{
					Status:           RuleDecisionRequireCheck,
					CheckDescription: "verify prerequisites",
				}
			},
		}},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest}

	_, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for require_check")
	}
	var checkErr *RequireCheckError
	if !errors.As(err, &checkErr) {
		t.Fatalf("error is not *RequireCheckError: %T: %v", err, err)
	}
	if checkErr.RuleID != "rule_check" {
		t.Fatalf("RuleID = %q, want rule_check", checkErr.RuleID)
	}
	if checkErr.Description != "verify prerequisites" {
		t.Fatalf("Description = %q, want 'verify prerequisites'", checkErr.Description)
	}
}

func TestApplyEventRaiseConflictReturnsTypedError(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_conflict",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{
					Status: RuleDecisionRaiseConflict,
					ConflictDetails: &RuleConflict{
						Kind:           "temporal",
						Description:    "event conflicts with existing timeline",
						ConflictingIDs: []model.EventID{"event_old"},
						Suggestions:    []string{"reschedule"},
					},
				}
			},
		}},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest}

	_, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for raise_conflict")
	}
	var conflictErr *RaiseConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("error is not *RaiseConflictError: %T: %v", err, err)
	}
	if conflictErr.RuleID != "rule_conflict" {
		t.Fatalf("RuleID = %q, want rule_conflict", conflictErr.RuleID)
	}
	if conflictErr.Conflict.Kind != "temporal" {
		t.Fatalf("Conflict.Kind = %q, want temporal", conflictErr.Conflict.Kind)
	}
	if len(conflictErr.Conflict.ConflictingIDs) != 1 || conflictErr.Conflict.ConflictingIDs[0] != "event_old" {
		t.Fatalf("ConflictingIDs = %v, want [event_old]", conflictErr.Conflict.ConflictingIDs)
	}
}

func TestRequireCheckBlocksEventApplication(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_check",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{
					Status:           RuleDecisionRequireCheck,
					CheckDescription: "must verify",
				}
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
		t.Fatal("ApplyEvent returned nil for require_check")
	}
	if got.Entities["door_1"].State != nil {
		t.Fatalf("effect was applied despite require_check: %#v", got.Entities["door_1"].State)
	}
	if len(got.EventLog) != 0 {
		t.Fatalf("event was logged despite require_check: %#v", got.EventLog)
	}
}

func TestRaiseConflictBlocksEventApplication(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_conflict",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{
					Status: RuleDecisionRaiseConflict,
					ConflictDetails: &RuleConflict{
						Kind:        "resource",
						Description: "resource already claimed",
					},
				}
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
		t.Fatal("ApplyEvent returned nil for raise_conflict")
	}
	if got.Entities["door_1"].State != nil {
		t.Fatalf("effect was applied despite raise_conflict: %#v", got.Entities["door_1"].State)
	}
	if len(got.EventLog) != 0 {
		t.Fatalf("event was logged despite raise_conflict: %#v", got.EventLog)
	}
}

func TestRequireCheckAfterModifyUsesModifiedContext(t *testing.T) {
	t.Parallel()

	modified := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "modified_desc",
	}
	var capturedDescription string
	rt := Runtime{
		Rules: []Rule{
			dynamicRule{
				id: "rule_modify",
				fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
					return RuleDecision{Status: RuleDecisionModify, ModifiedEvent: &modified}
				},
			},
			dynamicRule{
				id: "rule_check",
				fn: func(_ RuleContext, ev model.WorldEvent) RuleDecision {
					capturedDescription = ev.Description
					return RuleDecision{
						Status:           RuleDecisionRequireCheck,
						CheckDescription: "check after modify",
					}
				},
			},
		},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest}

	_, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for require_check after modify")
	}
	if capturedDescription != "modified_desc" {
		t.Fatalf("rule saw description = %q, want modified_desc", capturedDescription)
	}
	var checkErr *RequireCheckError
	if !errors.As(err, &checkErr) {
		t.Fatalf("error is not *RequireCheckError: %T: %v", err, err)
	}
}

func TestModifyThenRaiseConflictReferencesModifiedState(t *testing.T) {
	t.Parallel()

	modified := model.WorldEvent{
		ID:          "event_1",
		Type:        model.EventTypeNote,
		Source:      model.EventSourceTest,
		Description: "modified_for_conflict",
	}
	var capturedDescription string
	rt := Runtime{
		Rules: []Rule{
			dynamicRule{
				id: "rule_modify",
				fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
					return RuleDecision{Status: RuleDecisionModify, ModifiedEvent: &modified}
				},
			},
			dynamicRule{
				id: "rule_conflict",
				fn: func(_ RuleContext, ev model.WorldEvent) RuleDecision {
					capturedDescription = ev.Description
					return RuleDecision{
						Status: RuleDecisionRaiseConflict,
						ConflictDetails: &RuleConflict{
							Kind:        "logical",
							Description: "conflict with modified event",
						},
					}
				},
			},
		},
	}
	world := model.World{ID: "test_world", Name: "Test World"}
	event := model.WorldEvent{ID: "event_1", Type: model.EventTypeNote, Source: model.EventSourceTest}

	_, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for raise_conflict after modify")
	}
	if capturedDescription != "modified_for_conflict" {
		t.Fatalf("conflict rule saw description = %q, want modified_for_conflict", capturedDescription)
	}
	var conflictErr *RaiseConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("error is not *RaiseConflictError: %T: %v", err, err)
	}
	if conflictErr.Conflict.Description != "conflict with modified event" {
		t.Fatalf("conflict description = %q", conflictErr.Conflict.Description)
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
