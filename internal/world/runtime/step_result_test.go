package runtime

import (
	"reflect"
	"testing"

	"github.com/sizolity/nobody/internal/world/director"
	"github.com/sizolity/nobody/internal/world/model"
)

// --- S6D: Step() classifies rule-decision errors into StepResult ---

// rejectByTypeRule rejects any event whose Type equals the configured target.
type rejectByTypeRule struct {
	id     model.RuleID
	target string
	reason string
}

func (r rejectByTypeRule) ID() model.RuleID { return r.id }
func (r rejectByTypeRule) Evaluate(_ RuleContext, event model.WorldEvent) RuleDecision {
	if event.Type == r.target {
		return RuleDecision{Status: RuleDecisionReject, Reason: r.reason}
	}
	return RuleDecision{Status: RuleDecisionAllow}
}

// requireCheckByTypeRule emits require_check for events of the configured type.
type requireCheckByTypeRule struct {
	id          model.RuleID
	target      string
	description string
}

func (r requireCheckByTypeRule) ID() model.RuleID { return r.id }
func (r requireCheckByTypeRule) Evaluate(_ RuleContext, event model.WorldEvent) RuleDecision {
	if event.Type == r.target {
		return RuleDecision{Status: RuleDecisionRequireCheck, CheckDescription: r.description}
	}
	return RuleDecision{Status: RuleDecisionAllow}
}

// raiseConflictByTypeRule emits raise_conflict for events of the configured type.
type raiseConflictByTypeRule struct {
	id       model.RuleID
	target   string
	conflict RuleConflict
}

func (r raiseConflictByTypeRule) ID() model.RuleID { return r.id }
func (r raiseConflictByTypeRule) Evaluate(_ RuleContext, event model.WorldEvent) RuleDecision {
	if event.Type == r.target {
		c := r.conflict
		return RuleDecision{Status: RuleDecisionRaiseConflict, ConflictDetails: &c}
	}
	return RuleDecision{Status: RuleDecisionAllow}
}

func TestStepCollectsRejectedProposalAndContinues(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithRules(rejectByTypeRule{id: "rule_block_blocked", target: "blocked", reason: "policy violation"}),
		WithDirectors(
			director.NewScriptDirector("dir_a", []model.WorldEvent{{
				ID:     "event_x",
				Type:   "blocked",
				Source: model.EventSourceDirector,
			}}),
			director.NewScriptDirector("dir_b", []model.WorldEvent{{
				ID:     "event_y",
				Type:   model.EventTypeNote,
				Source: model.EventSourceDirector,
			}}),
		),
	)

	got, err := rt.Step(bg, model.World{ID: "world_1", Name: "World"})
	if err != nil {
		t.Fatalf("Step returned error, want nil for rule reject: %v", err)
	}
	if len(got.RejectedEvents) != 1 {
		t.Fatalf("RejectedEvents length = %d, want 1: %#v", len(got.RejectedEvents), got.RejectedEvents)
	}
	r := got.RejectedEvents[0]
	if r.Event.ID != "event_x" {
		t.Fatalf("RejectedEvents[0].Event.ID = %q, want event_x", r.Event.ID)
	}
	if r.RuleID != "rule_block_blocked" {
		t.Fatalf("RejectedEvents[0].RuleID = %q, want rule_block_blocked", r.RuleID)
	}
	if r.Reason != "policy violation" {
		t.Fatalf("RejectedEvents[0].Reason = %q, want 'policy violation'", r.Reason)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_y" {
		t.Fatalf("AppliedEvents = %#v, want only event_y", got.AppliedEvents)
	}
	if len(got.World.EventLog) != 1 || got.World.EventLog[0].ID != "event_y" {
		t.Fatalf("EventLog = %#v, want only event_y", got.World.EventLog)
	}
}

