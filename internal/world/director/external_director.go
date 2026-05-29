package director

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/sizolity/nobody/internal/world/model"
)

// ExternalTranslationPrompt is the system prompt used when translating
// natural language user input into WorldEvent JSON via an LLM.
const ExternalTranslationPrompt = `You are a world event translator for a narrative simulation engine.
Given a user's natural language action or description, convert it into a JSON array of WorldEvent objects.

Each event MUST have:
- "id": unique snake_case identifier (e.g. "event_user_opens_door")
- "type": one of "note", "world_fact_changed", "remember", "move", "thread_changed"
- "source": always "user_input"
- "description": one-sentence narrative description of the action

Events MAY include "effects" to mutate world state. Each effect has:
- "kind": the mutation type (e.g. "set_fact", "update_entity_state", "add_memory")
- "target_id": the target entity/fact/memory/thread ID
- "payload": key-value pairs where values are {"kind":"string|number|boolean|entity_ref","raw":<value>}

Return ONLY a valid JSON array. No markdown, no explanation.`

// ExternalInput represents a single user/API input to the ExternalDirector.
type ExternalInput struct {
	// Raw is a natural language user input that needs LLM translation.
	// Mutually exclusive with Events.
	Raw string

	// Events are pre-structured event proposals from the user/API.
	// Mutually exclusive with Raw.
	Events []model.WorldEvent

	// ActorID is the entity performing the action (optional).
	ActorID model.EntityID

	// Source labels the input origin. Defaults to "user_input".
	Source string
}

// ExternalDirector converts user/API input into event proposals.
// It holds pending inputs and emits them during the next runtime step.
type ExternalDirector struct {
	id         string
	mu         sync.Mutex
	pending    []ExternalInput
	translator TextGenerator
	noteSeq    atomic.Int64
}

// NewExternalDirector creates an ExternalDirector. translator may be nil;
// raw text inputs without a translator produce simple note events.
func NewExternalDirector(id string, translator TextGenerator) *ExternalDirector {
	return &ExternalDirector{
		id:         id,
		translator: translator,
	}
}

func (d *ExternalDirector) ID() string { return d.id }

// Submit adds an input to the pending queue. Thread-safe.
func (d *ExternalDirector) Submit(input ExternalInput) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pending = append(d.pending, input)
}

// SubmitText is a convenience for submitting a raw text input.
func (d *ExternalDirector) SubmitText(text string, actorID model.EntityID) {
	d.Submit(ExternalInput{Raw: text, ActorID: actorID})
}

// SubmitEvents is a convenience for submitting pre-structured events.
func (d *ExternalDirector) SubmitEvents(events ...model.WorldEvent) {
	d.Submit(ExternalInput{Events: events})
}

// Pending returns the number of pending inputs.
func (d *ExternalDirector) Pending() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}

// Propose drains all pending inputs, converts them to WorldEvents, and
// returns the collected proposals. Implements the Director interface.
func (d *ExternalDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	d.mu.Lock()
	inputs := d.pending
	d.pending = nil
	d.mu.Unlock()

	var collected []model.WorldEvent
	for _, input := range inputs {
		events, err := d.processInput(ctx.Ctx, input)
		if err != nil {
			return nil, err
		}

		source := input.Source
		if source == "" {
			source = model.EventSourceUser
		}

		for i := range events {
			events[i].Source = source
			if input.ActorID != "" {
				events[i].ActorIDs = prependEntityID(input.ActorID, events[i].ActorIDs)
			}
		}

		collected = append(collected, events...)
	}

	if collected == nil {
		return []model.WorldEvent{}, nil
	}
	return cloneEvents(collected), nil
}

func (d *ExternalDirector) processInput(ctx context.Context, input ExternalInput) ([]model.WorldEvent, error) {
	if len(input.Events) > 0 {
		return input.Events, nil
	}
	if input.Raw == "" {
		return nil, nil
	}
	if d.translator != nil {
		return d.translateRaw(ctx, input.Raw)
	}
	return []model.WorldEvent{d.makeNoteEvent(input.Raw)}, nil
}

func (d *ExternalDirector) translateRaw(ctx context.Context, raw string) ([]model.WorldEvent, error) {
	response, err := d.translator.Generate(ctx, ExternalTranslationPrompt, raw)
	if err != nil {
		return nil, fmt.Errorf("external translate: %w", err)
	}
	events, err := parseEventResponse(response)
	if err != nil {
		return nil, fmt.Errorf("external translate parse: %w", err)
	}
	return events, nil
}

func (d *ExternalDirector) makeNoteEvent(raw string) model.WorldEvent {
	seq := d.noteSeq.Add(1)
	return model.WorldEvent{
		ID:          model.EventID(fmt.Sprintf("event_external_note_%d", seq)),
		Type:        model.EventTypeNote,
		Description: raw,
	}
}

func prependEntityID(id model.EntityID, existing []model.EntityID) []model.EntityID {
	if len(existing) > 0 && existing[0] == id {
		return existing
	}
	out := make([]model.EntityID, 0, 1+len(existing))
	out = append(out, id)
	out = append(out, existing...)
	return out
}
