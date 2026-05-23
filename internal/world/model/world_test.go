package model

import "testing"

func TestWorldValidateRequiresIDAndName(t *testing.T) {
	world := World{Name: "Test World"}
	if err := world.Validate(); err == nil {
		t.Fatal("Validate returned nil without ID")
	}

	world = World{ID: "test_world"}
	if err := world.Validate(); err == nil {
		t.Fatal("Validate returned nil without Name")
	}
}

func TestWorldValidateAcceptsMinimalWorld(t *testing.T) {
	world := World{
		ID:   "test_world",
		Name: "Test World",
		Clock: WorldClock{
			Current: WorldTime{Kind: WorldTimeTick, Tick: 1},
		},
	}
	if err := world.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestWorldEventValidateRequiresCoreFields(t *testing.T) {
	event := WorldEvent{Type: EventTypeNote, Source: EventSourceTest}
	if err := event.Validate(); err == nil {
		t.Fatal("Validate returned nil without ID")
	}

	event = WorldEvent{ID: "event_1", Source: EventSourceTest}
	if err := event.Validate(); err == nil {
		t.Fatal("Validate returned nil without Type")
	}

	event = WorldEvent{ID: "event_1", Type: EventTypeNote}
	if err := event.Validate(); err == nil {
		t.Fatal("Validate returned nil without Source")
	}
}

func TestWorldEventValidateAcceptsSupportedEffects(t *testing.T) {
	event := WorldEvent{
		ID:     "event_1",
		Type:   EventTypeWorldFactChanged,
		Source: EventSourceTest,
		Effects: []Effect{{
			Kind:     EffectSetFact,
			TargetID: "fact_1",
			Payload: map[string]Value{
				"subject_id": {Kind: ValueKindEntityRef, Raw: "door_1"},
				"predicate":  {Kind: ValueKindString, Raw: "locked"},
				"value":      {Kind: ValueKindBoolean, Raw: true},
			},
		}},
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestEffectValidateRejectsUnsupportedKind(t *testing.T) {
	effect := Effect{Kind: "unsupported", TargetID: "target_1"}
	if err := effect.Validate(); err == nil {
		t.Fatal("Validate returned nil for unsupported effect kind")
	}
}

func TestMemoryRecordValidateRequiresOwnerAndContent(t *testing.T) {
	memory := MemoryRecord{ID: "memory_1", Content: "The king is dead."}
	if err := memory.Validate(); err == nil {
		t.Fatal("Validate returned nil without owner")
	}

	memory = MemoryRecord{
		ID:    "memory_1",
		Owner: MemoryOwner{Kind: MemoryOwnerKindCharacter, ID: "char_1"},
	}
	if err := memory.Validate(); err == nil {
		t.Fatal("Validate returned nil without content or summary")
	}
}

func TestMemoryRecordValidateAllowsWorldAndCharacterOwners(t *testing.T) {
	worldMemory := MemoryRecord{
		ID:          "memory_1",
		Owner:       MemoryOwner{Kind: MemoryOwnerKindWorld},
		Scope:       MemoryScopeFactual,
		Kind:        MemoryKindObservation,
		Content:     "D killed the king.",
		TruthStatus: TruthStatusTrue,
		Confidence:  1.0,
		Importance:  0.9,
	}
	if err := worldMemory.Validate(); err != nil {
		t.Fatalf("world memory Validate returned error: %v", err)
	}

	characterMemory := MemoryRecord{
		ID:          "memory_2",
		Owner:       MemoryOwner{Kind: MemoryOwnerKindCharacter, ID: "char_b"},
		Scope:       MemoryScopeSubjective,
		Kind:        MemoryKindBelief,
		Content:     "A killed the king.",
		TruthStatus: TruthStatusUnknown,
		Confidence:  0.8,
		Importance:  0.7,
	}
	if err := characterMemory.Validate(); err != nil {
		t.Fatalf("character memory Validate returned error: %v", err)
	}
}

func TestMemoryRecordValidateRejectsInvalidConfidence(t *testing.T) {
	memory := MemoryRecord{
		ID:         "memory_1",
		Owner:      MemoryOwner{Kind: MemoryOwnerKindWorld},
		Content:    "Bad confidence.",
		Confidence: 1.1,
	}
	if err := memory.Validate(); err == nil {
		t.Fatal("Validate returned nil for invalid confidence")
	}
}

func TestMemoryRecordValidateRejectsUnsupportedClassificationValues(t *testing.T) {
	memory := MemoryRecord{
		ID:          "memory_1",
		Owner:       MemoryOwner{Kind: MemoryOwnerKindWorld},
		Scope:       "private",
		Kind:        MemoryKindBelief,
		Content:     "A hidden belief.",
		TruthStatus: TruthStatusUnknown,
	}
	if err := memory.Validate(); err == nil {
		t.Fatal("Validate returned nil for invalid scope")
	}

	memory.Scope = MemoryScopeSubjective
	memory.Kind = "hidden"
	if err := memory.Validate(); err == nil {
		t.Fatal("Validate returned nil for invalid kind")
	}

	memory.Kind = MemoryKindBelief
	memory.TruthStatus = "hidden"
	if err := memory.Validate(); err == nil {
		t.Fatal("Validate returned nil for invalid truth status")
	}
}

func TestWorldEventValidateAcceptsMemoryEffects(t *testing.T) {
	event := WorldEvent{
		ID:     "event_1",
		Type:   EventTypeRemember,
		Source: EventSourceTest,
		Effects: []Effect{{
			Kind:     EffectAddMemory,
			TargetID: "memory_1",
			Payload: map[string]Value{
				"owner_kind":   {Kind: ValueKindString, Raw: MemoryOwnerKindCharacter},
				"owner_id":     {Kind: ValueKindEntityRef, Raw: "char_b"},
				"scope":        {Kind: ValueKindString, Raw: MemoryScopeSubjective},
				"kind":         {Kind: ValueKindString, Raw: MemoryKindBelief},
				"content":      {Kind: ValueKindString, Raw: "A killed the king."},
				"truth_status": {Kind: ValueKindString, Raw: TruthStatusUnknown},
				"confidence":   {Kind: ValueKindNumber, Raw: 0.8},
				"importance":   {Kind: ValueKindNumber, Raw: 0.7},
			},
		}},
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestWorldThreadValidateRequiresIDAndTitle(t *testing.T) {
	thread := WorldThread{Kind: ThreadKindMystery, Title: "Find the killer"}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil without ID")
	}

	thread = WorldThread{ID: "thread_1", Kind: ThreadKindMystery}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil without title")
	}
}

func TestWorldThreadValidateAcceptsMinimalThread(t *testing.T) {
	thread := WorldThread{
		ID:       "thread_1",
		Kind:     ThreadKindMystery,
		Title:    "Find the killer",
		Status:   ThreadStatusOpen,
		Priority: 0.8,
		Tension:  0.6,
	}
	if err := thread.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestWorldThreadValidateRejectsInvalidPriorityAndTension(t *testing.T) {
	thread := WorldThread{
		ID:       "thread_1",
		Kind:     ThreadKindMystery,
		Title:    "Find the killer",
		Status:   ThreadStatusOpen,
		Priority: 1.2,
		Tension:  0.6,
	}
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for invalid priority")
	}

	thread.Priority = 0.8
	thread.Tension = -0.1
	if err := thread.Validate(); err == nil {
		t.Fatal("Validate returned nil for invalid tension")
	}
}

func TestWorldEventValidateAcceptsThreadEffects(t *testing.T) {
	event := WorldEvent{
		ID:     "event_1",
		Type:   EventTypeThreadChanged,
		Source: EventSourceTest,
		Effects: []Effect{{
			Kind:     EffectOpenThread,
			TargetID: "thread_1",
			Payload: map[string]Value{
				"kind":     {Kind: ValueKindString, Raw: ThreadKindMystery},
				"title":    {Kind: ValueKindString, Raw: "Find the killer"},
				"summary":  {Kind: ValueKindString, Raw: "C investigates the king murder case."},
				"status":   {Kind: ValueKindString, Raw: ThreadStatusOpen},
				"priority": {Kind: ValueKindNumber, Raw: 0.8},
				"tension":  {Kind: ValueKindNumber, Raw: 0.6},
			},
		}},
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}
