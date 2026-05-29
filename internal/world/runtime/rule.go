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
}

const (
	RuleDecisionAllow     = "allow"
	RuleDecisionReject    = "reject"
	RuleDecisionModify    = "modify"
	RuleDecisionAddEffect = "add_effect"
	RuleDecisionEnqueue   = "enqueue"
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
	default:
		return fmt.Errorf("unsupported rule decision status %q", d.Status)
	}
}
