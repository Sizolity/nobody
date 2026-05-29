package director

import (
	"github.com/sizolity/nobody/internal/world/model"
)

// NarrativeRule is a user-supplied rule that fires when Condition holds.
type NarrativeRule struct {
	ID        string
	Condition func(world model.World) bool
	Propose   func(world model.World) []model.WorldEvent
}

// NarrativeDirectorConfig controls the built-in pacing heuristics.
type NarrativeDirectorConfig struct {
	Rules []NarrativeRule

	// MinTensionForAdvance is the minimum thread tension required before
	// the director proposes thread advancement. Default: 0.7
	MinTensionForAdvance float64

	// MaxConsecutiveSameType is how many events of the same type in a row
	// before the director proposes variety. Default: 3
	MaxConsecutiveSameType int

	// DormantThreadReviveAfter is how many events since a dormant thread
	// was last mentioned before the director considers reviving it. Default: 5
	DormantThreadReviveAfter int
}

// NarrativeDirector is a rule-based pacing manager that inspects thread
// tensions, recent event patterns, and proposes structural narrative events.
// It has no LLM dependency — pure logic only.
type NarrativeDirector struct {
	id     string
	config NarrativeDirectorConfig
}

func NewNarrativeDirector(id string, config NarrativeDirectorConfig) NarrativeDirector {
	if config.MinTensionForAdvance == 0 {
		config.MinTensionForAdvance = 0.7
	}
	if config.MaxConsecutiveSameType == 0 {
		config.MaxConsecutiveSameType = 3
	}
	if config.DormantThreadReviveAfter == 0 {
		config.DormantThreadReviveAfter = 5
	}
	return NarrativeDirector{id: id, config: config}
}

func (d NarrativeDirector) ID() string { return d.id }

func (d NarrativeDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	var events []model.WorldEvent

	// 1. Custom rules.
	for _, rule := range d.config.Rules {
		if rule.Condition(ctx.World) {
			events = append(events, rule.Propose(ctx.World)...)
		}
	}

	// 2a. High-tension advancement: active threads above the tension
	// threshold get a nudge toward escalation.
	for _, thread := range ctx.World.Threads {
		if thread.Status == model.ThreadStatusActive && thread.Tension >= d.config.MinTensionForAdvance {
			newTension := min(thread.Tension+0.1, 1.0)
			events = append(events, model.WorldEvent{
				ID:     model.EventID("narr_advance_" + string(thread.ID)),
				Type:   model.EventTypeThreadChanged,
				Source: model.EventSourceDirector,
				Effects: []model.Effect{{
					Kind:     model.EffectUpdateThread,
					TargetID: string(thread.ID),
					Payload: map[string]model.Value{
						"tension": {Kind: model.ValueKindNumber, Raw: newTension},
					},
				}},
			})
		}
	}

	// 2b. Scene variety: if the last N events in the log share the same
	// type, inject a note suggesting a scene shift.
	if consecutiveSameType(ctx.World.EventLog, d.config.MaxConsecutiveSameType) {
		events = append(events, model.WorldEvent{
			ID:          "narr_scene_shift",
			Type:        model.EventTypeNote,
			Source:      model.EventSourceDirector,
			Description: "The pace shifts...",
		})
	}

	// 2c. Dormant thread revival: threads that haven't been mentioned in
	// the event log for long enough get reactivated.
	for _, thread := range ctx.World.Threads {
		if thread.Status != model.ThreadStatusDormant {
			continue
		}
		if stepsSinceLastMention(thread.ID, ctx.World.EventLog) >= d.config.DormantThreadReviveAfter {
			events = append(events, model.WorldEvent{
				ID:     model.EventID("narr_revive_" + string(thread.ID)),
				Type:   model.EventTypeThreadChanged,
				Source: model.EventSourceDirector,
				Effects: []model.Effect{{
					Kind:     model.EffectUpdateThread,
					TargetID: string(thread.ID),
					Payload: map[string]model.Value{
						"status": {Kind: model.ValueKindString, Raw: model.ThreadStatusActive},
					},
				}},
			})
		}
	}

	if len(events) == 0 {
		return []model.WorldEvent{}, nil
	}
	return cloneEvents(events), nil
}

// consecutiveSameType reports whether the last n events in the log all share
// the same Type.
func consecutiveSameType(log []model.WorldEvent, n int) bool {
	if len(log) < n || n <= 0 {
		return false
	}
	tail := log[len(log)-n:]
	typ := tail[0].Type
	for _, e := range tail[1:] {
		if e.Type != typ {
			return false
		}
	}
	return true
}

// stepsSinceLastMention counts how many events at the tail of the log have
// passed without any effect targeting threadID. Returns len(log) when the
// thread was never mentioned.
func stepsSinceLastMention(threadID model.ThreadID, log []model.WorldEvent) int {
	id := string(threadID)
	for i := len(log) - 1; i >= 0; i-- {
		for _, eff := range log[i].Effects {
			if eff.TargetID == id {
				return len(log) - 1 - i
			}
		}
	}
	return len(log)
}
