package director

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sizolity/nobody/internal/world/llm"
	"github.com/sizolity/nobody/internal/world/model"
)

// TextGenerator abstracts an LLM inference call. Implementations wrap
// provider-specific chat APIs (llama.cpp, OpenAI, etc.).
type TextGenerator interface {
	Generate(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// ConversationGenerator extends TextGenerator with multi-turn support.
// Used by the repair loop to feed parse errors back to the LLM.
// Implementations that only support single-turn can omit this interface;
// the repair loop will fall back to a fresh Generate call with the error
// appended to the user prompt.
type ConversationGenerator interface {
	TextGenerator
	GenerateRepair(ctx context.Context, systemPrompt, originalUser, previousAssistant, repairUser string) (string, error)
}

// DefaultSystemPrompt documents the WorldEvent schema for LLM generators.
// Used when LLMDirectorConfig.SystemPrompt is empty.
//
// CRITICAL: the payload keys listed here MUST exactly match the keys read
// by the effect handlers in internal/world/runtime/runtime.go. The
// TestDefaultSystemPromptKeysMatchRuntimeHandlers test (in package
// runtime) pins this contract — if you rename a handler payload key,
// update both this prompt and that test in the same change.
const DefaultSystemPrompt = `You are a world director for a narrative simulation engine.
Given the current world state as JSON, propose one or more world events as a JSON array.

Each event MUST have:
- "id": unique snake_case identifier (e.g. "event_merchant_arrives")
- "type": one of "note", "world_fact_changed", "remember", "move", "thread_changed", "relationship_changed", "inventory_changed", "stats_changed", "actor_changed"
- "source": always "director"
- "description": one-sentence narrative description

Events MAY include "effects" to mutate world state. Each effect has:
- "kind": the mutation type
- "target_id": the target entity/fact/memory/thread/relation/event ID (always required, even for kinds that read no payload keys)
- "payload": key-value pairs where values are {"kind":"string|number|boolean|entity_ref|object","raw":<value>}

Supported effect kinds (payload keys listed are EXACTLY the keys the runtime reads):
- "set_fact": target_id is new fact ID. payload needs "subject_id" (entity_ref), "predicate" (string), "value" (any).
- "update_entity_state": target_id is existing entity ID. payload keys become state entries (each value typed as string/number/boolean/entity_ref).
- "set_entity_component": target_id is existing entity ID. payload needs "component" (string: "profile"|"actor"|"spatial"|"inventory"|"stats"|"skill"|"faction"|"lifecycle"|"dialogue") and "data" (object matching that component's shape).
- "add_relation": target_id is new relation ID. payload needs "type" (string), "source_id" (entity_ref), "target_id" (entity_ref).
- "remove_relation": target_id is existing relation ID. payload is empty.
- "add_memory": target_id is new memory ID. payload needs "owner_kind" (string: "world"|"character"|"faction"|"narrator") and "content" (string). Optional: "owner_id" (string; required unless owner_kind is "world"), "scope" (string: "canonical"|"factual"|"subjective"|"rumor"|"emotional"|"procedural"), "kind" (string: "observation"|"belief"|"rumor"|"summary"), "truth_status" (string: "true"|"false"|"unknown"|"disputed"|"outdated"|"secret"), "confidence" (number 0-1), "importance" (number 0-1).
- "revise_memory": target_id is existing memory ID. all keys optional: "content" (string), "summary" (string), "truth_status" (string), "confidence" (number 0-1), "importance" (number 0-1).
- "reconcile_memory": target_id is existing memory ID. all keys optional: "content" (string), "summary" (string), "truth_status" (string), "confidence_delta" (number, added to current confidence then clamped to 0-1), plus optional fork into a new belief memory via "add_memory_id" (string) and "add_memory_content" (string), with optional "add_memory_truth_status" (string), "add_memory_confidence" (number 0-1), "add_memory_importance" (number 0-1).
- "remove_memory": target_id is existing memory ID. payload is empty.
- "remove_fact": target_id is existing fact ID. payload is empty.
- "enqueue_event": target_id is a label for the queued event. payload needs "event" (object: a full nested WorldEvent). Optional: "priority" (number), "created_by" (string), "not_before" (object: WorldTime).
- "open_thread": target_id is new thread ID. payload needs "kind" (string: "mystery"|"quest"|"conflict"|"theme") and "title" (string). Optional: "summary" (string), "status" (string), "priority" (number), "tension" (number).
- "update_thread": target_id is existing thread ID. all keys optional: "kind", "title", "summary", "status" (strings), "priority", "tension" (numbers).
- "close_thread": target_id is existing thread ID. same optional keys as update_thread; if "status" is omitted it defaults to "resolved".
- "add_entity": target_id is new entity ID. payload needs "type" (string: "character"|"location"|"item"|other) and "name" (string). Optional: "description" (string).
- "remove_entity": target_id is existing entity ID. payload is empty.

A simple narrative event with no world mutation:
[{"id":"event_dawn","type":"note","source":"director","description":"Dawn breaks."}]

An event that also sets a world fact:
[{"id":"event_gate_sealed","type":"world_fact_changed","source":"director","description":"The city gate is sealed.","effects":[{"kind":"set_fact","target_id":"fact_gate","payload":{"subject_id":{"kind":"entity_ref","raw":"city_gate"},"predicate":{"kind":"string","raw":"status"},"value":{"kind":"string","raw":"sealed"}}}]}]

An event that adds a memory (the memory kind is under the payload key "kind"):
[{"id":"event_alice_suspects","type":"remember","source":"director","description":"Alice now suspects Bob.","effects":[{"kind":"add_memory","target_id":"mem_alice_suspect_bob","payload":{"owner_kind":{"kind":"string","raw":"character"},"owner_id":{"kind":"string","raw":"char_alice"},"scope":{"kind":"string","raw":"subjective"},"kind":{"kind":"string","raw":"belief"},"content":{"kind":"string","raw":"Bob looked guilty."},"truth_status":{"kind":"string","raw":"unknown"}}}]}]

An event that adds a relation (the relation target is under the payload key "target_id"):
[{"id":"event_alice_allies_bob","type":"relationship_changed","source":"director","description":"Alice allies with Bob.","effects":[{"kind":"add_relation","target_id":"rel_alice_bob","payload":{"type":{"kind":"string","raw":"ally"},"source_id":{"kind":"entity_ref","raw":"char_alice"},"target_id":{"kind":"entity_ref","raw":"char_bob"}}}]}]

Return ONLY a valid JSON array. No markdown, no explanation.`

const DefaultMaxRepairAttempts = 2

type LLMDirectorConfig struct {
	// SystemPrompt is a static system prompt string. Ignored when
	// PromptTemplate is set.
	SystemPrompt string

	// PromptTemplate is a dynamic system prompt rendered per Propose call
	// with live world state. Takes priority over SystemPrompt.
	PromptTemplate *llm.PromptTemplate

	Generator TextGenerator

	// MaxRepairAttempts is the number of times the LLM is asked to fix its
	// response after a parse/validation failure. 0 means no retries (fail
	// immediately). Negative values are treated as 0. Defaults to
	// DefaultMaxRepairAttempts when left at 0 by the caller — use -1 to
	// explicitly disable retries.
	MaxRepairAttempts *int
}

type LLMDirector struct {
	id     string
	config LLMDirectorConfig
}

func NewLLMDirector(id string, config LLMDirectorConfig) LLMDirector {
	return LLMDirector{id: id, config: config}
}

func (d LLMDirector) ID() string {
	return d.id
}

func (d LLMDirector) Propose(ctx Context) ([]model.WorldEvent, error) {
	systemPrompt, err := d.resolveSystemPrompt(ctx.World)
	if err != nil {
		return nil, fmt.Errorf("system prompt: %w", err)
	}
	userPrompt := buildWorldPrompt(ctx.World)

	response, err := d.config.Generator.Generate(ctx.Ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	events, parseErr := parseEventResponse(response)
	if parseErr == nil {
		return events, nil
	}

	maxAttempts := d.repairAttempts()
	for attempt := 0; attempt < maxAttempts; attempt++ {
		repairPrompt := fmt.Sprintf(
			"Your previous response could not be parsed:\n%s\n\nError: %s\n\nPlease return ONLY a valid JSON array of world events. No markdown, no explanation.",
			response, parseErr.Error(),
		)

		if cg, ok := d.config.Generator.(ConversationGenerator); ok {
			response, err = cg.GenerateRepair(ctx.Ctx, systemPrompt, userPrompt, response, repairPrompt)
		} else {
			response, err = d.config.Generator.Generate(ctx.Ctx, systemPrompt, repairPrompt)
		}
		if err != nil {
			return nil, fmt.Errorf("llm repair attempt %d: %w", attempt+1, err)
		}

		events, parseErr = parseEventResponse(response)
		if parseErr == nil {
			return events, nil
		}
	}

	return nil, fmt.Errorf("llm parse after %d repair attempt(s): %w", maxAttempts, parseErr)
}

// resolveSystemPrompt returns the system prompt for this call.
// Priority: PromptTemplate (dynamic) > SystemPrompt (static) > DefaultSystemPrompt.
func (d LLMDirector) resolveSystemPrompt(w model.World) (string, error) {
	if d.config.PromptTemplate != nil {
		return d.config.PromptTemplate.Render(w)
	}
	if d.config.SystemPrompt != "" {
		return d.config.SystemPrompt, nil
	}
	return DefaultSystemPrompt, nil
}

func (d LLMDirector) repairAttempts() int {
	if d.config.MaxRepairAttempts == nil {
		return DefaultMaxRepairAttempts
	}
	n := *d.config.MaxRepairAttempts
	if n < 0 {
		return 0
	}
	return n
}

func buildWorldPrompt(w model.World) string {
	data, err := json.Marshal(worldPromptContext{
		WorldID:     string(w.ID),
		Name:        w.Name,
		Description: w.Description,
		Clock:       w.Clock.Sequence,
		Entities:    llm.EntitySummaries(w.Entities),
		Facts:       llm.FactSummaries(w.Facts),
		Relations:   llm.RelationSummaries(w.Relations),
		Memories:    llm.MemorySummaries(w.Memory),
		Threads:     llm.ThreadSummaries(w.Threads),
	})
	if err != nil {
		return fmt.Sprintf(`{"world_id":%q,"name":%q}`, w.ID, w.Name)
	}
	return string(data)
}

type worldPromptContext struct {
	WorldID     string               `json:"world_id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Clock       int64                `json:"clock"`
	Entities    []llm.EntitySummary  `json:"entities,omitempty"`
	Facts       []llm.FactSummary    `json:"facts,omitempty"`
	Relations   []llm.RelationSummary `json:"relations,omitempty"`
	Memories    []llm.MemorySummary  `json:"memories,omitempty"`
	Threads     []llm.ThreadSummary  `json:"threads,omitempty"`
}

func parseEventResponse(response string) ([]model.WorldEvent, error) {
	cleaned := stripMarkdownFences(response)
	var events []model.WorldEvent
	if err := json.Unmarshal([]byte(cleaned), &events); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("event[%d]: %w", i, err)
		}
	}
	return cloneEvents(events), nil
}

// stripMarkdownFences removes ```json ... ``` or ``` ... ``` wrappers that
// LLMs commonly add despite being told not to.
func stripMarkdownFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	idx := strings.Index(trimmed, "\n")
	if idx < 0 {
		return trimmed
	}
	inner := trimmed[idx+1:]
	if last := strings.LastIndex(inner, "```"); last >= 0 {
		inner = inner[:last]
	}
	return strings.TrimSpace(inner)
}
