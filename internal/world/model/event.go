package model

import (
	"fmt"
	"time"
)

type WorldEvent struct {
	ID            EventID        `json:"id"`
	Type          string         `json:"type"`
	Source        string         `json:"source"`
	ActorIDs      []EntityID     `json:"actor_ids,omitempty"`
	TargetIDs     []EntityID     `json:"target_ids,omitempty"`
	LocationID    EntityID       `json:"location_id,omitempty"`
	Intent        string         `json:"intent,omitempty"`
	Description   string         `json:"description,omitempty"`
	Preconditions []Condition    `json:"preconditions,omitempty"`
	Effects       []Effect       `json:"effects,omitempty"`
	Visibility    *Visibility    `json:"visibility,omitempty"`
	Causes        []EventID      `json:"causes,omitempty"`
	Results       []EventID      `json:"results,omitempty"`
	Status        string         `json:"status,omitempty"`
	OccurredAt    WorldTime      `json:"occurred_at,omitempty"`
	RecordedAt    time.Time      `json:"recorded_at,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

const (
	EventTypeNote                = "note"
	EventTypeMove                = "move"
	EventTypeInventoryChanged    = "inventory_changed"
	EventTypeStatsChanged        = "stats_changed"
	EventTypeActorChanged        = "actor_changed"
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

const (
	EventStatusProposed   = "proposed"
	EventStatusValidated  = "validated"
	EventStatusApplied    = "applied"
	EventStatusRejected   = "rejected"
	EventStatusRolledBack = "rolled_back"
	EventStatusSuperseded = "superseded"
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
	for i, condition := range e.Preconditions {
		if err := condition.Validate(); err != nil {
			return fmt.Errorf("event.preconditions[%d]: %w", i, err)
		}
	}
	for i, effect := range e.Effects {
		if err := effect.Validate(); err != nil {
			return fmt.Errorf("event.effects[%d]: %w", i, err)
		}
	}
	if e.Visibility != nil {
		if err := e.Visibility.Validate(); err != nil {
			return fmt.Errorf("event.%w", err)
		}
	}
	if e.Status != "" && !isSupportedEventStatus(e.Status) {
		return fmt.Errorf("unsupported event status %q", e.Status)
	}
	return nil
}

func isSupportedEventStatus(status string) bool {
	switch status {
	case EventStatusProposed, EventStatusValidated, EventStatusApplied, EventStatusRejected, EventStatusRolledBack, EventStatusSuperseded:
		return true
	default:
		return false
	}
}

type Effect struct {
	Kind     string           `json:"kind"`
	TargetID string           `json:"target_id"`
	Payload  map[string]Value `json:"payload,omitempty"`
}

const (
	EffectSetFact            = "set_fact"
	EffectUpdateEntityState  = "update_entity_state"
	EffectSetEntityComponent = "set_entity_component"
	EffectAddRelation        = "add_relation"
	EffectAddMemory          = "add_memory"
	EffectReviseMemory       = "revise_memory"
	EffectReconcileMemory    = "reconcile_memory"
	EffectEnqueueEvent       = "enqueue_event"
	EffectOpenThread         = "open_thread"
	EffectUpdateThread       = "update_thread"
	EffectCloseThread        = "close_thread"
	EffectAddEntity          = "add_entity"
	EffectRemoveEntity       = "remove_entity"
	EffectRemoveRelation     = "remove_relation"
	EffectRemoveFact         = "remove_fact"
	EffectRemoveMemory       = "remove_memory"
)

func (e Effect) Validate() error {
	if e.Kind == "" {
		return fmt.Errorf("effect.kind is required")
	}
	switch e.Kind {
	case EffectSetFact, EffectUpdateEntityState, EffectSetEntityComponent, EffectAddRelation, EffectRemoveRelation, EffectAddMemory, EffectReviseMemory, EffectReconcileMemory, EffectRemoveMemory, EffectRemoveFact, EffectEnqueueEvent, EffectOpenThread, EffectUpdateThread, EffectCloseThread, EffectAddEntity, EffectRemoveEntity:
	default:
		return fmt.Errorf("unsupported effect kind %q", e.Kind)
	}
	if e.TargetID == "" {
		return fmt.Errorf("effect.target_id is required")
	}
	return nil
}