func TestStepCollectsBlockedProposal(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithRules(requireCheckByTypeRule{
			id:          "rule_check",
			target:      "blocked",
			description: "needs human approval",
		}),
		WithDirectors(
			director.NewScriptDirector("dir_a", []model.WorldEvent{{
				ID:     "event_x",
				Type:   "blocked",
				Source: model.EventSourceDirector,
			}}),
			director.NewScriptDirector("dir_b", []model.WorldEvent{{
				ID:     "event_y",
				Type:   model.EventTypeNote,
				Source: model.EventSourceDirector,
			}}),
		),
	)

	got, err := rt.Step(bg, model.World{ID: "world_1", Name: "World"})
	if err != nil {
		t.Fatalf("Step returned error, want nil for require_check: %v", err)
	}
	if len(got.BlockedEvents) != 1 {
		t.Fatalf("BlockedEvents length = %d, want 1: %#v", len(got.BlockedEvents), got.BlockedEvents)
	}
	b := got.BlockedEvents[0]
	if b.Event.ID != "event_x" {
		t.Fatalf("BlockedEvents[0].Event.ID = %q, want event_x", b.Event.ID)
	}
	if b.RuleID != "rule_check" {
		t.Fatalf("BlockedEvents[0].RuleID = %q, want rule_check", b.RuleID)
	}
	if b.Description != "needs human approval" {
		t.Fatalf("BlockedEvents[0].Description = %q, want 'needs human approval'", b.Description)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_y" {
		t.Fatalf("AppliedEvents = %#v, want only event_y", got.AppliedEvents)
	}
}

func TestStepCollectsConflictProposal(t *testing.T) {
	t.Parallel()

	conflict := RuleConflict{
		Kind:           "temporal",
		Description:    "event conflicts with existing timeline",
		ConflictingIDs: []model.EventID{"event_existing_a", "event_existing_b"},
		Suggestions:    []string{"reschedule", "split"},
	}
	rt := NewRuntime(
		WithRules(raiseConflictByTypeRule{
			id:       "rule_conflict",
			target:   "blocked",
			conflict: conflict,
		}),
		WithDirectors(
			director.NewScriptDirector("dir_a", []model.WorldEvent{{
				ID:     "event_x",
				Type:   "blocked",
				Source: model.EventSourceDirector,
			}}),
		),
	)

	got, err := rt.Step(bg, model.World{ID: "world_1", Name: "World"})
	if err != nil {
		t.Fatalf("Step returned error, want nil for raise_conflict: %v", err)
	}
	if len(got.Conflicts) != 1 {
		t.Fatalf("Conflicts length = %d, want 1: %#v", len(got.Conflicts), got.Conflicts)
	}
	c := got.Conflicts[0]
	if c.Event.ID != "event_x" {
		t.Fatalf("Conflicts[0].Event.ID = %q, want event_x", c.Event.ID)
	}
	if c.RuleID != "rule_conflict" {
		t.Fatalf("Conflicts[0].RuleID = %q, want rule_conflict", c.RuleID)
	}
	if !reflect.DeepEqual(c.Conflict, conflict) {
		t.Fatalf("Conflicts[0].Conflict = %#v, want %#v", c.Conflict, conflict)
	}
}

func TestStepClassifiesQueueRuleErrors(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithRules(requireCheckByTypeRule{
			id:          "rule_check",
			target:      "blocked",
			description: "queue item blocked",
		}),
		WithEventQueueLimit(10),
	)
	world := model.World{
		ID:   "world_1",
		Name: "World",
		EventQueue: []model.EventQueueItem{
			{Event: model.WorldEvent{ID: "event_blocked", Type: "blocked", Source: model.EventSourceRuntime}},
			{Event: model.WorldEvent{ID: "event_good", Type: model.EventTypeNote, Source: model.EventSourceRuntime}},
		},
	}

	got, err := rt.Step(bg, world)
	if err != nil {
		t.Fatalf("Step returned error, want nil for queued rule decision: %v", err)
	}
	if len(got.BlockedEvents) != 1 || got.BlockedEvents[0].Event.ID != "event_blocked" {
		t.Fatalf("BlockedEvents = %#v, want one entry for event_blocked", got.BlockedEvents)
	}
	if len(got.World.EventQueue) != 0 {
		t.Fatalf("EventQueue should be empty after rule classification: %#v", got.World.EventQueue)
	}
	if len(got.AppliedEvents) != 1 || got.AppliedEvents[0].ID != "event_good" {
		t.Fatalf("AppliedEvents = %#v, want only event_good", got.AppliedEvents)
	}
}

