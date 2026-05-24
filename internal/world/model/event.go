package model

import "fmt"

type WorldEvent struct {
	ID          EventID    `json:"id"`
	Type        string     `json:"type"`
	Source      string     `json:"source"`
	ActorIDs    []EntityID `json:"actor_ids,omitempty"`
	TargetIDs   []EntityID `json:"target_ids,omitempty"`
	LocationID  EntityID   `json:"location_id,omitempty"`
	Intent      string     `json:"intent,omitempty"`
	Description string     `json:"description,omitempty"`
	Effects     []Effect   `json:"effects,omitempty"`
}

const (
	EventTypeNote                = "note"
	EventTypeWorldFactChanged    = "world_fact_changed"
	EventTypeRelationshipChanged = "relationship_changed"
	EventTypeRemember            = "remember"
	EventTypeThreadChanged       = "thread_changed"
)

const (
	EventSourceTest     = "test"
	EventSourceUser     = "user_input"
	EventSourceRuntime  = "runtime"
	EventSourceDirector = "director"
)

func (e WorldEvent) Validate() error {
	if err := ValidateID(string(e.ID)); err != nil {
		return fmt.Errorf("event.id: %w", err)
	}
	if e.Type == "" {
		return fmt.Errorf("event.type is required")
	}
	if e.Source == "" {
		return fmt.Errorf("event.source is required")
	}
	for i, effect := range e.Effects {
		if err := effect.Validate(); err != nil {
			return fmt.Errorf("event.effects[%d]: %w", i, err)
		}
	}
	return nil
}

type Effect struct {
	Kind     string           `json:"kind"`
	TargetID string           `json:"target_id"`
	Payload  map[string]Value `json:"payload,omitempty"`
}

const (
	EffectSetFact           = "set_fact"
	EffectUpdateEntityState = "update_entity_state"
	EffectAddRelation       = "add_relation"
	EffectAddMemory         = "add_memory"
	EffectReviseMemory      = "revise_memory"
	EffectReconcileMemory   = "reconcile_memory"
	EffectOpenThread        = "open_thread"
	EffectUpdateThread      = "update_thread"
	EffectCloseThread       = "close_thread"
)

func (e Effect) Validate() error {
	if e.Kind == "" {
		return fmt.Errorf("effect.kind is required")
	}
	switch e.Kind {
	case EffectSetFact, EffectUpdateEntityState, EffectAddRelation, EffectAddMemory, EffectReviseMemory, EffectReconcileMemory, EffectOpenThread, EffectUpdateThread, EffectCloseThread:
	default:
		return fmt.Errorf("unsupported effect kind %q", e.Kind)
	}
	if e.TargetID == "" {
		return fmt.Errorf("effect.target_id is required")
	}
	return nil
}
