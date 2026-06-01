package runtime

import (
	"strings"
	"testing"

	"github.com/sizolity/nobody/internal/world/director"
	"github.com/sizolity/nobody/internal/world/model"
)

// The tests in this file pin the contract between
// director.DefaultSystemPrompt (the schema we promise the LLM) and the
// effect handlers in runtime.go (the schema the runtime actually reads).
// If a handler key is renamed without updating the prompt — or vice versa
// — these tests fail. See the doc comment on director.DefaultSystemPrompt.

// promptEffectSpec mirrors a single "Supported effect kinds" bullet in
// DefaultSystemPrompt. setupWorld may be nil for kinds that need no
// pre-existing entity/thread/etc.
type promptEffectSpec struct {
	name       string
	effect     model.Effect
	setupWorld func() model.World
	// expectPromptMention: a fragment of the prompt that must appear, to
	// catch silent removal of the kind from the schema.
	expectPromptMention string
}

func TestDefaultSystemPromptMentionsEveryDocumentedKind(t *testing.T) {
	t.Parallel()

	mustMention := []string{
		`"set_fact"`,
		`"update_entity_state"`,
		`"set_entity_component"`,
		`"add_relation"`,
		`"remove_relation"`,
		`"add_memory"`,
		`"revise_memory"`,
		`"reconcile_memory"`,
		`"remove_memory"`,
		`"remove_fact"`,
		`"enqueue_event"`,
		`"open_thread"`,
		`"update_thread"`,
		`"close_thread"`,
		`"add_entity"`,
		`"remove_entity"`,
	}
	for _, fragment := range mustMention {
		if !strings.Contains(director.DefaultSystemPrompt, fragment) {
			t.Errorf("DefaultSystemPrompt missing required effect kind %s", fragment)
		}
	}
}

func TestDefaultSystemPromptDoesNotUseStaleKeys(t *testing.T) {
	t.Parallel()

	// These keys appeared in the pre-S6F prompt but the runtime never read
	// them. If they come back in a future edit, the LLM will silently
	// generate broken effects again.
	staleKeys := []string{
		`"memory_kind"`,
		`"relation_target_id"`,
	}
	for _, key := range staleKeys {
		if strings.Contains(director.DefaultSystemPrompt, key) {
			t.Errorf("DefaultSystemPrompt still uses stale key %s — runtime does not read it", key)
		}
	}
}

