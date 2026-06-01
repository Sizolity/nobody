package runtime

import (
	"context"
	"strings"
	"testing"

	"github.com/sizolity/nobody/internal/world/model"
)

// --- Finding 1 (F3): rule outputs must be re-validated before mutating the world ---

func TestApplyEventRejectsRuleModifyToInvalidEvent(t *testing.T) {
	t.Parallel()

	invalid := model.WorldEvent{
		ID:     "event_modified",
		Type:   "",
		Source: model.EventSourceRuntime,
	}
	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_break",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{Status: RuleDecisionModify, ModifiedEvent: &invalid}
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
	if err == nil {
		t.Fatal("ApplyEvent returned nil for invalid rule-modified event")
	}
	if !strings.Contains(err.Error(), "rule output") {
		t.Fatalf("error = %q, want to mention 'rule output'", err.Error())
	}
	if len(got.EventLog) != 0 {
		t.Fatalf("invalid event was logged: %#v", got.EventLog)
	}
}

func TestApplyEventRejectsRuleEnqueueOfInvalidEvent(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_enqueue_bad",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{
					Status: RuleDecisionEnqueue,
					EnqueuedEvents: []model.WorldEvent{{
						ID:     "event_bad_queued",
						Type:   "",
						Source: model.EventSourceRuntime,
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
	if err == nil {
		t.Fatal("ApplyEvent returned nil for invalid enqueued event")
	}
	if !strings.Contains(err.Error(), "enqueued event") {
		t.Fatalf("error = %q, want to mention 'enqueued event'", err.Error())
	}
	if len(got.EventLog) != 0 {
		t.Fatalf("event was logged: %#v", got.EventLog)
	}
	if len(got.EventQueue) != 0 {
		t.Fatalf("invalid event was enqueued: %#v", got.EventQueue)
	}
}

func TestApplyEventRejectsRuleAddEffectWithInvalidEffect(t *testing.T) {
	t.Parallel()

	rt := Runtime{
		Rules: []Rule{dynamicRule{
			id: "rule_add_bad_effect",
			fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
				return RuleDecision{
					Status: RuleDecisionAddEffect,
					AddedEffects: []model.Effect{{
						Kind:     model.EffectSetFact,
						TargetID: "",
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
	if err == nil {
		t.Fatal("ApplyEvent returned nil for invalid added effect")
	}
	if !strings.Contains(err.Error(), "rule output") {
		t.Fatalf("error = %q, want to mention 'rule output'", err.Error())
	}
	if !strings.Contains(err.Error(), "target_id") {
		t.Fatalf("error = %q, want underlying validation message about target_id", err.Error())
	}
	if len(got.EventLog) != 0 {
		t.Fatalf("event was logged: %#v", got.EventLog)
	}
}

// --- Finding 2 (F4): registry.BuildAll() errors must propagate ---

func TestApplyEventPropagatesRegistryBuildError(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "test_world",
		Name: "Test World",
		Rules: []model.Rule{
			{ID: "rule_unknown", Kind: "nonexistent_kind", Enabled: true},
		},
	}
	rt := NewRuntime(WithoutRules(), WithWorldRules(DefaultRegistry()))
	event := model.WorldEvent{
		ID:     "event_1",
		Type:   model.EventTypeNote,
		Source: model.EventSourceTest,
	}

	got, err := rt.ApplyEvent(world, event)
	if err == nil {
		t.Fatal("ApplyEvent returned nil for unknown rule kind")
	}
	if !strings.Contains(err.Error(), "nonexistent_kind") {
		t.Fatalf("error = %q, want it to mention unknown kind 'nonexistent_kind'", err.Error())
	}
	if !strings.Contains(err.Error(), "collect rules") {
		t.Fatalf("error = %q, want it to wrap as 'collect rules'", err.Error())
	}
	if len(got.EventLog) != 0 {
		t.Fatalf("event was logged: %#v", got.EventLog)
	}
}

// --- Finding 3 (F6): default fail policy must not lose the queued item ---

func TestStepRestoresQueueItemOnFailPolicyError(t *testing.T) {
	t.Parallel()

	failing := model.EventQueueItem{
		Event: model.WorldEvent{
			ID:     "event_fail",
			Type:   model.EventTypeNote,
			Source: model.EventSourceRuntime,
			Effects: []model.Effect{{
				Kind:     model.EffectReviseMemory,
				TargetID: "missing_memory",
			}},
		},
		ErrorPolicy: model.QueueErrorPolicyFail,
		Priority:    5,
		NotBefore:   model.WorldTime{Kind: model.WorldTimeTick, Tick: 1},
		Attempts:    2,
		MaxAttempts: 4,
	}
	other := model.EventQueueItem{
		Event: model.WorldEvent{
			ID:     "event_other",
			Type:   model.EventTypeNote,
			Source: model.EventSourceRuntime,
		},
		ErrorPolicy: model.QueueErrorPolicyFail,
		Priority:    1,
	}
	world := model.World{
		ID:   "world_1",
		Name: "World",
		Clock: model.WorldClock{
			Current: model.WorldTime{Kind: model.WorldTimeTick, Tick: 10},
		},
		EventQueue: []model.EventQueueItem{failing, other},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(10)).Step(context.Background(), world)
	if err == nil {
		t.Fatal("Step returned nil for fail policy")
	}
	if len(got.World.EventQueue) != 2 {
		t.Fatalf("EventQueue length = %d, want 2 (failed item must be restored): %#v",
			len(got.World.EventQueue), got.World.EventQueue)
	}
	var found *model.EventQueueItem
	for i := range got.World.EventQueue {
		if got.World.EventQueue[i].Event.ID == "event_fail" {
			found = &got.World.EventQueue[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("failed item missing from restored queue: %#v", got.World.EventQueue)
	}
	if found.Priority != 5 {
		t.Fatalf("restored item Priority = %d, want 5", found.Priority)
	}
	if found.NotBefore.Kind != model.WorldTimeTick || found.NotBefore.Tick != 1 {
		t.Fatalf("restored item NotBefore = %#v, want tick 1", found.NotBefore)
	}
	if found.Attempts != 2 {
		t.Fatalf("restored item Attempts = %d, want 2 (unchanged)", found.Attempts)
	}
	if found.MaxAttempts != 4 {
		t.Fatalf("restored item MaxAttempts = %d, want 4", found.MaxAttempts)
	}
	if found.ErrorPolicy != model.QueueErrorPolicyFail {
		t.Fatalf("restored item ErrorPolicy = %q, want %q", found.ErrorPolicy, model.QueueErrorPolicyFail)
	}
}

func TestStepSkipPolicyStillRemovesItem(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{
			{
				Event: model.WorldEvent{
					ID:     "event_bad",
					Type:   model.EventTypeNote,
					Source: model.EventSourceRuntime,
					Effects: []model.Effect{{
						Kind:     model.EffectReviseMemory,
						TargetID: "missing_memory",
					}},
				},
				ErrorPolicy: model.QueueErrorPolicySkip,
			},
			{
				Event: model.WorldEvent{
					ID:     "event_good",
					Type:   model.EventTypeNote,
					Source: model.EventSourceRuntime,
				},
			},
		},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(10)).Step(context.Background(), world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.World.EventQueue) != 0 {
		t.Fatalf("EventQueue = %#v, want empty", got.World.EventQueue)
	}
	if len(got.SkippedEvents) != 1 || got.SkippedEvents[0].ID != "event_bad" {
		t.Fatalf("SkippedEvents = %#v, want event_bad", got.SkippedEvents)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_good" {
		t.Fatalf("AppliedEvents = %#v, want only event_good", got.AppliedEvents)
	}
}

func TestStepRetryPolicyKeepsItemAndIncrementsAttempt(t *testing.T) {
	t.Parallel()

	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{{
			Event: model.WorldEvent{
				ID:     "event_retry",
				Type:   model.EventTypeNote,
				Source: model.EventSourceRuntime,
				Effects: []model.Effect{{
					Kind:     model.EffectReviseMemory,
					TargetID: "missing_memory",
				}},
			},
			ErrorPolicy: model.QueueErrorPolicyRetry,
			MaxAttempts: 5,
		}},
	}

	got, err := NewRuntime(WithoutRules(), WithEventQueueLimit(10)).Step(context.Background(), world)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.World.EventQueue) != 1 {
		t.Fatalf("EventQueue length = %d, want 1 (retry must keep item)", len(got.World.EventQueue))
	}
	retained := got.World.EventQueue[0]
	if retained.Event.ID != "event_retry" {
		t.Fatalf("retained event ID = %q, want event_retry", retained.Event.ID)
	}
	if retained.Attempts != 1 {
		t.Fatalf("retained Attempts = %d, want 1 (incremented)", retained.Attempts)
	}
	if retained.MaxAttempts != 5 {
		t.Fatalf("retained MaxAttempts = %d, want 5", retained.MaxAttempts)
	}
	if len(got.AppliedEvents) != 0 {
		t.Fatalf("AppliedEvents should be empty: %#v", got.AppliedEvents)
	}
}
