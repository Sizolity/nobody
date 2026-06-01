package runtime

import (
	"fmt"

	"github.com/sizolity/nobody/internal/world/model"
)

type Rule interface {
	ID() model.RuleID
	Evaluate(RuleContext, model.WorldEvent) RuleDecision
}

type RuleContext struct {
	World model.World
}

type RuleDecision struct {
	Status string
	Reason string

	// ModifiedEvent is set when Status == RuleDecisionModify.
	// The runtime uses this event instead of the original.
	ModifiedEvent *model.WorldEvent

	// AddedEffects is set when Status == RuleDecisionAddEffect.
	// These effects are appended to the event's effects before application.
	AddedEffects []model.Effect

	// EnqueuedEvents is set when Status == RuleDecisionEnqueue.
	// These events are added to the world's EventQueue after the current event is applied.
	EnqueuedEvents []model.WorldEvent

	// CheckDescription is set when Status == RuleDecisionRequireCheck.
	// Describes what needs to be checked/reviewed before the event can proceed.
	CheckDescription string

	// ConflictDetails is set when Status == RuleDecisionRaiseConflict.
	// Structured conflict info for the caller to resolve.
	ConflictDetails *RuleConflict
}

type RuleConflict struct {
	Kind           string          `json:"kind"`
	Description    string          `json:"description"`
	ConflictingIDs []model.EventID `json:"conflicting_ids,omitempty"`
	Suggestions    []string        `json:"suggestions,omitempty"`
}

const (
	RuleDecisionAllow         = "allow"
	RuleDecisionReject        = "reject"
	RuleDecisionModify        = "modify"
	RuleDecisionAddEffect     = "add_effect"
	RuleDecisionEnqueue       = "enqueue"
	RuleDecisionRequireCheck  = "require_check"
	RuleDecisionRaiseConflict = "raise_conflict"
)

func (d RuleDecision) Validate() error {
	switch d.Status {
	case RuleDecisionAllow, RuleDecisionReject:
		return nil
	case RuleDecisionModify:
		if d.ModifiedEvent == nil {
			return fmt.Errorf("modify decision requires ModifiedEvent")
		}
		return nil
	case RuleDecisionAddEffect:
		if len(d.AddedEffects) == 0 {
			return fmt.Errorf("add_effect decision requires at least one effect")
		}
		return nil
	case RuleDecisionEnqueue:
		if len(d.EnqueuedEvents) == 0 {
			return fmt.Errorf("enqueue decision requires at least one event")
		}
		return nil
	case RuleDecisionRequireCheck:
		if d.CheckDescription == "" {
			return fmt.Errorf("require_check decision requires CheckDescription")
		}
		return nil
	case RuleDecisionRaiseConflict:
		if d.ConflictDetails == nil {
			return fmt.Errorf("raise_conflict decision requires ConflictDetails")
		}
		if d.ConflictDetails.Description == "" {
			return fmt.Errorf("raise_conflict decision requires non-empty ConflictDetails.Description")
		}
		return nil
	default:
		return fmt.Errorf("unsupported rule decision status %q", d.Status)
	}
}

// RuleRejectedError is returned by evaluateRules when a rule emits a
// RuleDecisionReject. Step() uses errors.As to classify the rejection as a
// domain decision instead of a programmer error.
type RuleRejectedError struct {
	RuleID model.RuleID
	Reason string
}

func (e *RuleRejectedError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("rule %q rejected event", e.RuleID)
	}
	return fmt.Sprintf("rule %q rejected event: %s", e.RuleID, e.Reason)
}

type RequireCheckError struct {
	RuleID      model.RuleID
	Description string
}

func (e *RequireCheckError) Error() string {
	return fmt.Sprintf("rule %q requires check: %s", e.RuleID, e.Description)
}

type RaiseConflictError struct {
	RuleID   model.RuleID
	Conflict RuleConflict
}

func (e *RaiseConflictError) Error() string {
	return fmt.Sprintf("rule %q raised conflict: %s", e.RuleID, e.Conflict.Description)
}