func TestDefaultSystemPromptKeysMatchRuntimeHandlers(t *testing.T) {
	t.Parallel()

	specs := []promptEffectSpec{
		{
			name: "set_fact",
			effect: model.Effect{
				Kind:     model.EffectSetFact,
				TargetID: "fact_gate",
				Payload: map[string]model.Value{
					"subject_id": {Kind: model.ValueKindEntityRef, Raw: "city_gate"},
					"predicate":  {Kind: model.ValueKindString, Raw: "status"},
					"value":      {Kind: model.ValueKindString, Raw: "sealed"},
				},
			},
			expectPromptMention: `"subject_id"`,
		},
		{
			name: "update_entity_state",
			effect: model.Effect{
				Kind:     model.EffectUpdateEntityState,
				TargetID: "char_alice",
				Payload: map[string]model.Value{
					"mood": {Kind: model.ValueKindString, Raw: "anxious"},
				},
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Entities: map[model.EntityID]model.Entity{
						"char_alice": {ID: "char_alice", Type: "character", Name: "Alice"},
					},
				}
			},
			expectPromptMention: `update_entity_state`,
		},
		{
			name: "set_entity_component",
			effect: model.Effect{
				Kind:     model.EffectSetEntityComponent,
				TargetID: "char_alice",
				Payload: map[string]model.Value{
					"component": {Kind: model.ValueKindString, Raw: model.ComponentSpatial},
					"data":      {Kind: model.ValueKindObject, Raw: model.NewSpatialComponent("tower")},
				},
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Entities: map[model.EntityID]model.Entity{
						"char_alice": {ID: "char_alice", Type: "character", Name: "Alice"},
					},
				}
			},
			expectPromptMention: `"component"`,
		},
		{
			name: "add_relation",
			effect: model.Effect{
				Kind:     model.EffectAddRelation,
				TargetID: "rel_alice_bob",
				Payload: map[string]model.Value{
					"type":      {Kind: model.ValueKindString, Raw: "ally"},
					"source_id": {Kind: model.ValueKindEntityRef, Raw: "char_alice"},
					"target_id": {Kind: model.ValueKindEntityRef, Raw: "char_bob"},
				},
			},
			expectPromptMention: `"target_id"`,
		},
		{
			name: "remove_relation",
			effect: model.Effect{
				Kind:     model.EffectRemoveRelation,
				TargetID: "rel_x",
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Relations: []model.Relation{
						{ID: "rel_x", Type: "ally", SourceID: "a", TargetID: "b"},
					},
				}
			},
			expectPromptMention: `"remove_relation"`,
		},
		{
			name: "add_memory",
			effect: model.Effect{
				Kind:     model.EffectAddMemory,
				TargetID: "mem_1",
				Payload: map[string]model.Value{
					"owner_kind":   {Kind: model.ValueKindString, Raw: model.MemoryOwnerKindCharacter},
					"owner_id":     {Kind: model.ValueKindString, Raw: "char_alice"},
					"scope":        {Kind: model.ValueKindString, Raw: model.MemoryScopeSubjective},
					"kind":         {Kind: model.ValueKindString, Raw: model.MemoryKindBelief},
					"content":      {Kind: model.ValueKindString, Raw: "Bob seems suspicious."},
					"truth_status": {Kind: model.ValueKindString, Raw: model.TruthStatusUnknown},
				},
			},
			// Without "kind" the prompt schema would say memory_kind which
			// the handler does NOT read. This row catches a regression.
			expectPromptMention: `"kind"`,
		},
		{
			name: "revise_memory",
			effect: model.Effect{
				Kind:     model.EffectReviseMemory,
				TargetID: "mem_1",
				Payload: map[string]model.Value{
					"content":      {Kind: model.ValueKindString, Raw: "Alice now thinks Bob is innocent."},
					"truth_status": {Kind: model.ValueKindString, Raw: model.TruthStatusUnknown},
				},
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Memory: []model.MemoryRecord{{
						ID:          "mem_1",
						Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_alice"},
						Scope:       model.MemoryScopeSubjective,
						Kind:        model.MemoryKindBelief,
						Content:     "Bob seems suspicious.",
						TruthStatus: model.TruthStatusUnknown,
					}},
				}
			},
			expectPromptMention: `"revise_memory"`,
		},
		{
			name: "reconcile_memory",
			effect: model.Effect{
				Kind:     model.EffectReconcileMemory,
				TargetID: "mem_1",
				Payload: map[string]model.Value{
					"truth_status": {Kind: model.ValueKindString, Raw: model.TruthStatusOutdated},
				},
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Memory: []model.MemoryRecord{{
						ID:          "mem_1",
						Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindCharacter, ID: "char_alice"},
						Scope:       model.MemoryScopeSubjective,
						Kind:        model.MemoryKindBelief,
						Content:     "Bob seems suspicious.",
						TruthStatus: model.TruthStatusUnknown,
					}},
				}
			},
			expectPromptMention: `"reconcile_memory"`,
		},
		{
			name: "remove_memory",
			effect: model.Effect{
				Kind:     model.EffectRemoveMemory,
				TargetID: "mem_1",
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Memory: []model.MemoryRecord{{
						ID:          "mem_1",
						Owner:       model.MemoryOwner{Kind: model.MemoryOwnerKindWorld},
						Scope:       model.MemoryScopeFactual,
						Kind:        model.MemoryKindObservation,
						Content:     "x",
						TruthStatus: model.TruthStatusTrue,
					}},
				}
			},
			expectPromptMention: `"remove_memory"`,
		},
		{
			name: "remove_fact",
			effect: model.Effect{
				Kind:     model.EffectRemoveFact,
				TargetID: "fact_x",
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Facts: []model.Fact{{
						ID: "fact_x", SubjectID: "s", Predicate: "p",
						Value: model.Value{Kind: model.ValueKindString, Raw: "v"},
					}},
				}
			},
			expectPromptMention: `"remove_fact"`,
		},
		{
			name: "enqueue_event",
			effect: model.Effect{
				Kind:     model.EffectEnqueueEvent,
				TargetID: "event_queued",
				Payload: map[string]model.Value{
					"event": {
						Kind: model.ValueKindObject,
						Raw: model.WorldEvent{
							ID:     "event_queued",
							Type:   model.EventTypeNote,
							Source: model.EventSourceRuntime,
						},
					},
					"priority":   {Kind: model.ValueKindNumber, Raw: float64(1)},
					"created_by": {Kind: model.ValueKindString, Raw: "test"},
				},
			},
			expectPromptMention: `"enqueue_event"`,
		},
		{
			name: "open_thread",
			effect: model.Effect{
				Kind:     model.EffectOpenThread,
				TargetID: "thread_x",
				Payload: map[string]model.Value{
					"kind":  {Kind: model.ValueKindString, Raw: "mystery"},
					"title": {Kind: model.ValueKindString, Raw: "The Sealed Tomb"},
				},
			},
			expectPromptMention: `"open_thread"`,
		},
		{
			name: "update_thread",
			effect: model.Effect{
				Kind:     model.EffectUpdateThread,
				TargetID: "thread_x",
				Payload: map[string]model.Value{
					"summary":  {Kind: model.ValueKindString, Raw: "Investigation continues."},
					"priority": {Kind: model.ValueKindNumber, Raw: 0.5},
				},
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Threads: []model.WorldThread{
						{ID: "thread_x", Kind: "mystery", Title: "T", Status: model.ThreadStatusOpen},
					},
				}
			},
			expectPromptMention: `"update_thread"`,
		},
		{
			name: "close_thread",
			effect: model.Effect{
				Kind:     model.EffectCloseThread,
				TargetID: "thread_x",
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Threads: []model.WorldThread{
						{ID: "thread_x", Kind: "mystery", Title: "T", Status: model.ThreadStatusOpen},
					},
				}
			},
			expectPromptMention: `"close_thread"`,
		},
		{
			name: "add_entity",
			effect: model.Effect{
				Kind:     model.EffectAddEntity,
				TargetID: "char_new",
				Payload: map[string]model.Value{
					"type": {Kind: model.ValueKindString, Raw: "character"},
					"name": {Kind: model.ValueKindString, Raw: "New Person"},
				},
			},
			expectPromptMention: `"add_entity"`,
		},
		{
			name: "remove_entity",
			effect: model.Effect{
				Kind:     model.EffectRemoveEntity,
				TargetID: "char_old",
			},
			setupWorld: func() model.World {
				return model.World{
					ID:   "w",
					Name: "W",
					Entities: map[model.EntityID]model.Entity{
						"char_old": {ID: "char_old", Type: "character", Name: "Old"},
					},
				}
			},
			expectPromptMention: `"remove_entity"`,
		},
	}

	for _, spec := range specs {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(director.DefaultSystemPrompt, spec.expectPromptMention) {
				t.Fatalf("DefaultSystemPrompt missing expected mention %q for kind %q", spec.expectPromptMention, spec.name)
			}

			world := model.World{ID: "w", Name: "W"}
			if spec.setupWorld != nil {
				world = spec.setupWorld()
			}

			_, err := applyEffect(world, spec.effect)
			if err != nil {
				// A "payload.X is required" or "payload.X must be ..." error
				// means the handler reads a key the prompt did not promise
				// (or vice versa). That is the bug class this test catches.
				if isPayloadKeyError(err) {
					t.Fatalf("effect kind %q: handler reports payload-key error %q — prompt schema and handler disagree", spec.name, err.Error())
				}
				// Any other domain error (e.g. an "entity not found" caused
				// by an incomplete setupWorld) is still a signal worth
				// surfacing, because the test row should be self-contained.
				t.Fatalf("effect kind %q applyEffect returned error: %v", spec.name, err)
			}
		})
	}
}

func isPayloadKeyError(err error) bool {
	msg := err.Error()
	if !strings.Contains(msg, "payload.") {
		return false
	}
	if strings.Contains(msg, "is required") {
		return true
	}
	if strings.Contains(msg, "must be") {
		return true
	}
	return false
}