func TestStepValidationErrorStillAborts(t *testing.T) {
	t.Parallel()

	// A WorldEvent with empty Type fails Validate before any rule runs, so
	// this represents a programmer error rather than a domain decision and
	// must still abort Step().
	rt := NewRuntime(
		WithoutRules(),
		WithDirectors(director.NewScriptDirector("dir_a", []model.WorldEvent{{
			ID:     "event_invalid",
			Type:   "",
			Source: model.EventSourceDirector,
		}})),
	)

	_, err := rt.Step(bg, model.World{ID: "world_1", Name: "World"})
	if err == nil {
		t.Fatal("Step returned nil error for validation failure, want non-nil")
	}
}

func TestStepClassifiedEventsAreNotInAppliedEvents(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(
		WithRules(rejectByTypeRule{id: "rule_reject", target: "blocked", reason: "no"}),
		WithDirectors(director.NewScriptDirector("dir_a", []model.WorldEvent{{
			ID:     "event_x",
			Type:   "blocked",
			Source: model.EventSourceDirector,
		}})),
	)

	got, err := rt.Step(bg, model.World{ID: "world_1", Name: "World"})
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.RejectedEvents) != 1 {
		t.Fatalf("RejectedEvents length = %d, want 1", len(got.RejectedEvents))
	}
	for _, applied := range got.AppliedEvents {
		if applied.ID == "event_x" {
			t.Fatalf("rejected event_x leaked into AppliedEvents: %#v", got.AppliedEvents)
		}
	}
	for _, logged := range got.World.EventLog {
		if logged.ID == "event_x" {
			t.Fatalf("rejected event_x leaked into EventLog: %#v", got.World.EventLog)
		}
	}
}

// TestStepClassifiedEventsHaveOriginalEventCopy pins the chosen semantics:
// the Event recorded in RejectedEvents / BlockedEvents / Conflicts is the
// PROPOSED event (the one passed to ApplyEvent), not the post-modify version
// observed by the deciding rule. This is because the typed rule errors
// (RuleRejectedError / RequireCheckError / RaiseConflictError) intentionally
// do not carry the event payload; classification happens in Step() using the
// proposal that triggered ApplyEvent, before any rule-driven mutation.
func TestStepClassifiedEventsHaveOriginalEventCopy(t *testing.T) {
	t.Parallel()

	modified := model.WorldEvent{
		ID:          "event_x",
		Type:        "blocked",
		Source:      model.EventSourceRuntime,
		Description: "rewritten by rule_modify",
	}
	modifyRule := dynamicRule{
		id: "rule_modify",
		fn: func(_ RuleContext, _ model.WorldEvent) RuleDecision {
			m := modified
			return RuleDecision{Status: RuleDecisionModify, ModifiedEvent: &m}
		},
	}
	rejectRule := rejectByTypeRule{id: "rule_reject", target: "blocked", reason: "blocked after modify"}

	rt := NewRuntime(
		WithRules(modifyRule, rejectRule),
		WithDirectors(director.NewScriptDirector("dir_a", []model.WorldEvent{{
			ID:          "event_x",
			Type:        "blocked",
			Source:      model.EventSourceDirector,
			Description: "original description",
		}})),
	)

	got, err := rt.Step(bg, model.World{ID: "world_1", Name: "World"})
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}
	if len(got.RejectedEvents) != 1 {
		t.Fatalf("RejectedEvents length = %d, want 1", len(got.RejectedEvents))
	}
	recorded := got.RejectedEvents[0].Event
	if recorded.Description != "original description" {
		t.Fatalf("RejectedEvents[0].Event.Description = %q, want 'original description' "+
			"(Step records the proposal, not the rule-modified intermediate)", recorded.Description)
	}
	if recorded.Source != model.EventSourceDirector {
		t.Fatalf("RejectedEvents[0].Event.Source = %q, want %q (proposed event)",
			recorded.Source, model.EventSourceDirector)
	}
}
